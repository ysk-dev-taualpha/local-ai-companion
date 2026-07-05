package agent

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSystemPromptInjectsRuntimeContext(t *testing.T) {
	now := time.Date(2026, 7, 5, 16, 50, 12, 0, time.UTC)
	prompt := BuildSystemPrompt("base", "Asia/Tokyo", "ja-JP", now)

	for _, want := range []string{
		"base",
		"Runtime context:",
		"Current date: 2026-07-06",
		"Current time: 01:50:12",
		"Timezone: Asia/Tokyo",
		"Locale: ja-JP",
		"Use this runtime date and time as authoritative",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}
