package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCohereProvider(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with defaults", func(t *testing.T) {
		t.Parallel()

		provider := NewCohereProvider(CohereConfig{
			APIKey: "test-key",
			Model:  "command-r",
		})

		if provider == nil {
			t.Fatal("NewCohereProvider() returned nil")
		}
		if provider.apiKey != "test-key" {
			t.Errorf("APIKey = %s, want test-key", provider.apiKey)
		}
		if provider.baseURL != "https://api.cohere.ai" {
			t.Errorf("BaseURL = %s, want https://api.cohere.ai", provider.baseURL)
		}
		if provider.model != "command-r" {
			t.Errorf("Model = %s, want command-r", provider.model)
		}
	})

	t.Run("uses custom base URL", func(t *testing.T) {
		t.Parallel()

		provider := NewCohereProvider(CohereConfig{
			APIKey:  "test-key",
			BaseURL: "https://custom.cohere.com",
		})

		if provider.baseURL != "https://custom.cohere.com" {
			t.Errorf("BaseURL = %s, want https://custom.cohere.com", provider.baseURL)
		}
	})
}

func TestCohereProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewCohereProvider(CohereConfig{APIKey: "test-key"})

	if provider.Name() != "cohere" {
		t.Errorf("Name() = %s, want cohere", provider.Name())
	}
}

func TestCohereProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/v1/chat" {
				t.Errorf("Path = %s, want /v1/chat", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization header not set correctly")
			}

			// Decode request to verify structure
			var req cohereChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.Model != "command-r" {
				t.Errorf("Model = %s, want command-r", req.Model)
			}
			if req.Message != "Hello" {
				t.Errorf("Message = %s, want Hello", req.Message)
			}

			// Return mock response
			resp := cohereChatResponse{
				ResponseID:   "resp-123",
				Text:         "Hello! How can I help you today?",
				GenerationID: "gen-456",
				FinishReason: "COMPLETE",
				Meta: cohereMeta{
					Tokens: struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					}{
						InputTokens:  5,
						OutputTokens: 10,
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewCohereProvider(CohereConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "command-r",
		})

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if resp.ID != "resp-123" {
			t.Errorf("ID = %s, want resp-123", resp.ID)
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
	})

	t.Run("with system message and history", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req cohereChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			// Verify preamble (system message)
			if req.Preamble != "You are a helpful assistant" {
				t.Errorf("Preamble = %s, want 'You are a helpful assistant'", req.Preamble)
			}

			// Verify chat history
			if len(req.ChatHistory) != 2 {
				t.Errorf("ChatHistory length = %d, want 2", len(req.ChatHistory))
			}
			if req.ChatHistory[0].Role != "USER" {
				t.Errorf("ChatHistory[0].Role = %s, want USER", req.ChatHistory[0].Role)
			}
			if req.ChatHistory[1].Role != "CHATBOT" {
				t.Errorf("ChatHistory[1].Role = %s, want CHATBOT", req.ChatHistory[1].Role)
			}

			// Verify current message
			if req.Message != "What about now?" {
				t.Errorf("Message = %s, want 'What about now?'", req.Message)
			}

			resp := cohereChatResponse{
				ResponseID: "resp-789",
				Text:       "Now is good too!",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewCohereProvider(CohereConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "command-r",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "How are you?"},
				{Role: "assistant", Content: "I'm doing great!"},
				{Role: "user", Content: "What about now?"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message": "Invalid request"}`))
		}))
		defer server.Close()

		provider := NewCohereProvider(CohereConfig{
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

	t.Run("handles response error message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := cohereChatResponse{
				Message: "Rate limit exceeded",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewCohereProvider(CohereConfig{
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
		if resp.Error.Message != "Rate limit exceeded" {
			t.Errorf("Error.Message = %s, want 'Rate limit exceeded'", resp.Error.Message)
		}
	})

	t.Run("uses model from request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req cohereChatRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Model != "command-r-plus" {
				t.Errorf("Model = %s, want command-r-plus", req.Model)
			}

			resp := cohereChatResponse{ResponseID: "test", Text: "OK"}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewCohereProvider(CohereConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "command-r", // Default model
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Model: "command-r-plus", // Override with request model
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})
}
