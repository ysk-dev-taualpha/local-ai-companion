package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/audit"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/memory"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

type Config struct {
	MaxToolLoops  int
	OllamaBaseURL string
	Model         string
	OllamaTimeout time.Duration
	SystemPrompt  string
	RuntimeCtx    *RuntimeContext
	MemoryStore   *memory.Store
}

type Loop struct {
	client      *OllamaClient
	executor    *tool.ExecutorService
	registry    *tool.Registry
	audit       *audit.Logger
	config      Config
	memoryStore *memory.Store
}

func NewLoop(registry *tool.Registry, executor *tool.ExecutorService, auditLogger *audit.Logger, config Config) *Loop {
	timeout := config.OllamaTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Loop{
		client:      NewOllamaClient(config.OllamaBaseURL, config.Model, timeout),
		executor:    executor,
		registry:    registry,
		audit:       auditLogger,
		config:      config,
		memoryStore: config.MemoryStore,
	}
}

func (l *Loop) Run(ctx context.Context, sessionID string, message string, requestID string) (string, error) {
	maxLoops := l.config.MaxToolLoops
	if maxLoops <= 0 {
		maxLoops = 5
	}

	// Load history from memory store
	var history []memory.Message
	if l.memoryStore != nil {
		history, _ = l.memoryStore.LoadHistory(sessionID)
	}

	messages := []Message{
		{Role: "user", Content: message},
	}

	// Prepend history messages (user/assistant pairs) before the current message
	if len(history) > 0 {
		historyMsgs := make([]Message, 0, len(history))
		for _, h := range history {
			historyMsgs = append(historyMsgs, Message{Role: h.Role, Content: h.Content})
		}
		// Insert history before current user message
		messages = append(historyMsgs, messages...)
	}

	if l.config.SystemPrompt != "" {
		systemContent := l.config.SystemPrompt
		if l.config.RuntimeCtx != nil {
			systemContent = systemContent + "\n\n" + l.config.RuntimeCtx.SystemInjection()
		}
		messages = append([]Message{{Role: "system", Content: systemContent}}, messages...)
	}

	toolDefs := l.registry.Definitions()

	for loop := 0; loop < maxLoops; loop++ {
		chatMsgs := make([]chatMessage, len(messages))
		for i, m := range messages {
			chatMsgs[i] = messageToChat(m)
		}

		resp, err := l.client.Chat(ctx, chatMsgs, toolDefs)
		if err != nil {
			return "", fmt.Errorf("agent loop iteration %d: %w", loop, err)
		}

		msg := chatToMessage(*resp)

		if len(msg.ToolCalls) == 0 {
			if msg.Content != "" {
				// Save to memory
				if l.memoryStore != nil {
					l.memoryStore.SaveTurn(sessionID, message, msg.Content)
				}
				return msg.Content, nil
			}
			return "", fmt.Errorf("agent: empty response from model")
		}

		messages = append(messages, msg)

		for _, tc := range msg.ToolCalls {
			start := time.Now()

			result := l.executor.Execute(tc)

			latency := time.Since(start)
			l.audit.LogToolCall(requestID, tc.Name, tc.ID, result.Error == "" || result.Content != "", result.Error, nil, latency)

			content := result.Content
			if result.Error != "" {
				content = result.Error
			}
			messages = append(messages, Message{
				Role:     "tool",
				ToolName: tc.Name,
				Content:  content,
			})
		}
	}

	return "", fmt.Errorf("agent: exceeded max tool call loops (%d)", maxLoops)
}
