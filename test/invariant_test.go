// Package test contains the invariant test suite for the agent runtime.
package test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

// =============================================================================
// Invariant 1: Tool Eligibility
// A tool can only execute in states where it is explicitly allowed.
// =============================================================================

func TestInvariant_ToolEligibility(t *testing.T) {
	t.Run("tool_executes_only_in_allowed_state", func(t *testing.T) {
		// Create a tool
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"content": "hello"}`)}, nil
			}).
			MustBuild()

		// Create registry with the tool
		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		// Create eligibility that only allows the tool in "explore" state
		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Create scripted planner that tries to call tool from intake state
		// This should fail because tool is not allowed in intake
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "attempt from intake"),
			},
		)

		// Create engine
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		// Run should fail because tool is not allowed in intake state
		ctx := context.Background()
		_, err = engine.Run(ctx, "test tool eligibility")

		// Expect error about tool not allowed
		if err == nil {
			t.Fatal("expected error for tool not allowed in state, got nil")
		}
		if !errors.Is(err, tool.ErrToolNotAllowed) {
			t.Errorf("expected ErrToolNotAllowed, got: %v", err)
		}
	})

	t.Run("tool_allowed_in_correct_state", func(t *testing.T) {
		// Create a tool
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"content": "hello"}`)}, nil
			}).
			MustBuild()

		// Create registry
		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		// Create eligibility that allows tool in explore state
		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Create scripted planner that transitions to explore then calls tool
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "go to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "read in explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "done exploring"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("completed", json.RawMessage(`{"status": "ok"}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test tool in correct state")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed status, got: %s", run.Status)
		}
	})
}

// =============================================================================
// Invariant 2: Transition Validity
// State transitions must follow the allowed transition graph.
// =============================================================================

func TestInvariant_TransitionValidity(t *testing.T) {
	t.Run("invalid_transition_rejected", func(t *testing.T) {
		registry := api.NewToolRegistry()

		// Create transitions that don't allow intake -> act
		transitions := api.NewStateTransitions()
		transitions.Allow(agent.StateIntake, agent.StateExplore)

		// Create planner that tries invalid transition
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateAct, "skip to act"),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithTransitions(transitions),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test invalid transition")

		if err == nil {
			t.Fatal("expected error for invalid transition, got nil")
		}
	})

	t.Run("valid_transition_allowed", func(t *testing.T) {
		registry := api.NewToolRegistry()

		// Create scripted planner with valid transitions
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("done", json.RawMessage(`{}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test valid transition")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed status, got: %s", run.Status)
		}
	})
}

// =============================================================================
// Invariant 3: Approval Enforcement
// Destructive tools require approval before execution.
// =============================================================================

func TestInvariant_ApprovalEnforcement(t *testing.T) {
	t.Run("destructive_tool_requires_approval", func(t *testing.T) {
		// Create destructive tool
		deleteTool := api.NewToolBuilder("delete_file").
			WithDescription("Deletes a file").
			WithAnnotations(api.Annotations{
				Destructive: true,
				RiskLevel:   api.RiskHigh,
			}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(deleteTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateAct, "delete_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewTransitionDecision(agent.StateAct, "to act"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewCallToolDecision("delete_file", json.RawMessage(`{"path": "/tmp/test"}`), "delete file"),
			},
		)

		// Create engine WITHOUT an approver - should fail
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test approval required")

		if err == nil {
			t.Fatal("expected error for approval required, got nil")
		}
		if !errors.Is(err, tool.ErrApprovalRequired) {
			t.Errorf("expected ErrApprovalRequired, got: %v", err)
		}
	})

	t.Run("destructive_tool_executes_with_approval", func(t *testing.T) {
		deleteTool := api.NewToolBuilder("delete_file").
			WithDescription("Deletes a file").
			WithAnnotations(api.Annotations{
				Destructive: true,
				RiskLevel:   api.RiskHigh,
			}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(deleteTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateAct, "delete_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewTransitionDecision(agent.StateAct, "to act"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewCallToolDecision("delete_file", json.RawMessage(`{}`), "delete"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewTransitionDecision(agent.StateValidate, "to validate"),
			},
			api.ScriptStep{
				ExpectState: agent.StateValidate,
				Decision:    api.NewFinishDecision("completed", json.RawMessage(`{}`)),
			},
		)

		// Create engine WITH an auto-approver
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithApprover(api.NewAutoApprover("test-approver")),
			api.WithMaxSteps(20),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test with approval")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed, got: %s", run.Status)
		}
	})

	t.Run("approval_denied_blocks_execution", func(t *testing.T) {
		deleteTool := api.NewToolBuilder("delete_file").
			WithDescription("Deletes a file").
			WithAnnotations(api.Annotations{
				Destructive: true,
				RiskLevel:   api.RiskHigh,
			}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(deleteTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateAct, "delete_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewTransitionDecision(agent.StateAct, "to act"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewCallToolDecision("delete_file", json.RawMessage(`{}`), "delete"),
			},
		)

		// Create engine with deny approver
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithApprover(api.NewDenyApprover("not allowed")),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test denied approval")

		if err == nil {
			t.Fatal("expected error for denied approval, got nil")
		}
		if !errors.Is(err, tool.ErrApprovalDenied) {
			t.Errorf("expected ErrApprovalDenied, got: %v", err)
		}
	})
}

// =============================================================================
// Invariant 4: Budget Enforcement
// Operations must respect budget limits.
// =============================================================================

func TestInvariant_BudgetEnforcement(t *testing.T) {
	t.Run("budget_exceeded_blocks_execution", func(t *testing.T) {
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Planner tries to call tool twice, but budget allows only 1
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first call"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second call"),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithBudgets(map[string]int{"tool_calls": 1}),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test budget exceeded")

		if err == nil {
			t.Fatal("expected error for budget exceeded, got nil")
		}
		if !errors.Is(err, policy.ErrBudgetExceeded) {
			t.Errorf("expected ErrBudgetExceeded, got: %v", err)
		}
	})

	t.Run("budget_allows_within_limit", func(t *testing.T) {
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("done", json.RawMessage(`{}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithBudgets(map[string]int{"tool_calls": 5}),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test within budget")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed, got: %s", run.Status)
		}
	})
}

// =============================================================================
// Invariant 5: State Semantics
// Only Act state allows side effects, terminal states are final.
// =============================================================================

func TestInvariant_StateSemantics(t *testing.T) {
	t.Run("terminal_states_end_execution", func(t *testing.T) {
		if !agent.StateDone.IsTerminal() {
			t.Error("done state should be terminal")
		}
		if !agent.StateFailed.IsTerminal() {
			t.Error("failed state should be terminal")
		}
		if agent.StateExplore.IsTerminal() {
			t.Error("explore state should not be terminal")
		}
	})

	t.Run("side_effects_semantics", func(t *testing.T) {
		if !agent.StateAct.AllowsSideEffects() {
			t.Error("act state should allow side effects")
		}
		if agent.StateExplore.AllowsSideEffects() {
			t.Error("explore state should not allow side effects")
		}
		if agent.StateDecide.AllowsSideEffects() {
			t.Error("decide state should not allow side effects")
		}
	})
}

// =============================================================================
// Invariant 6: Tool Registration Uniqueness
// Tool names must be unique within a registry.
// =============================================================================

func TestInvariant_ToolRegistration(t *testing.T) {
	t.Run("duplicate_tool_rejected", func(t *testing.T) {
		registry := api.NewToolRegistry()

		tool1 := api.NewToolBuilder("my_tool").
			WithDescription("First tool").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{}, nil
			}).
			MustBuild()

		tool2 := api.NewToolBuilder("my_tool").
			WithDescription("Second tool with same name").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{}, nil
			}).
			MustBuild()

		err := registry.Register(tool1)
		if err != nil {
			t.Fatalf("first registration should succeed: %v", err)
		}

		err = registry.Register(tool2)
		if err == nil {
			t.Fatal("second registration should fail with duplicate name")
		}
		if !errors.Is(err, tool.ErrToolExists) {
			t.Errorf("expected ErrToolAlreadyExists, got: %v", err)
		}
	})

	t.Run("tool_not_found_error", func(t *testing.T) {
		registry := api.NewToolRegistry()

		_, ok := registry.Get("nonexistent")
		if ok {
			t.Error("nonexistent tool should not be found")
		}
	})
}

// =============================================================================
// Invariant 7: Run Lifecycle
// A run progresses through states and reaches terminal state.
// =============================================================================

func TestInvariant_RunLifecycle(t *testing.T) {
	t.Run("run_starts_in_intake", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")
		if run.CurrentState != agent.StateIntake {
			t.Errorf("new run should start in intake, got: %s", run.CurrentState)
		}
		if run.Status != agent.RunStatusPending {
			t.Errorf("new run should be pending, got: %s", run.Status)
		}
	})

	t.Run("run_completion_sets_result", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")
		run.Start()
		result := json.RawMessage(`{"answer": 42}`)
		run.Complete(result)

		if run.Status != agent.RunStatusCompleted {
			t.Errorf("completed run should have completed status, got: %s", run.Status)
		}
		if string(run.Result) != `{"answer": 42}` {
			t.Errorf("result mismatch: got %s", run.Result)
		}
	})

	t.Run("run_failure_sets_error", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")
		run.Start()
		run.Fail("something went wrong")

		if run.Status != agent.RunStatusFailed {
			t.Errorf("failed run should have failed status, got: %s", run.Status)
		}
		if run.Error != "something went wrong" {
			t.Errorf("error mismatch: got %s", run.Error)
		}
	})
}

// =============================================================================
// Invariant 8: Evidence Accumulation
// Evidence is append-only and preserves order.
// =============================================================================

func TestInvariant_EvidenceAccumulation(t *testing.T) {
	t.Run("evidence_is_append_only", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")

		e1 := agent.NewToolEvidence("tool1", json.RawMessage(`{"result": 1}`))
		e2 := agent.NewToolEvidence("tool2", json.RawMessage(`{"result": 2}`))
		e3 := agent.NewToolEvidence("tool3", json.RawMessage(`{"result": 3}`))

		run.AddEvidence(e1)
		run.AddEvidence(e2)
		run.AddEvidence(e3)

		if len(run.Evidence) != 3 {
			t.Errorf("expected 3 evidence items, got: %d", len(run.Evidence))
		}

		// Verify order is preserved
		if run.Evidence[0].Source != "tool1" {
			t.Errorf("first evidence should be tool1, got: %s", run.Evidence[0].Source)
		}
		if run.Evidence[1].Source != "tool2" {
			t.Errorf("second evidence should be tool2, got: %s", run.Evidence[1].Source)
		}
		if run.Evidence[2].Source != "tool3" {
			t.Errorf("third evidence should be tool3, got: %s", run.Evidence[2].Source)
		}
	})

	t.Run("evidence_timestamps_are_sequential", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")

		run.AddEvidence(agent.NewToolEvidence("tool1", json.RawMessage(`{}`)))
		run.AddEvidence(agent.NewToolEvidence("tool2", json.RawMessage(`{}`)))

		// Second evidence should have timestamp >= first
		if run.Evidence[1].Timestamp.Before(run.Evidence[0].Timestamp) {
			t.Error("evidence timestamps should be sequential")
		}
	})
}

