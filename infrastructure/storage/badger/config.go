// Package badger provides BadgerDB-backed implementations of storage interfaces.
package badger

import (
	"errors"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Config configures BadgerDB storage.
type Config struct {
	// Dir is the directory to store data in.
	Dir string

	// InMemory uses in-memory storage (useful for testing).
	InMemory bool

	// SyncWrites enables synchronous writes for durability.
	SyncWrites bool

	// ValueLogFileSize sets the size of value log files in bytes.
	ValueLogFileSize int64

	// ValueLogMaxEntries sets the max entries in a value log file.
	ValueLogMaxEntries uint32

	// NumVersionsToKeep sets the number of versions to keep per key.
	NumVersionsToKeep int

	// GCDiscardRatio is the discard ratio for GC.
	GCDiscardRatio float64

	// GCInterval is the interval between GC runs.
	GCInterval time.Duration

	// KeyPrefix is added to all keys.
	KeyPrefix string

	// Logger is the logger to use (nil for default).
	Logger badger.Logger
}

// Option configures BadgerDB storage.
type Option func(*Config)

// WithDir sets the data directory.
func WithDir(dir string) Option {
	return func(c *Config) {
		c.Dir = dir
	}
}

// WithInMemory enables in-memory storage.
func WithInMemory() Option {
	return func(c *Config) {
		c.InMemory = true
	}
}

// WithSyncWrites enables synchronous writes.
func WithSyncWrites() Option {
	return func(c *Config) {
		c.SyncWrites = true
	}
}

// WithValueLogFileSize sets the value log file size.
func WithValueLogFileSize(size int64) Option {
	return func(c *Config) {
		c.ValueLogFileSize = size
	}
}

// WithNumVersionsToKeep sets the number of versions to keep.
func WithNumVersionsToKeep(n int) Option {
	return func(c *Config) {
		c.NumVersionsToKeep = n
	}
}

// WithGCDiscardRatio sets the GC discard ratio.
func WithGCDiscardRatio(ratio float64) Option {
	return func(c *Config) {
		c.GCDiscardRatio = ratio
	}
}

// WithGCInterval sets the GC interval.
func WithGCInterval(d time.Duration) Option {
	return func(c *Config) {
		c.GCInterval = d
	}
}

// WithKeyPrefix sets the key prefix.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}

// WithLogger sets the logger.
func WithLogger(logger badger.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Dir:                "",
		InMemory:           false,
		SyncWrites:         false,
		ValueLogFileSize:   1 << 28, // 256MB
		ValueLogMaxEntries: 1000000,
		NumVersionsToKeep:  1,
		GCDiscardRatio:     0.5,
		GCInterval:         5 * time.Minute,
	}
}

// Errors
var (
	ErrConnectionFailed = errors.New("badger: connection failed")
	ErrKeyNotFound      = errors.New("badger: key not found")
)

// openDB opens a BadgerDB database with the given configuration.
func openDB(cfg Config) (*badger.DB, error) {
	opts := badger.DefaultOptions(cfg.Dir)

	if cfg.InMemory {
		opts = opts.WithInMemory(true)
	}

	opts = opts.WithSyncWrites(cfg.SyncWrites)

	if cfg.ValueLogFileSize > 0 {
		opts = opts.WithValueLogFileSize(cfg.ValueLogFileSize)
	}

	if cfg.ValueLogMaxEntries > 0 {
		opts = opts.WithValueLogMaxEntries(cfg.ValueLogMaxEntries)
	}

	if cfg.NumVersionsToKeep > 0 {
		opts = opts.WithNumVersionsToKeep(cfg.NumVersionsToKeep)
	}

	if cfg.Logger != nil {
		opts = opts.WithLogger(cfg.Logger)
	} else {
		// Use a silent logger by default
		opts = opts.WithLogger(nil)
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.Join(ErrConnectionFailed, err)
	}

	return db, nil
}
