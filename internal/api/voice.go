package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/stt"
)

// WSVADEvent is sent to Unity on VAD speech_start/speech_end.
type WSVADEvent struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Event     string `json:"event"`
}

// WSSpeechRecognized is sent to Unity with STT result and cancel option.
type WSSpeechRecognized struct {
	Type       string `json:"type"`
	RequestID  string `json:"request_id"`
	Text       string `json:"text"`
	Cancelable bool   `json:"cancelable"`
}

// vadResponse is the JSON response from Python /vad/chunk endpoint.
type vadResponse struct {
	Event    string `json:"event"`
	AudioWAV string `json:"audio_wav,omitempty"`
}

// VoicePipeline manages the VAD -> STT -> LLM -> TTS flow for audio input.
type VoicePipeline struct {
	hub *WebSocketHub

	mu      sync.Mutex
	vadURL  string
	stt     stt.STTClient
	vadHTTP *http.Client
	buffers map[string]*audioBuffer
	closed  map[string]struct{}
}

// audioBuffer accumulates PCM chunks for a single utterance.
type audioBuffer struct {
	requestID string
	pcm       []byte
}

// NewVoicePipeline creates a new voice pipeline.
func NewVoicePipeline(vadURL string, sttClient stt.STTClient) *VoicePipeline {
	return &VoicePipeline{
		vadURL: vadURL,
		stt:    sttClient,
		vadHTTP: &http.Client{
			Timeout: 5 * time.Second,
		},
		buffers: make(map[string]*audioBuffer),
		closed:  make(map[string]struct{}),
	}
}

// SetHub binds the pipeline to a WebSocketHub (called after hub creation).
func (vp *VoicePipeline) SetHub(hub *WebSocketHub) {
	vp.hub = hub
}

// processChunk sends PCM to Python VAD and handles the response.
func (vp *VoicePipeline) processChunk(conn *websocket.Conn, chunk *AudioChunk) {
	vp.mu.Lock()
	if _, closed := vp.closed[chunk.RequestID]; closed {
		vp.mu.Unlock()
		return
	}
	buf, exists := vp.buffers[chunk.RequestID]
	if !exists {
		buf = &audioBuffer{requestID: chunk.RequestID}
		vp.buffers[chunk.RequestID] = buf
	}
	buf.pcm = append(buf.pcm, chunk.pcmBytes()...)
	vp.mu.Unlock()

	pcmPayload := chunk.pcmBytes()
	resp, err := vp.sendVADChunk(pcmPayload)
	if err != nil {
		log.Printf("voice: VAD request failed: %v", err)
		return
	}

	switch resp.Event {
	case "speech_start":
		vp.hub.broadcastVADEvent(chunk.RequestID, "speech_start")
		vp.hub.stateMu.Lock()
		if vp.hub.stateMachine.IsSpeaking() {
			log.Printf("voice: audio_chunk rejected: SPEAKING state")
			vp.hub.stateMu.Unlock()
			return
		}
		_ = vp.hub.stateMachine.Transition(state.LISTENING)
		vp.hub.broadcastState("LISTENING")
		vp.hub.stateMu.Unlock()

	case "speech_end":
		if vp.markClosed(chunk.RequestID) {
			go vp.handleSpeechEnd(conn, chunk.RequestID, resp.AudioWAV)
		} else {
			log.Printf("voice: duplicate speech_end ignored: request_id=%s", chunk.RequestID)
		}
	}
}

func (vp *VoicePipeline) markClosed(requestID string) bool {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	if _, exists := vp.closed[requestID]; exists {
		return false
	}
	vp.closed[requestID] = struct{}{}
	return true
}

func (vp *VoicePipeline) sendVADChunk(pcm []byte) (*vadResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", vp.vadURL+"/vad/chunk", bytes.NewReader(pcm))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := vp.vadHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VAD request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var vr vadResponse
	if err := json.Unmarshal(body, &vr); err != nil {
		return nil, fmt.Errorf("VAD response parse: %w", err)
	}
	return &vr, nil
}

func (vp *VoicePipeline) handleSpeechEnd(conn *websocket.Conn, requestID, wavBase64 string) {
	vp.hub.broadcastVADEvent(requestID, "speech_end")

	vp.mu.Lock()
	buf := vp.buffers[requestID]
	if buf != nil {
		delete(vp.buffers, requestID)
	}
	vp.mu.Unlock()

	var wavData []byte
	if wavBase64 != "" {
		var err error
		wavData, err = base64.StdEncoding.DecodeString(wavBase64)
		if err != nil {
			log.Printf("voice: failed to decode WAV: %v", err)
			vp.hub.sendError(conn, requestID, "failed to decode audio")
			return
		}
	} else if buf != nil {
		wavData = buf.pcm
	}

	if len(wavData) == 0 {
		log.Printf("voice: no audio data for STT")
		vp.hub.sendError(conn, requestID, "no audio data")
		return
	}
	log.Printf("voice: sending audio to STT: request_id=%s bytes=%d approx_duration=%.2fs", requestID, len(wavData), approxAudioDurationSeconds(wavData, 16000))

	vp.hub.stateMu.Lock()
	_ = vp.hub.stateMachine.Transition(state.THINKING)
	vp.hub.broadcastState("THINKING")
	vp.hub.stateMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := vp.stt.Transcribe(ctx, wavData, 16000, "ja")
	if err != nil || result.Error != "" {
		errMsg := "speech recognition failed"
		if result != nil && result.Error != "" {
			errMsg = result.Error
		}
		log.Printf("voice: STT failed: %v", errMsg)
		vp.hub.sendError(conn, requestID, errMsg)
		vp.hub.stateMu.Lock()
		vp.hub.broadcastState("IDLE")
		vp.hub.stateMachine.Reset()
		vp.hub.stateMu.Unlock()
		return
	}

	vp.hub.writeJSON(conn, WSSpeechRecognized{
		Type:       "speech_recognized",
		RequestID:  requestID,
		Text:       result.Text,
		Cancelable: true,
	})

	// Wait 3 seconds for possible cancel, then proceed.
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	vp.hub.stateMu.Lock()
	vp.hub.pendingCancels[requestID] = cancelFunc
	vp.hub.stateMu.Unlock()

	select {
	case <-cancelCtx.Done():
		log.Printf("voice: speech cancelled: request_id=%s", requestID)
	case <-time.After(3 * time.Second):
		vp.hub.stateMu.Lock()
		delete(vp.hub.pendingCancels, requestID)
		vp.hub.stateMu.Unlock()

		msg := WSMessage{
			Type:      "text",
			Payload:   result.Text,
			RequestID: requestID,
		}
		vp.hub.stateMu.Lock()
		vp.hub.stateMachine.Reset()
		vp.hub.broadcastState("IDLE")
		vp.hub.stateMu.Unlock()

		if vp.hub.agentLoop != nil {
			vp.hub.handleTextMessageAgent(conn, msg)
		} else {
			vp.hub.handleTextMessage(conn, msg)
		}
	}
}

func (ac *AudioChunk) pcmBytes() []byte {
	buf := make([]byte, len(ac.Samples)*2)
	for i, s := range ac.Samples {
		buf[i*2] = byte(s)
		buf[i*2+1] = byte(s >> 8)
	}
	return buf
}

func approxAudioDurationSeconds(data []byte, sampleRate int) float64 {
	if sampleRate <= 0 {
		return 0
	}
	pcmBytes := len(data)
	if len(data) >= 44 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		pcmBytes -= 44
	}
	if pcmBytes < 0 {
		pcmBytes = 0
	}
	return float64(pcmBytes) / float64(sampleRate*2)
}

func (h *WebSocketHub) broadcastVADEvent(requestID, event string) {
	h.Broadcast(WSVADEvent{
		Type:      "vad_event",
		RequestID: requestID,
		Event:     event,
	})
}
