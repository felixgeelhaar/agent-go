package sqlite_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/sqlite"
)

func TestNewRunStore(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
	}

	store, err := sqlite.NewRunStore(cfg)
	if err != nil {
		t.Fatalf("NewRunStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

func TestRunStore_SaveAndGet(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	r := &agent.Run{
		ID:           "test-run-1",
		Goal:         "Test goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	// Save
	err := store.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Get
	loaded, err := store.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if loaded.ID != r.ID {
		t.Errorf("expected ID %s, got %s", r.ID, loaded.ID)
	}
	if loaded.Goal != r.Goal {
		t.Errorf("expected Goal %s, got %s", r.Goal, loaded.Goal)
	}
	if loaded.Status != r.Status {
		t.Errorf("expected Status %s, got %s", r.Status, loaded.Status)
	}
}

func TestRunStore_GetNotFound(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
	if err != run.ErrRunNotFound {
		t.Errorf("expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_GetInvalidID(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	_, err := store.Get(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if err != run.ErrInvalidRunID {
		t.Errorf("expected ErrInvalidRunID, got %v", err)
	}
}

func TestRunStore_Update(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	r := &agent.Run{
		ID:           "test-run-update",
		Goal:         "Original goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	// Save
	err := store.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Update
	r.Status = agent.RunStatusCompleted
	r.Result = json.RawMessage(`"Success"`)
	r.EndTime = time.Now()

	err = store.Update(ctx, r)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	loaded, err := store.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if loaded.Status != agent.RunStatusCompleted {
		t.Errorf("expected StatusCompleted, got %s", loaded.Status)
	}
	if string(loaded.Result) != `"Success"` {
		t.Errorf("expected Result '\"Success\"', got %s", string(loaded.Result))
	}
}

func TestRunStore_UpdateNotFound(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	r := &agent.Run{
		ID:           "nonexistent",
		Goal:         "Goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	err := store.Update(ctx, r)
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
	if err != run.ErrRunNotFound {
		t.Errorf("expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_Delete(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	r := &agent.Run{
		ID:           "test-run-delete",
		Goal:         "Goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	// Save
	err := store.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete
	err = store.Delete(ctx, r.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(ctx, r.ID)
	if err != run.ErrRunNotFound {
		t.Errorf("expected ErrRunNotFound after delete, got %v", err)
	}
}

func TestRunStore_DeleteNotFound(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
	if err != run.ErrRunNotFound {
		t.Errorf("expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_List(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create runs
	for i := 0; i < 5; i++ {
		r := &agent.Run{
			ID:           "test-run-list-" + string(rune('a'+i)),
			Goal:         "Goal",
			Status:       agent.RunStatusRunning,
			CurrentState: agent.StateExplore,
			StartTime:    time.Now().Add(time.Duration(i) * time.Second),
		}
		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List all
	runs, err := store.List(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 5 {
		t.Errorf("expected 5 runs, got %d", len(runs))
	}
}

func TestRunStore_ListWithStatusFilter(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create runs with different statuses
	statuses := []agent.RunStatus{
		agent.RunStatusRunning,
		agent.RunStatusCompleted,
		agent.RunStatusFailed,
		agent.RunStatusCompleted,
		agent.RunStatusRunning,
	}

	for i, status := range statuses {
		r := &agent.Run{
			ID:           "test-run-status-" + string(rune('a'+i)),
			Goal:         "Goal",
			Status:       status,
			CurrentState: agent.StateExplore,
			StartTime:    time.Now(),
		}
		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List only completed
	runs, err := store.List(ctx, run.ListFilter{
		Status: []agent.RunStatus{agent.RunStatusCompleted},
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("expected 2 completed runs, got %d", len(runs))
	}

	for _, r := range runs {
		if r.Status != agent.RunStatusCompleted {
			t.Errorf("expected StatusCompleted, got %s", r.Status)
		}
	}
}

func TestRunStore_ListWithLimit(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create runs
	for i := 0; i < 10; i++ {
		r := &agent.Run{
			ID:           "test-run-limit-" + string(rune('a'+i)),
			Goal:         "Goal",
			Status:       agent.RunStatusRunning,
			CurrentState: agent.StateExplore,
			StartTime:    time.Now(),
		}
		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List with limit
	runs, err := store.List(ctx, run.ListFilter{Limit: 5})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 5 {
		t.Errorf("expected 5 runs with limit, got %d", len(runs))
	}
}

func TestRunStore_Count(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create runs
	for i := 0; i < 7; i++ {
		r := &agent.Run{
			ID:           "test-run-count-" + string(rune('a'+i)),
			Goal:         "Goal",
			Status:       agent.RunStatusRunning,
			CurrentState: agent.StateExplore,
			StartTime:    time.Now(),
		}
		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	count, err := store.Count(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 7 {
		t.Errorf("expected 7 runs, got %d", count)
	}
}

func TestRunStore_Summary(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create runs with different statuses
	testData := []struct {
		id     string
		status agent.RunStatus
	}{
		{"run-1", agent.RunStatusCompleted},
		{"run-2", agent.RunStatusCompleted},
		{"run-3", agent.RunStatusFailed},
		{"run-4", agent.RunStatusRunning},
		{"run-5", agent.RunStatusCompleted},
	}

	for _, td := range testData {
		r := &agent.Run{
			ID:           td.id,
			Goal:         "Goal",
			Status:       td.status,
			CurrentState: agent.StateExplore,
			StartTime:    time.Now(),
		}
		if td.status == agent.RunStatusCompleted || td.status == agent.RunStatusFailed {
			r.EndTime = time.Now().Add(time.Second)
		}
		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	summary, err := store.Summary(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("Summary failed: %v", err)
	}

	if summary.TotalRuns != 5 {
		t.Errorf("expected TotalRuns 5, got %d", summary.TotalRuns)
	}
	if summary.CompletedRuns != 3 {
		t.Errorf("expected CompletedRuns 3, got %d", summary.CompletedRuns)
	}
	if summary.FailedRuns != 1 {
		t.Errorf("expected FailedRuns 1, got %d", summary.FailedRuns)
	}
	if summary.RunningRuns != 1 {
		t.Errorf("expected RunningRuns 1, got %d", summary.RunningRuns)
	}
}

func TestRunStore_SaveDuplicate(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	r := &agent.Run{
		ID:           "test-run-dup",
		Goal:         "Goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	// First save
	err := store.Save(ctx, r)
	if err != nil {
		t.Fatalf("First Save failed: %v", err)
	}

	// Second save should fail
	err = store.Save(ctx, r)
	if err == nil {
		t.Fatal("expected error for duplicate save")
	}
	if err != run.ErrRunExists {
		t.Errorf("expected ErrRunExists, got %v", err)
	}
}

func TestRunStore_SaveInvalidID(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	r := &agent.Run{
		ID:           "",
		Goal:         "Goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	err := store.Save(ctx, r)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if err != run.ErrInvalidRunID {
		t.Errorf("expected ErrInvalidRunID, got %v", err)
	}
}

func TestRunStore_ContextCancelled(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &agent.Run{
		ID:           "test",
		Goal:         "Goal",
		Status:       agent.RunStatusRunning,
		CurrentState: agent.StateExplore,
		StartTime:    time.Now(),
	}

	// Operations should fail with cancelled context
	err := store.Save(ctx, r)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	_, err = store.Get(ctx, "test")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRunStore_ListWithGoalPattern(t *testing.T) {
	store := newTestRunStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create runs with different goals
	goals := []string{
		"Process files in directory",
		"Process data from API",
		"Analyze log files",
		"Process images",
	}

	for i, goal := range goals {
		r := &agent.Run{
			ID:           "test-run-goal-" + string(rune('a'+i)),
			Goal:         goal,
			Status:       agent.RunStatusRunning,
			CurrentState: agent.StateExplore,
			StartTime:    time.Now(),
		}
		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List with goal pattern
	runs, err := store.List(ctx, run.ListFilter{
		GoalPattern: "Process",
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 3 {
		t.Errorf("expected 3 runs matching 'Process', got %d", len(runs))
	}
}

func newTestRunStore(t *testing.T) *sqlite.RunStore {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
	}

	store, err := sqlite.NewRunStore(cfg)
	if err != nil {
		t.Fatalf("NewRunStore failed: %v", err)
	}

	return store
}
