// Package agent provides the agent tool calling infrastructure for Go Runtime.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Tool is a callable tool registered with the agent loop.
type Tool interface {
	// Name returns the unique tool identifier (e.g. "web_search").
	Name() string
	// Description returns a human-readable description for the LLM.
	Description() string
	// Parameters returns the JSON Schema for the tool's input parameters.
	Parameters() json.RawMessage
	// Execute runs the tool with the given arguments and returns the result as a JSON string.
	// The context carries the request timeout/cancellation from the AgentLoop.
	Execute(ctx context.Context, arguments json.RawMessage) (string, error)
}

// OllamaToolSchema returns the Ollama-compatible tool schema for a given tool.
func OllamaToolSchema(t Tool) map[string]any {
	params := map[string]any{}
	json.Unmarshal(t.Parameters(), &params)

	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters":  params,
		},
	}
}

// ToolRegistry manages a collection of registered tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry creates an empty ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. Returns an error if a tool
// with the same name is already registered.
func (r *ToolRegistry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q is already registered", name)
	}
	r.tools[name] = t
	return nil
}

// Unregister removes a tool from the registry by name.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get returns a tool by name, or nil if not found.
func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// All returns a snapshot of all registered tools.
func (r *ToolRegistry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// Schemas returns the Ollama-compatible tool schemas for all registered tools.
func (r *ToolRegistry) Schemas() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemas := make([]map[string]any, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, OllamaToolSchema(t))
	}
	return schemas
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
