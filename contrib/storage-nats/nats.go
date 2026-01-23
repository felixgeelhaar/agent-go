// Package nats provides NATS-backed implementations of agent-go storage interfaces.
//
// NATS is a high-performance messaging system that supports pub/sub, request/reply,
// and queue groups. With NATS JetStream, it also provides persistence, exactly-once
// delivery, and stream processing capabilities.
//
// This package uses NATS JetStream for event storage, providing durable, ordered
// event streams with replay capabilities.
//
// # Usage
//
//	nc, err := nats.Connect(nats.DefaultURL)
//	if err != nil {
//		return err
//	}
//	defer nc.Close()
//
//	js, err := nc.JetStream()
//	if err != nil {
//		return err
//	}
//
//	eventStore := storagenats.NewEventStore(js, "AGENT_EVENTS")
package nats

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/event"
)

// JetStreamContext represents a NATS JetStream context interface.
// This allows for mocking in tests.
type JetStreamContext interface{}

// EventStore is a NATS JetStream-backed implementation of event.Store.
// It stores events in a JetStream stream with subject-based routing per run.
type EventStore struct {
	js         JetStreamContext
	streamName string
}

// EventStoreConfig holds configuration for the NATS event store.
type EventStoreConfig struct {
	// StreamName is the JetStream stream name.
	StreamName string

	// SubjectPrefix is the prefix for event subjects (default: "agent.events").
	SubjectPrefix string

	// MaxMsgsPerSubject limits messages per subject (run) for retention.
	MaxMsgsPerSubject int64
}

// NewEventStore creates a new NATS JetStream event store with the given context and stream name.
func NewEventStore(js JetStreamContext, streamName string) *EventStore {
	return &EventStore{
		js:         js,
		streamName: streamName,
	}
}

// NewEventStoreWithConfig creates a new NATS JetStream event store with full configuration.
func NewEventStoreWithConfig(js JetStreamContext, cfg EventStoreConfig) *EventStore {
	return &EventStore{
		js:         js,
		streamName: cfg.StreamName,
	}
}

// Append persists one or more events atomically.
// Events are published to subjects formatted as: {prefix}.{runID}.{sequence}
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	// TODO: Implement JetStream publish for each event
	// 1. Assign sequence numbers if not set
	// 2. Serialize event to JSON
	// 3. Publish to subject: agent.events.{runID}
	// 4. Use PublishAsync for batch efficiency
	// 5. Wait for acks
	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
// Uses JetStream consumer to fetch all messages for the run subject.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	// TODO: Implement JetStream fetch for run events
	// 1. Create or get consumer for run subject
	// 2. Fetch all messages
	// 3. Deserialize and sort by sequence
	return nil, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
// Uses JetStream consumer with start sequence option.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	// TODO: Implement JetStream fetch with sequence filter
	// 1. Create consumer with DeliverByStartSequence option
	// 2. Fetch messages from that point
	return nil, nil
}

// Subscribe returns a channel that receives new events for a run.
// Uses JetStream push consumer for real-time event delivery.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	// TODO: Implement JetStream subscription
	// 1. Create push consumer for run subject
	// 2. Start goroutine to receive and forward events
	// 3. Handle context cancellation for cleanup
	ch := make(chan event.Event)
	close(ch)
	return ch, nil
}

// eventSubject returns the JetStream subject for a run's events.
func (s *EventStore) eventSubject(runID string) string {
	return "agent.events." + runID
}

// Ensure interface is implemented.
var _ event.Store = (*EventStore)(nil)
