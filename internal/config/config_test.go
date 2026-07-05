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
	if cfg.PythonService.Command != "" {
		t.Errorf("expected empty python command, got %s", cfg.PythonService.Command)
	}
	if cfg.PythonService.ReadyTimeoutMs != 10000 {
		t.Errorf("expected 10000 ready timeout, got %d", cfg.PythonService.ReadyTimeoutMs)
	}
	if cfg.PythonService.ShutdownTimeoutMs != 5000 {
		t.Errorf("expected 5000 shutdown timeout, got %d", cfg.PythonService.ShutdownTimeoutMs)
	}
}

func TestLoadDefaultsTTS(t *testing.T) {
	cfg, _ := Load("")
	if cfg.TTS.Enabled {
		t.Error("expected TTS disabled by default")
	}
	if cfg.TTS.VoicevoxURL != "http://127.0.0.1:50021" {
		t.Errorf("expected default voicevox_url 'http://127.0.0.1:50021', got %s", cfg.TTS.VoicevoxURL)
	}
	if cfg.TTS.SpeakerID != 3 {
		t.Errorf("expected default speaker_id 3, got %d", cfg.TTS.SpeakerID)
	}
}

func TestLoadDefaultsAgentRuntimeContext(t *testing.T) {
	cfg, _ := Load("")
	if cfg.Agent.Timezone != "Asia/Tokyo" {
		t.Errorf("expected default agent timezone Asia/Tokyo, got %s", cfg.Agent.Timezone)
	}
	if cfg.Agent.Locale != "ja-JP" {
		t.Errorf("expected default agent locale ja-JP, got %s", cfg.Agent.Locale)
	}
	if cfg.Agent.WebSearchURL != "https://ollama.com" {
		t.Errorf("expected default web search URL https://ollama.com, got %s", cfg.Agent.WebSearchURL)
	}
	if cfg.Agent.WebSearchAPIKeyEnv != "OLLAMA_API_KEY" {
		t.Errorf("expected default web search API key env OLLAMA_API_KEY, got %s", cfg.Agent.WebSearchAPIKeyEnv)
	}
}

func TestLoadFromFile(t *testing.T) {
	data := `{
		"runtime":{"listen_addr":"0.0.0.0:9090","request_timeout_ms":5000},
		"python_service":{
			"base_url":"http://127.0.0.1:8092",
			"command":"PYTHONPATH=./src python3 -m local_ai_companion --serve",
			"ready_timeout_ms":12000,
			"shutdown_timeout_ms":3000
		}
	}`
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
	if cfg.PythonService.BaseURL != "http://127.0.0.1:8092" {
		t.Errorf("expected python base url override, got %s", cfg.PythonService.BaseURL)
	}
	if cfg.PythonService.Command == "" {
		t.Error("expected python command override")
	}
	if cfg.PythonService.ReadyTimeoutMs != 12000 {
		t.Errorf("expected 12000 ready timeout, got %d", cfg.PythonService.ReadyTimeoutMs)
	}
	if cfg.PythonService.ShutdownTimeoutMs != 3000 {
		t.Errorf("expected 3000 shutdown timeout, got %d", cfg.PythonService.ShutdownTimeoutMs)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	cfg, _ := Load("")
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if len(b) == 0 {
		t.Error("marshal produced empty output")
	}
}
