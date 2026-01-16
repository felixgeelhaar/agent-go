package knowledge

import "errors"

// Domain errors for knowledge storage.
var (
	// ErrNotFound indicates the requested vector was not found.
	ErrNotFound = errors.New("vector not found")

	// ErrInvalidID indicates the vector ID is empty or invalid.
	ErrInvalidID = errors.New("invalid vector ID")

	// ErrInvalidEmbedding indicates the embedding is empty or invalid.
	ErrInvalidEmbedding = errors.New("invalid embedding")

	// ErrDimensionMismatch indicates the embedding dimension doesn't match the store's dimension.
	ErrDimensionMismatch = errors.New("embedding dimension mismatch")
)
