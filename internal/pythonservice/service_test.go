package pythonservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/config"
)

func TestStartNoCommandUsesExternalService(t *testing.T) {
	service := New(config.PythonServiceConfig{BaseURL: "http://127.0.0.1:8090"}, nil)

	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartCommandWaitsForBaseURLAndStops(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is required for shell command test")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := New(config.PythonServiceConfig{
		BaseURL:           server.URL,
		Command:           "trap 'exit 0' INT TERM; while true; do sleep 1; done",
		ReadyTimeoutMs:    1000,
		ShutdownTimeoutMs: 1000,
	}, nil)

	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestStartReturnsErrorWhenProcessExitsBeforeReadiness(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is required for shell command test")
	}

	service := New(config.PythonServiceConfig{
		BaseURL:        "http://127.0.0.1:1",
		Command:        "exit 7",
		ReadyTimeoutMs: 1000,
	}, nil)

	err := service.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "before readiness") {
		t.Fatalf("expected before readiness error, got %v", err)
	}
}
