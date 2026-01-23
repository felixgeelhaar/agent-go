package notification

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockProvider is a mock notification provider for testing.
type MockProvider struct {
	name string

	// SendFunc is called when Send is invoked.
	SendFunc func(ctx context.Context, req SendRequest) (SendResponse, error)

	// UpdateFunc is called when Update is invoked.
	UpdateFunc func(ctx context.Context, req UpdateRequest) (UpdateResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool

	// Internal state
	mu       sync.RWMutex
	messages map[string]storedMessage
	msgCount int
}

type storedMessage struct {
	id        string
	channel   string
	message   string
	title     string
	timestamp time.Time
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	p := &MockProvider{
		name:     name,
		messages: make(map[string]storedMessage),
	}

	p.SendFunc = p.defaultSend
	p.UpdateFunc = p.defaultUpdate
	p.AvailableFunc = func(_ context.Context) bool { return true }

	return p
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// Send sends a notification.
func (p *MockProvider) Send(ctx context.Context, req SendRequest) (SendResponse, error) {
	return p.SendFunc(ctx, req)
}

// Update updates an existing notification.
func (p *MockProvider) Update(ctx context.Context, req UpdateRequest) (UpdateResponse, error) {
	return p.UpdateFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

func (p *MockProvider) defaultSend(_ context.Context, req SendRequest) (SendResponse, error) {
	if req.Channel == "" {
		return SendResponse{}, ErrInvalidInput
	}

	if req.Message == "" {
		return SendResponse{}, ErrInvalidInput
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.msgCount++
	msgID := fmt.Sprintf("msg-%d", p.msgCount)
	now := time.Now()

	p.messages[msgID] = storedMessage{
		id:        msgID,
		channel:   req.Channel,
		message:   req.Message,
		title:     req.Title,
		timestamp: now,
	}

	return SendResponse{
		MessageID: msgID,
		Timestamp: now.Format(time.RFC3339),
		Channel:   req.Channel,
		Success:   true,
	}, nil
}

func (p *MockProvider) defaultUpdate(_ context.Context, req UpdateRequest) (UpdateResponse, error) {
	if req.MessageID == "" {
		return UpdateResponse{}, ErrInvalidInput
	}

	if req.Channel == "" {
		return UpdateResponse{}, ErrInvalidInput
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	msg, ok := p.messages[req.MessageID]
	if !ok {
		return UpdateResponse{
			MessageID: req.MessageID,
			Updated:   false,
		}, nil
	}

	if msg.channel != req.Channel {
		return UpdateResponse{
			MessageID: req.MessageID,
			Updated:   false,
		}, nil
	}

	now := time.Now()
	if req.Message != "" {
		msg.message = req.Message
	}
	if req.Title != "" {
		msg.title = req.Title
	}
	msg.timestamp = now
	p.messages[req.MessageID] = msg

	return UpdateResponse{
		MessageID: req.MessageID,
		Updated:   true,
		Timestamp: now.Format(time.RFC3339),
	}, nil
}

// GetMessage returns a stored message (for testing).
func (p *MockProvider) GetMessage(id string) (storedMessage, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	msg, ok := p.messages[id]
	return msg, ok
}

// MessageCount returns the number of sent messages (for testing).
func (p *MockProvider) MessageCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.messages)
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
