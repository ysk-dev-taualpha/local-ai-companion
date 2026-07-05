package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// StateReader is an interface for reading the companion's current state.
type StateReader interface {
	Current() string
}

// StateWriter is an interface for setting the companion's state.
type StateWriter interface {
	Set(state string) error
}

// SetStateParams is the JSON schema for the set_state tool.
type SetStateParams struct {
	State string `json:"state"`
}

// SetState is a tool that allows the LLM to set the companion's state.
type SetState struct {
	reader StateReader
	writer StateWriter
}

// NewSetState creates a new SetState tool.
func NewSetState(reader StateReader, writer StateWriter) *SetState {
	return &SetState{reader: reader, writer: writer}
}

// Name returns the tool name.
func (s *SetState) Name() string {
	return "set_state"
}

// Description returns the tool description.
func (s *SetState) Description() string {
	return "Set the companion's state. Returns the previous and new state values."
}

// Parameters returns the JSON Schema for the tool's parameters.
func (s *SetState) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"state": {
				"type": "string",
				"description": "The new state value to set"
			}
		},
		"required": ["state"]
	}`)
}

// Execute sets the companion's state.
func (s *SetState) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	var params SetStateParams
	if err := json.Unmarshal(arguments, &params); err != nil {
		return "", fmt.Errorf("set_state: invalid arguments: %w", err)
	}

	if params.State == "" {
		return "", fmt.Errorf("set_state: state is required")
	}

	prevState := s.reader.Current()
	if err := s.writer.Set(params.State); err != nil {
		return "", fmt.Errorf("set_state: failed to set state: %w", err)
	}

	out, _ := json.Marshal(map[string]string{
		"previous": prevState,
		"current":  params.State,
	})
	return string(out), nil
}

// DefaultStateStore is a simple in-memory state store for set_state.
type DefaultStateStore struct {
	state string
}

func NewDefaultStateStore(initial string) *DefaultStateStore {
	return &DefaultStateStore{state: initial}
}

func (s *DefaultStateStore) Current() string {
	return s.state
}

func (s *DefaultStateStore) Set(state string) error {
	s.state = state
	return nil
}
