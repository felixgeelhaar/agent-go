package vectordb

import (
	"context"
	"math"
	"sort"
)

// MockProvider is a mock vector database provider for testing.
type MockProvider struct {
	name string

	// UpsertFunc is called when Upsert is invoked.
	UpsertFunc func(ctx context.Context, req UpsertRequest) (UpsertResponse, error)

	// QueryFunc is called when Query is invoked.
	QueryFunc func(ctx context.Context, req QueryRequest) (QueryResponse, error)

	// DeleteFunc is called when Delete is invoked.
	DeleteFunc func(ctx context.Context, req DeleteRequest) (DeleteResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool

	// Internal storage for default implementation
	storage map[string]map[string]Vector // collection -> id -> vector
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	p := &MockProvider{
		name:    name,
		storage: make(map[string]map[string]Vector),
	}

	p.UpsertFunc = p.defaultUpsert
	p.QueryFunc = p.defaultQuery
	p.DeleteFunc = p.defaultDelete
	p.AvailableFunc = func(_ context.Context) bool { return true }

	return p
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// Upsert inserts or updates vectors.
func (p *MockProvider) Upsert(ctx context.Context, req UpsertRequest) (UpsertResponse, error) {
	return p.UpsertFunc(ctx, req)
}

// Query searches for similar vectors.
func (p *MockProvider) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	return p.QueryFunc(ctx, req)
}

// Delete removes vectors.
func (p *MockProvider) Delete(ctx context.Context, req DeleteRequest) (DeleteResponse, error) {
	return p.DeleteFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

func (p *MockProvider) defaultUpsert(_ context.Context, req UpsertRequest) (UpsertResponse, error) {
	if req.Collection == "" {
		return UpsertResponse{}, ErrInvalidInput
	}

	if _, ok := p.storage[req.Collection]; !ok {
		p.storage[req.Collection] = make(map[string]Vector)
	}

	for _, v := range req.Vectors {
		p.storage[req.Collection][v.ID] = v
	}

	return UpsertResponse{UpsertedCount: len(req.Vectors)}, nil
}

func (p *MockProvider) defaultQuery(_ context.Context, req QueryRequest) (QueryResponse, error) {
	if req.Collection == "" || len(req.Vector) == 0 {
		return QueryResponse{}, ErrInvalidInput
	}

	collection, ok := p.storage[req.Collection]
	if !ok {
		return QueryResponse{Matches: []Match{}}, nil
	}

	type scoredVector struct {
		id       string
		score    float64
		vector   Vector
		metadata map[string]interface{}
	}

	var scored []scoredVector
	for id, v := range collection {
		score := cosineSimilarity(req.Vector, v.Values)
		scored = append(scored, scoredVector{
			id:       id,
			score:    score,
			vector:   v,
			metadata: v.Metadata,
		})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	if topK > len(scored) {
		topK = len(scored)
	}

	matches := make([]Match, topK)
	for i := 0; i < topK; i++ {
		matches[i] = Match{
			ID:    scored[i].id,
			Score: scored[i].score,
		}
		if req.IncludeValues {
			matches[i].Values = scored[i].vector.Values
		}
		if req.IncludeMetadata {
			matches[i].Metadata = scored[i].metadata
		}
	}

	return QueryResponse{
		Matches:   matches,
		Namespace: req.Namespace,
	}, nil
}

func (p *MockProvider) defaultDelete(_ context.Context, req DeleteRequest) (DeleteResponse, error) {
	if req.Collection == "" {
		return DeleteResponse{}, ErrInvalidInput
	}

	collection, ok := p.storage[req.Collection]
	if !ok {
		return DeleteResponse{DeletedCount: 0}, nil
	}

	if req.DeleteAll {
		count := len(collection)
		delete(p.storage, req.Collection)
		return DeleteResponse{DeletedCount: count}, nil
	}

	count := 0
	for _, id := range req.IDs {
		if _, ok := collection[id]; ok {
			delete(collection, id)
			count++
		}
	}

	return DeleteResponse{DeletedCount: count}, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
