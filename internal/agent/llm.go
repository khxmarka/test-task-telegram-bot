package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// LLMClient abstracts any OpenAI-compatible chat completions API.
type LLMClient interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

type ChatRequest struct {
	Messages        []Message `json:"messages"`
	Tools           []ToolDef `json:"tools,omitempty"`
	ToolChoice      string    `json:"tool_choice,omitempty"`
	Temperature     float64   `json:"temperature"`
	MaxTokens       int       `json:"max_tokens"`
	Model           string    `json:"model"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolDef struct {
	Type     string             `json:"type"`
	Function ToolFunctionSchema `json:"function"`
}

type ToolFunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// StatusError keeps the HTTP status so callers can tell rate limits from bad requests.
type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("llm status %d: %s", e.Status, e.Body)
}

func IsStatus(err error, status int) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status == status
}

type OpenAIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
	maxWait    time.Duration
}

func NewOpenAIClient(baseURL, apiKey string, timeout time.Duration) *OpenAIClient {
	return &OpenAIClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: 2,
		maxWait:    20 * time.Second,
	}
}

// Groq puts the wait into the message: "Please try again in 7.66s".
var retryInRe = regexp.MustCompile(`try again in ([0-9.]+)(ms|s)`)

func retryDelay(header http.Header, body string, attempt int) time.Duration {
	if v := header.Get("Retry-After"); v != "" {
		if s, err := strconv.ParseFloat(v, 64); err == nil {
			return time.Duration(s * float64(time.Second))
		}
	}
	if m := retryInRe.FindStringSubmatch(body); m != nil {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			if m[2] == "ms" {
				return time.Duration(f * float64(time.Millisecond))
			}
			return time.Duration(f * float64(time.Second))
		}
	}
	return time.Duration(attempt+1) * 2 * time.Second
}

func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		out, wait, err := c.do(ctx, body, attempt)
		if err == nil {
			return out, nil
		}
		if wait <= 0 || attempt >= c.maxRetries || wait > c.maxWait {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait + 250*time.Millisecond):
		}
	}
}

// do performs one request; wait > 0 means the error is retryable after that delay.
func (c *OpenAIClient) do(ctx context.Context, body []byte, attempt int) (*ChatResponse, time.Duration, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode >= 300 {
		se := &StatusError{Status: resp.StatusCode, Body: string(data)}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, retryDelay(resp.Header, se.Body, attempt), se
		}
		return nil, 0, se
	}
	var out ChatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, 0, fmt.Errorf("decode llm: %w", err)
	}
	return &out, 0, nil
}
