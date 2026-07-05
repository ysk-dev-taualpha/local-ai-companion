package stt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewFasterWhisper(t *testing.T) {
	client := NewFasterWhisper("http://192.168.12.107:8093/v1/transcribe", 5*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.serverURL != "http://192.168.12.107:8093/v1/transcribe" {
		t.Errorf("expected serverURL, got %q", client.serverURL)
	}
}

func TestFasterWhisperClient_Transcribe_EmptyAudio(t *testing.T) {
	client := NewFasterWhisper("http://localhost:8093/v1/transcribe", 1*time.Second)
	ctx := context.Background()
	result, err := client.Transcribe(ctx, []byte{}, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "" {
		t.Errorf("expected empty text for empty audio, got %q", result.Text)
	}
	if result.Error != "empty audio" {
		t.Errorf("expected 'empty audio' error, got %q", result.Error)
	}
}

func TestFasterWhisperClient_Transcribe_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"こんにちは"}`))
	}))
	defer srv.Close()
	client := NewFasterWhisper(srv.URL+"/v1/transcribe", 5*time.Second)
	ctx := context.Background()
	pcmData := make([]byte, 16000)
	result, err := client.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "こんにちは" {
		t.Errorf("expected 'こんにちは', got %q", result.Text)
	}
}

func TestFasterWhisperClient_Transcribe_NoSpeech(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":""}`))
	}))
	defer srv.Close()
	client := NewFasterWhisper(srv.URL+"/v1/transcribe", 5*time.Second)
	ctx := context.Background()
	pcmData := make([]byte, 1600)
	result, err := client.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "no speech detected" {
		t.Errorf("expected 'no speech detected' error, got %q", result.Error)
	}
}

func TestFasterWhisperClient_Transcribe_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()
	client := NewFasterWhisper(srv.URL+"/v1/transcribe", 100*time.Millisecond)
	ctx := context.Background()
	pcmData := make([]byte, 1600)
	result, err := client.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "timeout" {
		t.Errorf("expected 'timeout' error, got %q", result.Error)
	}
}

func TestFasterWhisperClient_Transcribe_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()
	client := NewFasterWhisper(srv.URL+"/v1/transcribe", 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	pcmData := make([]byte, 1600)
	result, err := client.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "timeout" {
		t.Errorf("expected 'timeout' error, got %q", result.Error)
	}
}

func TestFasterWhisperClient_Transcribe_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer srv.Close()
	client := NewFasterWhisper(srv.URL+"/v1/transcribe", 5*time.Second)
	ctx := context.Background()
	pcmData := make([]byte, 1600)
	result, err := client.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected non-empty error string")
	}
}

func TestFasterWhisperClient_Transcribe_ConnectionRefused(t *testing.T) {
	client := NewFasterWhisper("http://127.0.0.1:19997/v1/transcribe", 100*time.Millisecond)
	ctx := context.Background()
	pcmData := make([]byte, 1600)
	result, err := client.Transcribe(ctx, pcmData, 16000, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected connection error string")
	}
}

func TestPCMToWAV(t *testing.T) {
	pcmData := make([]byte, 3200)
	wav, err := pcmToWAV(pcmData, 16000)
	if err != nil {
		t.Fatalf("pcmToWAV failed: %v", err)
	}
	if len(wav) < 44 {
		t.Errorf("WAV too small: %d bytes (min 44)", len(wav))
	}
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("expected RIFF header, got %q", string(wav[0:4]))
	}
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("expected WAVE, got %q", string(wav[8:12]))
	}
}

func TestPCMToWAV_OddLength(t *testing.T) {
	pcmData := make([]byte, 3)
	_, err := pcmToWAV(pcmData, 16000)
	if err == nil {
		t.Fatal("expected error for odd-length PCM data")
	}
}

func TestPCMToWAV_DifferentSampleRates(t *testing.T) {
	for _, sr := range []int{8000, 16000, 22050, 44100} {
		pcmData := make([]byte, 1600)
		wav, err := pcmToWAV(pcmData, sr)
		if err != nil {
			t.Errorf("pcmToWAV failed for sampleRate %d: %v", sr, err)
			continue
		}
		if len(wav) < 44 {
			t.Errorf("sampleRate %d: WAV too small: %d bytes", sr, len(wav))
		}
	}
}

func TestBuildMultipart(t *testing.T) {
	wavData := []byte("fake-wav-data")
	body, contentType, err := buildMultipart(wavData, "ja")
	if err != nil {
		t.Fatalf("buildMultipart failed: %v", err)
	}
	if contentType == "" {
		t.Error("expected non-empty Content-Type")
	}
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestBuildMultipart_DifferentLanguages(t *testing.T) {
	for _, lang := range []string{"ja", "en", "zh"} {
		wavData := []byte("fake")
		_, _, err := buildMultipart(wavData, lang)
		if err != nil {
			t.Errorf("buildMultipart failed for language %q: %v", lang, err)
		}
	}
}
