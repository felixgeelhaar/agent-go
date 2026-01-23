package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/google/uuid"
)

// EventStore is a SQLite-backed implementation of event.Store.
type EventStore struct {
	db          *sql.DB
	subscribers map[string][]chan event.Event
	mu          sync.RWMutex
}

// NewEventStore creates a new SQLite event store with the given configuration.
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
		subscribers: make(map[string][]chan event.Event),
	}

	// Auto-migrate if enabled
	if cfg.AutoMigrate {
		if err := s.migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return s, nil
}

// NewEventStoreFromDB creates an event store from an existing database connection.
func NewEventStoreFromDB(db *sql.DB) (*EventStore, error) {
	s := &EventStore{
		db:          db,
		subscribers: make(map[string][]chan event.Event),
	}

	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

// migrate creates the events table if it doesn't exist.
func (s *EventStore) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			type TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			timestamp INTEGER NOT NULL,
			data BLOB NOT NULL,
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id);
		CREATE INDEX IF NOT EXISTS idx_events_run_sequence ON events(run_id, sequence);
		CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_events_run_seq_unique ON events(run_id, sequence);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// Append persists one or more events atomically.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO events (id, run_id, type, sequence, timestamp, data, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().Unix()

	// Group events by run ID for sequence assignment
	byRun := make(map[string][]event.Event)
	for _, e := range events {
		byRun[e.RunID] = append(byRun[e.RunID], e)
	}

	// Get current sequence numbers for each run
	sequences := make(map[string]uint64)
	for runID := range byRun {
		var maxSeq sql.NullInt64
		err := tx.QueryRowContext(ctx,
			"SELECT MAX(sequence) FROM events WHERE run_id = ?",
			runID,
		).Scan(&maxSeq)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if maxSeq.Valid {
			sequences[runID] = uint64(maxSeq.Int64)
		}
	}

	// Process events
	var processedEvents []event.Event
	for runID, runEvents := range byRun {
		seq := sequences[runID]

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

			// Serialize event data
			data, err := json.Marshal(e)
			if err != nil {
				return err
			}

			// Insert event
			_, err = stmt.ExecContext(ctx,
				e.ID, e.RunID, string(e.Type), e.Sequence, e.Timestamp.Unix(), data, now,
			)
			if err != nil {
				return err
			}

			processedEvents = append(processedEvents, *e)
		}
	}

	if err := tx.Commit(); err != nil {
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

	rows, err := s.db.QueryContext(ctx,
		"SELECT data FROM events WHERE run_id = ? ORDER BY sequence",
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []event.Event
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}

		var e event.Event
		if err := json.Unmarshal(data, &e); err != nil {
			continue // Skip malformed entries
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT data FROM events WHERE run_id = ? AND sequence >= ? ORDER BY sequence",
		runID, fromSeq,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []event.Event
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}

		var e event.Event
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}

		events = append(events, e)
	}

	return events, rows.Err()
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

	query := "SELECT data FROM events WHERE run_id = ?"
	args := []interface{}{runID}

	// Filter by type
	if len(opts.Types) > 0 {
		placeholders := ""
		for i, t := range opts.Types {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, string(t))
		}
		query += " AND type IN (" + placeholders + ")"
	}

	// Filter by time range
	if opts.FromTime > 0 {
		query += " AND timestamp >= ?"
		args = append(args, opts.FromTime)
	}
	if opts.ToTime > 0 {
		query += " AND timestamp <= ?"
		args = append(args, opts.ToTime)
	}

	query += " ORDER BY sequence"

	// Apply limit and offset
	// SQLite requires LIMIT when using OFFSET
	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	} else if opts.Offset > 0 {
		// Use -1 for unlimited rows when only offset is specified
		query += " LIMIT -1"
	}
	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []event.Event
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}

		var e event.Event
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// CountEvents returns the number of events for a run.
func (s *EventStore) CountEvents(ctx context.Context, runID string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM events WHERE run_id = ?",
		runID,
	).Scan(&count)

	return count, err
}

// ListRuns returns all run IDs with events in the store.
func (s *EventStore) ListRuns(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT DISTINCT run_id FROM events ORDER BY run_id",
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []string
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return nil, err
		}
		runs = append(runs, runID)
	}

	return runs, rows.Err()
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

	_, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE run_id = ?", runID)
	return err
}

// Close closes the database connection and all subscriber channels.
func (s *EventStore) Close() error {
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

// DB returns the underlying database connection.
func (s *EventStore) DB() *sql.DB {
	return s.db
}

// Ensure EventStore implements event.Store and event.Querier
var (
	_ event.Store   = (*EventStore)(nil)
	_ event.Querier = (*EventStore)(nil)
)
