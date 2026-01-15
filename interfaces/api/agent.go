// Package api provides the public API for the agent runtime.
package api

import (
	"context"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/artifact"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/resilience"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// Re-export core types for convenience.
type (
	// Run represents a single execution of the agent.
	Run = agent.Run

	// State represents a structural constraint in the agent's execution.
	State = agent.State

	// Decision represents the planner's output.
	Decision = agent.Decision

	// Evidence represents an observation during a run.
	Evidence = agent.Evidence

	// Tool represents a registered capability the agent can invoke.
	Tool = tool.Tool

	// Annotations describe tool behavior.
	Annotations = tool.Annotations

	// RiskLevel indicates the potential impact of a tool execution.
	RiskLevel = tool.RiskLevel
)

// Re-export state constants.
const (
	StateIntake   = agent.StateIntake
	StateExplore  = agent.StateExplore
	StateDecide   = agent.StateDecide
	StateAct      = agent.StateAct
	StateValidate = agent.StateValidate
	StateDone     = agent.StateDone
	StateFailed   = agent.StateFailed
)

// Re-export risk levels.
const (
	RiskNone     = tool.RiskNone
	RiskLow      = tool.RiskLow
	RiskMedium   = tool.RiskMedium
	RiskHigh     = tool.RiskHigh
	RiskCritical = tool.RiskCritical
)

// Engine is the main runtime for agent execution.
type Engine struct {
	engine *application.Engine
}

// New creates a new Engine with the provided options.
func New(opts ...Option) (*Engine, error) {
	config := &engineConfig{
		registry: memory.NewToolRegistry(),
	}

	for _, opt := range opts {
		opt(config)
	}

	appConfig := application.EngineConfig{
		Registry:     config.registry,
		Planner:      config.planner,
		Executor:     config.executor,
		Artifacts:    config.artifacts,
		Eligibility:  config.eligibility,
		Transitions:  config.transitions,
		Approver:     config.approver,
		BudgetLimits: config.budgets,
		MaxSteps:     config.maxSteps,
	}

	engine, err := application.NewEngine(appConfig)
	if err != nil {
		return nil, err
	}

	return &Engine{engine: engine}, nil
}

// Run executes the agent with the given goal.
func (e *Engine) Run(ctx context.Context, goal string) (*Run, error) {
	return e.engine.Run(ctx, goal)
}

// RunWithVars executes the agent with the given goal and initial variables.
func (e *Engine) RunWithVars(ctx context.Context, goal string, vars map[string]any) (*Run, error) {
	return e.engine.RunWithVars(ctx, goal, vars)
}

// engineConfig holds configuration for engine creation.
type engineConfig struct {
	registry    tool.Registry
	planner     planner.Planner
	executor    *resilience.Executor
	artifacts   artifact.Store
	eligibility *policy.ToolEligibility
	transitions *policy.StateTransitions
	approver    policy.Approver
	budgets     map[string]int
	maxSteps    int
}

// Option configures the Engine.
type Option func(*engineConfig)

// WithRegistry sets the tool registry.
func WithRegistry(r tool.Registry) Option {
	return func(c *engineConfig) {
		c.registry = r
	}
}

// WithPlanner sets the planner.
func WithPlanner(p planner.Planner) Option {
	return func(c *engineConfig) {
		c.planner = p
	}
}

// WithExecutor sets the resilient executor.
func WithExecutor(e *resilience.Executor) Option {
	return func(c *engineConfig) {
		c.executor = e
	}
}

// WithArtifactStore sets the artifact store.
func WithArtifactStore(s artifact.Store) Option {
	return func(c *engineConfig) {
		c.artifacts = s
	}
}

// WithToolEligibility sets tool eligibility per state.
func WithToolEligibility(e *policy.ToolEligibility) Option {
	return func(c *engineConfig) {
		c.eligibility = e
	}
}

// WithTransitions sets allowed state transitions.
func WithTransitions(t *policy.StateTransitions) Option {
	return func(c *engineConfig) {
		c.transitions = t
	}
}

// WithApprover sets the approval handler.
func WithApprover(a policy.Approver) Option {
	return func(c *engineConfig) {
		c.approver = a
	}
}

// WithBudgets sets budget limits.
func WithBudgets(budgets map[string]int) Option {
	return func(c *engineConfig) {
		c.budgets = budgets
	}
}

// WithMaxSteps sets the maximum number of execution steps.
func WithMaxSteps(n int) Option {
	return func(c *engineConfig) {
		c.maxSteps = n
	}
}
