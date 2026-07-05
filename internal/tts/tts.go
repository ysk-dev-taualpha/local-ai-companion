// Package tts は VOICEVOX を使用したテキスト音声合成機能を提供します。
package tts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// VOICEVOXClient は VOICEVOX Engine の HTTP API クライアントです。
// audio_query と synthesis エンドポイントを順に呼び出し、
// テキストから WAV 音声データを生成します。
type VOICEVOXClient struct {
	baseURL   string
	speakerID int
	client    *http.Client
}

// NewVOICEVOX は新しい VOICEVOXClient を生成します。
// voicevoxURL は VOICEVOX Engine のベース URL (例: "http://127.0.0.1:50021")、
// speakerID は話者 ID (例: 3 = ずんだもん) です。
func NewVOICEVOX(voicevoxURL string, speakerID int) *VOICEVOXClient {
	return &VOICEVOXClient{
		baseURL:   voicevoxURL,
		speakerID: speakerID,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Speak はテキストを WAV 音声データに変換します。
// 内部で audio_query → synthesis の 2 ステップを実行します。
func (c *VOICEVOXClient) Speak(text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("tts: empty text")
	}

	// 1. audio_query: テキストから音声合成用クエリを取得
	queryParams, err := c.audioQuery(text)
	if err != nil {
		return nil, fmt.Errorf("tts: audio_query failed: %w", err)
	}

	// 2. synthesis: クエリから WAV 音声を生成
	wavData, err := c.synthesis(queryParams)
	if err != nil {
		return nil, fmt.Errorf("tts: synthesis failed: %w", err)
	}

	return wavData, nil
}

// audioQuery はテキストを音声合成用クエリパラメータに変換します。
func (c *VOICEVOXClient) audioQuery(text string) ([]byte, error) {
	u, err := url.Parse(c.baseURL + "/audio_query")
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	q := u.Query()
	q.Set("text", text)
	q.Set("speaker", fmt.Sprintf("%d", c.speakerID))
	u.RawQuery = q.Encode()

	resp, err := c.client.Post(u.String(), "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// synthesis はクエリパラメータから WAV 音声データを生成します。
func (c *VOICEVOXClient) synthesis(audioQueryJSON []byte) ([]byte, error) {
	u, err := url.Parse(c.baseURL + "/synthesis")
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	q := u.Query()
	q.Set("speaker", fmt.Sprintf("%d", c.speakerID))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(audioQueryJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/wav")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// TTSClient は TTS エンジンの共通インターフェースです。
// VOICEVOXClient の呼び出し側が依存するインターフェースとして使用します。
type TTSClient interface {
	Speak(text string) ([]byte, error)
}

// Ensure VOICEVOXClient implements TTSClient
var _ TTSClient = (*VOICEVOXClient)(nil)
