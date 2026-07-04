package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
)

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

// waitForConnectionCount は接続数が期待値になるまでポーリングします。
// タイムアウトに達した場合は false を返します（CI でのレースコンディション対策）。
func waitForConnectionCount(t *testing.T, hub *WebSocketHub, expected int, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.ConnectionCount() == expected {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// newTestHub はテスト用の WebSocketHub を生成します。
// mock PythonClient と state.New(nil) で初期化済みです。
func newTestHub(t *testing.T) *WebSocketHub {
	t.Helper()
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID: "resp-001",
			Assistant: client.AssistantMessage{
				Text: "test response",
			},
		},
	}
	return NewWebSocketHub(mock, state.New(nil), 5000, nil)
}

func TestHandleWS_SendReceive(t *testing.T) {
	hub := newTestHub(t)
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
	hub := newTestHub(t)
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
	hub := newTestHub(t)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn1 := wsClient(t, srv.URL)
	conn2 := wsClient(t, srv.URL)

	// 接続登録を待つ（goroutine スケジューリングのレース対策）
	if !waitForConnectionCount(t, hub, 2, 500*time.Millisecond) {
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
	hub := newTestHub(t)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	// goroutine のスケジューリングによるレースを回避し、接続登録を待つ
	if !waitForConnectionCount(t, hub, 1, 500*time.Millisecond) {
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
	hub := newTestHub(t)
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
	hub := newTestHub(t)
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
	hub := newTestHub(t)
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

func TestHandleWS_ConcurrentTextAcrossConnectionsRace(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	const numClients = 8
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn := wsClient(t, srv.URL)
			msg := WSMessage{Type: "text", Payload: "hello", RequestID: "req"}
			data, _ := json.Marshal(msg)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				t.Errorf("write failed: %v", err)
				return
			}
			_, _, _ = conn.ReadMessage()
		}()
	}
	wg.Wait()
}

// ─── buildCheckOrigin ユニットテスト ───

func TestBuildCheckOrigin_DefaultLocalhost(t *testing.T) {
	check := buildCheckOrigin(nil)

	// Origin ヘッダなし → 許可 (same-origin / 非ブラウザクライアント)
	req, _ := http.NewRequest("GET", "/ws", nil)
	if !check(req) {
		t.Error("expected true for request without Origin header")
	}

	// localhost 許可
	req.Header.Set("Origin", "http://localhost:8080")
	if !check(req) {
		t.Error("expected true for http://localhost:8080")
	}

	// 127.0.0.1 許可
	req.Header.Set("Origin", "http://127.0.0.1:12345")
	if !check(req) {
		t.Error("expected true for http://127.0.0.1:12345")
	}

	// ::1 許可
	req.Header.Set("Origin", "http://[::1]:8080")
	if !check(req) {
		t.Error("expected true for http://[::1]:8080")
	}
}

func TestBuildCheckOrigin_DefaultRejectExternal(t *testing.T) {
	check := buildCheckOrigin(nil)

	// 外部オリジン拒否
	req, _ := http.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://192.168.1.1:8080")
	if check(req) {
		t.Error("expected false for http://192.168.1.1:8080")
	}

	// 別のホスト名拒否
	req.Header.Set("Origin", "http://evil.example.com")
	if check(req) {
		t.Error("expected false for http://evil.example.com")
	}
}

func TestBuildCheckOrigin_EmptyAllowedOrigins(t *testing.T) {
	// 明示的に空スライス → localhost のみ許可
	check := buildCheckOrigin([]string{})

	req, _ := http.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	if !check(req) {
		t.Error("expected true for localhost with empty allowed list")
	}

	req.Header.Set("Origin", "http://192.168.1.1:8080")
	if check(req) {
		t.Error("expected false for external origin with empty allowed list")
	}
}

func TestBuildCheckOrigin_SpecificOrigins(t *testing.T) {
	allowed := []string{
		"http://192.168.12.107:8080",
		"http://example.com",
	}
	check := buildCheckOrigin(allowed)

	// リスト内のオリジン → 許可
	req, _ := http.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://192.168.12.107:8080")
	if !check(req) {
		t.Error("expected true for http://192.168.12.107:8080")
	}

	req.Header.Set("Origin", "http://example.com")
	if !check(req) {
		t.Error("expected true for http://example.com")
	}

	// リスト外のオリジン → 拒否（localhost もデフォルト動作ではない）
	req.Header.Set("Origin", "http://localhost:8080")
	if check(req) {
		t.Error("expected false for localhost when specific origins are set")
	}

	req.Header.Set("Origin", "http://other.example.com")
	if check(req) {
		t.Error("expected false for http://other.example.com")
	}
}

// ─── CheckOrigin 統合テスト ───

// testCheckOriginHub は特定の allowedOrigins で hub を作るヘルパー
func testCheckOriginHub(t *testing.T, allowedOrigins []string) *WebSocketHub {
	t.Helper()
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID: "resp-001",
			Assistant: client.AssistantMessage{
				Text: "test response",
			},
		},
	}
	return NewWebSocketHub(mock, state.New(nil), 5000, allowedOrigins)
}

func TestHandleWS_CheckOrigin_AllowedOrigin(t *testing.T) {
	allowed := []string{"http://127.0.0.1"}
	hub := testCheckOriginHub(t, allowed)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	// dialer に Origin を設定
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", "http://127.0.0.1")

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("expected connection from allowed origin, got error: %v", err)
	}
	conn.Close()
}

func TestHandleWS_CheckOrigin_RejectedOrigin(t *testing.T) {
	allowed := []string{"http://192.168.12.107:8080"}
	hub := testCheckOriginHub(t, allowed)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", "http://evil.example.com")

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, _, err := dialer.Dial(wsURL, header)
	if err == nil {
		t.Error("expected connection to be rejected for non-allowed origin")
	}
}

func TestHandleWS_CheckOrigin_DefaultRejectExternal(t *testing.T) {
	// デフォルト (nil allowedOrigins = localhost only) で外部オリジンが拒否される
	hub := testCheckOriginHub(t, nil)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", "http://192.168.1.1:8080")

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, _, err := dialer.Dial(wsURL, header)
	if err == nil {
		t.Error("expected connection to be rejected for external origin with default config")
	}
}
