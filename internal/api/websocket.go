package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// upgrader は HTTP 接続を WebSocket にアップグレードします。
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSMessage は WebSocket で送受信する JSON メッセージのフォーマットです。
type WSMessage struct {
	Type      string `json:"type"`
	Payload   string `json:"payload"`
	RequestID string `json:"request_id,omitempty"`
}

// WebSocketHub は複数の WebSocket 接続を goroutine-safe に管理します。
type WebSocketHub struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

// NewWebSocketHub は新しい WebSocketHub を生成します。
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

// HandleWS は /ws エンドポイントの WebSocket ハンドラです。
// HTTP 接続を WebSocket にアップグレードし、JSON メッセージの送受信を行います。
func (h *WebSocketHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	h.conns[conn] = struct{}{}
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

		resp := map[string]interface{}{
			"type":       msg.Type + "_ack",
			"payload":    msg.Payload,
			"request_id": msg.RequestID,
		}

		respBytes, err := json.Marshal(resp)
		if err != nil {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
			break
		}
	}
}

// Broadcast は接続中のすべてのクライアントにメッセージを送信します。
func (h *WebSocketHub) Broadcast(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// 送信エラーは無視して他の接続に継続
			continue
		}
	}
	return nil
}

// ConnectionCount は現在の接続数を返します。
func (h *WebSocketHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
