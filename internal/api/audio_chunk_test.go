package api

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/gorilla/websocket"
)

func buildAudioChunkFrame(requestID string, seq uint32, sampleRate uint16, samples []int16) []byte {
	reqIDBytes := []byte(requestID)
	frame := make([]byte, 4+len(reqIDBytes)+4+2+len(samples)*2)
	off := 0
	binary.BigEndian.PutUint32(frame[off:], uint32(len(reqIDBytes)))
	off += 4
	copy(frame[off:], reqIDBytes)
	off += len(reqIDBytes)
	binary.BigEndian.PutUint32(frame[off:], seq)
	off += 4
	binary.BigEndian.PutUint16(frame[off:], sampleRate)
	off += 2
	for i, s := range samples {
		binary.LittleEndian.PutUint16(frame[off+i*2:], uint16(s))
	}
	return frame
}

func TestParseAudioChunk(t *testing.T) {
	frame := buildAudioChunkFrame("test-req-001", 0, 16000, []int16{100, 200, -300, 400})
	chunk, err := ParseAudioChunk(frame)
	if err != nil {
		t.Fatalf("ParseAudioChunk failed: %v", err)
	}
	if chunk.RequestID != "test-req-001" || chunk.Seq != 0 || len(chunk.Samples) != 4 {
		t.Errorf("unexpected chunk: %+v", chunk)
	}
}

func TestParseAudioChunk_TooSmall(t *testing.T) {
	_, err := ParseAudioChunk([]byte{0, 0, 0, 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseAudioChunk_TruncatedRequestID(t *testing.T) {
	buf := make([]byte, 4+5)
	binary.BigEndian.PutUint32(buf, 10)
	copy(buf[4:], "hello")
	_, err := ParseAudioChunk(buf)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleWS_AudioChunkBinaryFrame(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	frame := buildAudioChunkFrame("test-req", 0, 16000, []int16{100, 200})
	conn.WriteMessage(websocket.BinaryMessage, frame)
	msg := WSMessage{Type: "ping", Payload: "after-binary"}
	msgBytes, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, msgBytes)
	_, respBytes, _ := conn.ReadMessage()
	if !strings.Contains(string(respBytes), "ping_ack") {
		t.Errorf("expected ping_ack, got: %s", string(respBytes))
	}
}
