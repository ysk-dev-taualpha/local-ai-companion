package agent

import (
	"fmt"
	"strings"
	"time"
)

func BuildSystemPrompt(basePrompt, timezone, locale string, now time.Time) string {
	if timezone == "" {
		timezone = "Local"
	}
	if locale == "" {
		locale = "ja-JP"
	}

	loc := time.Local
	if timezone != "Local" {
		if loaded, err := time.LoadLocation(timezone); err == nil {
			loc = loaded
		}
	}
	runtimeNow := now.In(loc)

	contextPrompt := fmt.Sprintf(`Runtime context:
- Current date: %s
- Current time: %s
- Timezone: %s
- Locale: %s

Use this runtime date and time as authoritative when answering questions about today, yesterday, tomorrow, the current year, weekdays, schedules, or relative dates. Use web_search only when external current information is required.`,
		runtimeNow.Format("2006-01-02"),
		runtimeNow.Format("15:04:05"),
		timezone,
		locale,
	)

	basePrompt = strings.TrimSpace(basePrompt)
	if basePrompt == "" {
		return contextPrompt
	}
	return basePrompt + "\n\n" + contextPrompt
}
