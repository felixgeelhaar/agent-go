package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/sqlite"
)

func TestNewEventStore(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
	}

	store, err := sqlite.NewEventStore(cfg)
	if err != nil {
		t.Fatalf("NewEventStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

func TestEventStore_AppendAndLoad(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-1"

	// Append events
	events := []event.Event{
		{RunID: runID, Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeRunCompleted, Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load events
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 events, got %d", len(loaded))
	}

	// Verify sequence numbers
	for i, e := range loaded {
		if e.Sequence != uint64(i+1) {
			t.Errorf("expected sequence %d, got %d", i+1, e.Sequence)
		}
	}
}

func TestEventStore_LoadEventsFrom(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-2"

	// Append events
	events := []event.Event{
		{RunID: runID, Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeStateTransitioned, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeRunCompleted, Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load from sequence 3
	loaded, err := store.LoadEventsFrom(ctx, runID, 3)
	if err != nil {
		t.Fatalf("LoadEventsFrom failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}

	if loaded[0].Sequence != 3 {
		t.Errorf("expected first event sequence 3, got %d", loaded[0].Sequence)
	}
}

func TestEventStore_CountEvents(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-3"

	// Append events
	events := []event.Event{
		{RunID: runID, Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	count, err := store.CountEvents(ctx, runID)
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 events, got %d", count)
	}
}

func TestEventStore_ListRuns(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Append events for multiple runs
	events := []event.Event{
		{RunID: "run-a", Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: "run-b", Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: "run-c", Type: event.TypeRunStarted, Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	runs, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
}

func TestEventStore_DeleteRun(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-delete"

	// Append events
	events := []event.Event{
		{RunID: runID, Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Delete run
	err = store.DeleteRun(ctx, runID)
	if err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}

	// Should have no events
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("expected 0 events after delete, got %d", len(loaded))
	}
}

func TestEventStore_Query(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-query"

	// Append events of different types
	events := []event.Event{
		{RunID: runID, Type: event.TypeRunStarted, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeStateTransitioned, Timestamp: time.Now()},
		{RunID: runID, Type: event.TypeRunCompleted, Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Query only ToolCalled events
	opts := event.QueryOptions{
		Types: []event.Type{event.TypeToolCalled},
	}

	queried, err := store.Query(ctx, runID, opts)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(queried) != 2 {
		t.Errorf("expected 2 ToolCalled events, got %d", len(queried))
	}

	for _, e := range queried {
		if e.Type != event.TypeToolCalled {
			t.Errorf("expected ToolCalled, got %s", e.Type)
		}
	}
}

func TestEventStore_QueryWithLimit(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-query-limit"

	// Append multiple events
	for i := 0; i < 10; i++ {
		events := []event.Event{
			{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
		}
		err := store.Append(ctx, events...)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query with limit
	opts := event.QueryOptions{
		Limit: 5,
	}

	queried, err := store.Query(ctx, runID, opts)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(queried) != 5 {
		t.Errorf("expected 5 events with limit, got %d", len(queried))
	}
}

func TestEventStore_QueryWithOffset(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()
	runID := "test-run-query-offset"

	// Append multiple events
	for i := 0; i < 10; i++ {
		events := []event.Event{
			{RunID: runID, Type: event.TypeToolCalled, Timestamp: time.Now()},
		}
		err := store.Append(ctx, events...)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query with offset
	opts := event.QueryOptions{
		Offset: 7,
	}

	queried, err := store.Query(ctx, runID, opts)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(queried) != 3 {
		t.Errorf("expected 3 events with offset, got %d", len(queried))
	}
}

func TestEventStore_Subscribe(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runID := "test-run-subscribe"

	// Subscribe
	ch, err := store.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Append events after subscribing
	go func() {
		time.Sleep(50 * time.Millisecond)
		events := []event.Event{
			{RunID: runID, Type: event.TypeRunStarted, Timestamp: time.Now()},
		}
		_ = store.Append(context.Background(), events...)
	}()

	// Wait for event
	select {
	case e := <-ch:
		if e.Type != event.TypeRunStarted {
			t.Errorf("expected RunStarted, got %s", e.Type)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for event")
	}
}

func TestEventStore_AppendEmpty(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Append empty slice should not error
	err := store.Append(ctx)
	if err != nil {
		t.Fatalf("Append empty failed: %v", err)
	}
}

func TestEventStore_AppendInvalidEvent(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Event without type should fail
	events := []event.Event{
		{RunID: "test", Type: "", Timestamp: time.Now()},
	}

	err := store.Append(ctx, events...)
	if err == nil {
		t.Fatal("expected error for invalid event")
	}
}

func TestEventStore_ContextCancelled(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should fail with cancelled context
	_, err := store.LoadEvents(ctx, "run")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func newTestEventStore(t *testing.T) *sqlite.EventStore {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
	}

	store, err := sqlite.NewEventStore(cfg)
	if err != nil {
		t.Fatalf("NewEventStore failed: %v", err)
	}

	return store
}
