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
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/pythonservice"
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

	pythonService := pythonservice.New(cfg.PythonService, logger)
	if err := pythonService.Start(context.Background()); err != nil {
		logger.Error("failed to start Python AI Service: %v", err)
		os.Exit(1)
	}
	defer func() {
		shutdownTimeout := time.Duration(cfg.PythonService.ShutdownTimeoutMs) * time.Millisecond
		if shutdownTimeout <= 0 {
			shutdownTimeout = 5 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := pythonService.Stop(ctx); err != nil {
			logger.Error("failed to stop Python AI Service: %v", err)
		}
	}()

	pythonClient := client.New(cfg.PythonService.BaseURL)
	handler := api.New(pythonClient, cfg.Runtime.RequestTimeoutMs)
	wsHub := api.NewWebSocketHub(pythonClient, state.New(nil), cfg.Runtime.RequestTimeoutMs, cfg.WebSocket.AllowedOrigins)

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
