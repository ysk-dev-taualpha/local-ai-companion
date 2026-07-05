package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ToolExecutor dispatches tool calls to registered tools with policy enforcement.
type ToolExecutor struct {
	registry *ToolRegistry
	policy   *ToolPolicy
	audit    *AuditLog
}

// NewToolExecutor creates a new ToolExecutor.
func NewToolExecutor(registry *ToolRegistry, policy *ToolPolicy, audit *AuditLog) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		policy:   policy,
		audit:    audit,
	}
}

// ToolCallResult represents the outcome of a single tool execution.
type ToolCallResult struct {
	ToolCallID string        `json:"-"`
	ToolName   string        `json:"-"`
	Result     string        `json:"result"`
	Error      string        `json:"error,omitempty"`
	Policy     PolicyDecision `json:"policy"`
	Duration   time.Duration `json:"duration_ms"`
}

// Execute runs a single tool call and returns the result.
// The tool call comes from an Ollama tool_calls response.
func (e *ToolExecutor) Execute(ctx context.Context, toolCall ToolCall) *ToolCallResult {
	start := time.Now()

	result := &ToolCallResult{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Function.Name,
	}

	defer func() {
		result.Duration = time.Since(start)
		if e.audit != nil {
			e.audit.Log(AuditEntry{
				ToolCallID: result.ToolCallID,
				ToolName:   result.ToolName,
				Policy:     result.Policy,
				Duration:   result.Duration,
				Error:      result.Error,
			})
		}
	}()

	// 1. Policy check
	result.Policy = e.policy.Check(toolCall.Function.Name)
	if result.Policy != PolicyAllowed {
		result.Error = fmt.Sprintf("tool %q execution denied by policy", toolCall.Function.Name)
		return result
	}

	// 2. Look up tool
	t := e.registry.Get(toolCall.Function.Name)
	if t == nil {
		result.Error = fmt.Sprintf("tool %q is not registered", toolCall.Function.Name)
		return result
	}

	// 3. Marshal arguments
	argsJSON, err := json.Marshal(toolCall.Function.Arguments)
	if err != nil {
		result.Policy = PolicyMalformed
		result.Error = fmt.Sprintf("failed to marshal arguments for %q: %v", toolCall.Function.Name, err)
		return result
	}

	// 4. Execute with context propagation
	output, err := t.Execute(ctx, argsJSON)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Result = output
	return result
}

// ExecutorResult is a collection of results from executing multiple tool calls.
type ExecutorResult struct {
	Results []*ToolCallResult
	Errors  int
}
