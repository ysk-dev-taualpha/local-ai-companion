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

// buildAudioChunkFrame は audio_chunk バイナリフレームを構築します。
//
// フォーマット:
//
//	[4B: request_id length, uint32 big-endian]
//	[NB: request_id UTF-8 bytes]
//	[4B: seq uint32 big-endian]
//	[2B: sample_rate uint16 big-endian]
//	[NB: PCM int16 samples, little-endian]
func buildAudioChunkFrame(requestID string, seq uint32, sampleRate uint16, samples []int16) []byte {
	reqIDBytes := []byte(requestID)
	buf := make([]byte, 4+len(reqIDBytes)+4+2+len(samples)*2)

	offset := 0
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(reqIDBytes)))
	offset += 4

	copy(buf[offset:], reqIDBytes)
	offset += len(reqIDBytes)

	binary.BigEndian.PutUint32(buf[offset:], seq)
	offset += 4

	binary.BigEndian.PutUint16(buf[offset:], sampleRate)
	offset += 2

	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[offset+i*2:], uint16(s))
	}

	return buf
}

// ─── parseAudioChunk ユニットテスト ───

func TestParseAudioChunk_Valid(t *testing.T) {
	samples := []int16{100, 200, 300, -100, -200}
	frame := buildAudioChunkFrame("req-001", 0, 16000, samples)

	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chunk.RequestID != "req-001" {
		t.Errorf("expected request_id 'req-001', got %q", chunk.RequestID)
	}
	if chunk.Seq != 0 {
		t.Errorf("expected seq 0, got %d", chunk.Seq)
	}
	if chunk.SampleRate != 16000 {
		t.Errorf("expected sample_rate 16000, got %d", chunk.SampleRate)
	}
	if len(chunk.Samples) != len(samples) {
		t.Fatalf("expected %d samples, got %d", len(samples), len(chunk.Samples))
	}
	for i, expected := range samples {
		if chunk.Samples[i] != expected {
			t.Errorf("samples[%d]: expected %d, got %d", i, expected, chunk.Samples[i])
		}
	}
}

func TestParseAudioChunk_ZeroSamples(t *testing.T) {
	// サンプルが0個のフレーム（seq=0 の発話開始シグナル的な使い方も想定）
	frame := buildAudioChunkFrame("start-req", 0, 16000, nil)

	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chunk.RequestID != "start-req" {
		t.Errorf("expected request_id 'start-req', got %q", chunk.RequestID)
	}
	if len(chunk.Samples) != 0 {
		t.Errorf("expected 0 samples, got %d", len(chunk.Samples))
	}
}

func TestParseAudioChunk_HighSeq(t *testing.T) {
	// seq が大きい値でも正しくパースできること
	frame := buildAudioChunkFrame("req-high", 9999, 16000, []int16{1, 2})

	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Seq != 9999 {
		t.Errorf("expected seq 9999, got %d", chunk.Seq)
	}
}

func TestParseAudioChunk_FrameTooSmall(t *testing.T) {
	// 最小ヘッダサイズ未満
	frame := make([]byte, MinAudioChunkHeaderSize-1)

	_, err := parseAudioChunk(frame)
	if err == nil {
		t.Fatal("expected error for too-small frame")
	}
	if !strings.Contains(err.Error(), "invalid_audio_chunk") {
		t.Errorf("expected 'invalid_audio_chunk' in error, got: %v", err)
	}
}

func TestParseAudioChunk_InsufficientDataForRequestID(t *testing.T) {
	// request_id 長がデータ長を超えている
	buf := make([]byte, 4+2) // 4B length + 2B data, but length says 10
	binary.BigEndian.PutUint32(buf, 10) // request_id length = 10

	_, err := parseAudioChunk(buf)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
	if !strings.Contains(err.Error(), "invalid_audio_chunk") {
		t.Errorf("expected 'invalid_audio_chunk' in error, got: %v", err)
	}
}

func TestParseAudioChunk_DifferentSampleRate(t *testing.T) {
	frame := buildAudioChunkFrame("req-48k", 0, 48000, []int16{1})

	chunk, err := parseAudioChunk(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.SampleRate != 48000 {
		t.Errorf("expected sample_rate 48000, got %d", chunk.SampleRate)
	}
}

// ─── HandleWS バイナリフレーム統合テスト ───

func TestHandleWS_AudioChunkBinaryFrame(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()

	conn := wsClient(t, srv.URL)

	// 有効な audio_chunk バイナリフレームを送信
	samples := make([]int16, 1600) // 100ms @ 16kHz
	for i := range samples {
		samples[i] = int16(i % 1000)
	}
	frame := buildAudioChunkFrame("req-audio-001", 0, 16000, samples)

	if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("failed to write binary frame: %v", err)
	}

	// バイナリフレーム送信後も接続が維持されることを確認
	// （テキストメッセージを送って応答が返ってくる）
	msg := WSMessage{Type: "ping", Payload: "after-binary"}
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to write text message: %v", err)
	}

	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response after binary frame: %v", err)
	}
	if !strings.Contains(string(respBytes), "ping_ack") {
		t.Errorf("expected ping_ack, got %s", string(respBytes))
	}
}
