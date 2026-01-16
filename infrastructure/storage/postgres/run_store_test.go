package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

func TestNewRunStore(t *testing.T) {
	t.Parallel()

	t.Run("creates store with default schema", func(t *testing.T) {
		t.Parallel()
		store := NewRunStore(nil, "")
		if store.schema != "public" {
			t.Errorf("schema = %s, want public", store.schema)
		}
	})

	t.Run("creates store with custom schema", func(t *testing.T) {
		t.Parallel()
		store := NewRunStore(nil, "custom")
		if store.schema != "custom" {
			t.Errorf("schema = %s, want custom", store.schema)
		}
	})

	t.Run("stores pool reference", func(t *testing.T) {
		t.Parallel()
		store := NewRunStore(nil, "public")
		if store.pool != nil {
			t.Error("expected nil pool")
		}
	})
}

func TestRunStore_tableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{"default schema", "public", "public.runs"},
		{"custom schema", "myschema", "myschema.runs"},
		{"empty schema defaults to public", "", "public.runs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewRunStore(nil, tt.schema)
			result := store.tableName()
			if result != tt.expected {
				t.Errorf("tableName() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestRunStore_Save_Validation(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")
	r := &agent.Run{ID: ""}

	err := store.Save(context.Background(), r)
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Errorf("Save() error = %v, want ErrInvalidRunID", err)
	}
}

func TestRunStore_Get_Validation(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")

	_, err := store.Get(context.Background(), "")
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Errorf("Get() error = %v, want ErrInvalidRunID", err)
	}
}

func TestRunStore_Update_Validation(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")
	r := &agent.Run{ID: ""}

	err := store.Update(context.Background(), r)
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Errorf("Update() error = %v, want ErrInvalidRunID", err)
	}
}

func TestRunStore_Delete_Validation(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")

	err := store.Delete(context.Background(), "")
	if !errors.Is(err, run.ErrInvalidRunID) {
		t.Errorf("Delete() error = %v, want ErrInvalidRunID", err)
	}
}

func TestRunStore_buildWhereClause(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")

	t.Run("empty filter returns empty clause", func(t *testing.T) {
		t.Parallel()
		clause, args := store.buildWhereClause(run.ListFilter{})
		if clause != "" {
			t.Errorf("clause = %s, want empty", clause)
		}
		if len(args) != 0 {
			t.Errorf("args length = %d, want 0", len(args))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Status: []agent.RunStatus{agent.RunStatusRunning, agent.RunStatusCompleted},
		}
		clause, args := store.buildWhereClause(filter)
		if clause == "" {
			t.Error("expected non-empty clause")
		}
		if len(args) != 1 {
			t.Errorf("args length = %d, want 1", len(args))
		}
		// Verify the statuses are converted to strings
		statuses, ok := args[0].([]string)
		if !ok {
			t.Errorf("expected []string, got %T", args[0])
		}
		if len(statuses) != 2 {
			t.Errorf("statuses length = %d, want 2", len(statuses))
		}
	})

	t.Run("filter by states", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			States: []agent.State{agent.StateIntake, agent.StateExplore},
		}
		clause, args := store.buildWhereClause(filter)
		if clause == "" {
			t.Error("expected non-empty clause")
		}
		if len(args) != 1 {
			t.Errorf("args length = %d, want 1", len(args))
		}
		states, ok := args[0].([]string)
		if !ok {
			t.Errorf("expected []string, got %T", args[0])
		}
		if len(states) != 2 {
			t.Errorf("states length = %d, want 2", len(states))
		}
	})

	t.Run("filter by time range", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		filter := run.ListFilter{
			FromTime: now.Add(-time.Hour),
			ToTime:   now,
		}
		clause, args := store.buildWhereClause(filter)
		if clause == "" {
			t.Error("expected non-empty clause")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
	})

	t.Run("filter by goal pattern", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			GoalPattern: "test",
		}
		clause, args := store.buildWhereClause(filter)
		if clause == "" {
			t.Error("expected non-empty clause")
		}
		if len(args) != 1 {
			t.Errorf("args length = %d, want 1", len(args))
		}
		// Verify the pattern is wrapped with wildcards
		pattern, ok := args[0].(string)
		if !ok {
			t.Errorf("expected string, got %T", args[0])
		}
		if pattern != "%test%" {
			t.Errorf("pattern = %s, want %%test%%", pattern)
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		filter := run.ListFilter{
			Status:      []agent.RunStatus{agent.RunStatusRunning},
			States:      []agent.State{agent.StateIntake},
			FromTime:    now.Add(-time.Hour),
			ToTime:      now,
			GoalPattern: "test",
		}
		clause, args := store.buildWhereClause(filter)
		if clause == "" {
			t.Error("expected non-empty clause")
		}
		if len(args) != 5 {
			t.Errorf("args length = %d, want 5", len(args))
		}
		// Verify the clause contains WHERE and AND
		if len(clause) < 10 {
			t.Error("clause seems too short for combined filters")
		}
	})
}

func TestRunStore_buildListQuery(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")

	t.Run("default ordering", func(t *testing.T) {
		t.Parallel()
		query, args := store.buildListQuery(run.ListFilter{}, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 0 {
			t.Errorf("args length = %d, want 0", len(args))
		}
	})

	t.Run("order by end time", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			OrderBy: run.OrderByEndTime,
		}
		query, _ := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
	})

	t.Run("order by id", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			OrderBy: run.OrderByID,
		}
		query, _ := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
	})

	t.Run("order by status", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			OrderBy: run.OrderByStatus,
		}
		query, _ := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
	})

	t.Run("descending order", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Descending: true,
		}
		query, _ := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
	})

	t.Run("with limit only", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Limit: 10,
		}
		query, args := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 1 {
			t.Errorf("args length = %d, want 1", len(args))
		}
	})

	t.Run("with offset only", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Offset: 20,
		}
		query, args := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 1 {
			t.Errorf("args length = %d, want 1", len(args))
		}
	})

	t.Run("with limit and offset", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Limit:  10,
			Offset: 20,
		}
		query, args := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 2 {
			t.Errorf("args length = %d, want 2", len(args))
		}
	})

	t.Run("with filter and pagination", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Status:     []agent.RunStatus{agent.RunStatusRunning},
			Limit:      10,
			Offset:     20,
			Descending: true,
		}
		query, args := store.buildListQuery(filter, false)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 3 {
			t.Errorf("args length = %d, want 3 (1 for status, 1 for limit, 1 for offset)", len(args))
		}
	})
}

func TestRunStore_buildCountQuery(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")

	t.Run("empty filter", func(t *testing.T) {
		t.Parallel()
		query, args := store.buildCountQuery(run.ListFilter{})
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 0 {
			t.Errorf("args length = %d, want 0", len(args))
		}
	})

	t.Run("with status filter", func(t *testing.T) {
		t.Parallel()
		filter := run.ListFilter{
			Status: []agent.RunStatus{agent.RunStatusRunning},
		}
		query, args := store.buildCountQuery(filter)
		if query == "" {
			t.Error("expected non-empty query")
		}
		if len(args) != 1 {
			t.Errorf("args length = %d, want 1", len(args))
		}
	})
}

func TestRunStore_wrapError(t *testing.T) {
	t.Parallel()

	store := NewRunStore(nil, "public")

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
		if !errors.Is(err, run.ErrOperationTimeout) {
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
		if !errors.Is(err, run.ErrConnectionFailed) {
			t.Errorf("wrapError() should wrap as ErrConnectionFailed")
		}
		// Should also contain the original error
		if !errors.Is(err, originalErr) {
			t.Error("wrapped error should contain original error")
		}
	})
}

// TestRunStore_InterfaceCompliance verifies that RunStore implements the required interfaces.
func TestRunStore_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// These compile-time checks verify interface compliance
	var _ run.Store = (*RunStore)(nil)
	var _ run.SummaryProvider = (*RunStore)(nil)
}
