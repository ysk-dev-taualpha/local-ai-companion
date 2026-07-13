package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/agent"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/memory"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tts"
)

const voiceSystemPrompt = `あなたは常駐型AIアシスタントです。応答は短く簡潔に3文以内で、必ず以下のJSON形式で返してください:
{"text": "返答本文", "emotion": "neutral", "motion": "nod", "speak_style": "normal", "interruptible": true}
emotion: neutral/happy/sad/surprised/thinking, motion: nod/idle/wave
Web検索結果を要約する場合は特に簡潔にまとめてください。`

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

type WSAudioMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Data      string `json:"data"`
	Format    string `json:"format"`
}

type WSStateNotification struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

type wsConnState struct {
	writeMu   sync.Mutex
	sessionID string
}

type WebSocketHub struct {
	mu               sync.RWMutex
	stateMu          sync.Mutex
	agentMu          sync.Mutex
	conns            map[*websocket.Conn]*wsConnState
	memoryStore      *memory.Store
	pythonClient     PythonClient
	ttsClient        tts.TTSClient
	stateMachine     *state.StateMachine
	agentLoop        *agent.Loop
	voicePipeline    *VoicePipeline
	pendingCancels   map[string]context.CancelFunc
	requestTimeoutMs int
	upgrader         websocket.Upgrader
}

func NewWebSocketHub(memStore *memory.Store, pythonClient PythonClient, ttsClient tts.TTSClient, stateMachine *state.StateMachine, requestTimeoutMs int, allowedOrigins []string, agentLoop *agent.Loop, vp *VoicePipeline) *WebSocketHub {
	return &WebSocketHub{
		conns:            make(map[*websocket.Conn]*wsConnState),
		memoryStore:      memStore,
		pythonClient:     pythonClient,
		ttsClient:        ttsClient,
		stateMachine:     stateMachine,
		agentLoop:        agentLoop,
		voicePipeline:    vp,
		pendingCancels:   make(map[string]context.CancelFunc),
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
	h.conns[conn] = &wsConnState{sessionID: newRequestID()}
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

		if msgType == websocket.BinaryMessage {
			h.handleAudioChunk(conn, msgBytes)
			continue
		}

		var msg WSMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "text":
			if h.agentLoop != nil {
				go h.handleTextMessageAgent(conn, msg)
			} else {
				h.handleTextMessage(conn, msg)
			}
		case "cancel_speech":
			h.handleCancelSpeech(msg.RequestID)
		default:
			h.handleEcho(conn, msg)
		}
	}
}

func (h *WebSocketHub) sessionID(conn *websocket.Conn) string {
	h.mu.RLock()
	cs := h.conns[conn]
	h.mu.RUnlock()
	if cs == nil {
		return "default"
	}
	return cs.sessionID
}

func (h *WebSocketHub) handleTextMessageAgent(conn *websocket.Conn, msg WSMessage) {
	requestID := msg.RequestID
	if requestID == "" {
		requestID = newRequestID()
	}

	h.agentMu.Lock()
	defer h.agentMu.Unlock()

	h.stateMu.Lock()
	if err := h.stateMachine.Transition(state.LISTENING); err != nil {
		h.stateMu.Unlock()
		h.sendError(conn, requestID, "busy: "+err.Error())
		return
	}
	h.broadcastState("LISTENING")
	if err := h.stateMachine.Transition(state.THINKING); err != nil {
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
		h.stateMu.Unlock()
		h.sendError(conn, requestID, "invalid state transition: "+err.Error())
		return
	}
	h.broadcastState("THINKING")
	h.stateMu.Unlock()

	responseText := h.callOllama(h.sessionID(conn), msg.Payload, "")
	if responseText == "" {
		h.sendError(conn, requestID, "LLM returned empty response")
		h.resetAndBroadcastIdle()
		return
	}

	assistant := parseAssistantResponse(responseText)
	aiResp := WSAIResponse{
		Type:      "ai_response",
		RequestID: requestID,
		Assistant: &assistant,
		Text:      assistant.Text,
	}
	log.Printf("websocket: ai_response: request_id=%s text=%q", requestID, assistant.Text)

	if err := h.writeJSON(conn, aiResp); err != nil {
		log.Printf("websocket: write ai_response failed: %v", err)
		h.resetAndBroadcastIdle()
		return
	}

	h.stateMu.Lock()
	_ = h.stateMachine.Transition(state.SPEAKING)
	h.broadcastState("SPEAKING")
	h.stateMu.Unlock()

	go h.sendTTSSeparately(conn, requestID, assistant.Text)

	h.stateMu.Lock()
	_ = h.stateMachine.Transition(state.IDLE)
	h.broadcastState("IDLE")
	h.stateMu.Unlock()
}

func (h *WebSocketHub) HandleVoiceTextAgent(conn *websocket.Conn, text, requestID string) {
	h.agentMu.Lock()
	defer h.agentMu.Unlock()

	responseText := h.callOllama(h.sessionID(conn), text, voiceSystemPrompt)
	if responseText == "" {
		h.sendError(conn, requestID, "LLM returned empty response")
		h.resetAndBroadcastIdle()
		return
	}

	assistant := parseAssistantResponse(responseText)
	aiResp := WSAIResponse{
		Type:      "ai_response",
		RequestID: requestID,
		Assistant: &assistant,
		Text:      assistant.Text,
	}
	log.Printf("websocket: ai_response: request_id=%s text=%q", requestID, assistant.Text)

	if err := h.writeJSON(conn, aiResp); err != nil {
		log.Printf("websocket: write ai_response failed: %v", err)
		h.resetAndBroadcastIdle()
		return
	}

	h.stateMu.Lock()
	_ = h.stateMachine.Transition(state.SPEAKING)
	h.broadcastState("SPEAKING")
	h.stateMu.Unlock()

	go h.sendTTSSeparately(conn, requestID, assistant.Text)

	h.stateMu.Lock()
	_ = h.stateMachine.Transition(state.IDLE)
	h.broadcastState("IDLE")
	h.stateMu.Unlock()
}

func (h *WebSocketHub) callOllama(sessionID, prompt, systemPrompt string) string {
	// Load conversation history
	history, _ := h.memoryStore.LoadHistory(sessionID)
	h.memoryStore.SaveMessage(sessionID, "user", prompt)

	// Build messages
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := []msg{}
	if systemPrompt != "" {
		messages = append(messages, msg{"system", systemPrompt})
	}
	for _, m := range history {
		messages = append(messages, msg{m.Role, m.Content})
	}
	messages = append(messages, msg{"user", prompt})

	// First call: with tools
	tools := `[{"type":"function","function":{"name":"search_the_web","description":"Search the web for current information","parameters":{"type":"object","properties":{"query":{"type":"string","description":"Search query"}},"required":["query"]}}}]`

	msgsJSON, _ := json.Marshal(messages)
	body := fmt.Sprintf(`{"model":"g4v100","messages":%s,"tools":%s,"stream":false}`, string(msgsJSON), tools)
	
	result := h.ollamaRequest(body)
	if result.content == "" {
		return ""
	}

	// Check for tool call
	if result.toolName == "search_the_web" {
		searchResult := webSearch(result.toolArgs)
		if searchResult == "" {
			return result.content
		}
		// Feed tool result back
		messages = append(messages, msg{"assistant", ""}) // placeholder
		messages = append(messages, msg{"tool", fmt.Sprintf(`{"query":"%s","results":"%s"}`, escapeJSON(result.toolArgs), escapeJSON(searchResult))})
		msgsJSON2, _ := json.Marshal(messages)
		body2 := fmt.Sprintf(`{"model":"g4v100","messages":%s,"stream":false}`, string(msgsJSON2))
		content := h.ollamaRequest(body2).content
		h.memoryStore.SaveMessage(sessionID, "assistant", content)
		return content
	}

	h.memoryStore.SaveMessage(sessionID, "assistant", result.content)
	return result.content
}

type ollamaResult struct {
	content  string
	toolName string
	toolArgs string
}

func (h *WebSocketHub) ollamaRequest(body string) ollamaResult {
	req, _ := http.NewRequest("POST", "http://192.168.12.107:11434/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("ollama: request failed: %v", err)
		return ollamaResult{}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	var r struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil || len(r.Choices) == 0 {
		log.Printf("ollama: parse failed: %v body=%s", err, string(respBody[:min(200, len(respBody))]))
		return ollamaResult{}
	}

	or := ollamaResult{content: r.Choices[0].Message.Content}
	if len(r.Choices[0].Message.ToolCalls) > 0 {
		tc := r.Choices[0].Message.ToolCalls[0]
		or.toolName = tc.Function.Name
		or.toolArgs = tc.Function.Arguments
	}
	return or
}

func webSearch(query string) string {
	if query == "" {
		return ""
	}
	encoded := url.QueryEscape(query)
	req, _ := http.NewRequest("GET", "https://html.duckduckgo.com/html/?q="+encoded, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32768))
	// Simple extraction: find result snippets
	html := string(body)
	var results []string
	for i := 0; i < 3; i++ {
		snipStart := strings.Index(html, "result__snippet\"")
		if snipStart < 0 {
			break
		}
		html = html[snipStart:]
		endTag := strings.Index(html, "</a>")
		if endTag < 0 {
			break
		}
		snippet := strings.TrimSpace(stripHTML(html[:endTag+4]))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		results = append(results, snippet)
		html = html[endTag+4:]
	}
	return strings.Join(results, " | ")
}

func stripHTML(s string) string {
	var buf strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
		} else if !inTag {
			buf.WriteRune(c)
		}
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func (h *WebSocketHub) sendTTSSeparately(conn *websocket.Conn, requestID, text string) {
	func() {
		defer func() { recover() }()
		if h.ttsClient == nil || text == "" {
			return
		}
		audioData, ttsErr := h.ttsClient.Speak(text)
		if ttsErr != nil {
			log.Printf("websocket: tts synthesis failed: %v", ttsErr)
			return
		}
		audioMsg := WSAudioMessage{
			Type:      "audio",
			RequestID: requestID,
			Data:      base64.StdEncoding.EncodeToString(audioData),
			Format:    "wav",
		}
		if err := h.writeJSON(conn, audioMsg); err != nil {
			log.Printf("websocket: failed to write audio message: %v", err)
		}
	}()
}

func parseAssistantResponse(raw string) client.AssistantMessage {
	assistant := client.AssistantMessage{
		Text:          raw,
		SpeakStyle:    "normal",
		Interruptible: true,
	}
	if raw == "" {
		return assistant
	}

	clean := raw
	clean = strings.TrimSpace(clean)
	clean = strings.TrimPrefix(clean, "```json\n")
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```\n")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "\n```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var parsed client.AssistantMessage
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return assistant
	}
	if parsed.Text == "" {
		return assistant
	}
	if parsed.SpeakStyle == "" {
		parsed.SpeakStyle = "normal"
	}
	return parsed
}

func (h *WebSocketHub) handleTextMessage(conn *websocket.Conn, msg WSMessage) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()

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

func (h *WebSocketHub) resetAndBroadcastIdle() {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.stateMachine.Reset()
	h.broadcastState("IDLE")
}

func (h *WebSocketHub) handleCancelSpeech(requestID string) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()

	if cancel, ok := h.pendingCancels[requestID]; ok {
		cancel()
		delete(h.pendingCancels, requestID)
		log.Printf("voice: speech cancelled: request_id=%s", requestID)
		h.broadcastState("IDLE")
		h.stateMachine.Reset()
	}
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
