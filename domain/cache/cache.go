// Package cache provides the domain interface for tool result caching.
package cache

import (
	"context"
	"time"
)

// Cache defines the interface for tool result caching.
// Implementations may be in-memory, Redis, or any other backend.
type Cache interface {
	// Get retrieves a cached value by key.
	// Returns the value, whether it was found, and any error.
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Set stores a value with the given key and options.
	Set(ctx context.Context, key string, value []byte, opts SetOptions) error

	// Delete removes a cached entry by key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error
}

// SetOptions configures how a value is stored in the cache.
type SetOptions struct {
	// TTL is the time-to-live for the cached entry.
	// Zero means no expiration.
	TTL time.Duration
}

// Stats provides cache statistics.
type Stats struct {
	// Hits is the number of cache hits.
	Hits int64
	// Misses is the number of cache misses.
	Misses int64
	// Size is the current number of entries.
	Size int64
	// MaxSize is the maximum number of entries (0 = unlimited).
	MaxSize int64
}

// StatsProvider is an optional interface for caches that support statistics.
type StatsProvider interface {
	// Stats returns current cache statistics.
	Stats() Stats
}
