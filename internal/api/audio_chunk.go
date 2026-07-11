package api

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

const MinAudioChunkHeaderSize = 11

type AudioChunk struct {
	RequestID  string
	Seq        uint32
	SampleRate uint16
	Samples    []int16
}

func ParseAudioChunk(data []byte) (*AudioChunk, error) {
	if len(data) < MinAudioChunkHeaderSize {
		return nil, fmt.Errorf("invalid_audio_chunk: frame too small (%d bytes, min %d)", len(data), MinAudioChunkHeaderSize)
	}
	offset := 0
	reqIDLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	if int(reqIDLen) > len(data)-offset-4-2 {
		return nil, fmt.Errorf("invalid_audio_chunk: insufficient data for request_id+seq+sample_rate (need %d, have %d)", reqIDLen+4+2, len(data)-offset)
	}
	requestID := string(data[offset : offset+int(reqIDLen)])
	offset += int(reqIDLen)
	seq := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	sampleRate := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	remainingBytes := len(data) - offset
	if remainingBytes%2 != 0 {
		return nil, fmt.Errorf("invalid_audio_chunk: PCM data length not aligned to 2 bytes (%d bytes)", remainingBytes)
	}
	sampleCount := remainingBytes / 2
	samples := make([]int16, sampleCount)
	for i := 0; i < sampleCount; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(data[offset+i*2 : offset+i*2+2]))
	}
	return &AudioChunk{RequestID: requestID, Seq: seq, SampleRate: sampleRate, Samples: samples}, nil
}

func (h *WebSocketHub) handleAudioChunk(conn *websocket.Conn, data []byte) {
	chunk, err := ParseAudioChunk(data)
	if err != nil {
		log.Printf("websocket: audio_chunk parse failed: %v", err)
		return
	}
	if h.voicePipeline != nil {
		h.voicePipeline.processChunk(conn, chunk)
		return
	}
	log.Printf("websocket: audio_chunk received: request_id=%s seq=%d sample_rate=%d samples=%d", chunk.RequestID, chunk.Seq, chunk.SampleRate, len(chunk.Samples))
}
