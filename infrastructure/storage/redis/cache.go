package redis

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/redis/go-redis/v9"
)

// Cache is a Redis-backed implementation of cache.Cache.
type Cache struct {
	client    *redis.Client
	keyPrefix string
	hits      atomic.Int64
	misses    atomic.Int64
}

// NewCache creates a new Redis cache with the given configuration.
func NewCache(cfg Config, opts ...ConfigOption) (*Cache, error) {
	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(cache.ErrConnectionFailed, err)
	}

	return &Cache{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

// NewCacheFromClient creates a cache from an existing Redis client.
func NewCacheFromClient(client *redis.Client, keyPrefix string) *Cache {
	return &Cache{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// prefixKey adds the key prefix.
func (c *Cache) prefixKey(key string) string {
	return c.keyPrefix + "cache:" + key
}

// Get retrieves a value from the cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	result, err := c.client.Get(ctx, c.prefixKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.misses.Add(1)
			return nil, false, nil
		}
		return nil, false, c.wrapError(err)
	}

	c.hits.Add(1)
	return result, true, nil
}

// Set stores a value in the cache.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" {
		return cache.ErrInvalidKey
	}

	var expiration time.Duration
	if opts.TTL > 0 {
		expiration = opts.TTL
	}

	err := c.client.Set(ctx, c.prefixKey(key), value, expiration).Err()
	if err != nil {
		return c.wrapError(err)
	}

	return nil
}

// Delete removes a value from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := c.client.Del(ctx, c.prefixKey(key)).Err()
	if err != nil {
		return c.wrapError(err)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	result, err := c.client.Exists(ctx, c.prefixKey(key)).Result()
	if err != nil {
		return false, c.wrapError(err)
	}

	return result > 0, nil
}

// Clear removes all entries with the cache prefix.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Use SCAN to find all keys with our prefix
	pattern := c.keyPrefix + "cache:*"
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		// Delete in batches of 100
		if len(keys) >= 100 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return c.wrapError(err)
			}
			keys = keys[:0]
		}
	}

	if err := iter.Err(); err != nil {
		return c.wrapError(err)
	}

	// Delete remaining keys
	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return c.wrapError(err)
		}
	}

	return nil
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	return cache.Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
		// Size and MaxSize are not tracked for Redis
	}
}

// Close closes the Redis connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Ping checks the Redis connection.
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client returns the underlying Redis client for advanced operations.
func (c *Cache) Client() *redis.Client {
	return c.client
}

// wrapError wraps Redis errors with domain errors.
func (c *Cache) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// Check for timeout
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	// Connection errors
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	return err
}

// Ensure Cache implements cache.Cache and cache.StatsProvider
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)
