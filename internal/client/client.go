package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ConversationRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
}

type ConversationResponse struct {
	RequestID      string           `json:"request_id"`
	ConversationID string           `json:"conversation_id"`
	Assistant      AssistantMessage `json:"assistant"`
}

type AssistantMessage struct {
	Text          string `json:"text"`
	Emotion       string `json:"emotion"`
	Motion        string `json:"motion"`
	SpeakStyle    string `json:"speak_style"`
	Interruptible bool   `json:"interruptible"`
}

type Client struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, client: &http.Client{}}
}

func (c *Client) Send(ctx context.Context, req ConversationRequest) (*ConversationResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/conversation", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("python service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("python service error: %s", resp.Status)
	}

	var convResp ConversationResponse
	if err := json.NewDecoder(resp.Body).Decode(&convResp); err != nil {
		return nil, fmt.Errorf("invalid response from python service: %w", err)
	}
	return &convResp, nil
}
