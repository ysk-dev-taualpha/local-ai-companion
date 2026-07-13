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

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/agent"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/api"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/audit"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/config"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/logging"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/memory"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/pythonservice"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/state"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/stt"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool/tools"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tts"
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

	var ttsClient *tts.VOICEVOXClient
	if cfg.TTS.Enabled {
		ttsClient = tts.NewVOICEVOX(cfg.TTS.VoicevoxURL, cfg.TTS.SpeakerID)
		logger.Info("TTS enabled: VOICEVOX at %s, speaker=%d", cfg.TTS.VoicevoxURL, cfg.TTS.SpeakerID)
	}

	// Agent tool calling setup
	var agentLoop *agent.Loop
	if cfg.Ollama.Enabled && cfg.Agent.Enabled {
		agentLoop = setupAgent(cfg)
		logger.Info("Agent loop enabled: model=%s, max_loops=%d, tools=%v",
			cfg.Ollama.Model, cfg.Agent.MaxToolLoops, cfg.Agent.AllowedTools)
	}

	// Voice input pipeline (VAD + STT)
	var vp *api.VoicePipeline
	if cfg.VoiceInput.Enabled {
		sttClient := stt.NewFasterWhisper(cfg.VoiceInput.STTServerURL, time.Duration(cfg.VoiceInput.STTTimeoutMs)*time.Millisecond)
		vp = api.NewVoicePipeline(sttClient)
		logger.Info("Voice input enabled: vad=%s, stt=%s", cfg.VoiceInput.VADURL, cfg.VoiceInput.STTServerURL)
	}

	memStore, err := memory.NewStore("file:conversation.db", 10)
	if err != nil {
		logger.Error("memory store unavailable: %v", err)
	}
	if memStore != nil {
		_ = memStore.CleanOldSessions(24 * time.Hour)
	}

	wsHub := api.NewWebSocketHub(memStore, pythonClient, ttsClient, state.New(nil), cfg.Runtime.RequestTimeoutMs, cfg.WebSocket.AllowedOrigins, agentLoop, vp)
	if vp != nil {
		vp.SetHub(wsHub)
	}

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

func setupAgent(cfg *config.Config) *agent.Loop {
	registry := tool.NewRegistry()

	webSearch := tools.NewWebSearch(cfg.Agent.WebSearchURL, cfg.Agent.WebSearchAPIKeyEnv)
	webFetch := tools.NewWebFetch()
	audioControl := tools.NewAudioControl(nil)
	setState := tools.NewSetState(nil)

	registry.Register(tool.Definition{
		Name:        "web_search",
		Description: "Search the web for information. Use this to find current information, facts, or answers to questions.",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"query":       {Type: "string", Description: "The search query"},
				"max_results": {Type: "integer", Description: "Maximum number of results to return (default: 3)"},
			},
			Required: []string{"query"},
		},
	}, webSearch)

	registry.Register(tool.Definition{
		Name:        "web_fetch",
		Description: "Fetch and read the content of a web page by URL. Returns the text content.",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"url": {Type: "string", Description: "The URL to fetch"},
			},
			Required: []string{"url"},
		},
	}, webFetch)

	registry.Register(tool.Definition{
		Name:        "audio_control",
		Description: "Control audio playback. Actions: stop, clear_queue.",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"action": {
					Type:        "string",
					Description: "The audio action to perform",
					Enum:        []string{"stop", "clear_queue"},
				},
			},
			Required: []string{"action"},
		},
	}, audioControl)

	registry.Register(tool.Definition{
		Name:        "set_state",
		Description: "Change the AI companion's state. Valid states: IDLE, LISTENING, THINKING, SPEAKING.",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"state": {
					Type:        "string",
					Description: "The target state",
					Enum:        []string{"IDLE", "LISTENING", "THINKING", "SPEAKING"},
				},
			},
			Required: []string{"state"},
		},
	}, setState)

	policy := tool.NewPolicy()
	for _, name := range cfg.Agent.AllowedTools {
		policy.Allow(name)
	}

	executor := tool.NewExecutor(registry, policy)
	auditLogger := audit.NewLogger()

	return agent.NewLoop(registry, executor, auditLogger, agent.Config{
		MaxToolLoops:  cfg.Agent.MaxToolLoops,
		OllamaBaseURL: cfg.Ollama.BaseURL,
		Model:         cfg.Ollama.Model,
		OllamaTimeout: time.Duration(cfg.Ollama.TimeoutMs) * time.Millisecond,
		SystemPrompt:  cfg.Agent.SystemPrompt,
		RuntimeCtx: &agent.RuntimeContext{
			Timezone: cfg.Agent.Timezone,
			Locale:   cfg.Agent.Locale,
		},
	})
}
