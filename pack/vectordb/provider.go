// Package vectordb provides tools for interacting with vector databases.
package vectordb

import (
	"context"
	"errors"
)

// Common errors for vector database operations.
var (
	ErrProviderNotConfigured = errors.New("vector database provider not configured")
	ErrCollectionNotFound    = errors.New("collection not found")
	ErrVectorNotFound        = errors.New("vector not found")
	ErrInvalidInput          = errors.New("invalid input")
	ErrDimensionMismatch     = errors.New("vector dimension mismatch")
	ErrProviderUnavailable   = errors.New("provider unavailable")
)

// Provider defines the interface for vector database providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// Upsert inserts or updates vectors in the collection.
	Upsert(ctx context.Context, req UpsertRequest) (UpsertResponse, error)

	// Query searches for similar vectors.
	Query(ctx context.Context, req QueryRequest) (QueryResponse, error)

	// Delete removes vectors by ID.
	Delete(ctx context.Context, req DeleteRequest) (DeleteResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// UpsertRequest represents a request to insert or update vectors.
type UpsertRequest struct {
	// Collection is the target collection name.
	Collection string `json:"collection"`

	// Vectors are the vectors to upsert.
	Vectors []Vector `json:"vectors"`

	// Namespace is an optional namespace within the collection.
	Namespace string `json:"namespace,omitempty"`
}

// Vector represents a single vector with metadata.
type Vector struct {
	// ID is the unique identifier for this vector.
	ID string `json:"id"`

	// Values are the vector dimensions.
	Values []float64 `json:"values"`

	// Metadata is optional key-value data associated with the vector.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UpsertResponse represents the result of an upsert operation.
type UpsertResponse struct {
	// UpsertedCount is the number of vectors upserted.
	UpsertedCount int `json:"upserted_count"`
}

// QueryRequest represents a vector similarity search request.
type QueryRequest struct {
	// Collection is the collection to search.
	Collection string `json:"collection"`

	// Vector is the query vector.
	Vector []float64 `json:"vector"`

	// TopK is the number of results to return.
	TopK int `json:"top_k"`

	// Namespace is an optional namespace filter.
	Namespace string `json:"namespace,omitempty"`

	// Filter is optional metadata filter.
	Filter map[string]interface{} `json:"filter,omitempty"`

	// IncludeValues includes vector values in results.
	IncludeValues bool `json:"include_values,omitempty"`

	// IncludeMetadata includes metadata in results.
	IncludeMetadata bool `json:"include_metadata,omitempty"`
}

// QueryResponse represents the results of a similarity search.
type QueryResponse struct {
	// Matches are the similar vectors found.
	Matches []Match `json:"matches"`

	// Namespace is the namespace that was searched.
	Namespace string `json:"namespace,omitempty"`
}

// Match represents a single search result.
type Match struct {
	// ID is the vector identifier.
	ID string `json:"id"`

	// Score is the similarity score.
	Score float64 `json:"score"`

	// Values are the vector values (if requested).
	Values []float64 `json:"values,omitempty"`

	// Metadata is the vector metadata (if requested).
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DeleteRequest represents a request to delete vectors.
type DeleteRequest struct {
	// Collection is the collection to delete from.
	Collection string `json:"collection"`

	// IDs are the vector IDs to delete.
	IDs []string `json:"ids,omitempty"`

	// Namespace is an optional namespace filter.
	Namespace string `json:"namespace,omitempty"`

	// Filter is optional metadata filter for batch delete.
	Filter map[string]interface{} `json:"filter,omitempty"`

	// DeleteAll deletes all vectors in the collection/namespace.
	DeleteAll bool `json:"delete_all,omitempty"`
}

// DeleteResponse represents the result of a delete operation.
type DeleteResponse struct {
	// DeletedCount is the number of vectors deleted.
	DeletedCount int `json:"deleted_count"`
}
