package dynamodb

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/felixgeelhaar/agent-go/domain/cache"
)

// cacheItem represents a cache entry in DynamoDB.
type cacheItem struct {
	Key       string `dynamodbav:"key"`
	Value     []byte `dynamodbav:"value"`
	ExpiresAt int64  `dynamodbav:"expires_at,omitempty"`
}

// Cache is a DynamoDB-backed implementation of cache.Cache.
type Cache struct {
	client       *dynamodb.Client
	tableName    string
	queryTimeout time.Duration
	hits         atomic.Int64
	misses       atomic.Int64
}

// NewCache creates a new DynamoDB cache.
func NewCache(client *Client) *Cache {
	return &Cache{
		client:       client.DynamoDB(),
		tableName:    client.config.CacheTableName,
		queryTimeout: client.config.QueryTimeout,
	}
}

// Get retrieves a cached value by key.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	result, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return nil, false, c.wrapError(err)
	}

	if result.Item == nil {
		c.misses.Add(1)
		return nil, false, nil
	}

	var item cacheItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, false, err
	}

	// Check expiration
	if item.ExpiresAt > 0 && time.Now().Unix() > item.ExpiresAt {
		c.misses.Add(1)
		// Delete expired item
		_, _ = c.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(c.tableName),
			Key: map[string]types.AttributeValue{
				"key": &types.AttributeValueMemberS{Value: key},
			},
		})
		return nil, false, nil
	}

	c.hits.Add(1)
	return item.Value, true, nil
}

// Set stores a value in the cache.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if key == "" {
		return cache.ErrInvalidKey
	}

	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	item := cacheItem{
		Key:   key,
		Value: value,
	}

	if opts.TTL > 0 {
		item.ExpiresAt = time.Now().Add(opts.TTL).Unix()
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	_, err = c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      av,
	})
	if err != nil {
		return c.wrapError(err)
	}

	return nil
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	_, err := c.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return c.wrapError(err)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	result, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		},
		ProjectionExpression: aws.String("#k, expires_at"),
		ExpressionAttributeNames: map[string]string{
			"#k": "key",
		},
	})
	if err != nil {
		return false, c.wrapError(err)
	}

	if result.Item == nil {
		return false, nil
	}

	// Check expiration
	if expiresAt, ok := result.Item["expires_at"]; ok {
		if n, ok := expiresAt.(*types.AttributeValueMemberN); ok {
			exp, _ := strconv.ParseInt(n.Value, 10, 64)
			if exp > 0 && time.Now().Unix() > exp {
				return false, nil
			}
		}
	}

	return true, nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	// Scan and delete all items
	var lastKey map[string]types.AttributeValue
	for {
		result, err := c.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(c.tableName),
			ExclusiveStartKey: lastKey,
			ProjectionExpression: aws.String("#k"),
			ExpressionAttributeNames: map[string]string{
				"#k": "key",
			},
		})
		if err != nil {
			return c.wrapError(err)
		}

		// Delete items in batches of 25
		for i := 0; i < len(result.Items); i += 25 {
			end := i + 25
			if end > len(result.Items) {
				end = len(result.Items)
			}

			writeRequests := make([]types.WriteRequest, 0, end-i)
			for _, item := range result.Items[i:end] {
				writeRequests = append(writeRequests, types.WriteRequest{
					DeleteRequest: &types.DeleteRequest{
						Key: map[string]types.AttributeValue{
							"key": item["key"],
						},
					},
				})
			}

			_, err = c.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					c.tableName: writeRequests,
				},
			})
			if err != nil {
				return c.wrapError(err)
			}
		}

		if result.LastEvaluatedKey == nil {
			break
		}
		lastKey = result.LastEvaluatedKey
	}

	return nil
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	return cache.Stats{
		Hits:    c.hits.Load(),
		Misses:  c.misses.Load(),
		Size:    0, // Would require a scan to calculate
		MaxSize: 0, // DynamoDB has no max size
	}
}

// wrapError wraps DynamoDB errors with domain errors.
func (c *Cache) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	// Check for provisioned throughput exceeded
	var throughputExceeded *types.ProvisionedThroughputExceededException
	if errors.As(err, &throughputExceeded) {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	return err
}

// Ensure Cache implements cache.Cache and cache.StatsProvider
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)
