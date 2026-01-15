package event

import "context"

// Store defines the interface for event persistence.
// Implementations may be in-memory, PostgreSQL, or any other backend.
type Store interface {
	// Append persists one or more events atomically.
	// Events are assigned sequence numbers in order of appearance.
	Append(ctx context.Context, events ...Event) error

	// LoadEvents retrieves all events for a run in sequence order.
	LoadEvents(ctx context.Context, runID string) ([]Event, error)

	// LoadEventsFrom retrieves events starting from a specific sequence number.
	// This enables incremental replay from a known checkpoint.
	LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]Event, error)

	// Subscribe returns a channel that receives new events for a run.
	// The channel is closed when the context is cancelled or the run completes.
	Subscribe(ctx context.Context, runID string) (<-chan Event, error)
}

// Snapshotter is an optional interface for stores that support snapshotting.
// Snapshots allow efficient replay by storing aggregate state at checkpoints.
type Snapshotter interface {
	// SaveSnapshot persists a snapshot of run state at a sequence number.
	SaveSnapshot(ctx context.Context, runID string, sequence uint64, data []byte) error

	// LoadSnapshot retrieves the latest snapshot for a run.
	// Returns the snapshot data and the sequence number it was taken at.
	LoadSnapshot(ctx context.Context, runID string) (data []byte, sequence uint64, err error)
}

// Pruner is an optional interface for stores that support event pruning.
// This enables cleanup of old events after snapshotting.
type Pruner interface {
	// PruneEvents removes events before a sequence number.
	// Typically called after a snapshot is taken.
	PruneEvents(ctx context.Context, runID string, beforeSeq uint64) error
}

// QueryOptions configures event queries.
type QueryOptions struct {
	// Types filters to specific event types (empty means all).
	Types []Type

	// FromTime filters events after this timestamp.
	FromTime int64

	// ToTime filters events before this timestamp.
	ToTime int64

	// Limit is the maximum number of events to return (0 = no limit).
	Limit int

	// Offset is the number of events to skip.
	Offset int
}

// Querier is an optional interface for stores that support advanced queries.
type Querier interface {
	// Query retrieves events matching the given options.
	Query(ctx context.Context, runID string, opts QueryOptions) ([]Event, error)

	// CountEvents returns the number of events for a run.
	CountEvents(ctx context.Context, runID string) (int64, error)

	// ListRuns returns all run IDs with events in the store.
	ListRuns(ctx context.Context) ([]string, error)
}
