package planner

import (
	"context"
	"encoding/json"
	"fmt"
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

// sanitizeProviderError creates an error message without exposing sensitive API details.
// It extracts only the HTTP status code and a generic error indicator, avoiding
// the full response body which may contain API keys, internal error codes, or
// sensitive debugging information.
func sanitizeProviderError(provider string, statusCode int, respBody []byte) error {
	// Try to extract a safe error message from common JSON error structures
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Message string `json:"message"` // Alternative format
	}

	if err := json.Unmarshal(respBody, &errResp); err == nil {
		// Use structured error message if available
		msg := errResp.Error.Message
		if msg == "" {
			msg = errResp.Message
		}
		if msg != "" {
			// Truncate long messages to prevent sensitive data leakage
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			return &APIError{
				Type:    errResp.Error.Type,
				Message: msg,
			}
		}
	}

	// Fallback to generic error without exposing response body
	return &APIError{
		Type:    "provider_error",
		Message: provider + " request failed with status " + httpStatusText(statusCode),
	}
}

// httpStatusText returns a human-readable status text for common HTTP codes.
func httpStatusText(code int) string {
	switch code {
	case 400:
		return "400 Bad Request"
	case 401:
		return "401 Unauthorized"
	case 403:
		return "403 Forbidden"
	case 404:
		return "404 Not Found"
	case 429:
		return "429 Too Many Requests"
	case 500:
		return "500 Internal Server Error"
	case 502:
		return "502 Bad Gateway"
	case 503:
		return "503 Service Unavailable"
	default:
		return fmt.Sprintf("HTTP %d", code)
	}
}
