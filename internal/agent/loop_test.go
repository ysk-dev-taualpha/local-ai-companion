package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/audit"
	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

// newMockOllamaServer creates an httptest server that returns pre-configured
// JSON responses in sequence. Each call consumes the next response.
func newMockOllamaServer(t *testing.T, responses []string) *httptest.Server {
	t.Helper()
	callCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if callCount >= len(responses) {
			t.Errorf("unexpected call %d, only %d responses configured", callCount, len(responses))
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(responses[callCount]))
		callCount++
	}))
}

// newTestLoop creates a Loop with web_search and web_fetch tools registered
// and pointed at the given base URL. maxLoops controls MaxToolLoops in Config.
func newTestLoop(baseURL string, maxLoops int) *Loop {
	registry := tool.NewRegistry()

	registry.Register(tool.Definition{
		Name:        "web_search",
		Description: "Search the web",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"query": {Type: "string", Description: "Search query"},
			},
			Required: []string{"query"},
		},
	}, tool.ExecutorFunc(func(args json.RawMessage) (string, error) {
		var params struct{ Query string `json:"query"` }
		json.Unmarshal(args, &params)
		return fmt.Sprintf("search results for: %s", params.Query), nil
	}))

	registry.Register(tool.Definition{
		Name:        "web_fetch",
		Description: "Fetch a web page",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"url": {Type: "string", Description: "URL to fetch"},
			},
			Required: []string{"url"},
		},
	}, tool.ExecutorFunc(func(args json.RawMessage) (string, error) {
		var params struct{ URL string `json:"url"` }
		json.Unmarshal(args, &params)
		return fmt.Sprintf("fetched content from: %s", params.URL), nil
	}))

	policy := tool.NewPolicy()
	policy.Allow("web_search")
	policy.Allow("web_fetch")
	executor := tool.NewExecutor(registry, policy)

	if maxLoops <= 0 {
		maxLoops = 3
	}

	return NewLoop(registry, executor, audit.NewLogger(), Config{
		MaxToolLoops:  maxLoops,
		OllamaBaseURL: baseURL,
		Model:         "test-model",
		OllamaTimeout: 5 * time.Second,
	})
}

// TestLoop_NormalResponse verifies that a simple text response without any
// tool calls is returned directly.
func TestLoop_NormalResponse(t *testing.T) {
	responses := []string{
		`{"model":"test-model","message":{"role":"assistant","content":"Hello! How can I help?"},"done":true}`,
	}
	srv := newMockOllamaServer(t, responses)
	defer srv.Close()

	loop := newTestLoop(srv.URL, 3)
	result, err := loop.Run(t.Context(), "sess-1", "hi", "req-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello! How can I help?" {
		t.Errorf("result = %q, want %q", result, "Hello! How can I help?")
	}
}

// TestLoop_SingleToolCall verifies the loop handles a single tool call:
// the model requests web_search, the tool executes, and the model's second
// response (with tool result context) is returned as the final text.
func TestLoop_SingleToolCall(t *testing.T) {
	responses := []string{
		// First response: model requests web_search tool
		`{"model":"test-model","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"golang testing\"}"}}]},"done":false}`,
		// Second response: model returns final answer after receiving tool result
		`{"model":"test-model","message":{"role":"assistant","content":"Go testing is well-documented with the testing package."},"done":true}`,
	}
	srv := newMockOllamaServer(t, responses)
	defer srv.Close()

	loop := newTestLoop(srv.URL, 3)
	result, err := loop.Run(t.Context(), "sess-1", "search for golang testing", "req-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Go testing is well-documented with the testing package." {
		t.Errorf("result = %q, want %q", result, "Go testing is well-documented with the testing package.")
	}
}

// TestLoop_MultipleToolCalls verifies the loop handles multiple tool calls
// in a single response. The model requests both web_search and web_fetch
// simultaneously, both are executed, and the model's final response is returned.
func TestLoop_MultipleToolCalls(t *testing.T) {
	responses := []string{
		// First response: model requests two tools at once
		`{"model":"test-model","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"hermes agent\"}"}},{"id":"call_2","type":"function","function":{"name":"web_fetch","arguments":"{\"url\":\"https://example.com/hermes\"}"}}]},"done":false}`,
		// Second response: model returns final answer
		`{"model":"test-model","message":{"role":"assistant","content":"Hermes Agent is an AI assistant by Nous Research. Fetched page confirms details."},"done":true}`,
	}
	srv := newMockOllamaServer(t, responses)
	defer srv.Close()

	loop := newTestLoop(srv.URL, 3)
	result, err := loop.Run(t.Context(), "sess-1", "tell me about hermes", "req-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hermes Agent is an AI assistant by Nous Research. Fetched page confirms details." {
		t.Errorf("result = %q, want %q", result, "Hermes Agent is an AI assistant by Nous Research. Fetched page confirms details.")
	}
}

// TestLoop_ToolCallMessageFormat verifies that tool call messages use correct
// Ollama native format: arguments as JSON objects, tool_name in tool results,
// and assistant tool_calls preserved in history.
func TestLoop_ToolCallMessageFormat(t *testing.T) {
	// arguments as JSON object (not escaped string) — Ollama native format
	responses := []string{
		`{"model":"test-model","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"web_search","arguments":{"query":"golang testing"}}}]},"done":false}`,
		`{"model":"test-model","message":{"role":"assistant","content":"Go testing is well-documented."},"done":true}`,
	}
	srv := newMockOllamaServer(t, responses)
	defer srv.Close()

	loop := newTestLoop(srv.URL, 3)
	result, err := loop.Run(t.Context(), "sess-1", "search for golang testing", "req-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Go testing is well-documented." {
		t.Errorf("result = %q", result)
	}
}

// TestLoop_ToolCallHistoryPreservation sends a request through the mock server
// and captures the intermediate messages sent to verify tool_name and tool_calls
// are correctly formatted.
func TestLoop_ToolCallHistoryPreservation(t *testing.T) {
	var capturedBodies []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		capturedBodies = append(capturedBodies, string(body[:n]))

		w.Header().Set("Content-Type", "application/json")
		// First call: model requests tool. Second call: final answer.
		if len(capturedBodies) == 1 {
			w.Write([]byte(`{"model":"test-model","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"web_search","arguments":{"query":"test"}}}]},"done":false}`))
		} else {
			w.Write([]byte(`{"model":"test-model","message":{"role":"assistant","content":"final answer"},"done":true}`))
		}
	}))
	defer srv.Close()

	loop := newTestLoop(srv.URL, 3)
	result, err := loop.Run(t.Context(), "sess-x", "search", "req-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "final answer" {
		t.Errorf("result = %q, want %q", result, "final answer")
	}

	// Verify first request: tools should use type/function wrapper
	if len(capturedBodies) < 1 {
		t.Fatal("no requests captured")
	}
	if !strings.Contains(capturedBodies[0], `"type":"function"`) {
		t.Error("first request missing tool type wrapper")
	}

	// Verify second request: should contain tool result with tool_name
	if len(capturedBodies) < 2 {
		t.Fatal("only 1 request captured, expected 2")
	}
	if !strings.Contains(capturedBodies[1], `"tool_name"`) {
		t.Error("second request missing tool_name in tool result")
	}
	if !strings.Contains(capturedBodies[1], `"role":"tool"`) {
		t.Error("second request missing tool role message")
	}
	if !strings.Contains(capturedBodies[1], `"tool_calls"`) {
		t.Error("second request missing assistant tool_calls in history")
	}
}
// keeps requesting tool calls beyond the configured MaxToolLoops limit.
func TestLoop_MaxLoopsExceeded(t *testing.T) {
	// Every response has tool_calls — the model never gives a final answer.
	responses := []string{
		`{"model":"test-model","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"first\"}"}}]},"done":false}`,
		`{"model":"test-model","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_2","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"second\"}"}}]},"done":false}`,
	}
	srv := newMockOllamaServer(t, responses)
	defer srv.Close()

	loop := newTestLoop(srv.URL, 2)
	_, err := loop.Run(t.Context(), "sess-1", "keep searching", "req-1")

	if err == nil {
		t.Fatal("expected error about exceeding max loops, got nil")
	}
	if !strings.Contains(err.Error(), "exceeded max tool call loops") {
		t.Errorf("error = %q, want error containing 'exceeded max tool call loops'", err.Error())
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error should mention max loops limit (2): %q", err.Error())
	}
	t.Logf("expected error: %v", err)
}
