package audit

import (
	"errors"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	l := NewLogger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if entries := l.Entries(); len(entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(entries))
	}
}

func TestLog(t *testing.T) {
	l := NewLogger()
	l.Log(Entry{
		Timestamp:    time.Now(),
		RequestID:    "req-1",
		ToolName:     "web_search",
		ToolCallID:   "call-1",
		PolicyResult: "allowed",
		ExecResult:   "success",
		Latency:      100 * time.Millisecond,
	})

	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.RequestID != "req-1" {
		t.Errorf("expected req-1, got %s", e.RequestID)
	}
	if e.ToolName != "web_search" {
		t.Errorf("expected web_search, got %s", e.ToolName)
	}
	if e.PolicyResult != "allowed" {
		t.Errorf("expected allowed, got %s", e.PolicyResult)
	}
	if e.ExecResult != "success" {
		t.Errorf("expected success, got %s", e.ExecResult)
	}
	if e.Latency != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", e.Latency)
	}
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestLogToolCall_Success(t *testing.T) {
	l := NewLogger()
	l.LogToolCall("req-1", "web_search", "call-1", true, "", nil, 50*time.Millisecond)

	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.PolicyResult != "allowed" {
		t.Errorf("expected allowed, got %s", e.PolicyResult)
	}
	if e.ExecResult != "success" {
		t.Errorf("expected success, got %s", e.ExecResult)
	}
}

func TestLogToolCall_Denied(t *testing.T) {
	l := NewLogger()
	l.LogToolCall("req-1", "dangerous", "call-1", false, "not in allowlist", nil, 0)

	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.PolicyResult != "denied: not in allowlist" {
		t.Errorf("expected 'denied: not in allowlist', got %s", e.PolicyResult)
	}
	if e.ExecResult != "skipped" {
		t.Errorf("expected skipped, got %s", e.ExecResult)
	}
}

func TestLogToolCall_ExecError(t *testing.T) {
	l := NewLogger()
	l.LogToolCall("req-1", "web_fetch", "call-1", true, "", errors.New("timeout"), 200*time.Millisecond)

	entries := l.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.PolicyResult != "allowed" {
		t.Errorf("expected allowed, got %s", e.PolicyResult)
	}
	if e.ExecResult != "error: timeout" {
		t.Errorf("expected 'error: timeout', got %s", e.ExecResult)
	}
}
