// Package postgres provides PostgreSQL-backed storage implementations.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL connection configuration.
type Config struct {
	// Host is the database server hostname.
	Host string

	// Port is the database server port.
	Port int

	// Database is the database name.
	Database string

	// User is the database username.
	User string

	// Password is the database password.
	Password string

	// SSLMode configures SSL (disable, require, verify-ca, verify-full).
	SSLMode string

	// MaxConns is the maximum number of connections in the pool.
	MaxConns int32

	// MinConns is the minimum number of connections in the pool.
	MinConns int32

	// MaxConnLifetime is the maximum lifetime of a connection.
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum idle time for a connection.
	MaxConnIdleTime time.Duration

	// ConnectTimeout is the timeout for establishing connections.
	ConnectTimeout time.Duration

	// Schema is the schema to use for tables (defaults to "public").
	Schema string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		Database:        "agent",
		User:            "postgres",
		SSLMode:         "disable",
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
		Schema:          "public",
	}
}

// ConnectionString returns a PostgreSQL connection string.
func (c Config) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.Database, c.User, c.Password, c.SSLMode,
	)
}

// ConfigOption configures the PostgreSQL connection.
type ConfigOption func(*Config)

// WithHost sets the database host.
func WithHost(host string) ConfigOption {
	return func(c *Config) {
		c.Host = host
	}
}

// WithPort sets the database port.
func WithPort(port int) ConfigOption {
	return func(c *Config) {
		c.Port = port
	}
}

// WithDatabase sets the database name.
func WithDatabase(db string) ConfigOption {
	return func(c *Config) {
		c.Database = db
	}
}

// WithCredentials sets the database credentials.
func WithCredentials(user, password string) ConfigOption {
	return func(c *Config) {
		c.User = user
		c.Password = password
	}
}

// WithSSLMode sets the SSL mode.
func WithSSLMode(mode string) ConfigOption {
	return func(c *Config) {
		c.SSLMode = mode
	}
}

// WithPoolSize sets the connection pool size.
func WithPoolSize(min, max int32) ConfigOption {
	return func(c *Config) {
		c.MinConns = min
		c.MaxConns = max
	}
}

// WithSchema sets the schema to use.
func WithSchema(schema string) ConfigOption {
	return func(c *Config) {
		c.Schema = schema
	}
}

// NewPool creates a new connection pool with the given configuration.
func NewPool(ctx context.Context, cfg Config, opts ...ConfigOption) (*pgxpool.Pool, error) {
	for _, opt := range opts {
		opt(&cfg)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Join(errors.New("connection failed"), err)
	}

	return pool, nil
}
