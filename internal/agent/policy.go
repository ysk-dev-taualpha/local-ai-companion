package agent

import (
	"fmt"
	"sync"
)

// PolicyDecision represents the result of a tool access policy check.
type PolicyDecision string

const (
	PolicyAllowed   PolicyDecision = "ALLOWED"
	PolicyDenied    PolicyDecision = "DENIED"
	PolicyMalformed PolicyDecision = "MALFORMED"
)

// ToolPolicy enforces access control for tool execution.
// The default implementation uses an allowlist.
type ToolPolicy struct {
	mu        sync.RWMutex
	allowlist map[string]bool
}

// NewToolPolicy creates a ToolPolicy that allows only the given tool names.
// If no names are provided, all registered tools are allowed by default.
func NewToolPolicy(allowlist []string) *ToolPolicy {
	p := &ToolPolicy{allowlist: make(map[string]bool)}
	for _, name := range allowlist {
		p.allowlist[name] = true
	}
	return p
}

// Check returns PolicyAllowed if the tool is in the allowlist.
// If the allowlist is empty (default-allow), all tools are permitted.
// Returns PolicyDenied if the tool is not in the allowlist.
func (p *ToolPolicy) Check(toolName string) PolicyDecision {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.allowlist) == 0 {
		return PolicyAllowed
	}
	if p.allowlist[toolName] {
		return PolicyAllowed
	}
	return PolicyDenied
}

// Allow adds a tool name to the allowlist.
func (p *ToolPolicy) Allow(toolName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allowlist[toolName] = true
}

// Deny removes a tool name from the allowlist.
// If the allowlist is empty, this is a no-op (all tools are already allowed).
func (p *ToolPolicy) Deny(toolName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.allowlist, toolName)
}

// DeniedError is returned when a tool execution is denied by policy.
type DeniedError struct {
	ToolName string
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("tool %q execution denied by policy", e.ToolName)
}
