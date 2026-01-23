// Package dynamodb provides DynamoDB-backed storage implementations.
package dynamodb

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Config contains DynamoDB connection configuration.
type Config struct {
	// Region is the AWS region.
	Region string

	// Endpoint is the DynamoDB endpoint (useful for local development).
	Endpoint string

	// QueryTimeout is the default timeout for queries.
	QueryTimeout time.Duration

	// CacheTableName is the table name for the cache.
	CacheTableName string

	// RunsTableName is the table name for runs.
	RunsTableName string
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Region:         "us-east-1",
		QueryTimeout:   30 * time.Second,
		CacheTableName: "agent_cache",
		RunsTableName:  "agent_runs",
	}
}

// ConfigOption configures the DynamoDB connection.
type ConfigOption func(*Config)

// WithRegion sets the AWS region.
func WithRegion(region string) ConfigOption {
	return func(c *Config) {
		c.Region = region
	}
}

// WithEndpoint sets the DynamoDB endpoint (for local development).
func WithEndpoint(endpoint string) ConfigOption {
	return func(c *Config) {
		c.Endpoint = endpoint
	}
}

// WithQueryTimeout sets the default query timeout.
func WithQueryTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.QueryTimeout = d
	}
}

// WithCacheTableName sets the cache table name.
func WithCacheTableName(name string) ConfigOption {
	return func(c *Config) {
		c.CacheTableName = name
	}
}

// WithRunsTableName sets the runs table name.
func WithRunsTableName(name string) ConfigOption {
	return func(c *Config) {
		c.RunsTableName = name
	}
}

// Client wraps a DynamoDB client with configuration.
type Client struct {
	client *dynamodb.Client
	config Config
}

// NewClient creates a new DynamoDB client.
func NewClient(ctx context.Context, opts ...ConfigOption) (*Client, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}

	var ddbOpts []func(*dynamodb.Options)
	if cfg.Endpoint != "" {
		ddbOpts = append(ddbOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := dynamodb.NewFromConfig(awsCfg, ddbOpts...)

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// DynamoDB returns the underlying DynamoDB client.
func (c *Client) DynamoDB() *dynamodb.Client {
	return c.client
}

// CreateCacheTable creates the cache table if it doesn't exist.
func (c *Client) CreateCacheTable(ctx context.Context) error {
	input := &dynamodb.CreateTableInput{
		TableName: aws.String(c.config.CacheTableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("key"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("key"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}

	_, err := c.client.CreateTable(ctx, input)
	if err != nil {
		// Ignore error if table already exists
		var resourceInUse *types.ResourceInUseException
		if isError(err, resourceInUse) {
			return nil
		}
		return err
	}

	// Wait for table to be active
	waiter := dynamodb.NewTableExistsWaiter(c.client)
	return waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(c.config.CacheTableName),
	}, 2*time.Minute)
}

// CreateRunsTable creates the runs table if it doesn't exist.
func (c *Client) CreateRunsTable(ctx context.Context) error {
	input := &dynamodb.CreateTableInput{
		TableName: aws.String(c.config.RunsTableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("status"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("start_time"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("status-start_time-index"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("status"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("start_time"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}

	_, err := c.client.CreateTable(ctx, input)
	if err != nil {
		// Ignore error if table already exists
		var resourceInUse *types.ResourceInUseException
		if isError(err, resourceInUse) {
			return nil
		}
		return err
	}

	// Wait for table to be active
	waiter := dynamodb.NewTableExistsWaiter(c.client)
	return waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(c.config.RunsTableName),
	}, 2*time.Minute)
}

// isError checks if an error is of a specific type.
func isError[T error](err error, _ T) bool {
	var target T
	return errors.As(err, &target)
}
