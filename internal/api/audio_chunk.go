package api

import (
	"encoding/binary"
	"fmt"
	"log"
)

// AudioChunk は Unity から受信する音声チャンクの内部表現です。
type AudioChunk struct {
	RequestID  string // 発話セッション識別子
	Seq        uint32 // チャンクシーケンス番号（0始まり）
	SampleRate uint16 // サンプルレート（Hz）
	Samples    []int16 // PCM 16bit サンプル配列
}

// MinAudioChunkHeaderSize はバイナリフレームの最小ヘッダサイズです。
// 4B (request_id length) + 1B (最小 request_id) + 4B (seq) + 2B (sample_rate)
const MinAudioChunkHeaderSize = 11

// parseAudioChunk はバイナリフレームを AudioChunk にパースします。
//
// バイナリフォーマット（全フィールド big-endian、samples は little-endian）:
//
//	[4B: request_id UTF-8 length, uint32]
//	[NB: request_id UTF-8 bytes]
//	[4B: seq uint32]
//	[2B: sample_rate uint16]
//	[NB: PCM int16 samples]
//
// エラー時は nil とエラーを返します。
func parseAudioChunk(data []byte) (*AudioChunk, error) {
	if len(data) < MinAudioChunkHeaderSize {
		return nil, fmt.Errorf("invalid_audio_chunk: frame too small (%d bytes, min %d)", len(data), MinAudioChunkHeaderSize)
	}

	offset := 0

	// request_id 長（4B big-endian uint32）
	reqIDLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 残りバイト数で境界チェック
	remaining := len(data) - offset
	if remaining < int(reqIDLen)+4+2 {
		return nil, fmt.Errorf("invalid_audio_chunk: insufficient data for request_id+seq+sample_rate (need %d, have %d)", reqIDLen+4+2, remaining)
	}

	// request_id（UTF-8）
	requestID := string(data[offset : offset+int(reqIDLen)])
	offset += int(reqIDLen)

	// seq（4B big-endian uint32）
	seq := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// sample_rate（2B big-endian uint16）
	sampleRate := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// PCM サンプル（int16 little-endian）
	sampleBytes := len(data) - offset
	numSamples := sampleBytes / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(data[offset+i*2 : offset+(i+1)*2]))
	}

	return &AudioChunk{
		RequestID:  requestID,
		Seq:        seq,
		SampleRate: sampleRate,
		Samples:    samples,
	}, nil
}

// handleAudioChunk は受信した audio_chunk をログ出力します。
// 将来の拡張: VAD パイプラインに転送、バッファリングなど。
func (h *WebSocketHub) handleAudioChunk(data []byte) {
	chunk, err := parseAudioChunk(data)
	if err != nil {
		log.Printf("websocket: audio_chunk parse failed: %v", err)
		return
	}

	log.Printf("websocket: audio_chunk received: request_id=%s seq=%d sample_rate=%d samples=%d",
		chunk.RequestID, chunk.Seq, chunk.SampleRate, len(chunk.Samples))
}
