package tools

import (
	"encoding/json"
	"fmt"
	"strings"

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
		params.State = strings.ToUpper(params.State)
		validStates := map[string]bool{
			"IDLE": true, "LISTENING": true, "THINKING": true, "SPEAKING": true,
		}
		if !validStates[params.State] {
			return "", fmt.Errorf("set_state: invalid state %q (allowed: IDLE, LISTENING, THINKING, SPEAKING)", params.State)
		}
		if setter != nil {
			if err := setter(params.State); err != nil {
				return "", fmt.Errorf("set_state: %w", err)
			}
		}
		return fmt.Sprintf("state set to %s", params.State), nil
	})
}
