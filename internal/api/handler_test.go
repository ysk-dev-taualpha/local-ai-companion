package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
)

type mockPythonClient struct {
	resp *client.ConversationResponse
	err  error
}

func (m *mockPythonClient) Send(ctx context.Context, req client.ConversationRequest) (*client.ConversationResponse, error) {
	return m.resp, m.err
}

func TestHandleConversationSuccess(t *testing.T) {
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID:      "req-1",
			ConversationID: "conv-1",
			Assistant: client.AssistantMessage{
				Text: "hello", Emotion: "neutral", Motion: "idle",
				SpeakStyle: "normal", Interruptible: true,
			},
		},
	}
	h := New(mock, 30000)

	body := `{"message":"hi"}`
	req := httptest.NewRequest("POST", "/v1/conversation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h.HandleConversation(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleConversationMissingMessage(t *testing.T) {
	h := New(&mockPythonClient{}, 30000)
	req := httptest.NewRequest("POST", "/v1/conversation", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	h.HandleConversation(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleConversationMethodNotAllowed(t *testing.T) {
	h := New(&mockPythonClient{}, 30000)
	req := httptest.NewRequest("GET", "/v1/conversation", nil)
	w := httptest.NewRecorder()
	h.HandleConversation(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHealthCheck(t *testing.T) {
	h := New(&mockPythonClient{}, 30000)
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.HandleHealth(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected ok, got %s", resp["status"])
	}
}
