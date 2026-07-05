package tool

import "encoding/json"

type Definition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type Call struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Result struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
}

type Executor interface {
	Execute(args json.RawMessage) (string, error)
}

type ExecutorFunc func(args json.RawMessage) (string, error)

func (f ExecutorFunc) Execute(args json.RawMessage) (string, error) {
	return f(args)
}
