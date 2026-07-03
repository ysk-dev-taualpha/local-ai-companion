package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("http://localhost:8090")
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.baseURL != "http://localhost:8090" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://localhost:8090")
	}
	if c.client == nil {
		t.Fatal("client is nil")
	}
}

func TestSend_Success(t *testing.T) {
	expectedResp := ConversationResponse{
		RequestID:      "req-1",
		ConversationID: "conv-1",
		Assistant: AssistantMessage{
			Text:          "hello",
			Emotion:       "happy",
			Motion:        "wave",
			SpeakStyle:    "normal",
			Interruptible: true,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/conversation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	}))
	defer server.Close()

	c := New(server.URL)
	req := ConversationRequest{Message: "hi", ConversationID: "conv-1", RequestID: "req-1"}
	resp, err := c.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp.RequestID != expectedResp.RequestID {
		t.Errorf("RequestID = %q, want %q", resp.RequestID, expectedResp.RequestID)
	}
	if resp.ConversationID != expectedResp.ConversationID {
		t.Errorf("ConversationID = %q, want %q", resp.ConversationID, expectedResp.ConversationID)
	}
	if resp.Assistant.Text != expectedResp.Assistant.Text {
		t.Errorf("Assistant.Text = %q, want %q", resp.Assistant.Text, expectedResp.Assistant.Text)
	}
	if resp.Assistant.Emotion != expectedResp.Assistant.Emotion {
		t.Errorf("Assistant.Emotion = %q, want %q", resp.Assistant.Emotion, expectedResp.Assistant.Emotion)
	}
	if resp.Assistant.Interruptible != expectedResp.Assistant.Interruptible {
		t.Errorf("Assistant.Interruptible = %v, want %v", resp.Assistant.Interruptible, expectedResp.Assistant.Interruptible)
	}
}

func TestSend_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := New(server.URL)
	req := ConversationRequest{Message: "hi"}
	_, err := c.Send(context.Background(), req)
	if err == nil {
		t.Fatal("Send() should return error for 500 status")
	}
	if !strings.Contains(err.Error(), "python service error") {
		t.Errorf("error = %q, expected 'python service error'", err.Error())
	}
}

func TestSend_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	c := New(server.URL)
	req := ConversationRequest{Message: "hi"}
	_, err := c.Send(context.Background(), req)
	if err == nil {
		t.Fatal("Send() should return error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid response") {
		t.Errorf("error = %q, expected 'invalid response'", err.Error())
	}
}

func TestSend_ConnectionError(t *testing.T) {
	c := New("http://127.0.0.1:1") // unreachable port
	req := ConversationRequest{Message: "hi"}
	_, err := c.Send(context.Background(), req)
	if err == nil {
		t.Fatal("Send() should return error for connection failure")
	}
	if !strings.Contains(err.Error(), "python service unavailable") {
		t.Errorf("error = %q, expected 'python service unavailable'", err.Error())
	}
}

func TestSend_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// block until context canceled
		<-r.Context().Done()
	}))
	defer server.Close()

	c := New(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	req := ConversationRequest{Message: "hi"}
	_, err := c.Send(ctx, req)
	if err == nil {
		t.Fatal("Send() should return error for canceled context")
	}
}

func TestSend_Non200Status(t *testing.T) {
	tests := []struct {
		status int
	}{
		{http.StatusBadRequest},
		{http.StatusNotFound},
		{http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
		}))
		c := New(server.URL)
		req := ConversationRequest{Message: "hi"}
		_, err := c.Send(context.Background(), req)
		if err == nil {
			t.Errorf("status %d: Send() should return error", tt.status)
		}
		server.Close()
	}
}

func TestSend_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	c := New(server.URL)
	req := ConversationRequest{Message: "hi"}
	resp, err := c.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp.RequestID != "" {
		t.Errorf("RequestID = %q, want empty", resp.RequestID)
	}
}
