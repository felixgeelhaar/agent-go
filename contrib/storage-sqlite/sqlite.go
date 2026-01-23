// Package sqlite provides SQLite-backed implementations of agent-go storage interfaces.
//
// This package offers lightweight, file-based storage suitable for development,
// testing, and single-node deployments. SQLite provides ACID compliance and
// requires no external database server.
//
// # Usage
//
//	db, err := sql.Open("sqlite3", "agent.db")
//	if err != nil {
//		return err
//	}
//
//	cache := sqlite.NewCache(db)
//	eventStore := sqlite.NewEventStore(db)
//	runStore := sqlite.NewRunStore(db)
package sqlite

import (
	"context"
	"database/sql"
	"io"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// Cache is a SQLite-backed implementation of cache.Cache.
// It stores cached values in a SQLite table with optional TTL support.
type Cache struct {
	db *sql.DB
}

// NewCache creates a new SQLite cache with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewCache(db *sql.DB) *Cache {
	return &Cache{db: db}
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// TODO: Implement SQLite-backed cache retrieval
	return nil, false, nil
}

// Set stores a value with the given key and options.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	// TODO: Implement SQLite-backed cache storage
	return nil
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	// TODO: Implement SQLite-backed cache deletion
	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	// TODO: Implement SQLite-backed existence check
	return false, nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(ctx context.Context) error {
	// TODO: Implement SQLite-backed cache clearing
	return nil
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

// EventStore is a SQLite-backed implementation of event.Store.
// It provides event sourcing capabilities with atomic append operations.
type EventStore struct {
	db *sql.DB
}

// NewEventStore creates a new SQLite event store with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewEventStore(db *sql.DB) *EventStore {
	return &EventStore{db: db}
}

// Append persists one or more events atomically.
// Events are assigned sequence numbers in order of appearance.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	// TODO: Implement SQLite-backed event append with transaction
	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	// TODO: Implement SQLite-backed event loading
	return nil, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
// This enables incremental replay from a known checkpoint.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	// TODO: Implement SQLite-backed incremental event loading
	return nil, nil
}

// Subscribe returns a channel that receives new events for a run.
// The channel is closed when the context is cancelled or the run completes.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	// TODO: Implement SQLite-backed event subscription with polling
	ch := make(chan event.Event)
	close(ch)
	return ch, nil
}

// Close closes the underlying database connection.
func (s *EventStore) Close() error {
	return s.db.Close()
}

// RunStore is a SQLite-backed implementation of run.Store.
// It provides persistent storage for agent run state and history.
type RunStore struct {
	db *sql.DB
}

// NewRunStore creates a new SQLite run store with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewRunStore(db *sql.DB) *RunStore {
	return &RunStore{db: db}
}

// Save persists a new run.
func (s *RunStore) Save(ctx context.Context, r *agent.Run) error {
	// TODO: Implement SQLite-backed run save
	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	// TODO: Implement SQLite-backed run retrieval
	return nil, nil
}

// Update updates an existing run.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	// TODO: Implement SQLite-backed run update
	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	// TODO: Implement SQLite-backed run deletion
	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	// TODO: Implement SQLite-backed run listing with filters
	return nil, nil
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	// TODO: Implement SQLite-backed run counting
	return 0, nil
}

// Close closes the underlying database connection.
func (s *RunStore) Close() error {
	return s.db.Close()
}

// Ensure interfaces are implemented.
var (
	_ cache.Cache = (*Cache)(nil)
	_ event.Store = (*EventStore)(nil)
	_ run.Store   = (*RunStore)(nil)
)

// Silence unused import warnings for io package used in doc comments.
var _ = io.EOF
