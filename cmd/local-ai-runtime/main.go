package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/config"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/logging"
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
	logger.Info("Python AI Service URL: %s", cfg.PythonService.BaseURL)
	logger.Info("Go Runtime ready (scaffold)")
}
