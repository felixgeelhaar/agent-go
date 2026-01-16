package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOpenAIProvider(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with defaults", func(t *testing.T) {
		t.Parallel()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey: "test-key",
			Model:  "gpt-4o",
		})

		if provider == nil {
			t.Fatal("NewOpenAIProvider() returned nil")
		}
		if provider.apiKey != "test-key" {
			t.Errorf("APIKey = %s, want test-key", provider.apiKey)
		}
		if provider.baseURL != "https://api.openai.com" {
			t.Errorf("BaseURL = %s, want https://api.openai.com", provider.baseURL)
		}
		if provider.model != "gpt-4o" {
			t.Errorf("Model = %s, want gpt-4o", provider.model)
		}
	})

	t.Run("uses custom base URL", func(t *testing.T) {
		t.Parallel()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: "https://custom.openai.com",
		})

		if provider.baseURL != "https://custom.openai.com" {
			t.Errorf("BaseURL = %s, want https://custom.openai.com", provider.baseURL)
		}
	})

	t.Run("uses custom timeout", func(t *testing.T) {
		t.Parallel()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			Timeout: 60,
		})

		if provider == nil {
			t.Fatal("NewOpenAIProvider() returned nil")
		}
	})
}

func TestOpenAIProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key"})

	if provider.Name() != "openai" {
		t.Errorf("Name() = %s, want openai", provider.Name())
	}
}

func TestOpenAIProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/v1/chat/completions" {
				t.Errorf("Path = %s, want /v1/chat/completions", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization header not set correctly")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type header not set correctly")
			}

			// Decode request to verify structure
			var req openAIChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.Model != "gpt-4o" {
				t.Errorf("Model = %s, want gpt-4o", req.Model)
			}
			if len(req.Messages) != 1 {
				t.Errorf("Messages length = %d, want 1", len(req.Messages))
			}
			if req.Messages[0].Content != "Hello" {
				t.Errorf("Message content = %s, want Hello", req.Messages[0].Content)
			}

			// Return mock response
			resp := openAIChatResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{
					{
						Index: 0,
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Role:    "assistant",
							Content: "Hello! How can I help you today?",
						},
						FinishReason: "stop",
					},
				},
				Usage: struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				}{
					PromptTokens:     5,
					CompletionTokens: 10,
					TotalTokens:      15,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		})

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if resp.ID != "chatcmpl-123" {
			t.Errorf("ID = %s, want chatcmpl-123", resp.ID)
		}
		if resp.Message.Content != "Hello! How can I help you today?" {
			t.Errorf("Content = %s, want 'Hello! How can I help you today?'", resp.Message.Content)
		}
		if resp.Message.Role != "assistant" {
			t.Errorf("Role = %s, want assistant", resp.Message.Role)
		}
		if resp.Usage.PromptTokens != 5 {
			t.Errorf("PromptTokens = %d, want 5", resp.Usage.PromptTokens)
		}
		if resp.Usage.CompletionTokens != 10 {
			t.Errorf("CompletionTokens = %d, want 10", resp.Usage.CompletionTokens)
		}
		if resp.Usage.TotalTokens != 15 {
			t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
		}
	})

	t.Run("handles API error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"message": "Invalid request", "type": "invalid_request_error"}}`))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err == nil {
			t.Error("Expected error for bad request")
		}
	})

	t.Run("handles error in response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := openAIChatResponse{
				Error: &struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				}{
					Type:    "rate_limit_error",
					Message: "Rate limit exceeded",
					Code:    "rate_limit",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if resp.Error == nil {
			t.Error("Expected Error in response")
		}
		if resp.Error.Type != "rate_limit_error" {
			t.Errorf("Error.Type = %s, want rate_limit_error", resp.Error.Type)
		}
		if resp.Error.Code != "rate_limit" {
			t.Errorf("Error.Code = %s, want rate_limit", resp.Error.Code)
		}
	})

	t.Run("handles empty choices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := openAIChatResponse{
				ID:      "chatcmpl-123",
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err == nil {
			t.Error("Expected error for empty choices")
		}
	})

	t.Run("uses model from request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openAIChatRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Model != "gpt-4-turbo" {
				t.Errorf("Model = %s, want gpt-4-turbo", req.Model)
			}

			resp := openAIChatResponse{
				ID:    "test",
				Model: "gpt-4-turbo",
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{
					{
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{Role: "assistant", Content: "OK"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o", // Default model
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Model: "gpt-4-turbo", // Override with request model
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		provider := NewOpenAIProvider(OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err == nil {
			t.Error("Expected error for invalid JSON response")
		}
	})
}
