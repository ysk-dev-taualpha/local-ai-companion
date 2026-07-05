package tools

import (
	"context"
	"encoding/json"
)

// AudioControlParams is the JSON schema for the audio_control tool.
type AudioControlParams struct {
	Action string `json:"action"`
}

// AudioControl is a tool that sends audio control actions to the client.
// The actual audio playback is handled by the client (e.g., Unity) — this tool
// just returns the control command as structured output.
type AudioControl struct{}

// NewAudioControl creates a new AudioControl tool.
func NewAudioControl() *AudioControl {
	return &AudioControl{}
}

// Name returns the tool name.
func (a *AudioControl) Name() string {
	return "audio_control"
}

// Description returns the tool description.
func (a *AudioControl) Description() string {
	return `Control audio output. Supported actions:
- "speak": start speaking the current text response
- "stop": stop current audio playback
- "pause": pause current audio playback
- "resume": resume paused audio playback`
}

// Parameters returns the JSON Schema for the tool's parameters.
func (a *AudioControl) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["speak", "stop", "pause", "resume"],
				"description": "The audio control action to perform"
			}
		},
		"required": ["action"]
	}`)
}

// Execute returns the audio control command.
func (a *AudioControl) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	var params AudioControlParams
	if err := json.Unmarshal(arguments, &params); err != nil {
		return "", err
	}

	validActions := map[string]bool{
		"speak":  true,
		"stop":   true,
		"pause":  true,
		"resume": true,
	}

	if !validActions[params.Action] {
		return "", &InvalidActionError{Action: params.Action, ValidActions: []string{"speak", "stop", "pause", "resume"}}
	}

	out, _ := json.Marshal(map[string]string{
		"action":  params.Action,
		"message": "audio_control: " + params.Action + " command issued",
	})
	return string(out), nil
}

// InvalidActionError is returned when an unsupported action is requested.
type InvalidActionError struct {
	Action       string
	ValidActions []string
}

func (e *InvalidActionError) Error() string {
	validJSON, _ := json.Marshal(e.ValidActions)
	return "audio_control: invalid action \"" + e.Action + "\", valid actions: " + string(validJSON)
}
