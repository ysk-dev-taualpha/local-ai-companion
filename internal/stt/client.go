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
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

type STTClient interface {
	Transcribe(ctx context.Context, pcmData []byte, sampleRate int, language string) (*Result, error)
}

type FasterWhisperClient struct {
	serverURL string
	client    *http.Client
}

func NewFasterWhisper(serverURL string, timeout time.Duration) *FasterWhisperClient {
	return &FasterWhisperClient{serverURL: serverURL, client: &http.Client{Timeout: timeout}}
}

func (c *FasterWhisperClient) Transcribe(ctx context.Context, pcmData []byte, sampleRate int, language string) (*Result, error) {
	if len(pcmData) == 0 { return &Result{Error: "empty audio"}, nil }
	wavData, err := pcmToWAV(pcmData, sampleRate)
	if err != nil { return nil, fmt.Errorf("stt: wav: %w", err) }
	body, contentType, err := buildMultipart(wavData, language)
	if err != nil { return nil, fmt.Errorf("stt: multipart: %w", err) }
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL, bytes.NewReader(body))
	if err != nil { return nil, fmt.Errorf("stt: request: %w", err) }
	req.Header.Set("Content-Type", contentType)
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil { return &Result{Error: "timeout"}, nil }
		return &Result{Error: fmt.Sprintf("connection failed: %v", err)}, nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var sr struct{ Text string `json:"text"`; Error string `json:"error,omitempty"` }
	json.Unmarshal(respBody, &sr)
	if sr.Text == "" && sr.Error == "" { return &Result{Error: "no speech detected"}, nil }
	if sr.Text == "" && sr.Error != "" { return &Result{Error: sr.Error}, nil }
	return &Result{Text: sr.Text}, nil
}

func pcmToWAV(pcmData []byte, sampleRate int) ([]byte, error) {
	if len(pcmData)%2 != 0 { return nil, fmt.Errorf("odd pcm len: %d", len(pcmData)) }
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+len(pcmData)))
	buf.WriteString("WAVE"); buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)))
	buf.Write(pcmData)
	return buf.Bytes(), nil
}

func buildMultipart(wavData []byte, language string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	p, _ := w.CreateFormFile("audio_file", "audio.wav")
	p.Write(wavData)
	w.WriteField("language", language)
	w.Close()
	return buf.Bytes(), w.FormDataContentType(), nil
}
