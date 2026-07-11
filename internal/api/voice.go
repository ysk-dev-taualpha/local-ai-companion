package api

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/stt"
)

type WSVADEvent struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Event     string `json:"event"`
}

type WSSpeechRecognized struct {
	Type       string `json:"type"`
	RequestID  string `json:"request_id"`
	Text       string `json:"text"`
	Cancelable bool   `json:"cancelable"`
}

type VoicePipeline struct {
	hub           *WebSocketHub
	mu            sync.Mutex
	stt           stt.STTClient
	buffers       map[string]*audioBuffer
	energySpeech  bool
	silenceCount  int
	speechThresh  int16
	silenceThresh int16
	silenceMax    int
}

type audioBuffer struct {
	requestID string
	pcm       []byte
}

func NewVoicePipeline(sttClient stt.STTClient) *VoicePipeline {
	return &VoicePipeline{
		stt:           sttClient,
		buffers:       make(map[string]*audioBuffer),
		speechThresh:  1000,
		silenceThresh: 700,
		silenceMax:    12,
	}
}

func (vp *VoicePipeline) SetHub(hub *WebSocketHub) { vp.hub = hub }

func (vp *VoicePipeline) processChunk(conn *websocket.Conn, chunk *AudioChunk) {
	var maxAmp int16
	for _, s := range chunk.Samples {
		a := s
		if a < 0 {
			a = -a
		}
		if a > maxAmp {
			maxAmp = a
		}
	}

	vp.mu.Lock()
	defer vp.mu.Unlock()

	buf, exists := vp.buffers[chunk.RequestID]
	if !exists {
		buf = &audioBuffer{requestID: chunk.RequestID}
		vp.buffers[chunk.RequestID] = buf
	}
	buf.pcm = append(buf.pcm, chunk.pcmBytes()...)

	if !vp.energySpeech {
		if maxAmp >= vp.speechThresh {
			vp.energySpeech = true
			vp.silenceCount = 0
			vp.hub.stateMu.Lock()
			if vp.hub.stateMachine.IsSpeaking() {
				vp.hub.stateMu.Unlock()
				return
			}
			_ = vp.hub.stateMachine.Transition(state.LISTENING)
			vp.hub.broadcastState("LISTENING")
			vp.hub.stateMu.Unlock()
			vp.hub.broadcastVADEvent(chunk.RequestID, "speech_start")
			log.Printf("voice: speech_start maxAmp=%d", maxAmp)
		}
		return
	}

	if maxAmp < vp.silenceThresh {
		vp.silenceCount++
		if vp.silenceCount >= vp.silenceMax {
			vp.energySpeech = false
			vp.silenceCount = 0
			wavData := make([]byte, len(buf.pcm))
			copy(wavData, buf.pcm)
			rid := chunk.RequestID
			vp.hub.broadcastVADEvent(rid, "speech_end")
			log.Printf("voice: speech_end")
			go vp.handleSpeechEnd(conn, rid, wavData)
		}
	} else {
		vp.silenceCount = 0
	}
}

func (vp *VoicePipeline) handleSpeechEnd(conn *websocket.Conn, requestID string, pcmData []byte) {
	if len(pcmData) == 0 {
		vp.hub.sendError(conn, requestID, "no audio data")
		return
	}

	vp.hub.stateMu.Lock()
	_ = vp.hub.stateMachine.Transition(state.THINKING)
	vp.hub.broadcastState("THINKING")
	vp.hub.stateMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := vp.stt.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		log.Printf("voice: STT error: %v", err)
		vp.hub.sendError(conn, requestID, fmt.Sprintf("STT error: %v", err))
		vp.resetState()
		return
	}
	if result.Error != "" {
		log.Printf("voice: STT result error: %s", result.Error)
		vp.hub.sendError(conn, requestID, result.Error)
		vp.resetState()
		return
	}

	log.Printf("voice: STT result: %q", result.Text)

	vp.hub.writeJSON(conn, WSSpeechRecognized{
		Type: "speech_recognized", RequestID: requestID,
		Text: result.Text, Cancelable: true,
	})

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	vp.hub.stateMu.Lock()
	vp.hub.pendingCancels[requestID] = cancelFunc
	vp.hub.stateMu.Unlock()

	select {
	case <-cancelCtx.Done():
		log.Printf("voice: speech cancelled: %s", requestID)
	case <-time.After(3 * time.Second):
		vp.hub.stateMu.Lock()
		delete(vp.hub.pendingCancels, requestID)
		vp.hub.stateMu.Unlock()
		log.Printf("voice: sending text to agent: %q", result.Text)
		// Reset state so handleTextMessageAgent can transition from IDLE
		vp.hub.stateMu.Lock()
		vp.hub.stateMachine.Reset()
		vp.hub.broadcastState("IDLE")
		vp.hub.stateMu.Unlock()

		msg := WSMessage{Type: "text", Payload: result.Text, RequestID: requestID}
		vp.hub.handleTextMessageAgent(conn, msg)
	}
}

func (vp *VoicePipeline) resetState() {
	vp.hub.stateMu.Lock()
	vp.hub.broadcastState("IDLE")
	vp.hub.stateMachine.Reset()
	vp.hub.stateMu.Unlock()
}

func (ac *AudioChunk) pcmBytes() []byte {
	buf := make([]byte, len(ac.Samples)*2)
	for i, s := range ac.Samples {
		buf[i*2] = byte(s)
		buf[i*2+1] = byte(s >> 8)
	}
	return buf
}

func (h *WebSocketHub) broadcastVADEvent(requestID, event string) {
	h.Broadcast(WSVADEvent{
		Type: "vad_event", RequestID: requestID, Event: event,
	})
}
