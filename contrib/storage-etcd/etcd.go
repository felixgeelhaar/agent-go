// Package etcd provides etcd-backed implementations of agent-go storage interfaces.
//
// etcd is a distributed, reliable key-value store for the most critical data of a
// distributed system. It provides strong consistency guarantees and is commonly used
// for configuration management, service discovery, and coordination.
//
// # Usage
//
//	client, err := clientv3.New(clientv3.Config{
//		Endpoints:   []string{"localhost:2379"},
//		DialTimeout: 5 * time.Second,
//	})
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	cache := etcd.NewCache(client)
package etcd

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/cache"
)

// Client represents an etcd client interface.
// This allows for mocking in tests.
type Client interface {
	Close() error
}

// Cache is an etcd-backed implementation of cache.Cache.
// It stores cached values as key-value pairs in etcd with optional lease-based TTL.
type Cache struct {
	client    Client
	keyPrefix string
}

// CacheConfig holds configuration for the etcd cache.
type CacheConfig struct {
	// KeyPrefix is an optional prefix for all cache keys.
	KeyPrefix string
}

// NewCache creates a new etcd cache with the given client.
func NewCache(client Client) *Cache {
	return &Cache{
		client:    client,
		keyPrefix: "agent/cache/",
	}
}

// NewCacheWithConfig creates a new etcd cache with full configuration.
func NewCacheWithConfig(client Client, cfg CacheConfig) *Cache {
	prefix := cfg.KeyPrefix
	if prefix == "" {
		prefix = "agent/cache/"
	}
	return &Cache{
		client:    client,
		keyPrefix: prefix,
	}
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// TODO: Implement etcd Get operation
	// 1. Build full key with prefix
	// 2. Execute Get request
	// 3. Check if key exists (count > 0)
	// 4. Return value from first Kv
	return nil, false, nil
}

// Set stores a value with the given key and options.
// TTL is implemented using etcd leases.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	// TODO: Implement etcd Put operation with optional lease
	// 1. If TTL > 0, create lease with Grant
	// 2. Build full key with prefix
	// 3. Execute Put request with lease ID (if applicable)
	return nil
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	// TODO: Implement etcd Delete operation
	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	// TODO: Implement etcd existence check using CountOnly option
	return false, nil
}

// Clear removes all entries from the cache with the configured prefix.
// Uses etcd's prefix delete for atomic removal.
func (c *Cache) Clear(ctx context.Context) error {
	// TODO: Implement etcd Delete with prefix option (WithPrefix)
	return nil
}

// Close closes the underlying etcd client connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// prefixKey returns the full key with prefix.
func (c *Cache) prefixKey(key string) string {
	return c.keyPrefix + key
}

// Ensure interface is implemented.
var _ cache.Cache = (*Cache)(nil)
