package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOllamaClient(baseURL, model string, timeout time.Duration) *OllamaClient {
	return &OllamaClient{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type chatRequest struct {
	Model    string            `json:"model"`
	Messages []chatMessage     `json:"messages"`
	Tools    []tool.Definition `json:"tools,omitempty"`
	Stream   bool              `json:"stream"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolCallFunc `json:"function"`
}

type toolCallFunc struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type chatResponse struct {
	Model   string      `json:"model"`
	Message chatMessage `json:"message"`
	Done    bool        `json:"done"`
}

func (c *OllamaClient) Chat(ctx context.Context, messages []chatMessage, tools []tool.Definition) (*chatMessage, error) {
	reqBody := chatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	endpoint := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("ollama: parse response: %w", err)
	}

	return &chatResp.Message, nil
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []tool.Call
	ToolCallID string
}

func messageToChat(m Message) chatMessage {
	cm := chatMessage{
		Role:    m.Role,
		Content: m.Content,
	}
	if len(m.ToolCalls) > 0 {
		cm.ToolCalls = make([]toolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			cm.ToolCalls[i] = toolCall{
				ID:   tc.ID,
				Type: "function",
				Function: toolCallFunc{
					Name:      tc.Name,
					Arguments: tc.Args,
				},
			}
		}
	}
	if m.ToolCallID != "" {
		cm.ToolCallID = m.ToolCallID
	}
	return cm
}

func chatToMessage(cm chatMessage) Message {
	m := Message{
		Role:       cm.Role,
		Content:    cm.Content,
		ToolCallID: cm.ToolCallID,
	}
	if len(cm.ToolCalls) > 0 {
		m.ToolCalls = make([]tool.Call, len(cm.ToolCalls))
		for i, tc := range cm.ToolCalls {
			m.ToolCalls[i] = tool.Call{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: tc.Function.Arguments,
			}
		}
	}
	return m
}
