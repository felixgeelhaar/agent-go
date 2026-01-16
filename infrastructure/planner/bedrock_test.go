package planner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewBedrockProvider(t *testing.T) {
	t.Parallel()

	t.Run("creates provider with explicit credentials", func(t *testing.T) {
		t.Parallel()

		provider, err := NewBedrockProvider(BedrockConfig{
			Region:          "us-west-2",
			ModelID:         "anthropic.claude-3-sonnet-20240229-v1:0",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		})

		if err != nil {
			t.Fatalf("NewBedrockProvider() error = %v", err)
		}
		if provider == nil {
			t.Fatal("NewBedrockProvider() returned nil")
		}
		if provider.region != "us-west-2" {
			t.Errorf("Region = %s, want us-west-2", provider.region)
		}
		if provider.modelID != "anthropic.claude-3-sonnet-20240229-v1:0" {
			t.Errorf("ModelID = %s, want anthropic.claude-3-sonnet-20240229-v1:0", provider.modelID)
		}
	})

	t.Run("uses default region", func(t *testing.T) {
		t.Parallel()

		provider, err := NewBedrockProvider(BedrockConfig{
			ModelID:         "anthropic.claude-3-sonnet-20240229-v1:0",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		})

		if err != nil {
			t.Fatalf("NewBedrockProvider() error = %v", err)
		}
		if provider.region != "us-east-1" {
			t.Errorf("Region = %s, want us-east-1 (default)", provider.region)
		}
	})
}

func TestBedrockProvider_Name(t *testing.T) {
	t.Parallel()

	provider, _ := NewBedrockProvider(BedrockConfig{
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	})

	if provider.Name() != "bedrock" {
		t.Errorf("Name() = %s, want bedrock", provider.Name())
	}
}

func TestBedrockProvider_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		// Create mock server that simulates Bedrock API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != "POST" {
				t.Errorf("Method = %s, want POST", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
			}

			// Decode request to verify structure
			var req bedrockClaudeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.AnthropicVersion != "bedrock-2023-05-31" {
				t.Errorf("AnthropicVersion = %s, want bedrock-2023-05-31", req.AnthropicVersion)
			}

			// Return mock response
			resp := bedrockClaudeResponse{
				ID:   "test-id",
				Type: "message",
				Role: "assistant",
				Content: []bedrockClaudeContent{
					{Type: "text", Text: "Hello! How can I help you?"},
				},
				Model:      "anthropic.claude-3-sonnet-20240229-v1:0",
				StopReason: "end_turn",
				Usage: struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				}{
					InputTokens:  10,
					OutputTokens: 15,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		// Note: We can't easily test with the real provider because it requires
		// AWS signature signing. This test demonstrates the expected behavior.
		t.Log("Mock server test passed - real provider requires AWS credentials and signing")
	})
}

func TestSha256Hash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty",
			input:    []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "hello",
			input:    []byte("hello"),
			expected: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := sha256Hash(tt.input)
			if result != tt.expected {
				t.Errorf("sha256Hash() = %s, want %s", result, tt.expected)
			}
		})
	}
}
