package planner

import (
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	t.Parallel()

	t.Run("with code", func(t *testing.T) {
		t.Parallel()

		err := &APIError{
			Type:    "invalid_request_error",
			Message: "The model does not exist",
			Code:    "model_not_found",
		}

		msg := err.Error()
		if msg != "invalid_request_error: The model does not exist (model_not_found)" {
			t.Errorf("Error() = %s, want 'invalid_request_error: The model does not exist (model_not_found)'", msg)
		}
	})

	t.Run("without code", func(t *testing.T) {
		t.Parallel()

		err := &APIError{
			Type:    "rate_limit_error",
			Message: "Too many requests",
		}

		msg := err.Error()
		if msg != "rate_limit_error: Too many requests" {
			t.Errorf("Error() = %s, want 'rate_limit_error: Too many requests'", msg)
		}
	})
}

func TestCompletionRequest_Fields(t *testing.T) {
	t.Parallel()

	req := CompletionRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	if req.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Errorf("len(Messages) = %d, want 2", len(req.Messages))
	}
	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.Temperature)
	}
	if req.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want 1000", req.MaxTokens)
	}
}

func TestCompletionResponse_Fields(t *testing.T) {
	t.Parallel()

	resp := CompletionResponse{
		ID:    "resp-123",
		Model: "gpt-4",
		Message: Message{
			Role:    "assistant",
			Content: "Hello! How can I help you?",
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	if resp.ID != "resp-123" {
		t.Errorf("ID = %s, want resp-123", resp.ID)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", resp.Model)
	}
	if resp.Message.Role != "assistant" {
		t.Errorf("Message.Role = %s, want assistant", resp.Message.Role)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("Usage.TotalTokens = %d, want 30", resp.Usage.TotalTokens)
	}
}

func TestMessage_Fields(t *testing.T) {
	t.Parallel()

	msg := Message{
		Role:       "tool",
		Content:    "Result from tool",
		ToolCallID: "tc-123",
		Name:       "read_file",
	}

	if msg.Role != "tool" {
		t.Errorf("Role = %s, want tool", msg.Role)
	}
	if msg.ToolCallID != "tc-123" {
		t.Errorf("ToolCallID = %s, want tc-123", msg.ToolCallID)
	}
	if msg.Name != "read_file" {
		t.Errorf("Name = %s, want read_file", msg.Name)
	}
}

func TestToolCall_Fields(t *testing.T) {
	t.Parallel()

	tc := ToolCall{
		ID:   "call-123",
		Type: "function",
		Function: FunctionCall{
			Name:      "read_file",
			Arguments: `{"path": "/test.txt"}`,
		},
	}

	if tc.ID != "call-123" {
		t.Errorf("ID = %s, want call-123", tc.ID)
	}
	if tc.Type != "function" {
		t.Errorf("Type = %s, want function", tc.Type)
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("Function.Name = %s, want read_file", tc.Function.Name)
	}
}

func TestUsage_Fields(t *testing.T) {
	t.Parallel()

	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", usage.PromptTokens)
	}
	if usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d, want 50", usage.CompletionTokens)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", usage.TotalTokens)
	}
}

func TestProviderConfig_Fields(t *testing.T) {
	t.Parallel()

	config := ProviderConfig{
		APIKey:      "test-key",
		BaseURL:     "https://api.example.com",
		Model:       "gpt-4",
		Temperature: 0.5,
		MaxTokens:   2000,
		Timeout:     30,
	}

	if config.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", config.APIKey)
	}
	if config.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %s, want https://api.example.com", config.BaseURL)
	}
	if config.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", config.Model)
	}
	if config.Timeout != 30 {
		t.Errorf("Timeout = %d, want 30", config.Timeout)
	}
}
