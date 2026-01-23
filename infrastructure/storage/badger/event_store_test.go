package badger_test

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/badger"
)

func TestNewEventStore(t *testing.T) {
	cfg := badger.Config{
		InMemory: true,
	}

	store, err := badger.NewEventStore(cfg)
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

	// Create events
	events := []event.Event{
		{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
			Payload:   []byte(`{"key": "value1"}`),
		},
		{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
			Payload:   []byte(`{"key": "value2"}`),
		},
	}

	// Append events
	err := store.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load events
	loaded, err := store.LoadEvents(ctx, "run-1")
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 events, got %d", len(loaded))
	}

	// Verify sequence numbers are assigned
	if loaded[0].Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 2 {
		t.Errorf("expected sequence 2, got %d", loaded[1].Sequence)
	}

	// Verify IDs are assigned
	if loaded[0].ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestEventStore_LoadEventsFrom(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create 5 events
	for i := 0; i < 5; i++ {
		err := store.Append(ctx, event.Event{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
			Payload:   []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Load from sequence 3
	loaded, err := store.LoadEventsFrom(ctx, "run-1", 3)
	if err != nil {
		t.Fatalf("LoadEventsFrom failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("expected 3 events, got %d", len(loaded))
	}

	if loaded[0].Sequence != 3 {
		t.Errorf("expected first event sequence 3, got %d", loaded[0].Sequence)
	}
}

func TestEventStore_CountEvents(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events
	for i := 0; i < 7; i++ {
		err := store.Append(ctx, event.Event{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	count, err := store.CountEvents(ctx, "run-1")
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}

	if count != 7 {
		t.Errorf("expected 7 events, got %d", count)
	}
}

func TestEventStore_ListRuns(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events for multiple runs
	runs := []string{"run-a", "run-b", "run-c"}
	for _, runID := range runs {
		err := store.Append(ctx, event.Event{
			RunID:     runID,
			Type:      "test.event",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// List runs
	listed, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("expected 3 runs, got %d", len(listed))
	}
}

func TestEventStore_DeleteRun(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events
	for i := 0; i < 3; i++ {
		err := store.Append(ctx, event.Event{
			RunID:     "run-to-delete",
			Type:      "test.event",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Verify events exist
	count, _ := store.CountEvents(ctx, "run-to-delete")
	if count != 3 {
		t.Errorf("expected 3 events before delete, got %d", count)
	}

	// Delete run
	err := store.DeleteRun(ctx, "run-to-delete")
	if err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}

	// Verify events are deleted
	count, err = store.CountEvents(ctx, "run-to-delete")
	if err != nil {
		t.Fatalf("CountEvents after delete failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 events after delete, got %d", count)
	}
}

func TestEventStore_Query(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events of different types
	types := []event.Type{"type.a", "type.b", "type.a", "type.c", "type.a"}
	for _, typ := range types {
		err := store.Append(ctx, event.Event{
			RunID:     "run-1",
			Type:      typ,
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query by type
	events, err := store.Query(ctx, "run-1", event.QueryOptions{
		Types: []event.Type{"type.a"},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events of type.a, got %d", len(events))
	}
}

func TestEventStore_Query_WithLimit(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events
	for i := 0; i < 10; i++ {
		err := store.Append(ctx, event.Event{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query with limit
	events, err := store.Query(ctx, "run-1", event.QueryOptions{
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events with limit, got %d", len(events))
	}
}

func TestEventStore_Query_WithOffset(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events
	for i := 0; i < 10; i++ {
		err := store.Append(ctx, event.Event{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query with offset
	events, err := store.Query(ctx, "run-1", event.QueryOptions{
		Offset: 7,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events after offset 7, got %d", len(events))
	}
}

func TestEventStore_Subscribe(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe
	ch, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Append event in goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = store.Append(context.Background(), event.Event{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
		})
	}()

	// Wait for event
	select {
	case e := <-ch:
		if e.Type != "test.event" {
			t.Errorf("expected type test.event, got %s", e.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventStore_Subscribe_MultipleSubscribers(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create two subscribers
	ch1, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}

	ch2, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	// Append event
	err = store.Append(context.Background(), event.Event{
		RunID:     "run-1",
		Type:      "test.event",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Both should receive the event
	received := 0
	timeout := time.After(500 * time.Millisecond)

	for received < 2 {
		select {
		case <-ch1:
			received++
		case <-ch2:
			received++
		case <-timeout:
			t.Fatalf("timeout: only received %d of 2 events", received)
		}
	}
}

func TestEventStore_AppendInvalidEvent(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Event with empty type
	err := store.Append(ctx, event.Event{
		RunID:     "run-1",
		Type:      "", // Invalid
		Timestamp: time.Now(),
	})
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
	err := store.Append(ctx, event.Event{
		RunID:     "run-1",
		Type:      "test.event",
		Timestamp: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	_, err = store.LoadEvents(ctx, "run-1")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestEventStore_WithKeyPrefix(t *testing.T) {
	cfg := badger.Config{
		InMemory:  true,
		KeyPrefix: "prefix:",
	}

	store, err := badger.NewEventStore(cfg)
	if err != nil {
		t.Fatalf("NewEventStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Append event
	err = store.Append(ctx, event.Event{
		RunID:     "run-1",
		Type:      "test.event",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load should work
	events, err := store.LoadEvents(ctx, "run-1")
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestEventStore_AppendEmpty(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Append empty slice should be no-op
	err := store.Append(ctx)
	if err != nil {
		t.Fatalf("Append empty failed: %v", err)
	}
}

func TestEventStore_LoadEventsEmpty(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Load from nonexistent run
	events, err := store.LoadEvents(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestEventStore_MultipleRuns(t *testing.T) {
	store := newTestEventStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create events for multiple runs
	err := store.Append(ctx,
		event.Event{RunID: "run-1", Type: "test.event", Timestamp: time.Now()},
		event.Event{RunID: "run-2", Type: "test.event", Timestamp: time.Now()},
		event.Event{RunID: "run-1", Type: "test.event", Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify counts
	count1, _ := store.CountEvents(ctx, "run-1")
	if count1 != 2 {
		t.Errorf("expected 2 events for run-1, got %d", count1)
	}

	count2, _ := store.CountEvents(ctx, "run-2")
	if count2 != 1 {
		t.Errorf("expected 1 event for run-2, got %d", count2)
	}
}

func newTestEventStore(t *testing.T) *badger.EventStore {
	t.Helper()

	cfg := badger.Config{
		InMemory: true,
	}

	store, err := badger.NewEventStore(cfg)
	if err != nil {
		t.Fatalf("NewEventStore failed: %v", err)
	}

	return store
}
