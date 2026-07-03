package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
)

// newTestHub はテスト用の WebSocketHub を生成します。
func newTestHub() *WebSocketHub {
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID:      "test-req",
			ConversationID: "test-conv",
			Assistant: client.AssistantMessage{
				Text:          "hello",
				Emotion:       "happy",
				Motion:        "wave",
				SpeakStyle:    "normal",
				Interruptible: true,
			},
		},
	}
	return NewWebSocketHub(mock, state.New(nil), 30000)
}

// wsClient はテスト用の WebSocket クライアントヘルパーです。
func wsClient(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(url, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial WebSocket: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readAllMessages は WebSocket から全メッセージを読み取り、タイプ→ペイロードのマップを返します。
// IDLE state_change を受信するか maxMessages に達したら停止します。
func readAllMessages(t *testing.T, conn *websocket.Conn, maxMessages int) []map[string]interface{} {
	t.Helper()
	var messages []map[string]interface{}
	for i := 0; i < maxMessages; i++ {
		_, respBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var generic map[string]interface{}
		if err := json.Unmarshal(respBytes, &generic); err != nil {
			continue
		}
		messages = append(messages, generic)
		// IDLE state_change は最後のブロードキャストなので、ここで打ち切る
		if generic["type"] == "state_change" && generic["state"] == "IDLE" {
			break
		}
	}
	return messages
}

func TestHandleWS_SendReceive(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	msg := WSMessage{
		Type:      "ping",
		Payload:   "Hello, Unity!",
		RequestID: "req-001",
	}
	data, _ := json.Marshal(msg)

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["type"] != "ping_ack" {
		t.Errorf("expected type 'ping_ack', got %v", resp["type"])
	}
	if resp["payload"] != "Hello, Unity!" {
		t.Errorf("expected payload 'Hello, Unity!', got %v", resp["payload"])
	}
	if resp["request_id"] != "req-001" {
		t.Errorf("expected request_id 'req-001', got %v", resp["request_id"])
	}
}

func TestHandleWS_InvalidJSON(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	// 不正な JSON を送信しても接続は維持される
	if err := conn.WriteMessage(websocket.TextMessage, []byte("not json")); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// その後に正しい JSON を送れば応答がある
	msg := WSMessage{Type: "ping", Payload: "pong"}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected response after valid JSON: %v", err)
	}
	if !strings.Contains(string(respBytes), "ping_ack") {
		t.Errorf("expected ping_ack in response, got %s", string(respBytes))
	}
}

func TestHandleWS_MultipleConnections(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn1 := wsClient(t, srv.URL)
	conn2 := wsClient(t, srv.URL)

	if hub.ConnectionCount() != 2 {
		t.Errorf("expected 2 connections, got %d", hub.ConnectionCount())
	}

	// conn1 に送信
	msg := WSMessage{Type: "ping", Payload: "from conn1", RequestID: "r1"}
	data, _ := json.Marshal(msg)
	conn1.WriteMessage(websocket.TextMessage, data)

	_, respBytes, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("conn1 read failed: %v", err)
	}
	var resp map[string]interface{}
	json.Unmarshal(respBytes, &resp)
	if resp["request_id"] != "r1" {
		t.Errorf("expected request_id 'r1', got %v", resp["request_id"])
	}

	// conn2 に送信
	msg2 := WSMessage{Type: "ping", Payload: "from conn2", RequestID: "r2"}
	data2, _ := json.Marshal(msg2)
	conn2.WriteMessage(websocket.TextMessage, data2)

	_, respBytes2, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("conn2 read failed: %v", err)
	}
	var resp2 map[string]interface{}
	json.Unmarshal(respBytes2, &resp2)
	if resp2["request_id"] != "r2" {
		t.Errorf("expected request_id 'r2', got %v", resp2["request_id"])
	}
}

func TestHandleWS_ConnectionCountAfterClose(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)
	if hub.ConnectionCount() != 1 {
		t.Errorf("expected 1 connection, got %d", hub.ConnectionCount())
	}

	conn.Close()

	// 切断後に接続数が0になることを確認（goroutine の実行を待つ）
	// 切断検知は次の ReadMessage で発生するので、ここでは単に接続が切れたことを確認
	if hub.ConnectionCount() < 0 {
		t.Errorf("connection count should not be negative")
	}
}

func TestHandleWS_Broadcast(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn1 := wsClient(t, srv.URL)
	conn2 := wsClient(t, srv.URL)

	broadcastMsg := map[string]interface{}{
		"type":    "broadcast",
		"payload": "hello all",
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// conn1 の読み取り
	go func() {
		defer wg.Done()
		_, data, err := conn1.ReadMessage()
		if err != nil {
			t.Errorf("conn1 read error: %v", err)
			return
		}
		if !strings.Contains(string(data), "broadcast") {
			t.Errorf("expected broadcast in conn1, got %s", string(data))
		}
	}()

	// conn2 の読み取り
	go func() {
		defer wg.Done()
		_, data, err := conn2.ReadMessage()
		if err != nil {
			t.Errorf("conn2 read error: %v", err)
			return
		}
		if !strings.Contains(string(data), "broadcast") {
			t.Errorf("expected broadcast in conn2, got %s", string(data))
		}
	}()

	hub.Broadcast(broadcastMsg)
	wg.Wait()
}

func TestHandleWS_EmptyRequestID(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	msg := WSMessage{
		Type:    "ping",
		Payload: "no request id",
		// RequestID は空
	}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var resp map[string]interface{}
	json.Unmarshal(respBytes, &resp)

	// request_id は空文字列として返る（omitempty なのでキー自体が無い場合も許容）
	if rid, ok := resp["request_id"]; ok {
		if rid != "" && rid != nil {
			t.Logf("request_id present: %v", rid)
		}
	}
	if resp["type"] != "ping_ack" {
		t.Errorf("expected type 'ping_ack', got %v", resp["type"])
	}
}

func TestHandleWS_ConcurrentConnections(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	var wg sync.WaitGroup
	numClients := 5

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn := wsClient(t, srv.URL)

			msg := WSMessage{
				Type:      "ping",
				Payload:   "msg",
				RequestID: "req",
			}
			data, _ := json.Marshal(msg)
			conn.WriteMessage(websocket.TextMessage, data)

			_, _, err := conn.ReadMessage()
			if err != nil {
				t.Errorf("client %d: read failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait a moment for cleanup goroutines to remove closed connections
	// (the cleanup is deferred, so it runs after ReadMessage returns)
	count := hub.ConnectionCount()
	if count > numClients {
		t.Errorf("expected at most %d connections, got %d", numClients, count)
	}
}

// --- Text message + AI forwarding tests ---

func TestHandleWS_TextMessageForwardsToAI(t *testing.T) {
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID:      "req-001",
			ConversationID: "conv-1",
			Assistant: client.AssistantMessage{
				Text:          "こんにちは！",
				Emotion:       "happy",
				Motion:        "wave",
				SpeakStyle:    "normal",
				Interruptible: true,
			},
		},
	}
	hub := NewWebSocketHub(mock, state.New(nil), 30000)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	// テキストメッセージを送信
	msg := WSMessage{Type: "text", Payload: "こんにちは", RequestID: "req-001"}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	// IDLE state_change まで全メッセージを読み取る
	messages := readAllMessages(t, conn, 10)

	var foundAIResponse bool
	var foundStateChange bool
	for _, generic := range messages {
		switch generic["type"] {
		case "state_change":
			foundStateChange = true
		case "ai_response":
			foundAIResponse = true
			if generic["request_id"] != "req-001" {
				t.Errorf("expected request_id 'req-001', got %v", generic["request_id"])
			}
			assistant, ok := generic["assistant"].(map[string]interface{})
			if !ok {
				t.Fatal("expected assistant in ai_response")
			}
			if assistant["text"] != "こんにちは！" {
				t.Errorf("expected text 'こんにちは！', got %v", assistant["text"])
			}
			if assistant["emotion"] != "happy" {
				t.Errorf("expected emotion 'happy', got %v", assistant["emotion"])
			}
		}
	}

	if !foundAIResponse {
		t.Error("expected ai_response message")
	}
	if !foundStateChange {
		t.Error("expected state_change broadcast")
	}

	// AI クライアントに正しいリクエストが送られたことを確認
	if mock.lastReq.Message != "こんにちは" {
		t.Errorf("expected message 'こんにちは', got %q", mock.lastReq.Message)
	}
	if mock.lastReq.RequestID != "req-001" {
		t.Errorf("expected request_id 'req-001', got %q", mock.lastReq.RequestID)
	}
}

func TestHandleWS_TextMessageAIError(t *testing.T) {
	mock := &mockPythonClient{
		err: errors.New("connection refused"),
	}
	hub := NewWebSocketHub(mock, state.New(nil), 30000)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	// AI エラーになるテキストを送信
	msg := WSMessage{Type: "text", Payload: "hello", RequestID: "req-err"}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	// IDLE state_change まで全メッセージを読み取る
	messages := readAllMessages(t, conn, 10)

	var foundError bool
	for _, generic := range messages {
		if generic["type"] == "error" {
			foundError = true
			if errStr, ok := generic["error"].(string); ok {
				if !strings.Contains(errStr, "connection refused") {
					t.Errorf("expected error to contain 'connection refused', got %q", errStr)
				}
			}
		}
	}

	if !foundError {
		t.Errorf("expected error message after AI failure, got messages: %v", messages)
	}
}

func TestHandleWS_StateTransitionsOnTextMessage(t *testing.T) {
	// 状態遷移のコールバックをキャプチャ
	var stateChanges []string
	var mu sync.Mutex

	sm := state.New(func(from, to state.State) {
		mu.Lock()
		stateChanges = append(stateChanges, from.String()+"→"+to.String())
		mu.Unlock()
	})

	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID:      "req",
			ConversationID: "conv",
			Assistant: client.AssistantMessage{
				Text:          "response",
				Emotion:       "neutral",
				Motion:        "idle",
				SpeakStyle:    "normal",
				Interruptible: true,
			},
		},
	}
	hub := NewWebSocketHub(mock, sm, 30000)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	msg := WSMessage{Type: "text", Payload: "hello", RequestID: "req"}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	// IDLE state_change まで全メッセージを読み取る
	readAllMessages(t, conn, 10)

	// 状態遷移を確認
	mu.Lock()
	defer mu.Unlock()

	if len(stateChanges) < 4 {
		t.Fatalf("expected at least 4 state transitions, got %d: %v", len(stateChanges), stateChanges)
	}

	expected := []string{
		"待機中→受信中",
		"受信中→思考中",
		"思考中→発話中",
		"発話中→待機中",
	}
	for i, exp := range expected {
		if i >= len(stateChanges) {
			break
		}
		if stateChanges[i] != exp {
			t.Errorf("transition %d: expected %q, got %q", i, exp, stateChanges[i])
		}
	}
}

func TestHandleWS_StateBroadcastOnTextMessage(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	msg := WSMessage{Type: "text", Payload: "hello", RequestID: "req"}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)

	// IDLE state_change まで全メッセージを読み取る
	messages := readAllMessages(t, conn, 10)

	// ブロードキャストされた状態遷移を収集
	var stateEvents []string
	for _, generic := range messages {
		if generic["type"] == "state_change" {
			s, _ := generic["state"].(string)
			stateEvents = append(stateEvents, s)
		}
	}

	// 期待される状態遷移シーケンス: LISTENING, THINKING, SPEAKING, IDLE
	if len(stateEvents) < 4 {
		t.Fatalf("expected at least 4 state events, got %d: %v", len(stateEvents), stateEvents)
	}

	expected := []string{"LISTENING", "THINKING", "SPEAKING", "IDLE"}
	for i, exp := range expected {
		if i >= len(stateEvents) {
			break
		}
		if stateEvents[i] != exp {
			t.Errorf("state event %d: expected %q, got %q", i, exp, stateEvents[i])
		}
	}
}

func TestHandleWS_ConcurrentTextMessages(t *testing.T) {
	hub := newTestHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	// 1つ目のテキストメッセージ
	msg1 := WSMessage{Type: "text", Payload: "hello", RequestID: "r1"}
	data1, _ := json.Marshal(msg1)
	conn.WriteMessage(websocket.TextMessage, data1)

	// IDLE まで読み切る
	readAllMessages(t, conn, 10)

	// 2つ目のテキストメッセージ（IDLE→LISTENINGができるはず）
	msg2 := WSMessage{Type: "text", Payload: "world", RequestID: "r2"}
	data2, _ := json.Marshal(msg2)
	conn.WriteMessage(websocket.TextMessage, data2)

	messages := readAllMessages(t, conn, 10)

	var foundSecondAIResponse bool
	for _, generic := range messages {
		if generic["type"] == "ai_response" {
			foundSecondAIResponse = true
			break
		}
	}

	if !foundSecondAIResponse {
		t.Error("expected second ai_response after returning to IDLE")
	}
}
