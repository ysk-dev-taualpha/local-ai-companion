// Package stt は faster-whisper を使用した音声認識 (Speech-to-Text) 機能を提供します。
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

// Result は音声認識の結果を表します。
type Result struct {
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

// STTClient は音声認識エンジンの共通インターフェースです。
type STTClient interface {
	Transcribe(ctx context.Context, pcmData []byte, sampleRate int, language string) (*Result, error)
}

// FasterWhisperClient は faster-whisper HTTP サーバーのクライアントです。
type FasterWhisperClient struct {
	serverURL string
	client    *http.Client
}

// NewFasterWhisper は新しい FasterWhisperClient を生成します。
func NewFasterWhisper(serverURL string, timeout time.Duration) *FasterWhisperClient {
	return &FasterWhisperClient{
		serverURL: serverURL,
		client:    &http.Client{Timeout: timeout},
	}
}

// Transcribe は PCM 音声データをテキストに変換します。
func (c *FasterWhisperClient) Transcribe(ctx context.Context, pcmData []byte, sampleRate int, language string) (*Result, error) {
	if len(pcmData) == 0 {
		return &Result{Text: "", Error: "empty audio"}, nil
	}

	wavData, err := pcmToWAV(pcmData, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("stt: pcm to wav conversion failed: %w", err)
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
		Text  string `json:"text"`
		Error string `json:"error,omitempty"`
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

	return &Result{Text: text}, nil
}

// pcmToWAV は raw PCM int16 モノラルデータを WAV 形式に変換します。
func pcmToWAV(pcmData []byte, sampleRate int) ([]byte, error) {
	if len(pcmData)%2 != 0 {
		return nil, fmt.Errorf("pcm data length must be even (int16 samples), got %d bytes", len(pcmData))
	}

	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	dataSize := uint32(36 + len(pcmData))
	binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	byteRate := uint32(sampleRate * 2)
	binary.Write(&buf, binary.LittleEndian, byteRate)
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)))
	buf.Write(pcmData)

	return buf.Bytes(), nil
}

// buildMultipart は WAV データと language を含む multipart/form-data リクエストボディを構築します。
func buildMultipart(wavData []byte, language string) (body []byte, contentType string, err error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	audioPart, err := writer.CreateFormFile("audio_file", "audio.wav")
	if err != nil {
		return nil, "", err
	}
	audioPart.Write(wavData)

	writer.WriteField("language", language)

	writer.Close()

	return buf.Bytes(), writer.FormDataContentType(), nil
}
