package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryProvider is an in-memory implementation of Provider for testing.
type MemoryProvider struct {
	mu       sync.RWMutex
	topics   map[string]*memoryTopic
	messages map[string]*Message // message ID -> message
}

type memoryTopic struct {
	name     string
	messages []*Message
}

// NewMemoryProvider creates a new in-memory messaging provider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		topics:   make(map[string]*memoryTopic),
		messages: make(map[string]*Message),
	}
}

// Name returns the provider name.
func (p *MemoryProvider) Name() string {
	return "memory"
}

// Publish sends a message to a topic.
func (p *MemoryProvider) Publish(ctx context.Context, topic string, message []byte, opts PublishOptions) (*PublishResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Create topic if it doesn't exist
	t, ok := p.topics[topic]
	if !ok {
		t = &memoryTopic{name: topic, messages: make([]*Message, 0)}
		p.topics[topic] = t
	}

	msgID := uuid.New().String()
	now := time.Now()

	msg := &Message{
		ID:        msgID,
		Topic:     topic,
		Key:       opts.Key,
		Value:     message,
		Headers:   opts.Headers,
		Timestamp: now,
		Offset:    int64(len(t.messages)),
	}

	t.messages = append(t.messages, msg)
	p.messages[msgID] = msg

	return &PublishResult{
		MessageID: msgID,
		Topic:     topic,
		Offset:    msg.Offset,
		Timestamp: now,
	}, nil
}

// Subscribe creates a subscription to a topic.
func (p *MemoryProvider) Subscribe(ctx context.Context, topic string, opts SubscribeOptions) (<-chan Message, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	ch := make(chan Message, 100)

	go func() {
		defer close(ch)

		p.mu.RLock()
		t, ok := p.topics[topic]
		if !ok {
			p.mu.RUnlock()
			return
		}

		messages := make([]*Message, len(t.messages))
		copy(messages, t.messages)
		p.mu.RUnlock()

		for _, msg := range messages {
			select {
			case <-ctx.Done():
				return
			case ch <- *msg:
			}
		}
	}()

	return ch, nil
}

// Acknowledge marks a message as processed.
func (p *MemoryProvider) Acknowledge(ctx context.Context, msgID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.messages[msgID]; !ok {
		return fmt.Errorf("message %s not found", msgID)
	}

	// In memory provider, we just mark it as acknowledged by removing from the map
	delete(p.messages, msgID)
	return nil
}

// Peek retrieves messages without acknowledging them.
func (p *MemoryProvider) Peek(ctx context.Context, topic string, limit int) ([]Message, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	t, ok := p.topics[topic]
	if !ok {
		return []Message{}, nil
	}

	result := make([]Message, 0, limit)
	for i := 0; i < limit && i < len(t.messages); i++ {
		result = append(result, *t.messages[i])
	}

	return result, nil
}

// ListTopics returns available topics.
func (p *MemoryProvider) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]TopicInfo, 0, len(p.topics))
	for name, t := range p.topics {
		result = append(result, TopicInfo{
			Name:     name,
			Messages: int64(len(t.messages)),
		})
	}

	return result, nil
}

// TopicExists checks if a topic exists.
func (p *MemoryProvider) TopicExists(ctx context.Context, topic string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	_, ok := p.topics[topic]
	return ok, nil
}

// Close releases provider resources.
func (p *MemoryProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.topics = make(map[string]*memoryTopic)
	p.messages = make(map[string]*Message)
	return nil
}

// CreateTopic creates a topic for testing.
func (p *MemoryProvider) CreateTopic(topic string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.topics[topic]; !ok {
		p.topics[topic] = &memoryTopic{name: topic, messages: make([]*Message, 0)}
	}
}

// MessageCount returns the number of messages in a topic for testing.
func (p *MemoryProvider) MessageCount(topic string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if t, ok := p.topics[topic]; ok {
		return len(t.messages)
	}
	return 0
}
