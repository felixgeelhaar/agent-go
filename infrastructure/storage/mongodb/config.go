// Package mongodb provides MongoDB-backed storage implementations.
package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Config contains MongoDB connection configuration.
type Config struct {
	// URI is the MongoDB connection string.
	URI string

	// Database is the database name.
	Database string

	// ConnectTimeout is the timeout for initial connection.
	ConnectTimeout time.Duration

	// QueryTimeout is the default timeout for queries.
	QueryTimeout time.Duration

	// MaxPoolSize is the maximum connection pool size.
	MaxPoolSize uint64

	// MinPoolSize is the minimum connection pool size.
	MinPoolSize uint64
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		URI:            "mongodb://localhost:27017",
		Database:       "agent",
		ConnectTimeout: 10 * time.Second,
		QueryTimeout:   30 * time.Second,
		MaxPoolSize:    100,
		MinPoolSize:    10,
	}
}

// ConfigOption configures the MongoDB connection.
type ConfigOption func(*Config)

// WithURI sets the MongoDB connection URI.
func WithURI(uri string) ConfigOption {
	return func(c *Config) {
		c.URI = uri
	}
}

// WithDatabase sets the database name.
func WithDatabase(db string) ConfigOption {
	return func(c *Config) {
		c.Database = db
	}
}

// WithConnectTimeout sets the connection timeout.
func WithConnectTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.ConnectTimeout = d
	}
}

// WithQueryTimeout sets the default query timeout.
func WithQueryTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.QueryTimeout = d
	}
}

// WithMaxPoolSize sets the maximum pool size.
func WithMaxPoolSize(size uint64) ConfigOption {
	return func(c *Config) {
		c.MaxPoolSize = size
	}
}

// WithMinPoolSize sets the minimum pool size.
func WithMinPoolSize(size uint64) ConfigOption {
	return func(c *Config) {
		c.MinPoolSize = size
	}
}

// Client wraps a MongoDB client with configuration.
type Client struct {
	client   *mongo.Client
	database *mongo.Database
	config   Config
}

// NewClient creates a new MongoDB client.
func NewClient(ctx context.Context, opts ...ConfigOption) (*Client, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	clientOpts := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize)

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		return nil, err
	}

	// Verify connection
	pingCtx, pingCancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer pingCancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, err
	}

	return &Client{
		client:   client,
		database: client.Database(cfg.Database),
		config:   cfg,
	}, nil
}

// Database returns the configured database.
func (c *Client) Database() *mongo.Database {
	return c.database
}

// Collection returns a collection from the database.
func (c *Client) Collection(name string) *mongo.Collection {
	return c.database.Collection(name)
}

// Close disconnects from MongoDB.
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// CreateIndexes creates the recommended indexes for agent-go collections.
func (c *Client) CreateIndexes(ctx context.Context) error {
	// Runs collection indexes
	runsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "start_time", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "current_state", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "start_time", Value: -1},
			},
		},
	}

	_, err := c.Collection("runs").Indexes().CreateMany(ctx, runsIndexes)
	if err != nil {
		return err
	}

	// Events collection indexes
	eventsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "run_id", Value: 1},
				{Key: "sequence", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "run_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
			},
		},
	}

	_, err = c.Collection("events").Indexes().CreateMany(ctx, eventsIndexes)
	if err != nil {
		return err
	}

	return nil
}
