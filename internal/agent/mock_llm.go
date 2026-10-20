package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
)

// MockLLM returns scripted tool-calls for tests and local demo without an API key.
type MockLLM struct {
	mu       sync.Mutex
	Scripted []ChatResponse
	idx      int
}

func NewMockLLM() *MockLLM {
	return &MockLLM{}
}

func (m *MockLLM) Push(resp ChatResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Scripted = append(m.Scripted, resp)
}

func (m *MockLLM) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	m.mu.Lock()
	if m.idx < len(m.Scripted) {
		resp := m.Scripted[m.idx]
		m.idx++
		m.mu.Unlock()
		return &resp, nil
	}
	m.mu.Unlock()

	if len(req.Tools) == 0 || req.ToolChoice == "none" {
		var toolOut string
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "tool" {
				toolOut = req.Messages[i].Content
				break
			}
		}
		if toolOut == "" {
			toolOut = "Готово."
		}
		return textResp(toolOut), nil
	}

	var userText string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userText = req.Messages[i].Content
			break
		}
	}
	lower := strings.ToLower(userText)

	if LooksLikeBulkDeleteOwn(userText) {
		return toolResp(ToolDeleteExpenses, mustJSON(map[string]any{"all": true})), nil
	}
	if LooksLikeCategoryCatalog(userText) {
		return toolResp(ToolListCategories, `{}`), nil
	}
	if strings.Contains(lower, "удали") || strings.Contains(lower, "delete") {
		if leftover := ExtractCommentQuery(userText); leftover != "" {
			return toolResp(ToolDeleteExpenses, mustJSON(map[string]any{"query": leftover})), nil
		}
		args, _ := json.Marshal(map[string]any{"query": userText})
		return toolResp(ToolDeleteExpense, string(args)), nil
	}
	if strings.Contains(lower, "измени") || strings.Contains(lower, "последн") || strings.Contains(lower, "update") || strings.Contains(lower, "было не") {
		args, _ := json.Marshal(map[string]any{"query": userText, "use_last": true})
		return toolResp(ToolUpdateExpense, string(args)), nil
	}

	ask := strings.Contains(lower, "сколько") || strings.Contains(lower, "summary") ||
		strings.Contains(lower, "потратил") || strings.Contains(lower, "потратила") ||
		strings.Contains(lower, "потрачено") || strings.Contains(lower, "ушло") ||
		strings.Contains(lower, "итого")
	if ask {
		args := map[string]any{}
		enrichFilterArgs(args, userText)
		return toolResp(ToolGetSummary, mustJSON(args)), nil
	}
	if strings.Contains(lower, "по категориям") || strings.Contains(lower, "by category") {
		args := map[string]any{}
		enrichFilterArgs(args, userText)
		return toolResp(ToolGetByCategory, mustJSON(args)), nil
	}
	if strings.Contains(lower, "покажи") || strings.Contains(lower, "list") || strings.Contains(lower, "траты") || strings.Contains(lower, "show") {
		args := map[string]any{"limit": 10, "offset": 0}
		enrichFilterArgs(args, userText)
		return toolResp(ToolListExpenses, mustJSON(args)), nil
	}

	args, _ := json.Marshal(map[string]any{"raw_text": userText})
	return toolResp(ToolCreateExpense, string(args)), nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func textResp(content string) *ChatResponse {
	return &ChatResponse{
		Choices: []struct {
			Message Message `json:"message"`
		}{
			{Message: Message{Role: "assistant", Content: content}},
		},
	}
}

func toolResp(name, args string) *ChatResponse {
	return &ChatResponse{
		Choices: []struct {
			Message Message `json:"message"`
		}{
			{
				Message: Message{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: ToolCallFunction{
								Name:      name,
								Arguments: args,
							},
						},
					},
				},
			},
		},
	}
}
