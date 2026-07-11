package stt

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Result struct {
	Text     string  `json:"text"`
	Duration float64 `json:"duration,omitempty"`
	Error    string  `json:"error,omitempty"`
}

type STTClient interface {
	Transcribe(ctx context.Context, pcmData []byte, sampleRate int, language string) (*Result, error)
}

type FasterWhisperClient struct {
	serverURL string
	client    *http.Client
}

func NewFasterWhisper(serverURL string, timeout time.Duration) *FasterWhisperClient {
	return &FasterWhisperClient{
		serverURL: serverURL,
		client:    &http.Client{Timeout: timeout},
	}
}

func (c *FasterWhisperClient) Transcribe(ctx context.Context, pcmData []byte, sampleRate int, language string) (*Result, error) {
	if len(pcmData) == 0 {
		return &Result{Text: "", Error: "empty audio"}, nil
	}

	wavData := pcmData
	if !isWAV(pcmData) {
		var err error
		wavData, err = pcmToWAV(pcmData, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("stt: pcm to wav conversion failed: %w", err)
		}
	}

	body, contentType, err := buildMultipart(wavData, language)
	if err != nil {
		return nil, fmt.Errorf("stt: failed to build multipart request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("stt: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return &Result{Text: "", Error: "timeout"}, nil
		}
		return &Result{Text: "", Error: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return &Result{Text: "", Error: fmt.Sprintf("failed to read response: %v", err)}, nil
	}

	var sttResp struct {
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
		Error    string  `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &sttResp); err != nil {
		return &Result{Text: "", Error: fmt.Sprintf("invalid response: %v", err)}, nil
	}

	text := sttResp.Text
	if text == "" && sttResp.Error == "" {
		return &Result{Text: "", Error: "no speech detected"}, nil
	}
	if text == "" && sttResp.Error != "" {
		return &Result{Text: "", Error: sttResp.Error}, nil
	}

	return &Result{Text: text, Duration: sttResp.Duration}, nil
}

func isWAV(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE"
}

func pcmToWAV(pcmData []byte, sampleRate int) ([]byte, error) {
	return pcmToWAVGo(pcmData, sampleRate)
}

func pcmToWAVGo(pcmData []byte, sampleRate int) ([]byte, error) {
	if len(pcmData)%2 != 0 {
		return nil, fmt.Errorf("pcm data length must be even (int16 samples), got %d bytes", len(pcmData))
	}

	var buf bytes.Buffer

	buf.WriteString("RIFF")
	dataSize := uint32(36 + len(pcmData))
	binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	byteRate := uint32(sampleRate * 2)
	binary.Write(&buf, binary.LittleEndian, byteRate)
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))

	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)))
	buf.Write(pcmData)

	return buf.Bytes(), nil
}

func buildMultipart(wavData []byte, language string) (body []byte, contentType string, err error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	audioPart, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, "", err
	}
	audioPart.Write(wavData)

	writer.WriteField("language", language)

	writer.Close()

	return buf.Bytes(), writer.FormDataContentType(), nil
}
