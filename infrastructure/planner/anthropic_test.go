package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAnthropicProvider(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with defaults", func(t *testing.T) {
		t.Parallel()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey: "test-key",
			Model:  "claude-3-sonnet-20240229",
		})

		if provider == nil {
			t.Fatal("NewAnthropicProvider() returned nil")
		}
		if provider.apiKey != "test-key" {
			t.Errorf("APIKey = %s, want test-key", provider.apiKey)
		}
		if provider.baseURL != "https://api.anthropic.com" {
			t.Errorf("BaseURL = %s, want https://api.anthropic.com", provider.baseURL)
		}
		if provider.model != "claude-3-sonnet-20240229" {
			t.Errorf("Model = %s, want claude-3-sonnet-20240229", provider.model)
		}
	})

	t.Run("uses custom base URL", func(t *testing.T) {
		t.Parallel()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: "https://custom.anthropic.com",
		})

		if provider.baseURL != "https://custom.anthropic.com" {
			t.Errorf("BaseURL = %s, want https://custom.anthropic.com", provider.baseURL)
		}
	})

	t.Run("uses custom timeout", func(t *testing.T) {
		t.Parallel()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			Timeout: 60,
		})

		// Client timeout is set internally - we can verify provider was created
		if provider == nil {
			t.Fatal("NewAnthropicProvider() returned nil")
		}
	})
}

func TestAnthropicProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewAnthropicProvider(AnthropicConfig{APIKey: "test-key"})

	if provider.Name() != "anthropic" {
		t.Errorf("Name() = %s, want anthropic", provider.Name())
	}
}

func TestAnthropicProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/v1/messages" {
				t.Errorf("Path = %s, want /v1/messages", r.URL.Path)
			}
			if r.Header.Get("x-api-key") != "test-key" {
				t.Errorf("x-api-key header not set correctly")
			}
			if r.Header.Get("anthropic-version") != "2023-06-01" {
				t.Errorf("anthropic-version header not set correctly")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type header not set correctly")
			}

			// Decode request to verify structure
			var req anthropicRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.Model != "claude-3-sonnet-20240229" {
				t.Errorf("Model = %s, want claude-3-sonnet-20240229", req.Model)
			}
			if len(req.Messages) != 1 {
				t.Errorf("Messages length = %d, want 1", len(req.Messages))
			}
			if req.Messages[0].Content != "Hello" {
				t.Errorf("Message content = %s, want Hello", req.Messages[0].Content)
			}

			// Return mock response
			resp := anthropicResponse{
				ID:   "msg-123",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: "Hello! How can I help you today?"},
				},
				Model:      "claude-3-sonnet-20240229",
				StopReason: "end_turn",
				Usage: struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				}{
					InputTokens:  5,
					OutputTokens: 10,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "claude-3-sonnet-20240229",
		})

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if resp.ID != "msg-123" {
			t.Errorf("ID = %s, want msg-123", resp.ID)
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

	t.Run("with system message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req anthropicRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			// Verify system prompt is extracted
			if req.System != "You are a helpful assistant" {
				t.Errorf("System = %s, want 'You are a helpful assistant'", req.System)
			}

			// Verify messages don't include system
			for _, msg := range req.Messages {
				if msg.Role == "system" {
					t.Error("System message should not be in messages array")
				}
			}

			resp := anthropicResponse{
				ID:   "msg-456",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: "I understand!"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "claude-3-sonnet-20240229",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})

	t.Run("handles API error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"type": "invalid_request_error", "message": "Invalid request"}}`))
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
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
			resp := anthropicResponse{
				Error: &struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}{
					Type:    "rate_limit_error",
					Message: "Rate limit exceeded",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
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
		if resp.Error.Message != "Rate limit exceeded" {
			t.Errorf("Error.Message = %s, want 'Rate limit exceeded'", resp.Error.Message)
		}
	})

	t.Run("uses model from request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req anthropicRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Model != "claude-3-opus-20240229" {
				t.Errorf("Model = %s, want claude-3-opus-20240229", req.Model)
			}

			resp := anthropicResponse{
				ID:   "test",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: "OK"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "claude-3-sonnet-20240229", // Default model
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Model: "claude-3-opus-20240229", // Override with request model
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})

	t.Run("uses default max tokens", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req anthropicRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.MaxTokens != 1024 {
				t.Errorf("MaxTokens = %d, want 1024 (default)", req.MaxTokens)
			}

			resp := anthropicResponse{
				ID:   "test",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: "OK"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "claude-3-sonnet-20240229",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			// MaxTokens not set - should default to 1024
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})

	t.Run("uses provided max tokens", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req anthropicRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.MaxTokens != 2048 {
				t.Errorf("MaxTokens = %d, want 2048", req.MaxTokens)
			}

			resp := anthropicResponse{
				ID:   "test",
				Type: "message",
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: "OK"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "claude-3-sonnet-20240229",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			MaxTokens: 2048,
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

		provider := NewAnthropicProvider(AnthropicConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "claude-3-sonnet-20240229",
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
