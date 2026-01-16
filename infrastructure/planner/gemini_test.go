package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewGeminiProvider(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with defaults", func(t *testing.T) {
		t.Parallel()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey: "test-key",
			Model:  "gemini-1.5-pro",
		})

		if provider == nil {
			t.Fatal("NewGeminiProvider() returned nil")
		}
		if provider.apiKey != "test-key" {
			t.Errorf("APIKey = %s, want test-key", provider.apiKey)
		}
		if provider.baseURL != "https://generativelanguage.googleapis.com" {
			t.Errorf("BaseURL = %s, want https://generativelanguage.googleapis.com", provider.baseURL)
		}
		if provider.model != "gemini-1.5-pro" {
			t.Errorf("Model = %s, want gemini-1.5-pro", provider.model)
		}
	})

	t.Run("uses custom base URL", func(t *testing.T) {
		t.Parallel()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: "https://custom.googleapis.com",
		})

		if provider.baseURL != "https://custom.googleapis.com" {
			t.Errorf("BaseURL = %s, want https://custom.googleapis.com", provider.baseURL)
		}
	})

	t.Run("uses custom timeout", func(t *testing.T) {
		t.Parallel()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			Timeout: 60,
		})

		if provider == nil {
			t.Fatal("NewGeminiProvider() returned nil")
		}
	})
}

func TestGeminiProvider_Name(t *testing.T) {
	t.Parallel()

	provider := NewGeminiProvider(GeminiConfig{APIKey: "test-key"})

	if provider.Name() != "gemini" {
		t.Errorf("Name() = %s, want gemini", provider.Name())
	}
}

func TestGeminiProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Method = %s, want POST", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/v1beta/models/gemini-1.5-pro:generateContent") {
				t.Errorf("Path = %s, want /v1beta/models/gemini-1.5-pro:generateContent", r.URL.Path)
			}
			if r.URL.Query().Get("key") != "test-key" {
				t.Errorf("API key not set in query string")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type header not set correctly")
			}

			// Decode request to verify structure
			var req geminiRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if len(req.Contents) != 1 {
				t.Errorf("Contents length = %d, want 1", len(req.Contents))
			}
			if req.Contents[0].Parts[0].Text != "Hello" {
				t.Errorf("Content = %s, want Hello", req.Contents[0].Parts[0].Text)
			}

			// Return mock response
			resp := geminiResponse{
				Candidates: []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
						Role string `json:"role"`
					} `json:"content"`
					FinishReason  string `json:"finishReason"`
					SafetyRatings []struct {
						Category    string `json:"category"`
						Probability string `json:"probability"`
					} `json:"safetyRatings"`
				}{
					{
						Content: struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
							Role string `json:"role"`
						}{
							Parts: []struct {
								Text string `json:"text"`
							}{
								{Text: "Hello! How can I help you today?"},
							},
							Role: "model",
						},
						FinishReason: "STOP",
					},
				},
				UsageMetadata: struct {
					PromptTokenCount     int `json:"promptTokenCount"`
					CandidatesTokenCount int `json:"candidatesTokenCount"`
					TotalTokenCount      int `json:"totalTokenCount"`
				}{
					PromptTokenCount:     5,
					CandidatesTokenCount: 10,
					TotalTokenCount:      15,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gemini-1.5-pro",
		})

		resp, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err != nil {
			t.Fatalf("Complete() error = %v", err)
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
			var req geminiRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			// Verify system instruction is set
			if req.SystemInstruction == nil {
				t.Error("SystemInstruction should not be nil")
			} else if req.SystemInstruction.Parts[0].Text != "You are a helpful assistant" {
				t.Errorf("SystemInstruction = %s, want 'You are a helpful assistant'", req.SystemInstruction.Parts[0].Text)
			}

			// Verify system is not in contents
			for _, content := range req.Contents {
				if content.Role == "system" {
					t.Error("System message should not be in contents")
				}
			}

			resp := geminiResponse{
				Candidates: []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
						Role string `json:"role"`
					} `json:"content"`
					FinishReason  string `json:"finishReason"`
					SafetyRatings []struct {
						Category    string `json:"category"`
						Probability string `json:"probability"`
					} `json:"safetyRatings"`
				}{
					{
						Content: struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
							Role string `json:"role"`
						}{
							Parts: []struct {
								Text string `json:"text"`
							}{{Text: "OK"}},
							Role: "model",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gemini-1.5-pro",
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

	t.Run("maps assistant role to model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req geminiRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			// Check that assistant was mapped to model
			for _, content := range req.Contents {
				if content.Role == "assistant" {
					t.Error("assistant role should be mapped to model")
				}
			}

			// Find the model role message
			found := false
			for _, content := range req.Contents {
				if content.Role == "model" {
					found = true
					break
				}
			}
			if !found {
				t.Error("expected model role in contents")
			}

			resp := geminiResponse{
				Candidates: []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
						Role string `json:"role"`
					} `json:"content"`
					FinishReason  string `json:"finishReason"`
					SafetyRatings []struct {
						Category    string `json:"category"`
						Probability string `json:"probability"`
					} `json:"safetyRatings"`
				}{
					{
						Content: struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
							Role string `json:"role"`
						}{
							Parts: []struct {
								Text string `json:"text"`
							}{{Text: "OK"}},
							Role: "model",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gemini-1.5-pro",
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

	t.Run("handles API error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"code": 400, "message": "Invalid request", "status": "INVALID_ARGUMENT"}}`))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
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
			resp := geminiResponse{
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Status  string `json:"status"`
				}{
					Code:    429,
					Message: "Rate limit exceeded",
					Status:  "RESOURCE_EXHAUSTED",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
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
		if resp.Error.Type != "RESOURCE_EXHAUSTED" {
			t.Errorf("Error.Type = %s, want RESOURCE_EXHAUSTED", resp.Error.Type)
		}
	})

	t.Run("handles empty candidates", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := geminiResponse{
				Candidates: []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
						Role string `json:"role"`
					} `json:"content"`
					FinishReason  string `json:"finishReason"`
					SafetyRatings []struct {
						Category    string `json:"category"`
						Probability string `json:"probability"`
					} `json:"safetyRatings"`
				}{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gemini-1.5-pro",
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		})

		if err == nil {
			t.Error("Expected error for empty candidates")
		}
	})

	t.Run("uses model from request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "gemini-1.5-flash") {
				t.Errorf("URL path does not contain request model: %s", r.URL.Path)
			}

			resp := geminiResponse{
				Candidates: []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
						Role string `json:"role"`
					} `json:"content"`
					FinishReason  string `json:"finishReason"`
					SafetyRatings []struct {
						Category    string `json:"category"`
						Probability string `json:"probability"`
					} `json:"safetyRatings"`
				}{
					{
						Content: struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
							Role string `json:"role"`
						}{
							Parts: []struct {
								Text string `json:"text"`
							}{{Text: "OK"}},
							Role: "model",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gemini-1.5-pro", // Default model
		})

		_, err := provider.Complete(context.Background(), CompletionRequest{
			Model: "gemini-1.5-flash", // Override with request model
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

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gemini-1.5-pro",
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
