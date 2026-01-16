package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
)

func TestNewEventStore(t *testing.T) {
	t.Parallel()

	t.Run("creates store with default schema", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "")
		if store.schema != "public" {
			t.Errorf("schema = %s, want public", store.schema)
		}
	})

	t.Run("creates store with custom schema", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "events")
		if store.schema != "events" {
			t.Errorf("schema = %s, want events", store.schema)
		}
	})

	t.Run("initializes subscribers map", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "public")
		if store.subscribers == nil {
			t.Error("subscribers should be initialized")
		}
	})

	t.Run("stores pool reference", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "public")
		if store.pool != nil {
			t.Error("expected nil pool")
		}
	})
}

func TestEventStore_tableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{"default schema", "public", "public.events"},
		{"custom schema", "myschema", "myschema.events"},
		{"empty schema defaults to public", "", "public.events"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewEventStore(nil, tt.schema)
			result := store.tableName()
			if result != tt.expected {
				t.Errorf("tableName() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestEventStore_snapshotTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{"default schema", "public", "public.snapshots"},
		{"custom schema", "myschema", "myschema.snapshots"},
		{"empty schema defaults to public", "", "public.snapshots"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewEventStore(nil, tt.schema)
			result := store.snapshotTableName()
			if result != tt.expected {
				t.Errorf("snapshotTableName() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestJoinConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conditions []string
		expected   string
	}{
		{
			name:       "empty conditions",
			conditions: []string{},
			expected:   "",
		},
		{
			name:       "single condition",
			conditions: []string{"a = 1"},
			expected:   "a = 1",
		},
		{
			name:       "two conditions",
			conditions: []string{"a = 1", "b = 2"},
			expected:   "a = 1 AND b = 2",
		},
		{
			name:       "multiple conditions",
			conditions: []string{"a = 1", "b = 2", "c = 3"},
			expected:   "a = 1 AND b = 2 AND c = 3",
		},
		{
			name:       "conditions with complex expressions",
			conditions: []string{"status = ANY($1)", "created_at >= $2"},
			expected:   "status = ANY($1) AND created_at >= $2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := joinConditions(tt.conditions)
			if result != tt.expected {
				t.Errorf("joinConditions() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestEventStore_Append_Validation(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	t.Run("returns nil for empty events", func(t *testing.T) {
		t.Parallel()
		err := store.Append(context.Background())
		if err != nil {
			t.Errorf("Append() with no events error = %v, want nil", err)
		}
	})
}

func TestEventStore_Subscribe(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if ch == nil {
		t.Error("Subscribe() returned nil channel")
	}

	// Verify subscriber was added
	store.mu.RLock()
	subs := store.subscribers["run-1"]
	store.mu.RUnlock()

	if len(subs) != 1 {
		t.Errorf("subscribers count = %d, want 1", len(subs))
	}

	// Cancel context to trigger cleanup
	cancel()

	// Give goroutine time to cleanup
	time.Sleep(50 * time.Millisecond)

	store.mu.RLock()
	subs = store.subscribers["run-1"]
	store.mu.RUnlock()

	if len(subs) != 0 {
		t.Errorf("subscribers count after cancel = %d, want 0", len(subs))
	}
}

func TestEventStore_Subscribe_MultipleSubscribers(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	ctx1 := context.Background()
	ctx2 := context.Background()

	ch1, err := store.Subscribe(ctx1, "run-1")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	ch2, err := store.Subscribe(ctx2, "run-1")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if ch1 == nil || ch2 == nil {
		t.Error("Subscribe() returned nil channel")
	}

	// Verify both subscribers were added
	store.mu.RLock()
	subs := store.subscribers["run-1"]
	store.mu.RUnlock()

	if len(subs) != 2 {
		t.Errorf("subscribers count = %d, want 2", len(subs))
	}
}

func TestEventStore_buildQuerySQL(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	t.Run("basic query", func(t *testing.T) {
		t.Parallel()
		query, args := store.buildQuerySQL("run-1", event.QueryOptions{})
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 1 || args[0] != "run-1" {
			t.Errorf("args = %v, want [run-1]", args)
		}
	})

	t.Run("filter by types", func(t *testing.T) {
		t.Parallel()
		opts := event.QueryOptions{
			Types: []event.Type{event.TypeToolCalled, event.TypeStateTransitioned},
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
		// First arg is run_id, second is types
		types, ok := args[1].([]string)
		if !ok {
			t.Errorf("expected []string for types, got %T", args[1])
		}
		if len(types) != 2 {
			t.Errorf("types length = %d, want 2", len(types))
		}
	})

	t.Run("filter by from time", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		opts := event.QueryOptions{
			FromTime: now.Add(-time.Hour).Unix(),
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
	})

	t.Run("filter by to time", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		opts := event.QueryOptions{
			ToTime: now.Unix(),
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
	})

	t.Run("filter by time range", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		opts := event.QueryOptions{
			FromTime: now.Add(-time.Hour).Unix(),
			ToTime:   now.Unix(),
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 3 {
			t.Errorf("args length = %d, want 3", len(args))
		}
	})

	t.Run("with limit only", func(t *testing.T) {
		t.Parallel()
		opts := event.QueryOptions{
			Limit: 10,
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
	})

	t.Run("with offset only", func(t *testing.T) {
		t.Parallel()
		opts := event.QueryOptions{
			Offset: 20,
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
	})

	t.Run("with limit and offset", func(t *testing.T) {
		t.Parallel()
		opts := event.QueryOptions{
			Limit:  10,
			Offset: 20,
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 3 {
			t.Errorf("args length = %d, want 3", len(args))
		}
	})

	t.Run("combined options", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		opts := event.QueryOptions{
			Types:    []event.Type{event.TypeToolCalled},
			FromTime: now.Add(-time.Hour).Unix(),
			ToTime:   now.Unix(),
			Limit:    10,
			Offset:   5,
		}
		query, args := store.buildQuerySQL("run-1", opts)
		if query == "" {
			t.Error("expected non-empty query")
		}
		// run_id + types + from_time + to_time + limit + offset = 6
		if len(args) != 6 {
			t.Errorf("args length = %d, want 6", len(args))
		}
	})
}

func TestEventStore_wrapError(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	t.Run("returns nil for nil error", func(t *testing.T) {
		t.Parallel()
		err := store.wrapError(nil)
		if err != nil {
			t.Errorf("wrapError(nil) = %v, want nil", err)
		}
	})

	t.Run("wraps deadline exceeded as timeout", func(t *testing.T) {
		t.Parallel()
		err := store.wrapError(context.DeadlineExceeded)
		if !errors.Is(err, event.ErrOperationTimeout) {
			t.Errorf("wrapError(DeadlineExceeded) should wrap as ErrOperationTimeout")
		}
		// Should also contain the original error
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error("wrapped error should contain original error")
		}
	})

	t.Run("wraps other errors as connection failed", func(t *testing.T) {
		t.Parallel()
		originalErr := errors.New("some database error")
		err := store.wrapError(originalErr)
		if !errors.Is(err, event.ErrConnectionFailed) {
			t.Errorf("wrapError() should wrap as ErrConnectionFailed")
		}
		// Should also contain the original error
		if !errors.Is(err, originalErr) {
			t.Error("wrapped error should contain original error")
		}
	})
}

func TestEventStore_notifySubscribers(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	// Create subscriber
	ch := make(chan event.Event, 10)
	store.mu.Lock()
	store.subscribers["run-1"] = []chan event.Event{ch}
	store.mu.Unlock()

	// Send events
	events := []event.Event{
		{ID: "evt-1", RunID: "run-1", Type: event.TypeToolCalled},
		{ID: "evt-2", RunID: "run-1", Type: event.TypeStateTransitioned},
	}

	store.notifySubscribers(events)

	// Verify events were received
	if len(ch) != 2 {
		t.Errorf("channel has %d events, want 2", len(ch))
	}

	evt1 := <-ch
	if evt1.ID != "evt-1" {
		t.Errorf("first event ID = %s, want evt-1", evt1.ID)
	}

	evt2 := <-ch
	if evt2.ID != "evt-2" {
		t.Errorf("second event ID = %s, want evt-2", evt2.ID)
	}
}

func TestEventStore_notifySubscribers_FullChannel(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	// Create a channel with capacity 1
	ch := make(chan event.Event, 1)
	store.mu.Lock()
	store.subscribers["run-1"] = []chan event.Event{ch}
	store.mu.Unlock()

	// Send more events than channel capacity
	events := []event.Event{
		{ID: "evt-1", RunID: "run-1", Type: event.TypeToolCalled},
		{ID: "evt-2", RunID: "run-1", Type: event.TypeToolCalled},
		{ID: "evt-3", RunID: "run-1", Type: event.TypeToolCalled},
	}

	// Should not block even if channel is full
	store.notifySubscribers(events)

	// Only first event should be in channel (others dropped)
	if len(ch) != 1 {
		t.Errorf("channel has %d events, want 1 (overflow dropped)", len(ch))
	}
}

func TestEventStore_notifySubscribers_NoSubscribers(t *testing.T) {
	t.Parallel()

	store := NewEventStore(nil, "public")

	// Send events with no subscribers - should not panic
	events := []event.Event{
		{ID: "evt-1", RunID: "run-1", Type: event.TypeToolCalled},
	}

	store.notifySubscribers(events)
	// Test passes if no panic occurs
}

func TestEventStore_unsubscribe(t *testing.T) {
	t.Parallel()

	t.Run("removes single subscriber", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "public")

		ch := make(chan event.Event, 10)
		store.mu.Lock()
		store.subscribers["run-1"] = []chan event.Event{ch}
		store.mu.Unlock()

		store.unsubscribe("run-1", ch)

		store.mu.RLock()
		_, exists := store.subscribers["run-1"]
		store.mu.RUnlock()

		if exists {
			t.Error("run-1 should be removed from subscribers map")
		}
	})

	t.Run("removes one of multiple subscribers", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "public")

		ch1 := make(chan event.Event, 10)
		ch2 := make(chan event.Event, 10)
		store.mu.Lock()
		store.subscribers["run-1"] = []chan event.Event{ch1, ch2}
		store.mu.Unlock()

		store.unsubscribe("run-1", ch1)

		store.mu.RLock()
		subs := store.subscribers["run-1"]
		store.mu.RUnlock()

		if len(subs) != 1 {
			t.Errorf("subscribers count = %d, want 1", len(subs))
		}
	})

	t.Run("removes all subscribers", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "public")

		ch1 := make(chan event.Event, 10)
		ch2 := make(chan event.Event, 10)
		store.mu.Lock()
		store.subscribers["run-1"] = []chan event.Event{ch1, ch2}
		store.mu.Unlock()

		store.unsubscribe("run-1", ch1)
		store.unsubscribe("run-1", ch2)

		store.mu.RLock()
		_, exists := store.subscribers["run-1"]
		store.mu.RUnlock()

		if exists {
			t.Error("run-1 should be removed from subscribers map when empty")
		}
	})

	t.Run("handles non-existent channel gracefully", func(t *testing.T) {
		t.Parallel()
		store := NewEventStore(nil, "public")

		ch1 := make(chan event.Event, 10)
		ch2 := make(chan event.Event, 10) // Not subscribed
		store.mu.Lock()
		store.subscribers["run-1"] = []chan event.Event{ch1}
		store.mu.Unlock()

		// Should not panic
		store.unsubscribe("run-1", ch2)

		store.mu.RLock()
		subs := store.subscribers["run-1"]
		store.mu.RUnlock()

		if len(subs) != 1 {
			t.Errorf("subscribers count = %d, want 1", len(subs))
		}
	})
}

// TestEventStore_InterfaceCompliance verifies that EventStore implements the required interfaces.
func TestEventStore_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// These compile-time checks verify interface compliance
	var _ event.Store = (*EventStore)(nil)
	var _ event.Querier = (*EventStore)(nil)
	var _ event.Snapshotter = (*EventStore)(nil)
	var _ event.Pruner = (*EventStore)(nil)
}
