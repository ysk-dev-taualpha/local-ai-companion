package agent

import (
	"strings"
	"testing"
)

func TestRuntimeContext_SystemInjection(t *testing.T) {
	rc := RuntimeContext{
		Timezone: "Asia/Tokyo",
		Locale:   "ja-JP",
	}
	result := rc.SystemInjection()

	if !strings.Contains(result, "Current date:") {
		t.Error("expected 'Current date:' in injection")
	}
	if !strings.Contains(result, "Current time:") {
		t.Error("expected 'Current time:' in injection")
	}
	if !strings.Contains(result, "Timezone: Asia/Tokyo") {
		t.Error("expected timezone info in injection")
	}
	if !strings.Contains(result, "Locale: ja-JP") {
		t.Error("expected locale info in injection")
	}
}

func TestRuntimeContext_SystemInjection_InvalidTimezone(t *testing.T) {
	rc := RuntimeContext{
		Timezone: "Invalid/Zone",
		Locale:   "en-US",
	}
	result := rc.SystemInjection()

	if !strings.Contains(result, "Timezone: Invalid/Zone") {
		t.Error("expected timezone info in injection even with invalid timezone")
	}
	if !strings.Contains(result, "Locale: en-US") {
		t.Error("expected locale info in injection")
	}
}

func TestRuntimeContext_SystemInjection_UTC(t *testing.T) {
	rc := RuntimeContext{
		Timezone: "UTC",
		Locale:   "en-US",
	}
	result := rc.SystemInjection()

	if !strings.Contains(result, "Timezone: UTC") {
		t.Error("expected UTC timezone in injection")
	}
}
