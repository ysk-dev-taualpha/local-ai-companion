package audit

import (
	"fmt"
	"time"
)

type Entry struct {
	Timestamp    time.Time     `json:"timestamp"`
	RequestID    string        `json:"request_id"`
	ToolName     string        `json:"tool_name"`
	ToolCallID   string        `json:"tool_call_id"`
	PolicyResult string        `json:"policy_result"`
	ExecResult   string        `json:"exec_result"`
	Latency      time.Duration `json:"latency_ns"`
}

type Logger struct {
	entries []Entry
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Log(e Entry) {
	l.entries = append(l.entries, e)
}

func (l *Logger) Entries() []Entry {
	result := make([]Entry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *Logger) LogToolCall(requestID, toolName, toolCallID string, policyAllowed bool, policyReason string, execErr error, latency time.Duration) {
	policyResult := "allowed"
	if !policyAllowed {
		if policyReason != "" {
			policyResult = fmt.Sprintf("denied: %s", policyReason)
		} else {
			policyResult = "denied"
		}
	}

	execResult := "success"
	if execErr != nil {
		execResult = fmt.Sprintf("error: %s", execErr.Error())
	} else if !policyAllowed {
		execResult = "skipped"
	}

	l.Log(Entry{
		Timestamp:    time.Now(),
		RequestID:    requestID,
		ToolName:     toolName,
		ToolCallID:   toolCallID,
		PolicyResult: policyResult,
		ExecResult:   execResult,
		Latency:      latency,
	})
}
