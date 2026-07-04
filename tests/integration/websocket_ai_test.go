//go:build integration
// +build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/api"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
)

// readAllMessages は指定されたメッセージ数分 WebSocket から読み取ります。
// タイムアウト（1秒）または IDLE の state_change を受け取った時点で停止します。
func readAllMessages(t *testing.T, conn *websocket.Conn) []map[string]interface{} {
	t.Helper()
	var msgs []map[string]interface{}
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		msgs = append(msgs, msg)

		// state_change で IDLE を受け取ったら終了（一連の遷移が完了）
		if msg["type"] == "state_change" && msg["state"] == "IDLE" {
			break
		}
	}
	return msgs
}

// startMockAIService は AI Service のモック HTTP サーバーを起動します。
func startMockAIService(t *testing.T) (*httptest.Server, *client.ConversationResponse) {
	t.Helper()
	expectedResp := &client.ConversationResponse{
		RequestID:      "resp-001",
		ConversationID: "conv-001",
		Assistant: client.AssistantMessage{
			Text:          "こんにちは、元気ですか？",
			Emotion:       "happy",
			Motion:        "wave",
			SpeakStyle:    "genki",
			Interruptible: true,
		},
	}

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/conversation" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	})

	srv := httptest.NewServer(mockHandler)
	t.Cleanup(srv.Close)
	return srv, expectedResp
}

// TestWebSocketToAIServiceIntegration は WebSocket → AI Service → StateMachine の
// 一連の結合テストです。
func TestWebSocketToAIServiceIntegration(t *testing.T) {
	// 1. モック AI Service を起動
	mockAIServer, expectedResp := startMockAIService(t)

	// 2. 実クライアントを作成（モック AI Service を指す）
	aiClient := client.New(mockAIServer.URL)

	// 3. StateMachine を作成し、遷移履歴を記録
	var transitions []string
	sm := state.New(func(from, to state.State) {
		transitions = append(transitions, from.String()+"→"+to.String())
	})

	// 4. WebSocketHub を作成
	hub := api.NewWebSocketHub(aiClient, sm, 5000, nil)

	// 5. HTTP サーバーを起動（WebSocket エンドポイントのみ）
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 6. WebSocket クライアント接続
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 接続失敗: %v", err)
	}
	defer conn.Close()

	// 7. テキストメッセージ送信
	msg := api.WSMessage{
		Type:      "text",
		Payload:   "こんにちは",
		RequestID: "test-req-001",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("JSON エンコード失敗: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("メッセージ送信失敗: %v", err)
	}

	// 8. 全レスポンスメッセージを受信
	msgs := readAllMessages(t, conn)

	// 9. メッセージの分類
	var stateChanges []string
	var aiResponse map[string]interface{}
	for _, m := range msgs {
		switch m["type"] {
		case "state_change":
			if s, ok := m["state"].(string); ok {
				stateChanges = append(stateChanges, s)
			}
		case "ai_response":
			aiResponse = m
		}
	}

	// --- 検証 ---

	// StateMachine 遷移の検証: IDLE→LISTENING→THINKING→SPEAKING→IDLE
	t.Run("state_machine_transitions", func(t *testing.T) {
		expectedOrder := []string{"LISTENING", "THINKING", "SPEAKING", "IDLE"}
		if len(stateChanges) != len(expectedOrder) {
			t.Fatalf("state_change の数が期待と異なります: want=%d, got=%d, states=%v",
				len(expectedOrder), len(stateChanges), stateChanges)
		}
		for i, expected := range expectedOrder {
			if stateChanges[i] != expected {
				t.Errorf("state_change[%d]: want=%s, got=%s", i, expected, stateChanges[i])
			}
		}
	})

	// StateMachine 内部の遷移ログ検証
	t.Run("state_machine_callback", func(t *testing.T) {
		expectedTransitions := []string{
			"待機中→受信中", // IDLE→LISTENING
			"受信中→思考中", // LISTENING→THINKING
			"思考中→発話中", // THINKING→SPEAKING
			"発話中→待機中", // SPEAKING→IDLE
		}
		if len(transitions) != len(expectedTransitions) {
			t.Fatalf("遷移コールバックの数が期待と異なります: want=%d, got=%d, transitions=%v",
				len(expectedTransitions), len(transitions), transitions)
		}
		for i, expected := range expectedTransitions {
			if transitions[i] != expected {
				t.Errorf("transition[%d]: want=%q, got=%q", i, expected, transitions[i])
			}
		}
	})

	// AI 応答の検証
	t.Run("ai_response_content", func(t *testing.T) {
		if aiResponse == nil {
			t.Fatal("ai_response メッセージが見つかりません")
		}
		if aiResponse["request_id"] != expectedResp.RequestID {
			t.Errorf("request_id: want=%s, got=%v", expectedResp.RequestID, aiResponse["request_id"])
		}
		if aiResponse["conversation_id"] != expectedResp.ConversationID {
			t.Errorf("conversation_id: want=%s, got=%v", expectedResp.ConversationID, aiResponse["conversation_id"])
		}

		// assistant オブジェクトの検証
		assistant, ok := aiResponse["assistant"].(map[string]interface{})
		if !ok {
			t.Fatal("assistant フィールドがオブジェクトではありません")
		}
		if assistant["text"] != expectedResp.Assistant.Text {
			t.Errorf("assistant.text: want=%s, got=%v", expectedResp.Assistant.Text, assistant["text"])
		}
		if assistant["emotion"] != expectedResp.Assistant.Emotion {
			t.Errorf("assistant.emotion: want=%s, got=%v", expectedResp.Assistant.Emotion, assistant["emotion"])
		}
		if assistant["motion"] != expectedResp.Assistant.Motion {
			t.Errorf("assistant.motion: want=%s, got=%v", expectedResp.Assistant.Motion, assistant["motion"])
		}
	})
}

// TestWebSocketToAIServiceErrorHandling は AI Service エラー時の
// WebSocket エラーハンドリングを検証します。
func TestWebSocketToAIServiceErrorHandling(t *testing.T) {
	// エラーを返すモック AI Service
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mockAIServer := httptest.NewServer(mockHandler)
	defer mockAIServer.Close()

	aiClient := client.New(mockAIServer.URL)
	sm := state.New(nil)
	hub := api.NewWebSocketHub(aiClient, sm, 5000, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 接続失敗: %v", err)
	}
	defer conn.Close()

	msg := api.WSMessage{
		Type:      "text",
		Payload:   "こんにちは",
		RequestID: "test-req-err",
	}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("メッセージ送信失敗: %v", err)
	}

	msgs := readAllMessages(t, conn)

	// エラーレスポンスがあることを確認
	var hasError bool
	var lastState string
	for _, m := range msgs {
		if m["type"] == "error" {
			hasError = true
		}
		if m["type"] == "state_change" {
			lastState = m["state"].(string)
		}
	}

	if !hasError {
		t.Error("エラーレスポンスが見つかりません")
	}
	if lastState != "IDLE" {
		t.Errorf("エラー後は IDLE 状態になるはず: got=%s", lastState)
	}
}
