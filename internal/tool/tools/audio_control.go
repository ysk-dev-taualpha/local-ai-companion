package tools

import (
	"encoding/json"
	"fmt"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

type AudioActionFunc func(action string) (string, error)

func NewAudioControl(callback AudioActionFunc) tool.Executor {
	return tool.ExecutorFunc(func(args json.RawMessage) (string, error) {
		var params struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("audio_control: invalid args: %w", err)
		}
		if params.Action == "" {
			return "", fmt.Errorf("audio_control: action is required")
		}
		validActions := map[string]bool{
			"mute": true, "unmute": true, "volume_up": true,
			"volume_down": true, "pause": true, "resume": true,
		}
		if !validActions[params.Action] {
			return "", fmt.Errorf("audio_control: invalid action %q (allowed: mute, unmute, volume_up, volume_down, pause, resume)", params.Action)
		}
		if callback == nil {
			return fmt.Sprintf("audio %s acknowledged (no-op)", params.Action), nil
		}
		return callback(params.Action)
	})
}
