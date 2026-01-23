// Package messaging provides message queue operation tools.
package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Provider defines the interface for message queue operations.
// Implementations exist for Kafka, RabbitMQ, and SQS.
type Provider interface {
	// Name returns the provider name (e.g., "kafka", "rabbitmq", "sqs").
	Name() string

	// Publish sends a message to a topic/queue.
	Publish(ctx context.Context, topic string, message []byte, opts PublishOptions) (*PublishResult, error)

	// Subscribe creates a subscription to a topic/queue.
	// Returns a channel that receives messages until context is cancelled.
	Subscribe(ctx context.Context, topic string, opts SubscribeOptions) (<-chan Message, error)

	// Acknowledge marks a message as processed.
	Acknowledge(ctx context.Context, msgID string) error

	// Peek retrieves messages without acknowledging them.
	Peek(ctx context.Context, topic string, limit int) ([]Message, error)

	// ListTopics returns available topics/queues.
	ListTopics(ctx context.Context) ([]TopicInfo, error)

	// TopicExists checks if a topic/queue exists.
	TopicExists(ctx context.Context, topic string) (bool, error)

	// Close releases provider resources.
	Close() error
}

// PublishOptions configures message publishing.
type PublishOptions struct {
	// Key for partitioning (Kafka) or routing (RabbitMQ).
	Key string `json:"key,omitempty"`

	// Headers for message metadata.
	Headers map[string]string `json:"headers,omitempty"`

	// DelaySeconds defers message delivery (SQS).
	DelaySeconds int `json:"delay_seconds,omitempty"`

	// Priority for message ordering (RabbitMQ).
	Priority int `json:"priority,omitempty"`

	// Persistent ensures message survives broker restart.
	Persistent bool `json:"persistent,omitempty"`
}

// PublishResult contains the result of publishing a message.
type PublishResult struct {
	MessageID string    `json:"message_id"`
	Topic     string    `json:"topic"`
	Partition int       `json:"partition,omitempty"`
	Offset    int64     `json:"offset,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// SubscribeOptions configures message subscription.
type SubscribeOptions struct {
	// ConsumerGroup for load balancing across consumers.
	ConsumerGroup string `json:"consumer_group,omitempty"`

	// MaxMessages limits messages per batch.
	MaxMessages int `json:"max_messages,omitempty"`

	// WaitTimeout for polling.
	WaitTimeout time.Duration `json:"wait_timeout,omitempty"`

	// AutoAcknowledge automatically acknowledges messages.
	AutoAcknowledge bool `json:"auto_acknowledge,omitempty"`

	// StartOffset controls where to start consuming (kafka: "earliest", "latest").
	StartOffset string `json:"start_offset,omitempty"`
}

// Message represents a received message.
type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Key       string            `json:"key,omitempty"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Partition int               `json:"partition,omitempty"`
	Offset    int64             `json:"offset,omitempty"`

	// Provider-specific attributes
	Attributes map[string]string `json:"attributes,omitempty"`
}

// TopicInfo contains information about a topic/queue.
type TopicInfo struct {
	Name       string `json:"name"`
	Partitions int    `json:"partitions,omitempty"`
	Replicas   int    `json:"replicas,omitempty"`
	Messages   int64  `json:"messages,omitempty"`
}

// Config configures the messaging pack.
type Config struct {
	// Provider is the message queue provider (required).
	Provider Provider

	// ReadOnly disables publish operations.
	ReadOnly bool

	// Timeout for operations.
	Timeout time.Duration

	// MaxMessageSize limits outgoing message size.
	MaxMessageSize int

	// MaxPeekMessages limits peek operation results.
	MaxPeekMessages int
}

// Option configures the messaging pack.
type Option func(*Config)

// WithWriteAccess enables publish operations.
func WithWriteAccess() Option {
	return func(c *Config) {
		c.ReadOnly = false
	}
}

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithMaxMessageSize sets the maximum outgoing message size.
func WithMaxMessageSize(size int) Option {
	return func(c *Config) {
		c.MaxMessageSize = size
	}
}

// WithMaxPeekMessages sets the maximum messages returned by peek.
func WithMaxPeekMessages(count int) Option {
	return func(c *Config) {
		c.MaxPeekMessages = count
	}
}

// New creates the messaging pack.
func New(provider Provider, opts ...Option) (*pack.Pack, error) {
	if provider == nil {
		return nil, errors.New("messaging provider is required")
	}

	cfg := Config{
		Provider:        provider,
		ReadOnly:        true, // Read-only by default for safety
		Timeout:         30 * time.Second,
		MaxMessageSize:  1024 * 1024, // 1MB default
		MaxPeekMessages: 100,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	builder := pack.NewBuilder("messaging").
		WithDescription(fmt.Sprintf("Message queue operations (%s)", provider.Name())).
		WithVersion("1.0.0").
		AddTools(
			listTopicsTool(&cfg),
			peekTool(&cfg),
			acknowledgeTool(&cfg),
		).
		AllowInState(agent.StateExplore, "msg_list_topics", "msg_peek").
		AllowInState(agent.StateValidate, "msg_list_topics", "msg_peek")

	readTools := []string{"msg_list_topics", "msg_peek"}

	// Add publish tool if enabled
	if !cfg.ReadOnly {
		builder = builder.AddTools(publishTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, append(readTools, "msg_publish", "msg_acknowledge")...)
	} else {
		builder = builder.AllowInState(agent.StateAct, append(readTools, "msg_acknowledge")...)
	}

	return builder.Build(), nil
}

// listTopicsOutput is the output for the msg_list_topics tool.
type listTopicsOutput struct {
	Provider string      `json:"provider"`
	Topics   []TopicInfo `json:"topics"`
	Count    int         `json:"count"`
}

func listTopicsTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("msg_list_topics").
		WithDescription("List all available topics/queues").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			topics, err := cfg.Provider.ListTopics(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list topics: %w", err)
			}

			out := listTopicsOutput{
				Provider: cfg.Provider.Name(),
				Topics:   topics,
				Count:    len(topics),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// publishInput is the input for the msg_publish tool.
type publishInput struct {
	Topic        string            `json:"topic"`
	Message      string            `json:"message"`
	Key          string            `json:"key,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	DelaySeconds int               `json:"delay_seconds,omitempty"`
	Priority     int               `json:"priority,omitempty"`
	Persistent   bool              `json:"persistent,omitempty"`
}

// publishOutput is the output for the msg_publish tool.
type publishOutput struct {
	MessageID string    `json:"message_id"`
	Topic     string    `json:"topic"`
	Partition int       `json:"partition,omitempty"`
	Offset    int64     `json:"offset,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func publishTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("msg_publish").
		WithDescription("Publish a message to a topic/queue").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in publishInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Topic == "" {
				return tool.Result{}, errors.New("topic is required")
			}
			if in.Message == "" {
				return tool.Result{}, errors.New("message is required")
			}

			if len(in.Message) > cfg.MaxMessageSize {
				return tool.Result{}, fmt.Errorf("message size %d exceeds maximum %d", len(in.Message), cfg.MaxMessageSize)
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			opts := PublishOptions{
				Key:          in.Key,
				Headers:      in.Headers,
				DelaySeconds: in.DelaySeconds,
				Priority:     in.Priority,
				Persistent:   in.Persistent,
			}

			result, err := cfg.Provider.Publish(ctx, in.Topic, []byte(in.Message), opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to publish message: %w", err)
			}

			out := publishOutput{
				MessageID: result.MessageID,
				Topic:     result.Topic,
				Partition: result.Partition,
				Offset:    result.Offset,
				Timestamp: result.Timestamp,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// peekInput is the input for the msg_peek tool.
type peekInput struct {
	Topic string `json:"topic"`
	Limit int    `json:"limit,omitempty"`
}

// peekOutput is the output for the msg_peek tool.
type peekOutput struct {
	Topic    string    `json:"topic"`
	Messages []Message `json:"messages"`
	Count    int       `json:"count"`
}

func peekTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("msg_peek").
		WithDescription("Peek at messages in a topic/queue without acknowledging").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in peekInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Topic == "" {
				return tool.Result{}, errors.New("topic is required")
			}

			limit := in.Limit
			if limit == 0 || limit > cfg.MaxPeekMessages {
				limit = cfg.MaxPeekMessages
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			messages, err := cfg.Provider.Peek(ctx, in.Topic, limit)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to peek messages: %w", err)
			}

			out := peekOutput{
				Topic:    in.Topic,
				Messages: messages,
				Count:    len(messages),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// acknowledgeInput is the input for the msg_acknowledge tool.
type acknowledgeInput struct {
	MessageID string `json:"message_id"`
}

// acknowledgeOutput is the output for the msg_acknowledge tool.
type acknowledgeOutput struct {
	MessageID    string `json:"message_id"`
	Acknowledged bool   `json:"acknowledged"`
}

func acknowledgeTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("msg_acknowledge").
		WithDescription("Acknowledge a message as processed").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in acknowledgeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MessageID == "" {
				return tool.Result{}, errors.New("message_id is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			err := cfg.Provider.Acknowledge(ctx, in.MessageID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to acknowledge message: %w", err)
			}

			out := acknowledgeOutput{
				MessageID:    in.MessageID,
				Acknowledged: true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
