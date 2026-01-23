// Package nats provides NATS JetStream-based storage implementations.
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/felixgeelhaar/agent-go/domain/event"
)

// Client defines the interface for NATS JetStream operations.
// This allows for mock implementations in testing.
type Client interface {
	// Publish publishes a message to a subject.
	Publish(ctx context.Context, subject string, data []byte) error

	// Subscribe subscribes to a subject with a durable consumer.
	Subscribe(ctx context.Context, subject string, handler func([]byte) error) (Subscription, error)

	// GetMessages retrieves all messages from a stream for a subject.
	GetMessages(ctx context.Context, subject string) ([][]byte, error)

	// GetMessagesFrom retrieves messages from a specific sequence.
	GetMessagesFrom(ctx context.Context, subject string, fromSeq uint64) ([][]byte, error)

	// Close closes the client connection.
	Close() error
}

// Subscription represents an active subscription.
type Subscription interface {
	// Unsubscribe stops the subscription.
	Unsubscribe() error
}

// EventStore implements event.Store using NATS JetStream.
type EventStore struct {
	client       Client
	subjectPrefix string
	mu           sync.RWMutex
	sequences    map[string]*uint64
}

// Config holds configuration for the NATS event store.
type Config struct {
	// Client is the NATS JetStream client to use.
	Client Client

	// SubjectPrefix is the prefix for all event subjects.
	SubjectPrefix string
}

// NewEventStore creates a new NATS event store.
func NewEventStore(cfg Config) (*EventStore, error) {
	if cfg.Client == nil {
		return nil, errors.New("nats client is required")
	}

	prefix := cfg.SubjectPrefix
	if prefix == "" {
		prefix = "events"
	}

	return &EventStore{
		client:        cfg.Client,
		subjectPrefix: prefix,
		sequences:     make(map[string]*uint64),
	}, nil
}

// Append persists one or more events atomically.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	for i := range events {
		// Assign sequence numbers
		seq := s.nextSequence(events[i].RunID)
		events[i].Sequence = seq

		// Generate ID if not set
		if events[i].ID == "" {
			events[i].ID = fmt.Sprintf("%s-%d", events[i].RunID, seq)
		}

		// Serialize event
		data, err := json.Marshal(events[i])
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		// Publish to NATS
		subject := s.subject(events[i].RunID)
		if err := s.client.Publish(ctx, subject, data); err != nil {
			return fmt.Errorf("failed to publish event: %w", err)
		}
	}

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	subject := s.subject(runID)
	messages, err := s.client.GetMessages(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	events := make([]event.Event, 0, len(messages))
	for _, data := range messages {
		var evt event.Event
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}
		events = append(events, evt)
	}

	return events, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	subject := s.subject(runID)
	messages, err := s.client.GetMessagesFrom(ctx, subject, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	events := make([]event.Event, 0, len(messages))
	for _, data := range messages {
		var evt event.Event
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}
		if evt.Sequence >= fromSeq {
			events = append(events, evt)
		}
	}

	return events, nil
}

// Subscribe returns a channel that receives new events for a run.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	subject := s.subject(runID)
	ch := make(chan event.Event, 100)

	sub, err := s.client.Subscribe(ctx, subject, func(data []byte) error {
		var evt event.Event
		if err := json.Unmarshal(data, &evt); err != nil {
			return err
		}

		select {
		case ch <- evt:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// Close channel when context is done
	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
		close(ch)
	}()

	return ch, nil
}

// subject constructs the NATS subject for a run.
func (s *EventStore) subject(runID string) string {
	return s.subjectPrefix + "." + runID
}

// nextSequence returns the next sequence number for a run.
func (s *EventStore) nextSequence(runID string) uint64 {
	s.mu.Lock()
	seq, ok := s.sequences[runID]
	if !ok {
		var newSeq uint64
		s.sequences[runID] = &newSeq
		seq = &newSeq
	}
	s.mu.Unlock()

	return atomic.AddUint64(seq, 1)
}

// Ensure EventStore implements event.Store
var _ event.Store = (*EventStore)(nil)

// MockClient is a mock NATS client for testing.
type MockClient struct {
	mu       sync.RWMutex
	messages map[string][][]byte
	subs     map[string][]func([]byte) error
}

// NewMockClient creates a new mock NATS client.
func NewMockClient() *MockClient {
	return &MockClient{
		messages: make(map[string][][]byte),
		subs:     make(map[string][]func([]byte) error),
	}
}

// Publish implements Client.Publish.
func (c *MockClient) Publish(_ context.Context, subject string, data []byte) error {
	c.mu.Lock()
	c.messages[subject] = append(c.messages[subject], data)
	handlers := c.subs[subject]
	c.mu.Unlock()

	// Notify subscribers
	for _, handler := range handlers {
		if err := handler(data); err != nil {
			return err
		}
	}

	return nil
}

// Subscribe implements Client.Subscribe.
func (c *MockClient) Subscribe(_ context.Context, subject string, handler func([]byte) error) (Subscription, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.subs[subject] = append(c.subs[subject], handler)

	return &mockSubscription{
		client:  c,
		subject: subject,
		handler: handler,
	}, nil
}

// GetMessages implements Client.GetMessages.
func (c *MockClient) GetMessages(_ context.Context, subject string) ([][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msgs := c.messages[subject]
	if msgs == nil {
		return [][]byte{}, nil
	}

	result := make([][]byte, len(msgs))
	copy(result, msgs)
	return result, nil
}

// GetMessagesFrom implements Client.GetMessagesFrom.
func (c *MockClient) GetMessagesFrom(_ context.Context, subject string, _ uint64) ([][]byte, error) {
	// For mock, just return all messages (filtering is done by EventStore)
	return c.GetMessages(context.Background(), subject)
}

// Close implements Client.Close.
func (c *MockClient) Close() error {
	return nil
}

// MessageCount returns the number of messages for a subject (for testing).
func (c *MockClient) MessageCount(subject string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages[subject])
}

// Ensure MockClient implements Client
var _ Client = (*MockClient)(nil)

type mockSubscription struct {
	client  *MockClient
	subject string
	handler func([]byte) error
}

func (s *mockSubscription) Unsubscribe() error {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	handlers := s.client.subs[s.subject]
	for i, h := range handlers {
		// Compare function pointers (this is a simplification)
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", s.handler) {
			s.client.subs[s.subject] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
	return nil
}
