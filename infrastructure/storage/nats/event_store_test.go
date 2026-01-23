package nats

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
)

func TestNewEventStore(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Client:        NewMockClient(),
				SubjectPrefix: "test-events",
			},
			wantErr: false,
		},
		{
			name: "default prefix",
			cfg: Config{
				Client: NewMockClient(),
			},
			wantErr: false,
		},
		{
			name: "missing client",
			cfg: Config{
				SubjectPrefix: "test-events",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEventStore(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEventStore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppend(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	runID := "run-123"

	tests := []struct {
		name    string
		events  []event.Event
		wantErr bool
	}{
		{
			name:    "append single event",
			events:  []event.Event{makeEvent(runID, event.TypeRunStarted)},
			wantErr: false,
		},
		{
			name: "append multiple events",
			events: []event.Event{
				makeEvent(runID, event.TypeStateTransitioned),
				makeEvent(runID, event.TypeToolCalled),
			},
			wantErr: false,
		},
		{
			name:    "append empty events",
			events:  []event.Event{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Append(ctx, tt.events...)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadEvents(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	runID := "run-456"

	// Append some events
	events := []event.Event{
		makeEvent(runID, event.TypeRunStarted),
		makeEvent(runID, event.TypeStateTransitioned),
		makeEvent(runID, event.TypeToolCalled),
	}
	_ = store.Append(ctx, events...)

	// Load events
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents error: %v", err)
	}

	if len(loaded) != len(events) {
		t.Errorf("expected %d events, got %d", len(events), len(loaded))
	}

	// Verify sequence numbers
	for i, evt := range loaded {
		if evt.Sequence != uint64(i+1) {
			t.Errorf("expected sequence %d, got %d", i+1, evt.Sequence)
		}
	}
}

func TestLoadEventsFrom(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	runID := "run-789"

	// Append some events
	events := []event.Event{
		makeEvent(runID, event.TypeRunStarted),
		makeEvent(runID, event.TypeStateTransitioned),
		makeEvent(runID, event.TypeToolCalled),
		makeEvent(runID, event.TypeToolSucceeded),
	}
	_ = store.Append(ctx, events...)

	// Load events from sequence 3
	loaded, err := store.LoadEventsFrom(ctx, runID, 3)
	if err != nil {
		t.Fatalf("LoadEventsFrom error: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 events, got %d", len(loaded))
	}

	// Verify sequence numbers
	if loaded[0].Sequence != 3 {
		t.Errorf("expected first sequence 3, got %d", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 4 {
		t.Errorf("expected second sequence 4, got %d", loaded[1].Sequence)
	}
}

func TestSubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	runID := "run-sub"

	// Subscribe to events
	ch, err := store.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	// Append event after subscribing
	evt := makeEvent(runID, event.TypeRunStarted)
	_ = store.Append(ctx, evt)

	// Wait for event
	select {
	case received := <-ch:
		if received.Type != event.TypeRunStarted {
			t.Errorf("expected type %s, got %s", event.TypeRunStarted, received.Type)
		}
	case <-ctx.Done():
		t.Error("timeout waiting for event")
	}
}

func TestSubscribeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	runID := "run-cancel"

	// Subscribe to events
	ch, err := store.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	// Cancel context
	cancel()

	// Wait for channel to close
	select {
	case <-ch:
		// Channel closed or event received, both are acceptable
	case <-time.After(time.Second):
		t.Error("channel not closed after context cancellation")
	}
}

func TestLoadEventsEmpty(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	// Load events from non-existent run
	events, err := store.LoadEvents(ctx, "non-existent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestSequenceAssignment(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	store, _ := NewEventStore(Config{
		Client:        client,
		SubjectPrefix: "events",
	})

	runID1 := "run-seq-1"
	runID2 := "run-seq-2"

	// Append to different runs
	_ = store.Append(ctx, makeEvent(runID1, event.TypeRunStarted))
	_ = store.Append(ctx, makeEvent(runID2, event.TypeRunStarted))
	_ = store.Append(ctx, makeEvent(runID1, event.TypeToolCalled))
	_ = store.Append(ctx, makeEvent(runID2, event.TypeToolCalled))

	// Load events for run1
	events1, _ := store.LoadEvents(ctx, runID1)
	if events1[0].Sequence != 1 || events1[1].Sequence != 2 {
		t.Errorf("run1 sequences incorrect: got %d, %d", events1[0].Sequence, events1[1].Sequence)
	}

	// Load events for run2
	events2, _ := store.LoadEvents(ctx, runID2)
	if events2[0].Sequence != 1 || events2[1].Sequence != 2 {
		t.Errorf("run2 sequences incorrect: got %d, %d", events2[0].Sequence, events2[1].Sequence)
	}
}

func TestMockClient(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()

	// Test Publish
	err := client.Publish(ctx, "test.subject", []byte("test message"))
	if err != nil {
		t.Errorf("Publish error: %v", err)
	}

	// Test GetMessages
	msgs, err := client.GetMessages(ctx, "test.subject")
	if err != nil {
		t.Errorf("GetMessages error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
	if string(msgs[0]) != "test message" {
		t.Errorf("expected 'test message', got %s", string(msgs[0]))
	}

	// Test MessageCount
	if client.MessageCount("test.subject") != 1 {
		t.Errorf("expected message count 1, got %d", client.MessageCount("test.subject"))
	}

	// Test Subscribe with handler
	received := make(chan []byte, 1)
	_, err = client.Subscribe(ctx, "test.subject", func(data []byte) error {
		received <- data
		return nil
	})
	if err != nil {
		t.Errorf("Subscribe error: %v", err)
	}

	// Publish another message
	_ = client.Publish(ctx, "test.subject", []byte("second message"))

	select {
	case msg := <-received:
		if string(msg) != "second message" {
			t.Errorf("expected 'second message', got %s", string(msg))
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for subscribed message")
	}

	// Test Close
	if err := client.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func makeEvent(runID string, eventType event.Type) event.Event {
	return event.Event{
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   []byte(`{}`),
		Version:   1,
	}
}
