package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/api"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/config"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/logging"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
)

func main() {
	configPath := flag.String("config", "", "Path to runtime config JSON file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.Logging.Level)
	logger.Info("Go Runtime starting on %s", cfg.Runtime.ListenAddr)

	pythonClient := client.New(cfg.PythonService.BaseURL)
	handler := api.New(pythonClient, cfg.Runtime.RequestTimeoutMs)
	stateMachine := state.New(nil)
	wsHub := api.NewWebSocketHub(pythonClient, stateMachine, cfg.Runtime.RequestTimeoutMs)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/conversation", handler.HandleConversation)
	mux.HandleFunc("/healthz", handler.HandleHealth)
	mux.HandleFunc("/ws", wsHub.HandleWS)

	server := &http.Server{
		Addr:    cfg.Runtime.ListenAddr,
		Handler: mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	logger.Info("Go Runtime ready")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error: %v", err)
		os.Exit(1)
	}
}
