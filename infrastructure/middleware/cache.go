package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Cache provides in-memory caching for tool results.
type Cache struct {
	entries map[string]tool.Result
	mu      sync.RWMutex
	maxSize int
}

// NewCache creates a new cache with the specified maximum entries.
func NewCache(maxEntries int) *Cache {
	return &Cache{
		entries: make(map[string]tool.Result),
		maxSize: maxEntries,
	}
}

// Get retrieves a cached result by key.
func (c *Cache) Get(key string) (tool.Result, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.entries[key]
	return result, ok
}

// Set stores a result in the cache.
func (c *Cache) Set(key string, result tool.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) < c.maxSize {
		c.entries[key] = result
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]tool.Result)
}

// Len returns the number of cached entries.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Caching returns middleware that caches cacheable tool results.
// Only tools marked as cacheable (via annotations) will be cached.
func Caching(cache *Cache) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if cache not provided
			if cache == nil {
				return next(ctx, execCtx)
			}

			// Only cache if tool is cacheable
			if !execCtx.Tool.Annotations().CanCache() {
				return next(ctx, execCtx)
			}

			// Generate cache key
			key := cacheKey(execCtx.Tool.Name(), execCtx.Input)

			// Check cache
			if result, ok := cache.Get(key); ok {
				result.Cached = true
				return result, nil
			}

			// Execute
			result, err := next(ctx, execCtx)
			if err != nil {
				return result, err
			}

			// Store in cache
			cache.Set(key, result)

			return result, nil
		}
	}
}

// cacheKey generates a unique key for a tool invocation.
func cacheKey(toolName string, input []byte) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte(":"))
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}
