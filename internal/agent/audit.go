package agent

import (
	"sync"
	"time"
)

// AuditEntry records the details of a single tool execution.
type AuditEntry struct {
	RequestID  string        `json:"request_id,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolName   string        `json:"tool_name"`
	Policy     PolicyDecision `json:"policy"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// AuditLog stores audit entries for tool executions.
// It is goroutine-safe.
type AuditLog struct {
	mu       sync.RWMutex
	entries  []AuditEntry
	maxSize  int
}

// NewAuditLog creates a new AuditLog.
// maxSize limits the number of entries kept (0 = unlimited).
func NewAuditLog(maxSize int) *AuditLog {
	return &AuditLog{
		entries: make([]AuditEntry, 0),
		maxSize: maxSize,
	}
}

// Log appends an audit entry.
func (a *AuditLog) Log(entry AuditEntry) {
	entry.Timestamp = time.Now()

	a.mu.Lock()
	defer a.mu.Unlock()

	a.entries = append(a.entries, entry)
	if a.maxSize > 0 && len(a.entries) > a.maxSize {
		a.entries = a.entries[len(a.entries)-a.maxSize:]
	}
}

// Entries returns a snapshot of all audit entries.
func (a *AuditLog) Entries() []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]AuditEntry, len(a.entries))
	copy(result, a.entries)
	return result
}

// Count returns the number of audit entries.
func (a *AuditLog) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

// Clear removes all audit entries.
func (a *AuditLog) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = make([]AuditEntry, 0)
}
