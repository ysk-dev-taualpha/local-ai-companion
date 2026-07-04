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
	BaseURL string `json:"base_url"`
}

type LoggingConfig struct {
	Level string `json:"level"`
}

type Config struct {
	Runtime       RuntimeConfig       `json:"runtime"`
	WebSocket     WebSocketConfig     `json:"websocket"`
	PythonService PythonServiceConfig `json:"python_service"`
	Logging       LoggingConfig       `json:"logging"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Runtime: RuntimeConfig{
			ListenAddr:       "127.0.0.1:8080",
			RequestTimeoutMs: 30000,
		},
		PythonService: PythonServiceConfig{
			BaseURL: "http://127.0.0.1:8090",
		},
		Logging: LoggingConfig{
			Level: "info",
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
