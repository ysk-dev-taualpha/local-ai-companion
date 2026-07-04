package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("expected default listen addr, got %s", cfg.Runtime.ListenAddr)
	}
	if cfg.Runtime.RequestTimeoutMs != 30000 {
		t.Errorf("expected 30000 timeout, got %d", cfg.Runtime.RequestTimeoutMs)
	}
}

func TestLoadFromFile(t *testing.T) {
	data := `{"runtime":{"listen_addr":"0.0.0.0:9090","request_timeout_ms":5000},"websocket":{"allowed_origins":["http://localhost:3000","http://127.0.0.1:5173"]}}`
	f, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(data)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.ListenAddr != "0.0.0.0:9090" {
		t.Errorf("expected 0.0.0.0:9090, got %s", cfg.Runtime.ListenAddr)
	}
	if cfg.Runtime.RequestTimeoutMs != 5000 {
		t.Errorf("expected 5000, got %d", cfg.Runtime.RequestTimeoutMs)
	}
	if len(cfg.WebSocket.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 allowed origins, got %d", len(cfg.WebSocket.AllowedOrigins))
	}
	if cfg.WebSocket.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("unexpected first allowed origin: %s", cfg.WebSocket.AllowedOrigins[0])
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	cfg, _ := Load("")
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if len(b) == 0 {
		t.Error("marshal produced empty output")
	}
}
