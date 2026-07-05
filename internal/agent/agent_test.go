package agent

import (
	"context"
	"encoding/json"
	"testing"
)

// testTool is a minimal Tool implementation for testing.
type testTool struct {
	name        string
	description string
	params      json.RawMessage
	executeFn   func(ctx context.Context, args json.RawMessage) (string, error)
}

func (t *testTool) Name() string                                    { return t.name }
func (t *testTool) Description() string                             { return t.description }
func (t *testTool) Parameters() json.RawMessage                     { return t.params }
func (t *testTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.executeFn(ctx, args)
}

func echoTool() *testTool {
	return &testTool{
		name:        "echo",
		description: "Echoes back the input",
		params:      json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
		executeFn: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}
}

func errorTool() *testTool {
	return &testTool{
		name:        "error_tool",
		description: "Always errors",
		params:      json.RawMessage(`{"type":"object","properties":{}}`),
		executeFn: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", &testError{msg: "intentional error"}
		},
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// --- ToolRegistry tests ---

func TestToolRegistry_Register(t *testing.T) {
	r := NewToolRegistry()
	tool := echoTool()

	if err := r.Register(tool); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Count() != 1 {
		t.Fatalf("expected 1 tool, got %d", r.Count())
	}

	// Duplicate registration should fail
	if err := r.Register(tool); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestToolRegistry_Get(t *testing.T) {
	r := NewToolRegistry()
	tool := echoTool()
	r.Register(tool)

	got := r.Get("echo")
	if got == nil {
		t.Fatal("expected to find tool")
	}
	if got.Name() != "echo" {
		t.Fatalf("expected name echo, got %s", got.Name())
	}

	if r.Get("nonexistent") != nil {
		t.Fatal("expected nil for nonexistent tool")
	}
}

func TestToolRegistry_Unregister(t *testing.T) {
	r := NewToolRegistry()
	tool := echoTool()
	r.Register(tool)
	r.Unregister("echo")

	if r.Count() != 0 {
		t.Fatalf("expected 0 tools after unregister, got %d", r.Count())
	}
}

func TestToolRegistry_Schemas(t *testing.T) {
	r := NewToolRegistry()
	r.Register(echoTool())

	schemas := r.Schemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}

	schema := schemas[0]
	if schema["type"] != "function" {
		t.Fatal("expected type 'function'")
	}
}

// --- ToolPolicy tests ---

func TestToolPolicy_Allowlist(t *testing.T) {
	p := NewToolPolicy([]string{"echo"})

	if p.Check("echo") != PolicyAllowed {
		t.Fatal("expected echo to be ALLOWED")
	}
	if p.Check("web_search") != PolicyDenied {
		t.Fatal("expected web_search to be DENIED")
	}
}

func TestToolPolicy_EmptyAllowlist(t *testing.T) {
	p := NewToolPolicy([]string{})

	if p.Check("echo") != PolicyAllowed {
		t.Fatal("expected echo to be ALLOWED (empty allowlist)")
	}
	if p.Check("web_search") != PolicyAllowed {
		t.Fatal("expected web_search to be ALLOWED (empty allowlist)")
	}
}

func TestToolPolicy_AllowDeny(t *testing.T) {
	p := NewToolPolicy([]string{"echo"})
	if p.Check("web_search") != PolicyDenied {
		t.Fatal("expected web_search to be DENIED initially")
	}

	p.Allow("web_search")
	if p.Check("web_search") != PolicyAllowed {
		t.Fatal("expected web_search to be ALLOWED after Allow")
	}

	p.Deny("web_search")
	if p.Check("web_search") != PolicyDenied {
		t.Fatal("expected web_search to be DENIED after Deny")
	}
}

// --- ToolExecutor tests ---

func TestToolExecutor_ExecuteAllowed(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(echoTool())
	policy := NewToolPolicy([]string{})
	audit := NewAuditLog(100)
	executor := NewToolExecutor(registry, policy, audit)

	ctx := context.Background()
	tc := ToolCall{
		ID: "call_1",
		Function: ToolCallFunction{
			Name:      "echo",
			Arguments: json.RawMessage(`{"text":"hello"}`),
		},
	}

	result := executor.Execute(ctx, tc)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Policy != PolicyAllowed {
		t.Fatalf("expected ALLOWED, got %s", result.Policy)
	}
	if result.Result != `{"text":"hello"}` {
		t.Fatalf("unexpected result: %s", result.Result)
	}
}

func TestToolExecutor_PolicyDenied(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(echoTool())
	policy := NewToolPolicy([]string{"web_search"}) // echo not in allowlist
	audit := NewAuditLog(100)
	executor := NewToolExecutor(registry, policy, audit)

	ctx := context.Background()
	tc := ToolCall{
		ID: "call_1",
		Function: ToolCallFunction{
			Name:      "echo",
			Arguments: json.RawMessage(`{"text":"hello"}`),
		},
	}

	result := executor.Execute(ctx, tc)
	if result.Policy != PolicyDenied {
		t.Fatalf("expected DENIED, got %s", result.Policy)
	}
	if result.Error == "" {
		t.Fatal("expected error for denied tool")
	}
}

func TestToolExecutor_NotRegistered(t *testing.T) {
	registry := NewToolRegistry()
	policy := NewToolPolicy([]string{})
	audit := NewAuditLog(100)
	executor := NewToolExecutor(registry, policy, audit)

	ctx := context.Background()
	tc := ToolCall{
		ID: "call_1",
		Function: ToolCallFunction{
			Name:      "nonexistent",
			Arguments: json.RawMessage(`{}`),
		},
	}

	result := executor.Execute(ctx, tc)
	if result.Error == "" {
		t.Fatal("expected error for unregistered tool")
	}
}

func TestToolExecutor_AuditLog(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(echoTool())
	registry.Register(errorTool())
	policy := NewToolPolicy([]string{})
	audit := NewAuditLog(100)
	executor := NewToolExecutor(registry, policy, audit)

	ctx := context.Background()

	// Execute echo tool
	executor.Execute(ctx, ToolCall{
		ID: "call_1",
		Function: ToolCallFunction{
			Name:      "echo",
			Arguments: json.RawMessage(`{"text":"hello"}`),
		},
	})

	// Execute error tool
	executor.Execute(ctx, ToolCall{
		ID: "call_2",
		Function: ToolCallFunction{
			Name:      "error_tool",
			Arguments: json.RawMessage(`{}`),
		},
	})

	entries := audit.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}

	// First entry: echo (success)
	if entries[0].ToolName != "echo" {
		t.Fatalf("expected tool_name echo, got %s", entries[0].ToolName)
	}
	if entries[0].Error != "" {
		t.Fatalf("expected no error, got: %s", entries[0].Error)
	}
	if entries[0].Policy != PolicyAllowed {
		t.Fatalf("expected ALLOWED, got %s", entries[0].Policy)
	}
	if entries[0].Duration <= 0 {
		t.Fatal("expected non-zero duration")
	}

	// Second entry: error_tool
	if entries[1].ToolName != "error_tool" {
		t.Fatalf("expected tool_name error_tool, got %s", entries[1].ToolName)
	}
	if entries[1].Error == "" {
		t.Fatal("expected error for error_tool")
	}
}

func TestToolExecutor_ContextCancel(t *testing.T) {
	registry := NewToolRegistry()
	tt := &testTool{
		name:        "slow",
		description: "slow tool",
		params:      json.RawMessage(`{"type":"object","properties":{}}`),
		executeFn: func(ctx context.Context, args json.RawMessage) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-make(chan struct{}): // never completes
				return "", nil
			}
		},
	}
	registry.Register(tt)
	policy := NewToolPolicy([]string{})
	audit := NewAuditLog(100)
	executor := NewToolExecutor(registry, policy, audit)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result := executor.Execute(ctx, ToolCall{
		ID: "call_1",
		Function: ToolCallFunction{
			Name:      "slow",
			Arguments: json.RawMessage(`{}`),
		},
	})

	if result.Error == "" {
		t.Fatal("expected error from cancelled context")
	}
}

// --- AuditLog tests ---

func TestAuditLog_MaxSize(t *testing.T) {
	a := NewAuditLog(3)

	for i := 0; i < 5; i++ {
		a.Log(AuditEntry{ToolName: "echo"})
	}

	if a.Count() != 3 {
		t.Fatalf("expected 3 entries (max), got %d", a.Count())
	}
}

func TestAuditLog_Clear(t *testing.T) {
	a := NewAuditLog(100)
	a.Log(AuditEntry{ToolName: "echo"})
	a.Log(AuditEntry{ToolName: "web_search"})

	if a.Count() != 2 {
		t.Fatalf("expected 2 entries, got %d", a.Count())
	}

	a.Clear()
	if a.Count() != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", a.Count())
	}
}

func TestAuditLog_Unlimited(t *testing.T) {
	a := NewAuditLog(0) // unlimited
	for i := 0; i < 100; i++ {
		a.Log(AuditEntry{ToolName: "echo"})
	}
	if a.Count() != 100 {
		t.Fatalf("expected 100 entries (unlimited), got %d", a.Count())
	}
}

// --- Policy constants tests ---

func TestDeniedError(t *testing.T) {
	err := &DeniedError{ToolName: "web_search"}
	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
}
