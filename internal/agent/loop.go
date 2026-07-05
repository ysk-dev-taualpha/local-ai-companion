package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AgentLoop orchestrates multi-turn LLM conversations with tool calling.
//
// Flow:
//  1. Send user message + system prompt + tools to Ollama
//  2. If the LLM returns tool_calls, execute them via ToolExecutor
//  3. Inject tool results (role: "tool") and loop
//  4. When the LLM returns a text response (no tool_calls), return it
//  5. If max iterations exceeded, return an error
type AgentLoop struct {
	ollamaClient *OllamaClient
	executor     *ToolExecutor
	maxIter      int
	systemPrompt string
	requestID    string
}

// NewAgentLoop creates a new AgentLoop.
// maxIter limits the number of LLM+tool call rounds (1 = single LLM call, no tool loop).
func NewAgentLoop(
	ollamaClient *OllamaClient,
	executor *ToolExecutor,
	maxIter int,
	systemPrompt string,
	requestID string,
) *AgentLoop {
	if maxIter <= 0 {
		maxIter = 5
	}
	return &AgentLoop{
		ollamaClient: ollamaClient,
		executor:     executor,
		maxIter:      maxIter,
		systemPrompt: systemPrompt,
		requestID:    requestID,
	}
}

// AgentResponse is the result of an AgentLoop run.
type AgentResponse struct {
	Text     string `json:"text"`
	Done     bool   `json:"done"`
	Loops    int    `json:"loops"`
}

// Run executes the agent loop. It returns the final text response.
// ctx carries the request timeout and cancellation from the caller.
func (a *AgentLoop) Run(ctx context.Context, userMessage string) (*AgentResponse, error) {
	messages := a.buildInitialMessages(userMessage)

	for i := 0; i < a.maxIter; i++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("agent loop cancelled at iteration %d: %w", i+1, ctx.Err())
		default:
		}

		resp, err := a.ollamaClient.Chat(ctx, messages, a.executor.registry.Schemas())
		if err != nil {
			return nil, fmt.Errorf("ollama call at iteration %d: %w", i+1, err)
		}

		// Append assistant message
		messages = append(messages, *resp)

		// If the LLM returned text content (no tool calls), we're done
		if len(resp.ToolCalls) == 0 {
			return &AgentResponse{
				Text:  resp.Content,
				Done:  true,
				Loops: i + 1,
			}, nil
		}

		// Execute tool calls
		toolMessages := make([]ChatMessage, len(resp.ToolCalls))
		for j, tc := range resp.ToolCalls {
			result := a.executor.Execute(ctx, tc)
			toolMessages[j] = a.buildToolResultMessage(tc, result)
		}
		messages = append(messages, toolMessages...)
	}

	return nil, fmt.Errorf("agent loop: max iterations (%d) exceeded", a.maxIter)
}

func (a *AgentLoop) buildInitialMessages(userMessage string) []ChatMessage {
	msgs := []ChatMessage{}
	if a.systemPrompt != "" {
		msgs = append(msgs, ChatMessage{
			Role:    "system",
			Content: a.systemPrompt,
		})
	}
	msgs = append(msgs, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})
	return msgs
}

func (a *AgentLoop) buildToolResultMessage(tc ToolCall, result *ToolCallResult) ChatMessage {
	content := result.Result
	if result.Error != "" {
		errPayload, _ := json.Marshal(map[string]string{"error": result.Error})
		content = string(errPayload)
	}
	return ChatMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: tc.ID,
	}
}

// Config holds agent configuration.
type Config struct {
	OllamaURL     string        `json:"ollama_url"`
	OllamaModel   string        `json:"ollama_model"`
	OllamaTimeout time.Duration `json:"ollama_timeout"`
	MaxIter       int           `json:"max_iter"`
	SystemPrompt  string        `json:"system_prompt"`
	AllowedTools  []string      `json:"allowed_tools"`
	AuditSize     int           `json:"audit_size"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		OllamaURL:     "http://127.0.0.1:11434",
		OllamaModel:   "g4v100",
		OllamaTimeout: 60 * time.Second,
		MaxIter:       5,
		AllowedTools:  []string{}, // empty = allow all
		AuditSize:     1000,
	}
}

// NewFromConfig creates an AgentLoop from config.
func NewFromConfig(cfg Config, requestID string) *AgentLoop {
	audit := NewAuditLog(cfg.AuditSize)
	registry := NewToolRegistry()
	policy := NewToolPolicy(cfg.AllowedTools)

	ollamaClient := NewOllamaClient(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaTimeout)
	executor := NewToolExecutor(registry, policy, audit)

	return NewAgentLoop(ollamaClient, executor, cfg.MaxIter, cfg.SystemPrompt, requestID)
}
