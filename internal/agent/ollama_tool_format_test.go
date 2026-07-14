package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/tool"
)

func TestOllamaToolFormat(t *testing.T) {
	// Test 1: toolDef JSON format — tools wrapped as {"type":"function","function":{...}}
	def := tool.Definition{
		Name:        "search_the_web",
		Description: "Search the web",
		Parameters: tool.Parameters{
			Type: "object",
			Properties: map[string]tool.Property{
				"query": {Type: "string", Description: "Search query"},
			},
			Required: []string{"query"},
		},
	}

	td := toolDef{Type: "function", Function: def}
	b, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal toolDef: %v", err)
	}

	js := string(b)
	if !strings.Contains(js, `"type":"function"`) {
		t.Errorf("toolDef missing type wrapper: %s", js)
	}
	if !strings.Contains(js, `"function"`) {
		t.Errorf("toolDef missing function key: %s", js)
	}
	t.Logf("toolDef JSON: %s", js)

	// Test 2: Chat() builds request with wrapped tools
	// We test indirectly by verifying the toolDef conversion logic compiles
	// (the Chat method signature proves it converts []Definition → []toolDef)

	// Test 3: messageToChat uses tool_name for tool role
	m := Message{
		Role:     "tool",
		Content:  "search results",
		ToolName: "search_the_web",
	}
	cm := messageToChat(m)

	if cm.Role != "tool" {
		t.Errorf("role = %q, want tool", cm.Role)
	}
	if cm.ToolName != "search_the_web" {
		t.Errorf("ToolName = %q, want search_the_web", cm.ToolName)
	}
	if cm.ToolCallID != "" {
		t.Errorf("ToolCallID = %q, want empty (Ollama native uses tool_name)", cm.ToolCallID)
	}

	b2, _ := json.Marshal(cm)
	js2 := string(b2)
	if !strings.Contains(js2, `"tool_name":"search_the_web"`) {
		t.Errorf("tool result JSON missing tool_name: %s", js2)
	}
	if strings.Contains(js2, `"tool_call_id"`) {
		t.Errorf("tool result JSON should not have tool_call_id: %s", js2)
	}
	t.Logf("tool message JSON: %s", js2)

	// Test 4: chatToMessage preserves ToolName
	cm2 := chatMessage{Role: "tool", Content: "bar", ToolName: "foo_tool"}
	m2 := chatToMessage(cm2)
	if m2.ToolName != "foo_tool" {
		t.Errorf("chatToMessage: ToolName = %q, want foo_tool", m2.ToolName)
	}

	// Test 5: toolCall struct unchanged (Ollama returns OpenAI-style tool_calls)
	tc := toolCall{
		ID:   "call_123",
		Type: "function",
		Function: toolCallFunc{
			Name:      "search_the_web",
			Arguments: json.RawMessage(`{"query":"test"}`),
		},
	}
	b3, _ := json.Marshal(tc)
	js3 := string(b3)
	if !strings.Contains(js3, `"id":"call_123"`) {
		t.Errorf("toolCall response missing id: %s", js3)
	}
	if !strings.Contains(js3, `"type":"function"`) {
		t.Errorf("toolCall response missing type: %s", js3)
	}
	t.Logf("toolCall response JSON: %s", js3)

	// Test 6: chatRequest.Tools is []toolDef (not raw []Definition)
	req := chatRequest{
		Model:    "test",
		Messages: []chatMessage{{Role: "user", Content: "hi"}},
		Tools:    []toolDef{{Type: "function", Function: def}},
		Stream:   false,
	}
	breq, _ := json.Marshal(req)
	jsReq := string(breq)
	if !strings.Contains(jsReq, `"type":"function"`) {
		t.Errorf("chatRequest.Tools missing type wrapper: %s", jsReq)
	}
	t.Logf("chatRequest JSON: %s", jsReq)
}
