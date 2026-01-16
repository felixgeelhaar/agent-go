package planner

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

func TestNewScriptedPlanner(t *testing.T) {
	t.Parallel()

	t.Run("empty steps", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner()
		if planner == nil {
			t.Fatal("NewScriptedPlanner() returned nil")
		}
		if !planner.IsComplete() {
			t.Error("Empty planner should be complete")
		}
	})

	t.Run("with steps", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
			ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewFinishDecision("done", nil)},
		)

		if planner.IsComplete() {
			t.Error("Planner with steps should not be complete initially")
		}
		if planner.CurrentStep() != 0 {
			t.Errorf("CurrentStep() = %d, want 0", planner.CurrentStep())
		}
	})
}

func TestScriptedPlanner_Plan(t *testing.T) {
	t.Parallel()

	t.Run("returns decisions when state matches", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
			ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
			ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", nil)},
		)

		ctx := context.Background()

		// Step 1: intake -> explore
		d1, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
		if err != nil {
			t.Fatalf("Plan() step 1 error = %v", err)
		}
		if d1.Type != agent.DecisionTransition {
			t.Errorf("Step 1 decision type = %v, want transition", d1.Type)
		}
		if d1.Transition.ToState != agent.StateExplore {
			t.Errorf("Step 1 ToState = %v, want explore", d1.Transition.ToState)
		}

		// Step 2: explore -> decide
		d2, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
		if err != nil {
			t.Fatalf("Plan() step 2 error = %v", err)
		}
		if d2.Transition.ToState != agent.StateDecide {
			t.Errorf("Step 2 ToState = %v, want decide", d2.Transition.ToState)
		}

		// Step 3: decide -> finish
		d3, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateDecide})
		if err != nil {
			t.Fatalf("Plan() step 3 error = %v", err)
		}
		if d3.Type != agent.DecisionFinish {
			t.Errorf("Step 3 decision type = %v, want finish", d3.Type)
		}

		// Planner should be complete
		if !planner.IsComplete() {
			t.Error("Planner should be complete after all steps")
		}
	})

	t.Run("returns error on unexpected state", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
		)

		ctx := context.Background()

		// Wrong state
		_, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
		if err == nil {
			t.Fatal("Plan() should return error for unexpected state")
		}

		unexpectedErr, ok := err.(*UnexpectedStateError)
		if !ok {
			t.Fatalf("Error should be *UnexpectedStateError, got %T", err)
		}
		if unexpectedErr.Expected != agent.StateIntake {
			t.Errorf("Expected state = %v, want intake", unexpectedErr.Expected)
		}
		if unexpectedErr.Actual != agent.StateExplore {
			t.Errorf("Actual state = %v, want explore", unexpectedErr.Actual)
		}
	})

	t.Run("empty expect state matches any state", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: "", Decision: agent.NewFinishDecision("done", nil)},
		)

		ctx := context.Background()

		// Any state should match
		_, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateAct})
		if err != nil {
			t.Fatalf("Plan() error = %v, expected nil for empty ExpectState", err)
		}
	})
}

func TestScriptedPlanner_Plan_WithCondition(t *testing.T) {
	t.Parallel()

	t.Run("condition passes", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    agent.NewFinishDecision("done", nil),
				Condition: func(req PlanRequest) bool {
					return len(req.AllowedTools) > 0
				},
			},
		)

		ctx := context.Background()

		_, err := planner.Plan(ctx, PlanRequest{
			CurrentState: agent.StateExplore,
			AllowedTools: []string{"read_file"},
		})
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
	})

	t.Run("condition fails", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    agent.NewFinishDecision("done", nil),
				Condition: func(req PlanRequest) bool {
					return len(req.AllowedTools) > 0
				},
			},
		)

		ctx := context.Background()

		_, err := planner.Plan(ctx, PlanRequest{
			CurrentState: agent.StateExplore,
			AllowedTools: []string{}, // Empty - condition fails
		})
		if err == nil {
			t.Fatal("Plan() should return error when condition fails")
		}

		condErr, ok := err.(*ConditionFailedError)
		if !ok {
			t.Fatalf("Error should be *ConditionFailedError, got %T", err)
		}
		if condErr.State != agent.StateExplore {
			t.Errorf("State = %v, want explore", condErr.State)
		}
	})
}

func TestScriptedPlanner_OnUnexpected(t *testing.T) {
	t.Parallel()

	customHandler := func(req PlanRequest) agent.Decision {
		return agent.NewFinishDecision("custom finish", json.RawMessage(`{"custom": true}`))
	}

	planner := NewScriptedPlanner(
		ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
	).OnUnexpected(customHandler)

	ctx := context.Background()

	// Use up the single step
	_, _ = planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})

	// Next call should use custom handler
	d, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if d.Type != agent.DecisionFinish {
		t.Errorf("Decision type = %v, want finish", d.Type)
	}
	if d.Finish.Summary != "custom finish" {
		t.Errorf("Summary = %v, want 'custom finish'", d.Finish.Summary)
	}
}

func TestScriptedPlanner_Reset(t *testing.T) {
	t.Parallel()

	planner := NewScriptedPlanner(
		ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
		ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewFinishDecision("done", nil)},
	)

	ctx := context.Background()

	// Use first step
	_, _ = planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
	if planner.CurrentStep() != 1 {
		t.Errorf("CurrentStep() after one Plan = %d, want 1", planner.CurrentStep())
	}

	// Reset
	planner.Reset()
	if planner.CurrentStep() != 0 {
		t.Errorf("CurrentStep() after Reset = %d, want 0", planner.CurrentStep())
	}

	// Can use first step again
	d, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
	if err != nil {
		t.Fatalf("Plan() after reset error = %v", err)
	}
	if d.Transition.ToState != agent.StateExplore {
		t.Errorf("ToState after reset = %v, want explore", d.Transition.ToState)
	}
}

func TestScriptedPlanner_Concurrency(t *testing.T) {
	t.Parallel()

	numSteps := 100
	steps := make([]ScriptStep, numSteps)
	for i := 0; i < numSteps; i++ {
		steps[i] = ScriptStep{
			ExpectState: "", // Match any state
			Decision:    agent.NewFinishDecision("done", nil),
		}
	}
	planner := NewScriptedPlanner(steps...)

	var wg sync.WaitGroup
	for i := 0; i < numSteps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = planner.Plan(context.Background(), PlanRequest{})
		}()
	}

	wg.Wait()

	if !planner.IsComplete() {
		t.Errorf("Planner should be complete after all concurrent calls, CurrentStep = %d", planner.CurrentStep())
	}
}

func TestUnexpectedStateError_Error(t *testing.T) {
	t.Parallel()

	err := &UnexpectedStateError{
		Expected:  agent.StateIntake,
		Actual:    agent.StateExplore,
		StepIndex: 0,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should return non-empty string")
	}
}

func TestConditionFailedError_Error(t *testing.T) {
	t.Parallel()

	err := &ConditionFailedError{
		StepIndex: 1,
		State:     agent.StateExplore,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should return non-empty string")
	}
}
