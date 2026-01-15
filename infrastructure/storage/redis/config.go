// Package redis provides Redis-backed storage implementations.
package redis

import (
	"time"
)

// Config holds Redis connection configuration.
type Config struct {
	// Address is the Redis server address (host:port).
	Address string

	// Password for authentication (optional).
	Password string

	// DB selects the Redis database index.
	DB int

	// MaxRetries is the maximum number of retries before giving up.
	MaxRetries int

	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes.
	WriteTimeout time.Duration

	// PoolSize is the maximum number of socket connections.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int

	// KeyPrefix is prepended to all keys (for namespacing).
	KeyPrefix string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Address:      "localhost:6379",
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		KeyPrefix:    "agent:",
	}
}

// ConfigOption configures the Redis connection.
type ConfigOption func(*Config)

// WithAddress sets the Redis server address.
func WithAddress(addr string) ConfigOption {
	return func(c *Config) {
		c.Address = addr
	}
}

// WithPassword sets the authentication password.
func WithPassword(password string) ConfigOption {
	return func(c *Config) {
		c.Password = password
	}
}

// WithDB sets the database index.
func WithDB(db int) ConfigOption {
	return func(c *Config) {
		c.DB = db
	}
}

// WithKeyPrefix sets the key prefix for namespacing.
func WithKeyPrefix(prefix string) ConfigOption {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}

// WithPoolSize sets the connection pool size.
func WithPoolSize(size int) ConfigOption {
	return func(c *Config) {
		c.PoolSize = size
	}
}

// WithTimeouts sets connection timeouts.
func WithTimeouts(dial, read, write time.Duration) ConfigOption {
	return func(c *Config) {
		c.DialTimeout = dial
		c.ReadTimeout = read
		c.WriteTimeout = write
	}
}
