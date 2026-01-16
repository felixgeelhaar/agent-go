package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMemoryLogger(t *testing.T) {
	ctx := context.Background()
	logger := NewMemoryLogger()

	event := Event{
		EventType: EventToolExecution,
		ToolName:  "test_tool",
		Success:   true,
	}

	err := logger.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	events := logger.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].ToolName != "test_tool" {
		t.Errorf("expected tool name test_tool, got %s", events[0].ToolName)
	}

	if events[0].Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestMemoryLoggerMaxEvents(t *testing.T) {
	ctx := context.Background()
	logger := NewMemoryLogger(WithMaxEvents(5))

	// Log 10 events
	for i := 0; i < 10; i++ {
		logger.Log(ctx, Event{
			EventType: EventToolExecution,
			ToolName:  "test",
		})
	}

	events := logger.Events()
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestMemoryLoggerQuery(t *testing.T) {
	ctx := context.Background()
	logger := NewMemoryLogger()

	// Log various events
	logger.Log(ctx, Event{
		EventType: EventToolExecution,
		ToolName:  "tool_a",
		Success:   true,
		RunID:     "run-1",
	})
	logger.Log(ctx, Event{
		EventType: EventToolExecution,
		ToolName:  "tool_b",
		Success:   false,
		RunID:     "run-1",
	})
	logger.Log(ctx, Event{
		EventType: EventStateTransition,
		RunID:     "run-2",
		Success:   true,
	})

	// Query by event type
	events, err := logger.Query(ctx, Filter{
		EventTypes: []EventType{EventToolExecution},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 tool execution events, got %d", len(events))
	}

	// Query by run ID
	events, err = logger.Query(ctx, Filter{
		RunID: "run-1",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events for run-1, got %d", len(events))
	}

	// Query by success
	success := true
	events, err = logger.Query(ctx, Filter{
		Success: &success,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 successful events, got %d", len(events))
	}

	// Query by tool name
	events, err = logger.Query(ctx, Filter{
		ToolName: "tool_a",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event for tool_a, got %d", len(events))
	}

	// Query with limit
	events, err = logger.Query(ctx, Filter{
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event with limit, got %d", len(events))
	}
}

func TestMemoryLoggerQueryTimeRange(t *testing.T) {
	ctx := context.Background()
	logger := NewMemoryLogger()

	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	logger.Log(ctx, Event{
		Timestamp: past,
		EventType: EventToolExecution,
		ToolName:  "old_event",
	})
	logger.Log(ctx, Event{
		Timestamp: now,
		EventType: EventToolExecution,
		ToolName:  "current_event",
	})
	logger.Log(ctx, Event{
		Timestamp: future,
		EventType: EventToolExecution,
		ToolName:  "future_event",
	})

	// Query events after past
	events, err := logger.Query(ctx, Filter{
		StartTime: past.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events after past+30min, got %d", len(events))
	}

	// Query events before future
	events, err = logger.Query(ctx, Filter{
		EndTime: now.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events before now+30min, got %d", len(events))
	}
}

func TestJSONLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf)

	ctx := context.Background()
	event := Event{
		EventType: EventToolExecution,
		ToolName:  "test_tool",
		Success:   true,
		RunID:     "run-123",
	}

	err := logger.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Parse the output
	var logged Event
	if err := json.Unmarshal(buf.Bytes(), &logged); err != nil {
		t.Fatalf("failed to parse logged event: %v", err)
	}

	if logged.ToolName != "test_tool" {
		t.Errorf("expected tool name test_tool, got %s", logged.ToolName)
	}

	if logged.RunID != "run-123" {
		t.Errorf("expected run ID run-123, got %s", logged.RunID)
	}
}

func TestJSONLoggerSetsTimestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf)

	ctx := context.Background()
	event := Event{
		EventType: EventToolExecution,
		ToolName:  "test",
	}

	before := time.Now()
	logger.Log(ctx, event)
	after := time.Now()

	var logged Event
	json.Unmarshal(buf.Bytes(), &logged)

	if logged.Timestamp.Before(before) || logged.Timestamp.After(after) {
		t.Error("timestamp should be set to current time")
	}
}

func TestJSONLoggerQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf)

	ctx := context.Background()

	// Query should return nil, nil (not supported)
	events, err := logger.Query(ctx, Filter{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if events != nil {
		t.Error("expected nil events")
	}
}

func TestMultiLogger(t *testing.T) {
	ctx := context.Background()

	memory := NewMemoryLogger()
	var buf bytes.Buffer
	jsonLogger := NewJSONLogger(&buf)

	multi := NewMultiLogger(memory, jsonLogger)

	event := Event{
		EventType: EventToolExecution,
		ToolName:  "test_tool",
		Success:   true,
	}

	err := multi.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Verify it's in memory logger
	events := memory.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event in memory, got %d", len(events))
	}

	// Verify it's in JSON logger
	if buf.Len() == 0 {
		t.Error("expected JSON output")
	}
}

func TestMultiLoggerQuery(t *testing.T) {
	ctx := context.Background()

	memory := NewMemoryLogger()
	memory.Log(ctx, Event{
		EventType: EventToolExecution,
		ToolName:  "test",
	})

	var buf bytes.Buffer
	jsonLogger := NewJSONLogger(&buf)

	// JSON logger first (doesn't support query), memory second
	multi := NewMultiLogger(jsonLogger, memory)

	events, err := multi.Query(ctx, Filter{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should return results from memory logger
	if len(events) != 1 {
		t.Errorf("expected 1 event from query, got %d", len(events))
	}
}

func TestMultiLoggerClose(t *testing.T) {
	memory := NewMemoryLogger()
	var buf bytes.Buffer
	jsonLogger := NewJSONLogger(&buf)

	multi := NewMultiLogger(memory, jsonLogger)

	err := multi.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestEventTypes(t *testing.T) {
	// Verify event type constants are strings
	types := []EventType{
		EventToolExecution,
		EventToolApproval,
		EventToolRejection,
		EventStateTransition,
		EventValidationFailure,
		EventAuthorizationCheck,
		EventSecretAccess,
		EventPolicyViolation,
		EventBudgetExceeded,
		EventRunStart,
		EventRunComplete,
		EventRunFailed,
	}

	for _, et := range types {
		if string(et) == "" {
			t.Errorf("event type should not be empty")
		}
	}
}

func TestEventAnnotations(t *testing.T) {
	ctx := context.Background()
	logger := NewMemoryLogger()

	event := Event{
		EventType: EventToolExecution,
		Annotations: map[string]interface{}{
			"custom_field": "custom_value",
			"count":        42,
		},
	}

	err := logger.Log(ctx, event)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	events := logger.Events()
	if events[0].Annotations["custom_field"] != "custom_value" {
		t.Error("custom field not preserved")
	}
	if events[0].Annotations["count"] != 42 {
		t.Error("count field not preserved")
	}
}

func TestMemoryLoggerClose(t *testing.T) {
	logger := NewMemoryLogger()
	err := logger.Close()
	if err != nil {
		t.Errorf("Close should not fail: %v", err)
	}
}
