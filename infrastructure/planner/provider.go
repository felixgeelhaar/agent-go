package planner

import (
	"context"
	"encoding/json"
)

// Provider defines the interface for LLM providers.
type Provider interface {
	// Complete sends a chat completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Name returns the provider name for logging.
	Name() string
}

// CompletionRequest represents a chat completion request.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role       string          `json:"role"` // system, user, assistant, tool
	Content    string          `json:"content,omitempty"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
	RawContent json.RawMessage `json:"-"` // For complex content blocks
}

// Tool represents a tool definition for function calling.
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool invocation from the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// CompletionResponse represents a chat completion response.
type CompletionResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Message Message  `json:"message"`
	Usage   Usage    `json:"usage"`
	Error   *APIError `json:"error,omitempty"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// APIError represents an API error response.
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return e.Type + ": " + e.Message + " (" + e.Code + ")"
	}
	return e.Type + ": " + e.Message
}

// ProviderConfig contains common provider configuration.
type ProviderConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     int // seconds
}
