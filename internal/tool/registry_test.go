package tool

import (
	"encoding/json"
	"testing"
)

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	def := Definition{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"arg1": {Type: "string", Description: "test arg"},
			},
			Required: []string{"arg1"},
		},
	}
	err := r.Register(def, ExecutorFunc(func(args json.RawMessage) (string, error) {
		return "ok", nil
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.Has("test_tool") {
		t.Fatal("expected test_tool to be registered")
	}

	impl, ok := r.Lookup("test_tool")
	if !ok {
		t.Fatal("expected to find test_tool")
	}
	result, err := impl.Execute(json.RawMessage(`{"arg1":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	def := Definition{Name: "dup", Description: "test"}
	r.Register(def, ExecutorFunc(func(args json.RawMessage) (string, error) { return "", nil }))
	err := r.Register(def, ExecutorFunc(func(args json.RawMessage) (string, error) { return "", nil }))
	if err == nil {
		t.Fatal("expected error on duplicate register")
	}
}

func TestRegistry_LookupMissing(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Lookup("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRegistry_Definitions(t *testing.T) {
	r := NewRegistry()
	r.Register(Definition{Name: "a"}, ExecutorFunc(func(args json.RawMessage) (string, error) { return "", nil }))
	r.Register(Definition{Name: "b"}, ExecutorFunc(func(args json.RawMessage) (string, error) { return "", nil }))

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}
}

func TestPolicy_Allowlist(t *testing.T) {
	p := NewPolicy()
	p.Allow("safe_tool")

	if p.IsAllowed("safe_tool") != true {
		t.Fatal("expected safe_tool to be allowed")
	}
	if p.IsAllowed("dangerous_tool") != false {
		t.Fatal("expected dangerous_tool to be denied")
	}
}

func TestPolicy_AllowAll(t *testing.T) {
	p := AllowAllPolicy()
	if !p.IsAllowed("any_tool") {
		t.Fatal("expected any_tool to be allowed")
	}
	if !p.IsAllowed("another_tool") {
		t.Fatal("expected another_tool to be allowed")
	}
}

func TestExecutor_PolicyDenial(t *testing.T) {
	r := NewRegistry()
	def := Definition{Name: "blocked", Description: "blocked tool"}
	r.Register(def, ExecutorFunc(func(args json.RawMessage) (string, error) { return "should not run", nil }))

	p := NewPolicy()
	exec := NewExecutor(r, p)

	result := exec.Execute(Call{ID: "call-1", Name: "blocked", Args: nil})
	if result.Error == "" {
		t.Fatal("expected policy denial error")
	}
	if result.Content != "" {
		t.Errorf("expected empty content on denial, got %q", result.Content)
	}
}

func TestExecutor_MalformedArgs(t *testing.T) {
	r := NewRegistry()
	r.Register(Definition{Name: "validates", Description: "validates args"}, ExecutorFunc(func(args json.RawMessage) (string, error) {
		return "ok", nil
	}))

	p := AllowAllPolicy()
	exec := NewExecutor(r, p)

	result := exec.Execute(Call{ID: "call-1", Name: "validates", Args: json.RawMessage(`{invalid}`)})
	if result.Error == "" {
		t.Fatal("expected malformed args error")
	}
}

func TestExecutor_ToolNotFound(t *testing.T) {
	r := NewRegistry()
	p := AllowAllPolicy()
	exec := NewExecutor(r, p)

	result := exec.Execute(Call{ID: "call-1", Name: "nonexistent", Args: nil})
	if result.Error == "" {
		t.Fatal("expected tool not found error")
	}
}
