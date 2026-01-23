package llm

import (
	"context"
	"time"
)

// MockProvider is a mock LLM provider for testing.
type MockProvider struct {
	name string

	// CompleteFunc is called when Complete is invoked.
	CompleteFunc func(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// EmbedFunc is called when Embed is invoked.
	EmbedFunc func(ctx context.Context, req EmbedRequest) (EmbedResponse, error)

	// ClassifyFunc is called when Classify is invoked.
	ClassifyFunc func(ctx context.Context, req ClassifyRequest) (ClassifyResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name: name,
		CompleteFunc: func(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{
				Text:         "Mock completion response",
				Model:        req.Model,
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
				Latency: 100 * time.Millisecond,
			}, nil
		},
		EmbedFunc: func(_ context.Context, req EmbedRequest) (EmbedResponse, error) {
			embeddings := make([][]float64, len(req.Texts))
			dims := 1536
			if req.Dimensions > 0 {
				dims = req.Dimensions
			}
			for i := range embeddings {
				embeddings[i] = make([]float64, dims)
				for j := range embeddings[i] {
					embeddings[i][j] = float64(j) * 0.001
				}
			}
			return EmbedResponse{
				Embeddings: embeddings,
				Model:      req.Model,
				Dimensions: dims,
				Usage: Usage{
					PromptTokens: len(req.Texts) * 10,
					TotalTokens:  len(req.Texts) * 10,
				},
			}, nil
		},
		ClassifyFunc: func(_ context.Context, req ClassifyRequest) (ClassifyResponse, error) {
			classifications := make([]Classification, len(req.Labels))
			total := 0.0
			for i, label := range req.Labels {
				score := 1.0 / float64(len(req.Labels))
				if i == 0 {
					score = 0.7 // Make first label most likely
				}
				classifications[i] = Classification{
					Label: label,
					Score: score,
				}
				total += score
			}
			// Normalize scores
			for i := range classifications {
				classifications[i].Score /= total
			}
			return ClassifyResponse{
				Classifications: classifications,
				Model:           req.Model,
				Usage: Usage{
					PromptTokens: 20,
					TotalTokens:  20,
				},
			}, nil
		},
		AvailableFunc: func(_ context.Context) bool {
			return true
		},
	}
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// Complete generates a completion.
func (p *MockProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	return p.CompleteFunc(ctx, req)
}

// Embed generates embeddings.
func (p *MockProvider) Embed(ctx context.Context, req EmbedRequest) (EmbedResponse, error) {
	return p.EmbedFunc(ctx, req)
}

// Classify classifies text.
func (p *MockProvider) Classify(ctx context.Context, req ClassifyRequest) (ClassifyResponse, error) {
	return p.ClassifyFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
