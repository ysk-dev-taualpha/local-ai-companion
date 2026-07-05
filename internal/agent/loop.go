package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/audit"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

type Config struct {
	MaxToolLoops  int
	OllamaBaseURL string
	Model         string
	OllamaTimeout time.Duration
	SystemPrompt  string
	Timezone      string
	Locale        string
}

type Loop struct {
	client   *OllamaClient
	executor *tool.ExecutorService
	registry *tool.Registry
	audit    *audit.Logger
	config   Config
}

func NewLoop(registry *tool.Registry, executor *tool.ExecutorService, auditLogger *audit.Logger, config Config) *Loop {
	timeout := config.OllamaTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Loop{
		client:   NewOllamaClient(config.OllamaBaseURL, config.Model, timeout),
		executor: executor,
		registry: registry,
		audit:    auditLogger,
		config:   config,
	}
}

func (l *Loop) Run(ctx context.Context, message string, requestID string) (string, error) {
	maxLoops := l.config.MaxToolLoops
	if maxLoops <= 0 {
		maxLoops = 5
	}

	messages := []Message{
		{Role: "system", Content: BuildSystemPrompt(l.config.SystemPrompt, l.config.Timezone, l.config.Locale, time.Now())},
		{Role: "user", Content: message},
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
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    content,
			})
		}
	}

	return "", fmt.Errorf("agent: exceeded max tool call loops (%d)", maxLoops)
}
