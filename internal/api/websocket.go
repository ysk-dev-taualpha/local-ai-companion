package api

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/agent"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tts"
)

func buildCheckOrigin(allowedOrigins []string) func(r *http.Request) bool {
	if len(allowedOrigins) == 0 {
		return func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			u, err := url.Parse(origin)
			if err != nil {
				return false
			}
			host := u.Hostname()
			return host == "localhost" || host == "127.0.0.1" || host == "::1"
		}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		for _, allowed := range allowedOrigins {
			if strings.EqualFold(origin, allowed) {
				return true
			}
		}
		return false
	}
}

type WSMessage struct {
	Type      string `json:"type"`
	Payload   string `json:"payload"`
	RequestID string `json:"request_id,omitempty"`
}

type WSAIResponse struct {
	Type           string                   `json:"type"`
	RequestID      string                   `json:"request_id"`
	ConversationID string                   `json:"conversation_id,omitempty"`
	Assistant      *client.AssistantMessage `json:"assistant,omitempty"`
	Text           string                   `json:"text,omitempty"`
	Audio          string                   `json:"audio,omitempty"`
	Error          string                   `json:"error,omitempty"`
}

type WSStateNotification struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

type wsConnState struct {
	writeMu sync.Mutex
}

type WebSocketHub struct {
	mu               sync.RWMutex
	stateMu          sync.Mutex
	conns            map[*websocket.Conn]*wsConnState
	pythonClient     PythonClient
	ttsClient        tts.TTSClient
	stateMachine     *state.StateMachine
	agentLoop        *agent.Loop
	requestTimeoutMs int
	upgrader         websocket.Upgrader
}

func NewWebSocketHub(pythonClient PythonClient, ttsClient tts.TTSClient, stateMachine *state.StateMachine, requestTimeoutMs int, allowedOrigins []string, agentLoop *agent.Loop) *WebSocketHub {
	return &WebSocketHub{
		conns:            make(map[*websocket.Conn]*wsConnState),
		pythonClient:     pythonClient,
		ttsClient:        ttsClient,
		stateMachine:     stateMachine,
		agentLoop:        agentLoop,
		requestTimeoutMs: requestTimeoutMs,
		upgrader: websocket.Upgrader{
			CheckOrigin: buildCheckOrigin(allowedOrigins),
		},
	}
}

func (h *WebSocketHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
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
		msgType, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		switch msgType {
		case websocket.BinaryMessage:
			h.handleAudioChunk(conn, msgBytes)
		case websocket.TextMessage:
			var msg WSMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}
			h.routeTextMessage(conn, msg)
		}
	}
}

func (h *WebSocketHub) routeTextMessage(conn *websocket.Conn, msg WSMessage) {
	switch msg.Type {
	case "text":
		if h.agentLoop != nil {
			h.handleTextMessageAgent(conn, msg)
		} else {
			h.handleTextMessage(conn, msg)
		}
	default:
		h.handleEcho(conn, msg)
	}
}

// AudioChunk is a voice binary frame received from Unity.
// Binary frame format (Big Endian):
//
//	[4B reqIDlen][NB reqID UTF-8][4B sequence number][2B sample rate][NB PCM int16 LE]
type AudioChunk struct {
	RequestID  string
	Sequence   uint32
	SampleRate uint16
	PCMData    []byte
}

func parseAudioChunk(data []byte) (*AudioChunk, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("audio_chunk: frame too short, need at least 4 bytes for reqIDlen, got %d", len(data))
	}
	reqIDLen := binary.BigEndian.Uint32(data[0:4])
	if reqIDLen > 256 {
		return nil, fmt.Errorf("audio_chunk: reqIDlen too large: %d (max 256)", reqIDLen)
	}
	headerSize := int(4 + reqIDLen + 4 + 2)
	if len(data) < headerSize {
		return nil, fmt.Errorf("audio_chunk: frame too short for header, need %d bytes, got %d", headerSize, len(data))
	}
	offset := 4
	reqID := string(data[offset : offset+int(reqIDLen)])
	offset += int(reqIDLen)
	seq := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	sampleRate := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	pcmData := data[offset:]
	return &AudioChunk{RequestID: reqID, Sequence: seq, SampleRate: sampleRate, PCMData: pcmData}, nil
}

func (h *WebSocketHub) handleAudioChunk(conn *websocket.Conn, rawData []byte) {
	if h.stateMachine.IsSpeaking() {
		log.Printf("websocket: audio_chunk discarded during SPEAKING (%d bytes)", len(rawData))
		return
	}
	chunk, err := parseAudioChunk(rawData)
	if err != nil {
		log.Printf("websocket: failed to parse audio_chunk: %v", err)
		return
	}
	// TODO(v0.5): integrate STT pipeline here
	log.Printf("websocket: received audio_chunk reqID=%s seq=%d rate=%dHz size=%d bytes",
		chunk.RequestID, chunk.Sequence, chunk.SampleRate, len(chunk.PCMData))
}

func (h *WebSocketHub) handleTextMessageAgent(conn *websocket.Conn, msg WSMessage) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()

	if h.stateMachine.IsSpeaking() {
		h.sendError(conn, msg.RequestID, "busy: currently speaking (TTS feedback loop prevention)")
		return
	}

	if err := h.stateMachine.Transition(state.LISTENING); err != nil {
		h.sendError(conn, msg.RequestID, "invalid state for listening: "+err.Error())
		return
	}
	h.broadcastState("LISTENING")

	if err := h.stateMachine.Transition(state.THINKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.sendError(conn, msg.RequestID, "invalid state transition to thinking: "+err.Error())
		return
	}
	h.broadcastState("THINKING")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.requestTimeoutMs)*time.Millisecond)
	defer cancel()

	requestID := msg.RequestID
	if requestID == "" {
		requestID = newRequestID()
	}

	responseText, err := h.agentLoop.Run(ctx, msg.Payload, requestID)
	if err != nil {
		h.sendError(conn, requestID, err.Error())
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		return
	}

	if err := h.stateMachine.Transition(state.SPEAKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.sendError(conn, msg.RequestID, "invalid state transition to speaking: "+err.Error())
		return
	}
	h.broadcastState("SPEAKING")

	aiResp := WSAIResponse{
		Type:      "ai_response",
		RequestID: requestID,
		Text:      responseText,
	}

	if h.ttsClient != nil && responseText != "" {
		audioData, ttsErr := h.ttsClient.Speak(responseText)
		if ttsErr != nil {
			log.Printf("websocket: tts synthesis failed: %v", ttsErr)
		} else {
			aiResp.Audio = base64.StdEncoding.EncodeToString(audioData)
		}
	}
	if err := h.writeJSON(conn, aiResp); err != nil {
		log.Printf("websocket: failed to write ai_response: %v", err)
	}

	if err := h.stateMachine.Transition(state.IDLE); err != nil {
		log.Printf("websocket: failed to transition to IDLE: %v", err)
		h.stateMachine.Reset()
	}
	h.broadcastState("IDLE")
}

func (h *WebSocketHub) handleTextMessage(conn *websocket.Conn, msg WSMessage) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()

	if h.stateMachine.IsSpeaking() {
		h.sendError(conn, msg.RequestID, "busy: currently speaking (TTS feedback loop prevention)")
		return
	}

	if err := h.stateMachine.Transition(state.LISTENING); err != nil {
		h.sendError(conn, msg.RequestID, "invalid state for listening: "+err.Error())
		return
	}
	h.broadcastState("LISTENING")

	if err := h.stateMachine.Transition(state.THINKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.sendError(conn, msg.RequestID, "invalid state transition to thinking: "+err.Error())
		return
	}
	h.broadcastState("THINKING")

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

	if err := h.stateMachine.Transition(state.SPEAKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.sendError(conn, msg.RequestID, "invalid state transition to speaking: "+err.Error())
		return
	}
	h.broadcastState("SPEAKING")

	aiResp := WSAIResponse{
		Type:           "ai_response",
		RequestID:      resp.RequestID,
		ConversationID: resp.ConversationID,
		Assistant:      &resp.Assistant,
	}

	if h.ttsClient != nil && resp.Assistant.Text != "" {
		audioData, ttsErr := h.ttsClient.Speak(resp.Assistant.Text)
		if ttsErr != nil {
			log.Printf("websocket: tts synthesis failed: %v", ttsErr)
		} else {
			aiResp.Audio = base64.StdEncoding.EncodeToString(audioData)
		}
	}
	if err := h.writeJSON(conn, aiResp); err != nil {
		log.Printf("websocket: failed to write ai_response: %v", err)
	}

	if err := h.stateMachine.Transition(state.IDLE); err != nil {
		log.Printf("websocket: failed to transition to IDLE: %v", err)
		h.stateMachine.Reset()
	}
	h.broadcastState("IDLE")
}

func (h *WebSocketHub) handleEcho(conn *websocket.Conn, msg WSMessage) {
	resp := map[string]interface{}{
		"type":       msg.Type + "_ack",
		"payload":    msg.Payload,
		"request_id": msg.RequestID,
	}
	h.writeJSON(conn, resp)
}

func (h *WebSocketHub) sendError(conn *websocket.Conn, requestID, errMsg string) {
	errResp := WSAIResponse{
		Type:      "error",
		RequestID: requestID,
		Error:     errMsg,
	}
	h.writeJSON(conn, errResp)
}

func (h *WebSocketHub) broadcastState(stateName string) {
	notif := WSStateNotification{
		Type:  "state_change",
		State: stateName,
	}
	h.Broadcast(notif)
}

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

func (h *WebSocketHub) Broadcast(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

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

func (h *WebSocketHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
