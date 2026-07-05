package api

import (
	"encoding/binary"
	"fmt"
)

type AudioChunk struct {
	RequestID  string
	Sequence   uint32
	SampleRate uint16
	PCMData    []byte
}

func parseAudioChunk(data []byte) (*AudioChunk, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("audio_chunk: frame too short, got %d", len(data))
	}
	reqIDLen := binary.BigEndian.Uint32(data[0:4])
	if reqIDLen > 256 {
		return nil, fmt.Errorf("audio_chunk: reqIDlen too large: %d", reqIDLen)
	}
	headerSize := int(4 + reqIDLen + 4 + 2)
	if len(data) < headerSize {
		return nil, fmt.Errorf("audio_chunk: frame too short for header, need %d, got %d", headerSize, len(data))
	}
	offset := 4
	reqID := string(data[offset : offset+int(reqIDLen)])
	offset += int(reqIDLen)
	seq := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	sampleRate := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	return &AudioChunk{RequestID: reqID, Sequence: seq, SampleRate: sampleRate, PCMData: data[offset:]}, nil
}
