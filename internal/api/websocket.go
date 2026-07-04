package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
)

// WSMessage は WebSocket で送受信する JSON メッセージのフォーマットです。
type WSMessage struct {
	Type      string `json:"type"`
	Payload   string `json:"payload"`
	RequestID string `json:"request_id,omitempty"`
}

// WSAIResponse は AI 応答を含む WebSocket レスポンスです。
type WSAIResponse struct {
	Type           string                   `json:"type"`
	RequestID      string                   `json:"request_id"`
	ConversationID string                   `json:"conversation_id,omitempty"`
	Assistant      *client.AssistantMessage `json:"assistant,omitempty"`
	Error          string                   `json:"error,omitempty"`
}

// WSStateNotification は状態変化をブロードキャストするメッセージです。
type WSStateNotification struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

// wsConnState は接続ごとの書き込み制御を保持します。
type wsConnState struct {
	writeMu sync.Mutex
}

// WebSocketHub は複数の WebSocket 接続を goroutine-safe に管理し、
// 受信したテキストを AI Service に転送して応答を返します。
type WebSocketHub struct {
	mu               sync.RWMutex
	stateMu          sync.Mutex
	conns            map[*websocket.Conn]*wsConnState
	pythonClient     PythonClient
	stateMachine     *state.StateMachine
	requestTimeoutMs int
	allowedOrigins   []string
}

// NewWebSocketHub は新しい WebSocketHub を生成します。
func NewWebSocketHub(pythonClient PythonClient, stateMachine *state.StateMachine, requestTimeoutMs int, allowedOrigins []string) *WebSocketHub {
	return &WebSocketHub{
		conns:            make(map[*websocket.Conn]*wsConnState),
		pythonClient:     pythonClient,
		stateMachine:     stateMachine,
		requestTimeoutMs: requestTimeoutMs,
		allowedOrigins:   cleanOrigins(allowedOrigins),
	}
}

// HandleWS は /ws エンドポイントの WebSocket ハンドラです。
// テキストメッセージ受信時に AI Service へ転送し、
// StateMachine の状態遷移（LISTENING → THINKING → SPEAKING → IDLE）をブロードキャストします。
func (h *WebSocketHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: h.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	h.conns[conn] = &wsConnState{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "text":
			h.handleTextMessage(conn, msg)
		default:
			h.handleEcho(conn, msg)
		}
	}
}

// handleTextMessage はテキストメッセージを AI Service に転送し、
// StateMachine の状態遷移をブロードキャストします。
func (h *WebSocketHub) handleTextMessage(conn *websocket.Conn, msg WSMessage) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()

	// IDLE→LISTENING 遷移
	if err := h.stateMachine.Transition(state.LISTENING); err != nil {
		h.sendError(conn, msg.RequestID, "invalid state for listening: "+err.Error())
		return
	}
	h.broadcastState("LISTENING")

	// LISTENING→THINKING 遷移
	if err := h.stateMachine.Transition(state.THINKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.sendError(conn, msg.RequestID, "invalid state transition to thinking: "+err.Error())
		return
	}
	h.broadcastState("THINKING")

	// AI Service に転送
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.requestTimeoutMs)*time.Millisecond)
	defer cancel()

	req := client.ConversationRequest{
		Message:   msg.Payload,
		RequestID: msg.RequestID,
	}
	resp, err := h.pythonClient.Send(ctx, req)
	if err != nil {
		h.sendError(conn, msg.RequestID, err.Error())
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		return
	}

	// THINKING→SPEAKING 遷移
	if err := h.stateMachine.Transition(state.SPEAKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.sendError(conn, msg.RequestID, "invalid state transition to speaking: "+err.Error())
		return
	}
	h.broadcastState("SPEAKING")

	// AI 応答を返す
	aiResp := WSAIResponse{
		Type:           "ai_response",
		RequestID:      resp.RequestID,
		ConversationID: resp.ConversationID,
		Assistant:      &resp.Assistant,
	}
	if err := h.writeJSON(conn, aiResp); err != nil {
		log.Printf("websocket: failed to write ai_response: %v", err)
	}

	// SPEAKING→IDLE 遷移
	if err := h.stateMachine.Transition(state.IDLE); err != nil {
		log.Printf("websocket: failed to transition to IDLE: %v", err)
		h.stateMachine.Reset()
	}
	h.broadcastState("IDLE")
}

// handleEcho はテキスト以外のメッセージをエコーします（後方互換）。
func (h *WebSocketHub) handleEcho(conn *websocket.Conn, msg WSMessage) {
	resp := map[string]interface{}{
		"type":       msg.Type + "_ack",
		"payload":    msg.Payload,
		"request_id": msg.RequestID,
	}
	h.writeJSON(conn, resp)
}

// sendError はエラーレスポンスを WebSocket クライアントに送信します。
func (h *WebSocketHub) sendError(conn *websocket.Conn, requestID, errMsg string) {
	errResp := WSAIResponse{
		Type:      "error",
		RequestID: requestID,
		Error:     errMsg,
	}
	h.writeJSON(conn, errResp)
}

// broadcastState は現在の状態を全クライアントにブロードキャストします。
func (h *WebSocketHub) broadcastState(stateName string) {
	notif := WSStateNotification{
		Type:  "state_change",
		State: stateName,
	}
	h.Broadcast(notif)
}

// writeJSON は WebSocket 接続に JSON メッセージを書き込みます。
// 接続ごとの write mutex で並行書き込みを防止します。
func (h *WebSocketHub) writeJSON(conn *websocket.Conn, v interface{}) error {
	h.mu.RLock()
	cs := h.conns[conn]
	h.mu.RUnlock()
	if cs == nil {
		return nil
	}
	cs.writeMu.Lock()
	defer cs.writeMu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// Broadcast は接続中のすべてのクライアントにメッセージを送信します。
// 接続スナップショットを取得後に書き込むため、書き込み中に
// 接続が追加・削除されても安全です。
func (h *WebSocketHub) Broadcast(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 接続のスナップショットを取得
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for conn := range h.conns {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	for _, conn := range conns {
		h.mu.RLock()
		cs := h.conns[conn]
		h.mu.RUnlock()
		if cs == nil {
			continue
		}
		cs.writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, data)
		cs.writeMu.Unlock()
	}
	return nil
}

// ConnectionCount は現在の接続数を返します。
func (h *WebSocketHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

func (h *WebSocketHub) checkOrigin(r *http.Request) bool {
	return isAllowedOrigin(r.Header.Get("Origin"), h.allowedOrigins)
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil || parsedOrigin.Scheme == "" || parsedOrigin.Host == "" {
		return false
	}

	if len(allowedOrigins) == 0 {
		return isLocalhost(parsedOrigin.Hostname())
	}

	normalizedOrigin := normalizeOrigin(parsedOrigin)
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		parsedAllowed, err := url.Parse(allowed)
		if err != nil || parsedAllowed.Scheme == "" || parsedAllowed.Host == "" {
			continue
		}
		if normalizeOrigin(parsedAllowed) == normalizedOrigin {
			return true
		}
	}
	return false
}

func cleanOrigins(origins []string) []string {
	cleaned := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		cleaned = append(cleaned, origin)
	}
	return cleaned
}

func normalizeOrigin(u *url.URL) string {
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}

func isLocalhost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
