// Package etcd provides etcd-based distributed storage implementations.
package etcd

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
)

// Client defines the interface for etcd client operations.
// This allows for mock implementations in testing.
type Client interface {
	// Get retrieves a value by key.
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Put stores a value with optional lease for TTL.
	Put(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key.
	Delete(ctx context.Context, key string) error

	// DeletePrefix removes all keys with the given prefix.
	DeletePrefix(ctx context.Context, prefix string) error

	// Close closes the client connection.
	Close() error
}

// Cache is an etcd-backed implementation of cache.Cache.
type Cache struct {
	client    Client
	keyPrefix string
	hits      atomic.Int64
	misses    atomic.Int64
}

// Config holds configuration for the etcd cache.
type Config struct {
	// Client is the etcd client to use.
	Client Client

	// KeyPrefix is the prefix for all cache keys.
	KeyPrefix string
}

// NewCache creates a new etcd cache.
func NewCache(cfg Config) (*Cache, error) {
	if cfg.Client == nil {
		return nil, errors.New("etcd client is required")
	}

	return &Cache{
		client:    cfg.Client,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

// prefixKey adds the key prefix.
func (c *Cache) prefixKey(key string) string {
	return c.keyPrefix + "cache/" + key
}

// Get retrieves a value from the cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	value, found, err := c.client.Get(ctx, c.prefixKey(key))
	if err != nil {
		return nil, false, err
	}

	if !found {
		c.misses.Add(1)
		return nil, false, nil
	}

	c.hits.Add(1)
	return value, true, nil
}

// Set stores a value in the cache.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" {
		return cache.ErrInvalidKey
	}

	return c.client.Put(ctx, c.prefixKey(key), value, opts.TTL)
}

// Delete removes a value from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return c.client.Delete(ctx, c.prefixKey(key))
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	_, found, err := c.client.Get(ctx, c.prefixKey(key))
	return found, err
}

// Clear removes all entries with the cache prefix.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return c.client.DeletePrefix(ctx, c.keyPrefix+"cache/")
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	return cache.Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
	}
}

// Close closes the etcd connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Ensure Cache implements cache.Cache and cache.StatsProvider
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)

// MockClient is a mock etcd client for testing.
type MockClient struct {
	mu     sync.RWMutex
	data   map[string]cacheEntry
}

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// NewMockClient creates a new mock etcd client.
func NewMockClient() *MockClient {
	return &MockClient{
		data: make(map[string]cacheEntry),
	}
}

// Get implements Client.Get.
func (c *MockClient) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[key]
	if !ok {
		return nil, false, nil
	}

	// Check expiration
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return nil, false, nil
	}

	// Return a copy to prevent mutation
	value := make([]byte, len(entry.value))
	copy(value, entry.value)
	return value, true, nil
}

// Put implements Client.Put.
func (c *MockClient) Put(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := cacheEntry{
		value: make([]byte, len(value)),
	}
	copy(entry.value, value)

	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}

	c.data[key] = entry
	return nil
}

// Delete implements Client.Delete.
func (c *MockClient) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	return nil
}

// DeletePrefix implements Client.DeletePrefix.
func (c *MockClient) DeletePrefix(_ context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.data, key)
		}
	}
	return nil
}

// Close implements Client.Close.
func (c *MockClient) Close() error {
	return nil
}

// KeyCount returns the number of keys (for testing).
func (c *MockClient) KeyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// HasKey checks if a key exists (for testing).
func (c *MockClient) HasKey(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.data[key]
	return ok
}

// GetValue returns the raw value for a key (for testing).
func (c *MockClient) GetValue(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.data[key]
	if !ok {
		return nil, false
	}
	return entry.value, true
}

// Ensure MockClient implements Client
var _ Client = (*MockClient)(nil)
