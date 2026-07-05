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

// buildAudioChunkFrame はテスト用の audio_chunk バイナリフレームを構築します。
//
// バイナリフレーム形式（Big Endian）:
//
//	[4B reqIDlen][NB reqID UTF-8][4B sequence number][2B sample rate][NB PCM int16 LE]
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

func TestParseAudioChunk_Valid(t *testing.T) {
	frame := buildAudioChunkFrame("test-req-001", 0, 16000, []int16{100, 200, -300, 400})
	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("parseAudioChunk failed: %v", err)
	}
	if chunk.RequestID != "test-req-001" {
		t.Errorf("expected RequestID 'test-req-001', got %q", chunk.RequestID)
	}
	if chunk.Sequence != 0 {
		t.Errorf("expected Sequence 0, got %d", chunk.Sequence)
	}
	if chunk.SampleRate != 16000 {
		t.Errorf("expected SampleRate 16000, got %d", chunk.SampleRate)
	}
	if len(chunk.PCMData) != 8 {
		t.Errorf("expected PCMData length 8, got %d", len(chunk.PCMData))
	}
}

func TestParseAudioChunk_TooSmall(t *testing.T) {
	_, err := parseAudioChunk([]byte{0, 0, 0})
	if err == nil {
		t.Fatal("expected error for frame < 4 bytes")
	}
}

func TestParseAudioChunk_ReqIDLenTooLarge(t *testing.T) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], 300) // reqIDLen > 256
	_, err := parseAudioChunk(buf)
	if err == nil {
		t.Fatal("expected error for reqIDLen > 256")
	}
}

func TestParseAudioChunk_TruncatedHeader(t *testing.T) {
	// reqIDLen=10 but only 5 bytes of requestID provided
	buf := make([]byte, 4+5)
	binary.BigEndian.PutUint32(buf[0:4], 10)
	copy(buf[4:], "hello")
	_, err := parseAudioChunk(buf)
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestParseAudioChunk_ZeroLengthPCM(t *testing.T) {
	frame := buildAudioChunkFrame("req", 5, 8000, []int16{})
	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("parseAudioChunk failed: %v", err)
	}
	if len(chunk.PCMData) != 0 {
		t.Errorf("expected empty PCMData, got %d bytes", len(chunk.PCMData))
	}
	if chunk.Sequence != 5 {
		t.Errorf("expected Sequence 5, got %d", chunk.Sequence)
	}
	if chunk.SampleRate != 8000 {
		t.Errorf("expected SampleRate 8000, got %d", chunk.SampleRate)
	}
}

func TestParseAudioChunk_LargeSequence(t *testing.T) {
	frame := buildAudioChunkFrame("req", 0xFFFFFFFF, 16000, []int16{1})
	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("parseAudioChunk failed: %v", err)
	}
	if chunk.Sequence != 0xFFFFFFFF {
		t.Errorf("expected Sequence 0xFFFFFFFF, got %d", chunk.Sequence)
	}
}

func TestParseAudioChunk_JapaneseRequestID(t *testing.T) {
	frame := buildAudioChunkFrame("リクエスト", 1, 16000, []int16{0})
	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("parseAudioChunk failed: %v", err)
	}
	if chunk.RequestID != "リクエスト" {
		t.Errorf("expected RequestID 'リクエスト', got %q", chunk.RequestID)
	}
}

func TestParseAudioChunk_VariousSampleRates(t *testing.T) {
	tests := []uint16{8000, 16000, 22050, 44100, 48000}
	for _, sr := range tests {
		frame := buildAudioChunkFrame("req", 0, sr, []int16{1, 2, 3})
		chunk, err := parseAudioChunk(frame)
		if err != nil {
			t.Errorf("parseAudioChunk failed for sampleRate %d: %v", sr, err)
			continue
		}
		if chunk.SampleRate != sr {
			t.Errorf("sampleRate %d: expected %d, got %d", sr, sr, chunk.SampleRate)
		}
	}
}

func TestParseAudioChunk_LargePCMChunk(t *testing.T) {
	// 8000 samples (500ms @ 16kHz)
	samples := make([]int16, 8000)
	for i := range samples {
		samples[i] = int16(i % 32767)
	}
	frame := buildAudioChunkFrame("req", 0, 16000, samples)
	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("parseAudioChunk failed: %v", err)
	}
	if len(chunk.PCMData) != 16000 {
		t.Errorf("expected PCMData length 16000, got %d", len(chunk.PCMData))
	}
}

// TestHandleWS_AudioChunkBinaryFrame はバイナリフレーム受信後も
// テキストメッセージが正常に処理されることを確認します。
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

	// バイナリフレーム送信
	frame := buildAudioChunkFrame("test-req", 0, 16000, []int16{100, 200})
	if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	// バイナリフレームの後にテキストフレームを送信 → 応答が返ることを確認
	msg := WSMessage{Type: "ping", Payload: "after-binary"}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("write text: %v", err)
	}

	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(respBytes), "ping_ack") {
		t.Errorf("expected ping_ack, got: %s", string(respBytes))
	}
}

// TestHandleWS_MultipleBinaryFrames は複数のバイナリフレームを連続送信しても
// 接続が維持されることを確認します。
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

	// 10個のバイナリフレームを連続送信
	for i := 0; i < 10; i++ {
		frame := buildAudioChunkFrame("req", uint32(i), 16000, []int16{int16(i)})
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			t.Fatalf("write binary %d: %v", i, err)
		}
	}

	// その後にテキストメッセージ送信 → 応答が返る
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

// TestHandleWS_BinaryFrameThenTextFlow はバイナリフレーム後も
// text メッセージフロー（StateMachine 遷移）が正常に動作することを確認します。
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

	// audio_chunk バイナリフレームを送信
	frame := buildAudioChunkFrame("test-req", 0, 16000, []int16{100, 200})
	conn.WriteMessage(websocket.BinaryMessage, frame)

	// text メッセージを送信 → 通常の会話フローが動作する
	msg := WSMessage{Type: "text", Payload: "こんにちは", RequestID: "req-001"}
	msgBytes, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, msgBytes)

	// state_change 通知（LISTENING, THINKING, SPEAKING, IDLE）を読み取る
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
