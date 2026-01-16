package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
)

// mockEventStore implements event.Store for testing.
type mockEventStore struct {
	appendFn        func(ctx context.Context, events ...event.Event) error
	loadEventsFn    func(ctx context.Context, runID string) ([]event.Event, error)
	loadEventsFromFn func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error)
	subscribeFn     func(ctx context.Context, runID string) (<-chan event.Event, error)
}

func (m *mockEventStore) Append(ctx context.Context, events ...event.Event) error {
	if m.appendFn != nil {
		return m.appendFn(ctx, events...)
	}
	return nil
}

func (m *mockEventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	if m.loadEventsFn != nil {
		return m.loadEventsFn(ctx, runID)
	}
	return []event.Event{}, nil
}

func (m *mockEventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	if m.loadEventsFromFn != nil {
		return m.loadEventsFromFn(ctx, runID, fromSeq)
	}
	return []event.Event{}, nil
}

func (m *mockEventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	if m.subscribeFn != nil {
		return m.subscribeFn(ctx, runID)
	}
	return nil, nil
}

func createTestEvent(runID string, eventType event.Type, payload any, seq uint64) event.Event {
	data, _ := json.Marshal(payload)
	return event.Event{
		ID:        "evt-" + runID,
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   data,
		Sequence:  seq,
	}
}

func TestNewReplay(t *testing.T) {
	t.Parallel()

	store := &mockEventStore{}
	replay := application.NewReplay(store)

	if replay == nil {
		t.Error("NewReplay should return non-nil replay")
	}
}

func TestReplay_ReconstructRun(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error")
		}
	})

	t.Run("no events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if !errors.Is(err, event.ErrRunNotFound) {
			t.Errorf("ReconstructRun() error = %v, want %v", err, event.ErrRunNotFound)
		}
	})

	t.Run("reconstruct from run.started", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
						Vars: map[string]any{"key": "value"},
					}, 1),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Goal != "Test goal" {
			t.Errorf("Run.Goal = %s, want 'Test goal'", run.Goal)
		}
	})

	t.Run("reconstruct with state transitions", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
						FromState: agent.StateIntake,
						ToState:   agent.StateExplore,
						Reason:    "begin exploration",
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateExplore {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateExplore)
		}
	})

	t.Run("reconstruct with completion", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeRunCompleted, event.RunCompletedPayload{
						Result:   json.RawMessage(`{"success": true}`),
						Duration: 5 * time.Second,
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateDone {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateDone)
		}
	})

	t.Run("reconstruct with failure", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeRunFailed, event.RunFailedPayload{
						Error:    "something went wrong",
						State:    agent.StateAct,
						Duration: 3 * time.Second,
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateFailed {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateFailed)
		}
	})

	t.Run("reconstruct with pause and resume", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeRunPaused, nil, 2),
					createTestEvent(runID, event.TypeRunResumed, nil, 3),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})

	t.Run("reconstruct with evidence", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "observation",
						Source:  "read_file",
						Content: json.RawMessage(`{"data": "test"}`),
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if len(run.Evidence) != 1 {
			t.Errorf("len(Run.Evidence) = %d, want 1", len(run.Evidence))
		}
	})

	t.Run("reconstruct with variable set", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeVariableSet, event.VariableSetPayload{
						Key:   "myVar",
						Value: "myValue",
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Vars["myVar"] != "myValue" {
			t.Errorf("Run.Vars[myVar] = %v, want 'myValue'", run.Vars["myVar"])
		}
	})

	t.Run("handles tool and audit events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeToolCalled, event.ToolCalledPayload{
						ToolName: "read_file",
					}, 2),
					createTestEvent(runID, event.TypeToolSucceeded, event.ToolSucceededPayload{
						ToolName: "read_file",
					}, 3),
					createTestEvent(runID, event.TypeDecisionMade, event.DecisionMadePayload{
						DecisionType: "call_tool",
					}, 4),
					createTestEvent(runID, event.TypeBudgetConsumed, event.BudgetConsumedPayload{
						BudgetName: "calls",
					}, 5),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

func TestReplay_ReconstructRunFrom(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if err == nil {
			t.Error("ReconstructRunFrom() should return error")
		}
	})

	t.Run("no events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if !errors.Is(err, event.ErrRunNotFound) {
			t.Errorf("ReconstructRunFrom() error = %v, want %v", err, event.ErrRunNotFound)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 5),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if err != nil {
			t.Fatalf("ReconstructRunFrom() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

func TestReplay_NewEventIterator(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.NewEventIterator(context.Background(), "run-1")
		if err == nil {
			t.Error("NewEventIterator() should return error")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{ID: "e1", Sequence: 1},
					{ID: "e2", Sequence: 2},
					{ID: "e3", Sequence: 3},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		iter, err := replay.NewEventIterator(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("NewEventIterator() error = %v", err)
		}
		if iter.Len() != 3 {
			t.Errorf("Len() = %d, want 3", iter.Len())
		}
	})
}

func TestEventIterator(t *testing.T) {
	t.Parallel()

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{ID: "e1", Sequence: 1},
				{ID: "e2", Sequence: 2},
				{ID: "e3", Sequence: 3},
			}, nil
		},
	}
	replay := application.NewReplay(store)

	iter, _ := replay.NewEventIterator(context.Background(), "run-1")

	t.Run("next", func(t *testing.T) {
		e := iter.Next()
		if e == nil {
			t.Fatal("Next() should return event")
		}
		if e.ID != "e1" {
			t.Errorf("Event.ID = %s, want e1", e.ID)
		}
	})

	t.Run("index", func(t *testing.T) {
		if iter.Index() != 1 {
			t.Errorf("Index() = %d, want 1", iter.Index())
		}
	})

	t.Run("peek", func(t *testing.T) {
		e := iter.Peek()
		if e == nil {
			t.Fatal("Peek() should return event")
		}
		if e.ID != "e2" {
			t.Errorf("Event.ID = %s, want e2", e.ID)
		}
		// Peek should not advance
		if iter.Index() != 1 {
			t.Errorf("Index() = %d, want 1", iter.Index())
		}
	})

	t.Run("reset", func(t *testing.T) {
		iter.Reset()
		if iter.Index() != 0 {
			t.Errorf("Index() = %d, want 0", iter.Index())
		}
	})

	t.Run("iterate to end", func(t *testing.T) {
		iter.Reset()
		count := 0
		for iter.Next() != nil {
			count++
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
		// Next on exhausted iterator
		if iter.Next() != nil {
			t.Error("Next() should return nil after exhausted")
		}
		// Peek on exhausted iterator
		if iter.Peek() != nil {
			t.Error("Peek() should return nil after exhausted")
		}
	})
}

func TestReplay_NewTimeline(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.NewTimeline(context.Background(), "run-1")
		if err == nil {
			t.Error("NewTimeline() should return error")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		tl, err := replay.NewTimeline(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("NewTimeline() error = %v", err)
		}
		if tl == nil {
			t.Error("Timeline should not be nil")
		}
	})
}

func TestTimeline_Duration(t *testing.T) {
	t.Parallel()

	t.Run("no events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		if tl.Duration() != 0 {
			t.Errorf("Duration() = %v, want 0", tl.Duration())
		}
	})

	t.Run("one event", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Timestamp: time.Now()},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		if tl.Duration() != 0 {
			t.Errorf("Duration() = %v, want 0", tl.Duration())
		}
	})

	t.Run("multiple events", func(t *testing.T) {
		t.Parallel()

		start := time.Now()
		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Timestamp: start},
					{Timestamp: start.Add(5 * time.Second)},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		if tl.Duration() != 5*time.Second {
			t.Errorf("Duration() = %v, want 5s", tl.Duration())
		}
	})
}

func TestTimeline_EventsInRange(t *testing.T) {
	t.Parallel()

	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{ID: "e1", Timestamp: start},
				{ID: "e2", Timestamp: start.Add(1 * time.Minute)},
				{ID: "e3", Timestamp: start.Add(2 * time.Minute)},
				{ID: "e4", Timestamp: start.Add(3 * time.Minute)},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	t.Run("all events with zero times", func(t *testing.T) {
		events := tl.EventsInRange(time.Time{}, time.Time{})
		if len(events) != 4 {
			t.Errorf("len(events) = %d, want 4", len(events))
		}
	})

	t.Run("from time only", func(t *testing.T) {
		events := tl.EventsInRange(start.Add(90*time.Second), time.Time{})
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
	})

	t.Run("to time only", func(t *testing.T) {
		events := tl.EventsInRange(time.Time{}, start.Add(90*time.Second))
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
	})

	t.Run("both times", func(t *testing.T) {
		events := tl.EventsInRange(start.Add(30*time.Second), start.Add(150*time.Second))
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
	})
}

func TestTimeline_EventsByType(t *testing.T) {
	t.Parallel()

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{Type: event.TypeRunStarted},
				{Type: event.TypeStateTransitioned},
				{Type: event.TypeStateTransitioned},
				{Type: event.TypeRunCompleted},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	events := tl.EventsByType(event.TypeStateTransitioned)
	if len(events) != 2 {
		t.Errorf("len(events) = %d, want 2", len(events))
	}
}

func TestTimeline_StateTransitions(t *testing.T) {
	t.Parallel()

	transitionPayload1, _ := json.Marshal(event.StateTransitionedPayload{
		FromState: agent.StateIntake,
		ToState:   agent.StateExplore,
		Reason:    "begin",
	})
	transitionPayload2, _ := json.Marshal(event.StateTransitionedPayload{
		FromState: agent.StateExplore,
		ToState:   agent.StateDecide,
		Reason:    "ready",
	})

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{Type: event.TypeRunStarted},
				{Type: event.TypeStateTransitioned, Payload: transitionPayload1},
				{Type: event.TypeStateTransitioned, Payload: transitionPayload2},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	transitions := tl.StateTransitions()
	if len(transitions) != 2 {
		t.Errorf("len(transitions) = %d, want 2", len(transitions))
	}
	if transitions[0].From != agent.StateIntake {
		t.Errorf("transitions[0].From = %s, want %s", transitions[0].From, agent.StateIntake)
	}
	if transitions[0].To != agent.StateExplore {
		t.Errorf("transitions[0].To = %s, want %s", transitions[0].To, agent.StateExplore)
	}
}

func TestTimeline_ToolCalls(t *testing.T) {
	t.Parallel()

	calledPayload, _ := json.Marshal(event.ToolCalledPayload{
		ToolName: "read_file",
		Input:    json.RawMessage(`{"path": "/test"}`),
		State:    agent.StateExplore,
	})
	succeededPayload, _ := json.Marshal(event.ToolSucceededPayload{
		ToolName: "read_file",
		Output:   json.RawMessage(`{"content": "data"}`),
		Duration: 100 * time.Millisecond,
		Cached:   false,
	})
	failedPayload, _ := json.Marshal(event.ToolFailedPayload{
		ToolName: "write_file",
		Error:    "permission denied",
		Duration: 50 * time.Millisecond,
	})

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{Type: event.TypeToolCalled, Payload: calledPayload, Sequence: 1},
				{Type: event.TypeToolSucceeded, Payload: succeededPayload, Sequence: 2},
				{Type: event.TypeToolCalled, Payload: json.RawMessage(`{"tool_name": "write_file"}`), Sequence: 3},
				{Type: event.TypeToolFailed, Payload: failedPayload, Sequence: 4},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	calls := tl.ToolCalls()
	if len(calls) != 2 {
		t.Errorf("len(calls) = %d, want 2", len(calls))
	}

	// Find the successful call
	var successCall *application.ToolCall
	for i := range calls {
		if calls[i].ToolName == "read_file" {
			successCall = &calls[i]
			break
		}
	}
	if successCall == nil {
		t.Fatal("read_file call not found")
	}
	if !successCall.Success {
		t.Error("read_file call should be successful")
	}
}
