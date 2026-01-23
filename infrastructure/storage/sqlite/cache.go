package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Cache is a SQLite-backed implementation of cache.Cache.
type Cache struct {
	db        *sql.DB
	keyPrefix string
	hits      atomic.Int64
	misses    atomic.Int64
}

// NewCache creates a new SQLite cache with the given configuration.
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
	}

	// Auto-migrate if enabled
	if cfg.AutoMigrate {
		if err := c.migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return c, nil
}

// NewCacheFromDB creates a cache from an existing database connection.
func NewCacheFromDB(db *sql.DB, keyPrefix string) (*Cache, error) {
	c := &Cache{
		db:        db,
		keyPrefix: keyPrefix,
	}

	if err := c.migrate(); err != nil {
		return nil, err
	}

	return c, nil
}

// migrate creates the cache table if it doesn't exist.
func (c *Cache) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL,
			expires_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_cache_expires_at ON cache(expires_at);
	`

	_, err := c.db.Exec(schema)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// prefixKey adds the key prefix.
func (c *Cache) prefixKey(key string) string {
	return c.keyPrefix + key
}

// Get retrieves a value from the cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	prefixedKey := c.prefixKey(key)
	now := time.Now().Unix()

	var value []byte
	var expiresAt sql.NullInt64

	err := c.db.QueryRowContext(ctx,
		"SELECT value, expires_at FROM cache WHERE key = ?",
		prefixedKey,
	).Scan(&value, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		c.misses.Add(1)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	// Check expiration
	if expiresAt.Valid && expiresAt.Int64 <= now {
		// Delete expired entry
		_, _ = c.db.ExecContext(ctx, "DELETE FROM cache WHERE key = ?", prefixedKey)
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

	prefixedKey := c.prefixKey(key)
	now := time.Now().Unix()

	var expiresAt sql.NullInt64
	if opts.TTL > 0 {
		expiresAt = sql.NullInt64{Int64: time.Now().Add(opts.TTL).Unix(), Valid: true}
	}

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO cache (key, value, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   value = excluded.value,
		   expires_at = excluded.expires_at,
		   updated_at = excluded.updated_at`,
		prefixedKey, value, expiresAt, now, now,
	)

	return err
}

// Delete removes a value from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	prefixedKey := c.prefixKey(key)
	_, err := c.db.ExecContext(ctx, "DELETE FROM cache WHERE key = ?", prefixedKey)
	return err
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	prefixedKey := c.prefixKey(key)
	now := time.Now().Unix()

	var exists int
	err := c.db.QueryRowContext(ctx,
		"SELECT 1 FROM cache WHERE key = ? AND (expires_at IS NULL OR expires_at > ?)",
		prefixedKey, now,
	).Scan(&exists)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var query string
	if c.keyPrefix != "" {
		query = "DELETE FROM cache WHERE key LIKE '" + c.keyPrefix + "%'"
	} else {
		query = "DELETE FROM cache"
	}

	_, err := c.db.ExecContext(ctx, query)
	return err
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var size int64
	_ = c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cache").Scan(&size)

	return cache.Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
		Size:   size,
	}
}

// Cleanup removes expired entries.
func (c *Cache) Cleanup(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	now := time.Now().Unix()
	result, err := c.db.ExecContext(ctx,
		"DELETE FROM cache WHERE expires_at IS NOT NULL AND expires_at <= ?",
		now,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Close closes the database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

// DB returns the underlying database connection.
func (c *Cache) DB() *sql.DB {
	return c.db
}

// Ensure Cache implements cache.Cache and cache.StatsProvider
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)
