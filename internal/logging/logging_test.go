package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	l := New("debug")
	if l == nil {
		t.Fatal("New() returned nil")
	}
	if l.level != "debug" {
		t.Errorf("level = %q, want %q", l.level, "debug")
	}

	l2 := New("info")
	if l2.level != "info" {
		t.Errorf("level = %q, want %q", l2.level, "info")
	}

	l3 := New("error")
	if l3.level != "error" {
		t.Errorf("level = %q, want %q", l3.level, "error")
	}
}

func TestInfo_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{info: newInfoLogger(&buf), err: newErrorLogger(&buf), level: "debug"}

	l.Info("test message: %s", "hello")
	output := buf.String()
	if !strings.Contains(output, "[INFO]") || !strings.Contains(output, "test message: hello") {
		t.Errorf("expected '[INFO] ... test message: hello', got %q", output)
	}
}

func TestInfo_InfoLevel(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{info: newInfoLogger(&buf), err: newErrorLogger(&buf), level: "info"}

	l.Info("test message: %s", "world")
	output := buf.String()
	if !strings.Contains(output, "[INFO]") || !strings.Contains(output, "test message: world") {
		t.Errorf("expected '[INFO] ... test message: world', got %q", output)
	}
}

func TestInfo_ErrorLevel_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{info: newInfoLogger(&buf), err: newErrorLogger(&buf), level: "error"}

	l.Info("should not appear")
	if buf.Len() > 0 {
		t.Errorf("Info() should not write at error level, got %q", buf.String())
	}
}

func TestInfo_WarnLevel_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{info: newInfoLogger(&buf), err: newErrorLogger(&buf), level: "warn"}

	l.Info("should not appear")
	if buf.Len() > 0 {
		t.Errorf("Info() should not write at warn level, got %q", buf.String())
	}
}

func TestError_AlwaysWrites(t *testing.T) {
	tests := []string{"debug", "info", "warn", "error"}
	for _, level := range tests {
		var buf bytes.Buffer
		l := &Logger{info: newInfoLogger(&buf), err: newErrorLogger(&buf), level: level}

		l.Error("error at %s level", level)
		output := buf.String()
		if !strings.Contains(output, "[ERROR]") || !strings.Contains(output, "error at "+level+" level") {
			t.Errorf("level=%s: expected '[ERROR] ... error at %s level', got %q", level, level, output)
		}
	}
}

func TestError_MultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{info: newInfoLogger(&buf), err: newErrorLogger(&buf), level: "info"}

	l.Error("first error")
	l.Error("second error")

	output := buf.String()
	if !strings.Contains(output, "first error") {
		t.Error("missing 'first error'")
	}
	if !strings.Contains(output, "second error") {
		t.Error("missing 'second error'")
	}
}

// helpers to create log.Logger instances with buffers for testing
func newInfoLogger(buf *bytes.Buffer) *log.Logger {
	return log.New(buf, "[INFO] ", log.LstdFlags)
}

func newErrorLogger(buf *bytes.Buffer) *log.Logger {
	return log.New(buf, "[ERROR] ", log.LstdFlags)
}
