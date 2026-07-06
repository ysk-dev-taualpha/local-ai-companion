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

func TestParseAudioChunk_Valid(t *testing.T) {
	frame := buildAudioChunkFrame("test-req-001", 0, 16000, []int16{100, 200, -300, 400})
	chunk, err := ParseAudioChunk(frame)
	if err != nil {
		t.Fatalf("ParseAudioChunk failed: %v", err)
	}
	if chunk.RequestID != "test-req-001" {
		t.Errorf("expected RequestID 'test-req-001', got %q", chunk.RequestID)
	}
	if chunk.Seq != 0 {
		t.Errorf("expected Seq 0, got %d", chunk.Seq)
	}
	if chunk.SampleRate != 16000 {
		t.Errorf("expected SampleRate 16000, got %d", chunk.SampleRate)
	}
	if len(chunk.Samples) != 4 {
		t.Errorf("expected Samples length 4, got %d", len(chunk.Samples))
	}
}

func TestParseAudioChunk_ReqIDLenTooLarge(t *testing.T) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], 300)
	_, err := ParseAudioChunk(buf)
	if err == nil {
		t.Fatal("expected error for reqIDLen > 256")
	}
}

func TestParseAudioChunk_TruncatedHeader(t *testing.T) {
	buf := make([]byte, 4+5)
	binary.BigEndian.PutUint32(buf[0:4], 10)
	copy(buf[4:], "hello")
	_, err := ParseAudioChunk(buf)
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestParseAudioChunk_ZeroLengthPCM(t *testing.T) {
	frame := buildAudioChunkFrame("req", 5, 8000, []int16{})
	chunk, err := ParseAudioChunk(frame)
	if err != nil {
		t.Fatalf("ParseAudioChunk failed: %v", err)
	}
	if len(chunk.Samples) != 0 {
		t.Errorf("expected empty Samples, got %d", len(chunk.Samples))
	}
	if chunk.Seq != 5 {
		t.Errorf("expected Seq 5, got %d", chunk.Seq)
	}
	if chunk.SampleRate != 8000 {
		t.Errorf("expected SampleRate 8000, got %d", chunk.SampleRate)
	}
}

func TestParseAudioChunk_LargeSequence(t *testing.T) {
	frame := buildAudioChunkFrame("req", 0xFFFFFFFF, 16000, []int16{1})
	chunk, err := ParseAudioChunk(frame)
	if err != nil {
		t.Fatalf("ParseAudioChunk failed: %v", err)
	}
	if chunk.Seq != 0xFFFFFFFF {
		t.Errorf("expected Seq 0xFFFFFFFF, got %d", chunk.Seq)
	}
}

func TestParseAudioChunk_JapaneseRequestID(t *testing.T) {
	frame := buildAudioChunkFrame("リクエスト", 1, 16000, []int16{0})
	chunk, err := ParseAudioChunk(frame)
	if err != nil {
		t.Fatalf("ParseAudioChunk failed: %v", err)
	}
	if chunk.RequestID != "リクエスト" {
		t.Errorf("expected RequestID 'リクエスト', got %q", chunk.RequestID)
	}
}

func TestParseAudioChunk_VariousSampleRates(t *testing.T) {
	tests := []uint16{8000, 16000, 22050, 44100, 48000}
	for _, sr := range tests {
		frame := buildAudioChunkFrame("req", 0, sr, []int16{1, 2, 3})
		chunk, err := ParseAudioChunk(frame)
		if err != nil {
			t.Errorf("ParseAudioChunk failed for sampleRate %d: %v", sr, err)
			continue
		}
		if chunk.SampleRate != sr {
			t.Errorf("sampleRate %d: expected %d, got %d", sr, sr, chunk.SampleRate)
		}
	}
}

func TestParseAudioChunk_LargePCMChunk(t *testing.T) {
	samples := make([]int16, 8000)
	for i := range samples {
		samples[i] = int16(i % 32767)
	}
	frame := buildAudioChunkFrame("req", 0, 16000, samples)
	chunk, err := ParseAudioChunk(frame)
	if err != nil {
		t.Fatalf("ParseAudioChunk failed: %v", err)
	}
	if len(chunk.Samples) != 8000 {
		t.Errorf("expected Samples length 8000, got %d", len(chunk.Samples))
	}
}

func TestHandleWS_MultipleBinaryFrames(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	for i := 0; i < 10; i++ {
		frame := buildAudioChunkFrame("req", uint32(i), 16000, []int16{int16(i)})
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			t.Fatalf("write binary %d: %v", i, err)
		}
	}
	msg := WSMessage{Type: "ping", Payload: "after-10-binary"}
	msgBytes, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, msgBytes)
	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(respBytes), "ping_ack") {
		t.Errorf("expected ping_ack after 10 binary frames, got: %s", string(respBytes))
	}
}

func TestHandleWS_BinaryFrameThenTextFlow(t *testing.T) {
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
	msg := WSMessage{Type: "text", Payload: "こんにちは", RequestID: "req-001"}
	msgBytes, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, msgBytes)
	stateChanges := 0
	for i := 0; i < 10; i++ {
		_, respBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if strings.Contains(string(respBytes), "state_change") {
			stateChanges++
		}
		if strings.Contains(string(respBytes), "ai_response") {
			break
		}
	}
	if stateChanges < 2 {
		t.Errorf("expected at least 2 state_change messages, got %d", stateChanges)
	}
}
