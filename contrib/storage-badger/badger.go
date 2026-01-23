// Package badger provides BadgerDB-backed implementations of agent-go storage interfaces.
//
// BadgerDB is an embeddable, persistent, and fast key-value database written in pure Go.
// It provides LSM tree-based storage with ACID transactions and is suitable for
// high-performance, single-node deployments.
//
// # Usage
//
//	db, err := badger.Open(badger.DefaultOptions("/path/to/db"))
//	if err != nil {
//		return err
//	}
//	defer db.Close()
//
//	cache := storagebadger.NewCache(db)
//	eventStore := storagebadger.NewEventStore(db)
package badger

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/domain/event"
)

// DB represents a BadgerDB database interface.
// This allows for mocking in tests.
type DB interface {
	Close() error
}

// Cache is a BadgerDB-backed implementation of cache.Cache.
// It provides high-performance key-value caching with optional TTL support.
type Cache struct {
	db DB
}

// NewCache creates a new BadgerDB cache with the given database.
// The caller is responsible for managing the database lifecycle.
func NewCache(db DB) *Cache {
	return &Cache{db: db}
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// TODO: Implement BadgerDB-backed cache retrieval using db.View
	return nil, false, nil
}

// Set stores a value with the given key and options.
// TTL is supported natively by BadgerDB entries.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	// TODO: Implement BadgerDB-backed cache storage using db.Update
	// Use entry.WithTTL for TTL support
	return nil
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	// TODO: Implement BadgerDB-backed cache deletion
	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	// TODO: Implement BadgerDB-backed existence check
	return false, nil
}

// Clear removes all entries from the cache.
// Note: This operation can be expensive for large datasets.
func (c *Cache) Clear(ctx context.Context) error {
	// TODO: Implement BadgerDB-backed cache clearing using DropPrefix
	return nil
}

// Close closes the underlying database.
func (c *Cache) Close() error {
	return c.db.Close()
}

// EventStore is a BadgerDB-backed implementation of event.Store.
// It provides event sourcing capabilities with ordered key storage.
type EventStore struct {
	db DB
}

// NewEventStore creates a new BadgerDB event store with the given database.
// The caller is responsible for managing the database lifecycle.
func NewEventStore(db DB) *EventStore {
	return &EventStore{db: db}
}

// Append persists one or more events atomically.
// Events are assigned sequence numbers in order of appearance.
// Keys are structured as: events/{runID}/{sequence} for ordered iteration.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	// TODO: Implement BadgerDB-backed event append using WriteBatch
	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
// Uses BadgerDB's key ordering for efficient sequential reads.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	// TODO: Implement BadgerDB-backed event loading using Iterator with prefix
	return nil, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
// This enables incremental replay from a known checkpoint.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	// TODO: Implement BadgerDB-backed incremental event loading using Seek
	return nil, nil
}

// Subscribe returns a channel that receives new events for a run.
// The channel is closed when the context is cancelled or the run completes.
// Note: BadgerDB does not have native pub/sub, so this uses polling or
// Subscribe callbacks if available.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	// TODO: Implement BadgerDB-backed event subscription
	ch := make(chan event.Event)
	close(ch)
	return ch, nil
}

// Close closes the underlying database.
func (s *EventStore) Close() error {
	return s.db.Close()
}

// Ensure interfaces are implemented.
var (
	_ cache.Cache = (*Cache)(nil)
	_ event.Store = (*EventStore)(nil)
)
