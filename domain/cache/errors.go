package cache

import "errors"

// Domain errors for cache operations.
var (
	// ErrKeyNotFound is returned when a key does not exist in the cache.
	ErrKeyNotFound = errors.New("cache key not found")

	// ErrCacheFull is returned when the cache is at capacity and cannot accept new entries.
	ErrCacheFull = errors.New("cache is full")

	// ErrInvalidKey is returned when a key is invalid (e.g., empty).
	ErrInvalidKey = errors.New("invalid cache key")

	// ErrConnectionFailed is returned when connection to the cache backend fails.
	ErrConnectionFailed = errors.New("cache connection failed")

	// ErrOperationTimeout is returned when a cache operation times out.
	ErrOperationTimeout = errors.New("cache operation timeout")
)
