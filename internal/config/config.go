package config

import (
	"encoding/json"
	"os"
)

type RuntimeConfig struct {
	ListenAddr       string `json:"listen_addr"`
	RequestTimeoutMs int    `json:"request_timeout_ms"`
}

// WebSocketConfig は WebSocket 接続の設定です。
type WebSocketConfig struct {
	// AllowedOrigins は許可する Origin のリストです。
	// 空の場合は localhost のみ許可します。
	AllowedOrigins []string `json:"allowed_origins"`
}

type PythonServiceConfig struct {
	BaseURL           string `json:"base_url"`
	Command           string `json:"command"`
	ReadyTimeoutMs    int    `json:"ready_timeout_ms"`
	ShutdownTimeoutMs int    `json:"shutdown_timeout_ms"`
}

type LoggingConfig struct {
	Level string `json:"level"`
}

type TTSConfig struct {
	Enabled     bool   `json:"enabled"`
	VoicevoxURL string `json:"voicevox_url"`
	SpeakerID   int    `json:"speaker_id"`
}

// AgentConfig はエージェントツール呼び出しの設定です。
type AgentConfig struct {
	Enabled      bool     `json:"enabled"`
	OllamaURL    string   `json:"ollama_url"`
	OllamaModel  string   `json:"ollama_model"`
	MaxIter      int      `json:"max_iter"`
	SystemPrompt string   `json:"system_prompt"`
	AllowedTools []string `json:"allowed_tools"`
	AuditSize    int      `json:"audit_size"`
}

type Config struct {
	Runtime       RuntimeConfig       `json:"runtime"`
	WebSocket     WebSocketConfig     `json:"websocket"`
	PythonService PythonServiceConfig `json:"python_service"`
	Logging       LoggingConfig       `json:"logging"`
	TTS           TTSConfig           `json:"tts"`
	Agent         AgentConfig         `json:"agent"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Runtime: RuntimeConfig{
			ListenAddr:       "127.0.0.1:8080",
			RequestTimeoutMs: 30000,
		},
		PythonService: PythonServiceConfig{
			BaseURL:           "http://127.0.0.1:8090",
			ReadyTimeoutMs:    10000,
			ShutdownTimeoutMs: 5000,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		TTS: TTSConfig{
			Enabled:     false,
			VoicevoxURL: "http://127.0.0.1:50021",
			SpeakerID:   3,
		},
		Agent: AgentConfig{
			Enabled:     false,
			OllamaURL:   "http://192.168.12.107:11434",
			OllamaModel: "g4v100",
			MaxIter:     5,
			AllowedTools: []string{},
			AuditSize:   1000,
		},
	}

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
