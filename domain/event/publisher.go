package event

import "context"

// Publisher publishes domain events to the event store.
type Publisher interface {
	// Publish sends events to the event store.
	Publish(ctx context.Context, events ...Event) error

	// Close releases any resources held by the publisher.
	Close() error
}

// Subscriber receives events from a publisher or store.
type Subscriber interface {
	// Subscribe returns a channel that receives events for a run.
	Subscribe(ctx context.Context, runID string) (<-chan Event, error)
}
