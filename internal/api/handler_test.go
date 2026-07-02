package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/client"
)

type mockPythonClient struct {
	resp    *client.ConversationResponse
	err     error
	lastReq client.ConversationRequest
}

func (m *mockPythonClient) Send(req client.ConversationRequest) (*client.ConversationResponse, error) {
	m.lastReq = req
	return m.resp, m.err
}

func TestHandleConversationSuccess(t *testing.T) {
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID:      "req-1",
			ConversationID: "conv-1",
			Assistant: client.AssistantMessage{
				Text:          "hello",
				Emotion:       "neutral",
				Motion:        "idle",
				SpeakStyle:    "normal",
				Interruptible: true,
			},
		},
	}
	h := New(mock)

	body := `{"message":"hi","request_id":"req-1","conversation_id":"conv-1"}`
	req := httptest.NewRequest("POST", "/v1/conversation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	h.HandleConversation(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp client.ConversationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.RequestID != "req-1" {
		t.Errorf("expected req-1, got %s", resp.RequestID)
	}
}

func TestHandleConversationGeneratesRequestID(t *testing.T) {
	mock := &mockPythonClient{
		resp: &client.ConversationResponse{
			RequestID:      "generated-by-python",
			ConversationID: "default",
			Assistant: client.AssistantMessage{
				Text:          "hello",
				Emotion:       "neutral",
				Motion:        "idle",
				SpeakStyle:    "normal",
				Interruptible: true,
			},
		},
	}
	h := New(mock)

	body := `{"message":"hi"}`
	req := httptest.NewRequest("POST", "/v1/conversation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	h.HandleConversation(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.lastReq.RequestID == "" {
		t.Fatal("expected generated request_id")
	}
	if mock.lastReq.Message != "hi" {
		t.Errorf("expected message to be forwarded, got %s", mock.lastReq.Message)
	}
}

func TestHandleConversationMissingMessage(t *testing.T) {
	h := New(&mockPythonClient{})

	body := `{"conversation_id":"c1"}`
	req := httptest.NewRequest("POST", "/v1/conversation", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	h.HandleConversation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleConversationInvalidBody(t *testing.T) {
	h := New(&mockPythonClient{})

	req := httptest.NewRequest("POST", "/v1/conversation", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	h.HandleConversation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleConversationMethodNotAllowed(t *testing.T) {
	h := New(&mockPythonClient{})

	req := httptest.NewRequest("GET", "/v1/conversation", nil)
	w := httptest.NewRecorder()

	h.HandleConversation(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
