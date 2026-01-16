package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOllamaProvider(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with defaults", func(t *testing.T) {
		t.Parallel()

		provider := NewOllamaProvider(OllamaConfig{
			Model: "llama3.2",
		})

		if provider == nil {
			t.Fatal("NewOllamaProvider() returned nil")
		}
		if provider.baseURL != "http://localhost:11434" {
			t.Errorf("BaseURL = %s, want http://localhost:11434", provider.baseURL)
		}
		if provider.model != "llama3.2" {
			t.Errorf("Model = %s, want llama3.2", provider.model)
		}
	})

	t.Run("uses custom base URL", func(t *testing.T) {
		t.Parallel()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: "http://custom.ollama.local:11434",
		})

		if provider.baseURL != "http://custom.ollama.local:11434" {
			t.Errorf("BaseURL = %s, want http://custom.ollama.local:11434", provider.baseURL)
		}
	})

	t.Run("uses custom timeout", func(t *testing.T) {
		t.Parallel()

		provider := NewOllamaProvider(OllamaConfig{
			Timeout: 60,
		})

		if provider == nil {
			t.Fatal("NewOllamaProvider() returned nil")
		}
	})
}

func TestOllamaProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewOllamaProvider(OllamaConfig{})

	if provider.Name() != "ollama" {
		t.Errorf("Name() = %s, want ollama", provider.Name())
	}
}

func TestOllamaProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/api/chat" {
				t.Errorf("Path = %s, want /api/chat", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type header not set correctly")
			}

			// Decode request to verify structure
			var req ollamaChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.Model != "llama3.2" {
				t.Errorf("Model = %s, want llama3.2", req.Model)
			}
			if req.Stream != false {
				t.Error("Stream should be false")
			}
			if len(req.Messages) != 1 {
				t.Errorf("Messages length = %d, want 1", len(req.Messages))
			}
			if req.Messages[0].Content != "Hello" {
				t.Errorf("Message content = %s, want Hello", req.Messages[0].Content)
			}

			// Return mock response
			resp := ollamaChatResponse{
				Model:     "llama3.2",
				CreatedAt: "2024-01-01T00:00:00Z",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you today?",
				},
				Done: true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
			Model:   "llama3.2",
		})

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
		if resp.Model != "llama3.2" {
			t.Errorf("Model = %s, want llama3.2", resp.Model)
		}
		if resp.Message.Content != "Hello! How can I help you today?" {
			t.Errorf("Content = %s, want 'Hello! How can I help you today?'", resp.Message.Content)
		}
		if resp.Message.Role != "assistant" {
			t.Errorf("Role = %s, want assistant", resp.Message.Role)
		}
	})

	t.Run("with system message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			// Ollama passes system messages through
			if len(req.Messages) != 2 {
				t.Errorf("Messages length = %d, want 2", len(req.Messages))
			}
			if req.Messages[0].Role != "system" {
				t.Errorf("First message role = %s, want system", req.Messages[0].Role)
			}

			resp := ollamaChatResponse{
				Model: "llama3.2",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "I understand!",
				},
				Done: true,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
			Model:   "llama3.2",
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
			w.Write([]byte(`{"error": "model not found"}`))
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
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

	t.Run("uses model from request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaChatRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Model != "mistral" {
				t.Errorf("Model = %s, want mistral", req.Model)
			}

			resp := ollamaChatResponse{
				Model: "mistral",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "OK",
				},
				Done: true,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
			Model:   "llama3.2", // Default model
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Model: "mistral", // Override with request model
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})

	t.Run("passes options", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaChatRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.Options == nil {
				t.Error("Options should not be nil")
			} else {
				if req.Options.Temperature != 0.7 {
					t.Errorf("Temperature = %f, want 0.7", req.Options.Temperature)
				}
				if req.Options.NumPredict != 1000 {
					t.Errorf("NumPredict = %d, want 1000", req.Options.NumPredict)
				}
			}

			resp := ollamaChatResponse{
				Model: "llama3.2",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "OK",
				},
				Done: true,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
			Model:   "llama3.2",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0.7,
			MaxTokens:   1000,
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

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
			Model:   "llama3.2",
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

	t.Run("handles conversation history", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ollamaChatRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Messages) != 3 {
				t.Errorf("Messages length = %d, want 3", len(req.Messages))
			}

			// Verify message order
			if req.Messages[0].Role != "user" {
				t.Errorf("Messages[0].Role = %s, want user", req.Messages[0].Role)
			}
			if req.Messages[1].Role != "assistant" {
				t.Errorf("Messages[1].Role = %s, want assistant", req.Messages[1].Role)
			}
			if req.Messages[2].Role != "user" {
				t.Errorf("Messages[2].Role = %s, want user", req.Messages[2].Role)
			}

			resp := ollamaChatResponse{
				Model: "llama3.2",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "I'm doing well!",
				},
				Done: true,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewOllamaProvider(OllamaConfig{
			BaseURL: server.URL,
			Model:   "llama3.2",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	})
}
