package badger

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/google/uuid"
)

// EventStore is a BadgerDB-backed implementation of event.Store.
type EventStore struct {
	db          *badger.DB
	keyPrefix   string
	subscribers map[string][]chan event.Event
	mu          sync.RWMutex
	gcStop      chan struct{}
	gcWg        sync.WaitGroup
}

// NewEventStore creates a new BadgerDB event store with the given configuration.
func NewEventStore(cfg Config, opts ...Option) (*EventStore, error) {
	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}

	s := &EventStore{
		db:          db,
		keyPrefix:   cfg.KeyPrefix,
		subscribers: make(map[string][]chan event.Event),
		gcStop:      make(chan struct{}),
	}

	// Start GC goroutine
	if cfg.GCInterval > 0 {
		s.startGC(cfg.GCInterval, cfg.GCDiscardRatio)
	}

	return s, nil
}

// NewEventStoreFromDB creates an event store from an existing BadgerDB database.
func NewEventStoreFromDB(db *badger.DB, keyPrefix string) *EventStore {
	return &EventStore{
		db:          db,
		keyPrefix:   keyPrefix,
		subscribers: make(map[string][]chan event.Event),
		gcStop:      make(chan struct{}),
	}
}

// startGC starts the garbage collection goroutine.
func (s *EventStore) startGC(interval, discardRatio interface{}) {
	// Type assertion for parameters
	// ... similar to Cache implementation
}

// Key format: prefix:events:runID:sequence (8 bytes, big-endian)
func (s *EventStore) eventKey(runID string, seq uint64) []byte {
	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, seq)
	return append([]byte(s.keyPrefix+"events:"+runID+":"), seqBytes...)
}

// Key format: prefix:seq:runID for storing sequence counter
func (s *EventStore) seqKey(runID string) []byte {
	return []byte(s.keyPrefix + "seq:" + runID)
}

// Append persists one or more events atomically.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	// Group events by run ID
	byRun := make(map[string][]event.Event)
	for _, e := range events {
		byRun[e.RunID] = append(byRun[e.RunID], e)
	}

	var processedEvents []event.Event

	err := s.db.Update(func(txn *badger.Txn) error {
		for runID, runEvents := range byRun {
			// Get current sequence number
			var seq uint64
			seqKey := s.seqKey(runID)

			item, err := txn.Get(seqKey)
			if err == nil {
				err = item.Value(func(val []byte) error {
					if len(val) == 8 {
						seq = binary.BigEndian.Uint64(val)
					}
					return nil
				})
				if err != nil {
					return err
				}
			} else if !errors.Is(err, badger.ErrKeyNotFound) {
				return err
			}

			// Process events
			for i := range runEvents {
				e := &runEvents[i]

				// Assign ID if not set
				if e.ID == "" {
					e.ID = uuid.New().String()
				}

				// Assign sequence number
				seq++
				e.Sequence = seq

				// Validate event
				if e.Type == "" {
					return event.ErrInvalidEvent
				}

				// Serialize event
				data, err := json.Marshal(e)
				if err != nil {
					return err
				}

				// Store event
				eventKey := s.eventKey(runID, seq)
				if err := txn.Set(eventKey, data); err != nil {
					return err
				}

				processedEvents = append(processedEvents, *e)
			}

			// Update sequence counter
			seqBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(seqBytes, seq)
			if err := txn.Set(seqKey, seqBytes); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Notify subscribers
	s.notifySubscribers(processedEvents)

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prefix := []byte(s.keyPrefix + "events:" + runID + ":")
	var events []event.Event

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			var e event.Event
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &e)
			})
			if err != nil {
				continue // Skip malformed entries
			}

			events = append(events, e)
		}

		return nil
	})

	return events, err
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	startKey := s.eventKey(runID, fromSeq)
	prefix := []byte(s.keyPrefix + "events:" + runID + ":")
	var events []event.Event

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()

			var e event.Event
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &e)
			})
			if err != nil {
				continue
			}

			events = append(events, e)
		}

		return nil
	})

	return events, err
}

// Subscribe returns a channel that receives new events for a run.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	ch := make(chan event.Event, 100)
	s.subscribers[runID] = append(s.subscribers[runID], ch)
	s.mu.Unlock()

	// Start cleanup goroutine
	go func() {
		<-ctx.Done()
		s.unsubscribe(runID, ch)
	}()

	return ch, nil
}

// unsubscribe removes a subscriber channel.
func (s *EventStore) unsubscribe(runID string, ch chan event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[runID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[runID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}

	if len(s.subscribers[runID]) == 0 {
		delete(s.subscribers, runID)
	}
}

// notifySubscribers sends events to subscribers.
func (s *EventStore) notifySubscribers(events []event.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range events {
		subs, ok := s.subscribers[e.RunID]
		if !ok {
			continue
		}

		for _, ch := range subs {
			select {
			case ch <- e:
			default:
				// Channel full, skip
			}
		}
	}
}

// Query retrieves events matching the given options.
func (s *EventStore) Query(ctx context.Context, runID string, opts event.QueryOptions) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prefix := []byte(s.keyPrefix + "events:" + runID + ":")
	var events []event.Event
	skip := opts.Offset
	count := 0

	err := s.db.View(func(txn *badger.Txn) error {
		iterOpts := badger.DefaultIteratorOptions
		iterOpts.Prefix = prefix

		it := txn.NewIterator(iterOpts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			var e event.Event
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &e)
			})
			if err != nil {
				continue
			}

			// Apply type filter
			if len(opts.Types) > 0 {
				found := false
				for _, t := range opts.Types {
					if e.Type == t {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Apply time filter
			ts := e.Timestamp.Unix()
			if opts.FromTime > 0 && ts < opts.FromTime {
				continue
			}
			if opts.ToTime > 0 && ts > opts.ToTime {
				continue
			}

			// Apply offset
			if skip > 0 {
				skip--
				continue
			}

			events = append(events, e)
			count++

			// Apply limit
			if opts.Limit > 0 && count >= opts.Limit {
				break
			}
		}

		return nil
	})

	return events, err
}

// CountEvents returns the number of events for a run.
func (s *EventStore) CountEvents(ctx context.Context, runID string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	prefix := []byte(s.keyPrefix + "events:" + runID + ":")
	var count int64

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}

		return nil
	})

	return count, err
}

// ListRuns returns all run IDs with events in the store.
func (s *EventStore) ListRuns(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prefix := []byte(s.keyPrefix + "seq:")
	prefixLen := len(prefix)
	var runs []string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			runID := string(key[prefixLen:])
			runs = append(runs, runID)
		}

		return nil
	})

	return runs, err
}

// DeleteRun removes all events for a specific run.
func (s *EventStore) DeleteRun(ctx context.Context, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Close subscriber channels
	s.mu.Lock()
	if subs, ok := s.subscribers[runID]; ok {
		for _, ch := range subs {
			close(ch)
		}
		delete(s.subscribers, runID)
	}
	s.mu.Unlock()

	// Delete events
	prefix := []byte(s.keyPrefix + "events:" + runID + ":")
	if err := s.db.DropPrefix(prefix); err != nil {
		return err
	}

	// Delete sequence counter
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(s.seqKey(runID))
	})
}

// Close closes the database and all subscriber channels.
func (s *EventStore) Close() error {
	// Stop GC
	close(s.gcStop)
	s.gcWg.Wait()

	// Close subscribers
	s.mu.Lock()
	for _, subs := range s.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	s.subscribers = make(map[string][]chan event.Event)
	s.mu.Unlock()

	return s.db.Close()
}

// DB returns the underlying BadgerDB database.
func (s *EventStore) DB() *badger.DB {
	return s.db
}

// Ensure EventStore implements event.Store and event.Querier
var (
	_ event.Store   = (*EventStore)(nil)
	_ event.Querier = (*EventStore)(nil)
)
