package badger

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/felixgeelhaar/agent-go/domain/cache"
)

// Cache is a BadgerDB-backed implementation of cache.Cache.
type Cache struct {
	db        *badger.DB
	keyPrefix string
	hits      atomic.Int64
	misses    atomic.Int64
	gcStop    chan struct{}
	gcWg      sync.WaitGroup
}

// NewCache creates a new BadgerDB cache with the given configuration.
func NewCache(cfg Config, opts ...Option) (*Cache, error) {
	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}

	c := &Cache{
		db:        db,
		keyPrefix: cfg.KeyPrefix,
		gcStop:    make(chan struct{}),
	}

	// Start GC goroutine
	if cfg.GCInterval > 0 {
		c.startGC(cfg.GCInterval, cfg.GCDiscardRatio)
	}

	return c, nil
}

// NewCacheFromDB creates a cache from an existing BadgerDB database.
func NewCacheFromDB(db *badger.DB, keyPrefix string) *Cache {
	return &Cache{
		db:        db,
		keyPrefix: keyPrefix,
		gcStop:    make(chan struct{}),
	}
}

// startGC starts the garbage collection goroutine.
func (c *Cache) startGC(interval time.Duration, discardRatio float64) {
	c.gcWg.Add(1)
	go func() {
		defer c.gcWg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.gcStop:
				return
			case <-ticker.C:
				for {
					err := c.db.RunValueLogGC(discardRatio)
					if err != nil {
						break
					}
				}
			}
		}
	}()
}

// prefixKey adds the key prefix and cache namespace.
func (c *Cache) prefixKey(key string) []byte {
	return []byte(c.keyPrefix + "cache:" + key)
}

// Get retrieves a value from the cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	prefixedKey := c.prefixKey(key)
	var value []byte

	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(prefixedKey)
		if err != nil {
			return err
		}

		value, err = item.ValueCopy(nil)
		return err
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		c.misses.Add(1)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
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

	prefixedKey := c.prefixKey(key)

	return c.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(prefixedKey, value)

		if opts.TTL > 0 {
			e = e.WithTTL(opts.TTL)
		}

		return txn.SetEntry(e)
	})
}

// Delete removes a value from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	prefixedKey := c.prefixKey(key)

	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(prefixedKey)
	})
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	prefixedKey := c.prefixKey(key)

	err := c.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(prefixedKey)
		return err
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// Clear removes all entries with the cache prefix.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	prefix := []byte(c.keyPrefix + "cache:")

	return c.db.DropPrefix(prefix)
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	var size int64

	_ = c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(c.keyPrefix + "cache:")

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			size++
		}
		return nil
	})

	return cache.Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
		Size:   size,
	}
}

// Close closes the database.
func (c *Cache) Close() error {
	// Stop GC goroutine
	close(c.gcStop)
	c.gcWg.Wait()

	return c.db.Close()
}

// DB returns the underlying BadgerDB database.
func (c *Cache) DB() *badger.DB {
	return c.db
}

// GetWithMeta retrieves a value with its metadata.
func (c *Cache) GetWithMeta(ctx context.Context, key string) ([]byte, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}

	prefixedKey := c.prefixKey(key)
	var value []byte
	var expiresAt time.Time

	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(prefixedKey)
		if err != nil {
			return err
		}

		value, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}

		// Get expiration time
		if item.ExpiresAt() > 0 {
			expiresAt = time.Unix(int64(item.ExpiresAt()), 0)
		}

		return nil
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		c.misses.Add(1)
		return nil, time.Time{}, cache.ErrKeyNotFound
	}
	if err != nil {
		return nil, time.Time{}, err
	}

	c.hits.Add(1)
	return value, expiresAt, nil
}

// SetNX sets a value only if the key doesn't exist.
func (c *Cache) SetNX(ctx context.Context, key string, value []byte, opts cache.SetOptions) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	if key == "" {
		return false, cache.ErrInvalidKey
	}

	prefixedKey := c.prefixKey(key)
	set := false

	err := c.db.Update(func(txn *badger.Txn) error {
		// Check if key exists
		_, err := txn.Get(prefixedKey)
		if err == nil {
			// Key exists
			return nil
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		// Key doesn't exist, set it
		e := badger.NewEntry(prefixedKey, value)
		if opts.TTL > 0 {
			e = e.WithTTL(opts.TTL)
		}

		set = true
		return txn.SetEntry(e)
	})

	return set, err
}

// Increment atomically increments a numeric value.
func (c *Cache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if key == "" {
		return 0, cache.ErrInvalidKey
	}

	prefixedKey := c.prefixKey(key)
	var newValue int64

	err := c.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(prefixedKey)
		if errors.Is(err, badger.ErrKeyNotFound) {
			// Initialize to delta
			newValue = delta
		} else if err != nil {
			return err
		} else {
			// Read existing value
			err = item.Value(func(val []byte) error {
				if len(val) != 8 {
					return errors.New("invalid numeric value")
				}
				currentValue := int64(binary.BigEndian.Uint64(val))
				newValue = currentValue + delta
				return nil
			})
			if err != nil {
				return err
			}
		}

		// Write new value
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(newValue))
		return txn.Set(prefixedKey, buf)
	})

	return newValue, err
}

// Keys returns all cache keys matching the given prefix.
func (c *Cache) Keys(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPrefix := []byte(c.keyPrefix + "cache:" + prefix)
	prefixLen := len(c.keyPrefix) + 6 // len("cache:")

	var keys []string

	err := c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = fullPrefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key()[prefixLen:])
			keys = append(keys, key)
		}
		return nil
	})

	return keys, err
}

// Ensure Cache implements cache.Cache and cache.StatsProvider
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)
