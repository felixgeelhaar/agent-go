package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider:     provider,
		DefaultModel: "test-model",
	})

	if p.Name != "llm" {
		t.Errorf("expected pack name 'llm', got %s", p.Name)
	}

	if len(p.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"llm_complete", "llm_embed", "llm_classify"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestComplete(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful completion",
			input: map[string]interface{}{
				"prompt": "Hello, world!",
			},
			wantErr: false,
		},
		{
			name: "completion with all options",
			input: map[string]interface{}{
				"prompt":        "Test prompt",
				"model":         "custom-model",
				"system_prompt": "You are helpful",
				"max_tokens":    500,
				"temperature":   0.5,
			},
			wantErr: false,
		},
		{
			name: "empty prompt returns error",
			input: map[string]interface{}{
				"prompt": "",
			},
			wantErr: true,
		},
		{
			name:    "missing prompt returns error",
			input:   map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"prompt": "Test",
			},
			setupFunc: func(p *MockProvider) {
				p.CompleteFunc = func(context.Context, CompletionRequest) (CompletionResponse, error) {
					return CompletionResponse{}, errors.New("provider error")
				}
			},
			wantErr:     true,
			errContains: "provider error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:     provider,
				DefaultModel: "default-model",
			})

			var completeTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "llm_complete" {
					completeTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := completeTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp CompletionResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.Text == "" {
				t.Error("expected non-empty response text")
			}
		})
	}
}

func TestEmbed(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful embedding",
			input: map[string]interface{}{
				"texts": []string{"Hello", "World"},
			},
			wantErr: false,
		},
		{
			name: "embedding with model",
			input: map[string]interface{}{
				"texts": []string{"Test"},
				"model": "embedding-model",
			},
			wantErr: false,
		},
		{
			name: "embedding with dimensions",
			input: map[string]interface{}{
				"texts":      []string{"Test"},
				"dimensions": 512,
			},
			wantErr: false,
		},
		{
			name: "empty texts returns error",
			input: map[string]interface{}{
				"texts": []string{},
			},
			wantErr: true,
		},
		{
			name:    "missing texts returns error",
			input:   map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"texts": []string{"Test"},
			},
			setupFunc: func(p *MockProvider) {
				p.EmbedFunc = func(context.Context, EmbedRequest) (EmbedResponse, error) {
					return EmbedResponse{}, errors.New("embed error")
				}
			},
			wantErr:     true,
			errContains: "embed error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:              provider,
				DefaultEmbeddingModel: "default-embed",
			})

			var embedTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "llm_embed" {
					embedTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := embedTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp EmbedResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if len(resp.Embeddings) == 0 {
				t.Error("expected non-empty embeddings")
			}

			// Embeddings should be cacheable
			if !result.Cached {
				t.Error("expected embeddings to be cached")
			}
		})
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful classification",
			input: map[string]interface{}{
				"text":   "This is a great product!",
				"labels": []string{"positive", "negative", "neutral"},
			},
			wantErr: false,
		},
		{
			name: "classification with model",
			input: map[string]interface{}{
				"text":   "Test",
				"labels": []string{"a", "b"},
				"model":  "custom-model",
			},
			wantErr: false,
		},
		{
			name: "multi-label classification",
			input: map[string]interface{}{
				"text":        "Test",
				"labels":      []string{"tech", "sports", "politics"},
				"multi_label": true,
			},
			wantErr: false,
		},
		{
			name: "empty text returns error",
			input: map[string]interface{}{
				"text":   "",
				"labels": []string{"a", "b"},
			},
			wantErr: true,
		},
		{
			name: "empty labels returns error",
			input: map[string]interface{}{
				"text":   "Test",
				"labels": []string{},
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"text":   "Test",
				"labels": []string{"a", "b"},
			},
			setupFunc: func(p *MockProvider) {
				p.ClassifyFunc = func(context.Context, ClassifyRequest) (ClassifyResponse, error) {
					return ClassifyResponse{}, errors.New("classify error")
				}
			},
			wantErr:     true,
			errContains: "classify error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:     provider,
				DefaultModel: "default-model",
			})

			var classifyTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "llm_classify" {
					classifyTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := classifyTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp ClassifyResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if len(resp.Classifications) == 0 {
				t.Error("expected non-empty classifications")
			}

			// Classifications should be cacheable
			if !result.Cached {
				t.Error("expected classifications to be cached")
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"prompt": "test",
			"texts":  []string{"test"},
			"text":   "test",
			"labels": []string{"a"},
		})

		_, err := tool.Execute(context.Background(), input)
		if !errors.Is(err, ErrProviderNotConfigured) {
			t.Errorf("tool %s: expected ErrProviderNotConfigured, got %v", tool.Name(), err)
		}
	}
}

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider("test-mock")

	if provider.Name() != "test-mock" {
		t.Errorf("expected name 'test-mock', got %s", provider.Name())
	}

	ctx := context.Background()

	// Test Available
	if !provider.Available(ctx) {
		t.Error("expected provider to be available")
	}

	// Test Complete
	resp, err := provider.Complete(ctx, CompletionRequest{
		Model:  "test",
		Prompt: "Hello",
	})
	if err != nil {
		t.Errorf("Complete error: %v", err)
	}
	if resp.Text == "" {
		t.Error("expected non-empty completion text")
	}

	// Test Embed
	embedResp, err := provider.Embed(ctx, EmbedRequest{
		Model: "test",
		Texts: []string{"hello", "world"},
	})
	if err != nil {
		t.Errorf("Embed error: %v", err)
	}
	if len(embedResp.Embeddings) != 2 {
		t.Errorf("expected 2 embeddings, got %d", len(embedResp.Embeddings))
	}

	// Test Classify
	classifyResp, err := provider.Classify(ctx, ClassifyRequest{
		Model:  "test",
		Text:   "test",
		Labels: []string{"a", "b", "c"},
	})
	if err != nil {
		t.Errorf("Classify error: %v", err)
	}
	if len(classifyResp.Classifications) != 3 {
		t.Errorf("expected 3 classifications, got %d", len(classifyResp.Classifications))
	}
}

func TestUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Cost:             0.0015,
	}

	if usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
		t.Error("total tokens should equal prompt + completion tokens")
	}
}

func TestCompletionResponse_Latency(t *testing.T) {
	resp := CompletionResponse{
		Text:    "test",
		Latency: 100 * time.Millisecond,
	}

	if resp.Latency != 100*time.Millisecond {
		t.Errorf("expected latency 100ms, got %v", resp.Latency)
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.MaxTokensDefault != 1024 {
		t.Errorf("expected MaxTokensDefault 1024, got %d", cfg.MaxTokensDefault)
	}

	if cfg.TemperatureDefault != 0.7 {
		t.Errorf("expected TemperatureDefault 0.7, got %f", cfg.TemperatureDefault)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
