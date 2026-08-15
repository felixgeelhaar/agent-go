package api_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewEventStoreAndRunStore(t *testing.T) {
	t.Parallel()

	es := api.NewEventStore()
	if es == nil {
		t.Fatal("NewEventStore() nil")
	}
	rs := api.NewRunStore()
	if rs == nil {
		t.Fatal("NewRunStore() nil")
	}
}

func TestNewArtifactStore(t *testing.T) {
	t.Parallel()

	store, err := api.NewArtifactStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	if store == nil {
		t.Fatal("NewArtifactStore() nil")
	}
}

func TestNewExecutorWithOptions(t *testing.T) {
	t.Parallel()

	ex := api.NewExecutorWithOptions(
		api.WithExecutorTimeout(2*time.Second),
		api.WithExecutorMaxConcurrent(2),
		api.WithExecutorRetryAttempts(1),
	)
	if ex == nil {
		t.Fatal("NewExecutorWithOptions() nil")
	}
}

func TestNewDelegateTool(t *testing.T) {
	t.Parallel()

	childPlanner := api.NewMockPlanner(
		api.NewTransitionDecision(api.StateExplore, "start"),
		api.NewTransitionDecision(api.StateDecide, "decide"),
		api.NewFinishDecision("ok", json.RawMessage(`{"ok":true}`)),
	)
	child, err := api.New(api.WithPlanner(childPlanner), api.WithMaxSteps(10))
	if err != nil {
		t.Fatalf("child New: %v", err)
	}

	delegate := api.NewDelegateTool("child", "delegates", child)
	if delegate.Name() != "child" {
		t.Fatalf("Name = %q", delegate.Name())
	}

	parentPlanner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "go")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewCallToolDecision("child", json.RawMessage(`{"goal":"hi"}`), "delegate")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "done")},
		api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("done", nil)},
	)

	parent, err := api.New(
		api.WithPlanner(parentPlanner),
		api.WithTool(delegate),
		api.WithMaxSteps(20),
	)
	if err != nil {
		t.Fatalf("parent New: %v", err)
	}

	run, err := parent.Run(context.Background(), "parent goal")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != api.StatusCompleted {
		t.Fatalf("status = %s", run.Status)
	}
}
