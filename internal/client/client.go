package client

import (
	"bytes"
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
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) Send(req ConversationRequest) (*ConversationResponse, error) {
	body, _ := json.Marshal(req)
	resp, err := c.httpClient.Post(c.baseURL+"/v1/conversation", "application/json", bytes.NewReader(body))
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
