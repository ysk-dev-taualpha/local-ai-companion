package tools

import (
	"encoding/json"
	"fmt"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

type StateSetter func(state string) error

func NewSetState(setter StateSetter) tool.Executor {
	return tool.ExecutorFunc(func(args json.RawMessage) (string, error) {
		var params struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("set_state: invalid args: %w", err)
		}
		if params.State == "" {
			return "", fmt.Errorf("set_state: state is required")
		}
		validStates := map[string]bool{
			"idle": true, "listening": true, "thinking": true,
			"speaking": true, "sleeping": true,
		}
		if !validStates[params.State] {
			return "", fmt.Errorf("set_state: invalid state %q (allowed: idle, listening, thinking, speaking, sleeping)", params.State)
		}
		if setter != nil {
			if err := setter(params.State); err != nil {
				return "", fmt.Errorf("set_state: %w", err)
			}
		}
		return fmt.Sprintf("state set to %s", params.State), nil
	})
}
