package agent

import (
	"fmt"
	"time"
)

// RuntimeContext は LLM 呼び出し時に system message に注入される実行時コンテキストです。
type RuntimeContext struct {
	Timezone string
	Locale   string
}

// SystemInjection は RuntimeContext を system message に注入するテキストを生成します。
func (rc RuntimeContext) SystemInjection() string {
	now := time.Now()
	loc, err := time.LoadLocation(rc.Timezone)
	if err == nil {
		now = now.In(loc)
	}
	return fmt.Sprintf(
		"Current date: %s\nCurrent time: %s\nTimezone: %s\nLocale: %s",
		now.Format("2006-01-02"),
		now.Format("15:04:05"),
		rc.Timezone,
		rc.Locale,
	)
}
