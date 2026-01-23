// Package sqlite provides SQLite-backed implementations of storage interfaces.
package sqlite

import (
	"database/sql"
	"errors"
	"time"
)

// Config configures SQLite storage.
type Config struct {
	// DSN is the data source name (e.g., "file:test.db?cache=shared&mode=rwc").
	DSN string

	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum connection lifetime.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum idle time for connections.
	ConnMaxIdleTime time.Duration

	// AutoMigrate automatically creates tables if they don't exist.
	AutoMigrate bool

	// JournalMode sets the SQLite journal mode (e.g., "WAL").
	JournalMode string

	// BusyTimeout sets the busy timeout in milliseconds.
	BusyTimeout int

	// KeyPrefix is added to all keys (useful for multi-tenant scenarios).
	KeyPrefix string
}

// Option configures SQLite storage.
type Option func(*Config)

// WithDSN sets the data source name.
func WithDSN(dsn string) Option {
	return func(c *Config) {
		c.DSN = dsn
	}
}

// WithMaxOpenConns sets the maximum open connections.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) {
		c.MaxOpenConns = n
	}
}

// WithMaxIdleConns sets the maximum idle connections.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) {
		c.MaxIdleConns = n
	}
}

// WithConnMaxLifetime sets the connection max lifetime.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxLifetime = d
	}
}

// WithAutoMigrate enables automatic table creation.
func WithAutoMigrate() Option {
	return func(c *Config) {
		c.AutoMigrate = true
	}
}

// WithJournalMode sets the SQLite journal mode.
func WithJournalMode(mode string) Option {
	return func(c *Config) {
		c.JournalMode = mode
	}
}

// WithBusyTimeout sets the busy timeout in milliseconds.
func WithBusyTimeout(ms int) Option {
	return func(c *Config) {
		c.BusyTimeout = ms
	}
}

// WithKeyPrefix sets the key prefix.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DSN:             "file:agent.db?cache=shared&mode=rwc",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
		AutoMigrate:     true,
		JournalMode:     "WAL",
		BusyTimeout:     5000,
	}
}

// Errors
var (
	ErrConnectionFailed = errors.New("sqlite: connection failed")
	ErrMigrationFailed  = errors.New("sqlite: migration failed")
)

// openDB opens a SQLite database with the given configuration.
func openDB(cfg Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.DSN)
	if err != nil {
		return nil, errors.Join(ErrConnectionFailed, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Set pragmas
	pragmas := []string{}

	if cfg.JournalMode != "" {
		pragmas = append(pragmas, "PRAGMA journal_mode="+cfg.JournalMode)
	}

	if cfg.BusyTimeout > 0 {
		pragmas = append(pragmas, "PRAGMA busy_timeout="+string(rune(cfg.BusyTimeout)))
	}

	// Enable foreign keys
	pragmas = append(pragmas, "PRAGMA foreign_keys=ON")

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, errors.Join(ErrMigrationFailed, err)
		}
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, errors.Join(ErrConnectionFailed, err)
	}

	return db, nil
}
