// Package knowledge provides domain models for vector knowledge storage.
// This enables RAG (Retrieval-Augmented Generation) capabilities by allowing
// agents to store and retrieve knowledge based on semantic similarity.
package knowledge

import (
	"context"
	"time"
)

// Vector represents an embedding with associated text and metadata.
type Vector struct {
	ID        string            `json:"id"`
	Embedding []float32         `json:"embedding"`
	Text      string            `json:"text"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// SearchResult represents a similarity search result.
type SearchResult struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Score    float32           `json:"score"` // Cosine similarity [0,1]
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ListFilter provides filtering options for list operations.
type ListFilter struct {
	IDPrefix string            // Filter by ID prefix
	Metadata map[string]string // All keys must match
	FromTime time.Time         // Filter vectors created after this time
	ToTime   time.Time         // Filter vectors created before this time
	Limit    int               // Maximum number of results
	Offset   int               // Skip first N results
}

// Store defines the interface for vector knowledge storage.
type Store interface {
	// Upsert stores or updates a vector.
	Upsert(ctx context.Context, vector *Vector) error

	// Search finds similar vectors by embedding using cosine similarity.
	// Returns results sorted by similarity score (highest first).
	Search(ctx context.Context, embedding []float32, topK int) ([]SearchResult, error)

	// Get retrieves a vector by ID.
	Get(ctx context.Context, id string) (*Vector, error)

	// Delete removes a vector by ID.
	Delete(ctx context.Context, id string) error

	// List returns vectors matching the filter criteria.
	List(ctx context.Context, filter ListFilter) ([]*Vector, error)

	// Count returns the total number of vectors in the store.
	Count(ctx context.Context) (int64, error)
}

// Stats provides statistics about the store.
type Stats struct {
	VectorCount int64 `json:"vector_count"`
	Dimension   int   `json:"dimension"`
}

// StatsProvider is an optional interface for stores that can provide statistics.
type StatsProvider interface {
	Stats(ctx context.Context) (Stats, error)
}

// BatchStore is an optional interface for stores that support batch operations.
type BatchStore interface {
	Store
	UpsertBatch(ctx context.Context, vectors []*Vector) error
	DeleteBatch(ctx context.Context, ids []string) error
}
