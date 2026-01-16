// Package api provides the public API for the agent-go runtime.
//
// agent-go is a state-driven agent framework for Go that enables developers to build
// trustworthy, adaptable AI-powered systems by designing the structure and constraints
// of agent behavior rather than scripting intelligence with prompts.
//
// # Quick Start
//
// Create a minimal agent with a tool and scripted planner:
//
//	// 1. Create a tool
//	echoTool := api.NewToolBuilder("echo").
//	    WithDescription("Echoes input").
//	    WithAnnotations(api.Annotations{ReadOnly: true}).
//	    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
//	        return tool.Result{Output: input}, nil
//	    }).
//	    MustBuild()
//
//	// 2. Create a planner
//	planner := api.NewScriptedPlanner(
//	    api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "start")},
//	    api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewCallToolDecision("echo", input, "echo")},
//	    api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "done")},
//	    api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("completed", result)},
//	)
//
//	// 3. Configure tool eligibility
//	eligibility := api.NewToolEligibility()
//	eligibility.Allow(api.StateExplore, "echo")
//
//	// 4. Create and run the engine
//	engine, _ := api.New(
//	    api.WithTool(echoTool),
//	    api.WithPlanner(planner),
//	    api.WithToolEligibility(eligibility),
//	)
//	run, _ := engine.Run(ctx, "Echo a message")
//
// # States
//
// The agent operates within a canonical state graph:
//
//   - StateIntake: Normalize and understand the goal
//   - StateExplore: Gather information (read-only tools only)
//   - StateDecide: Choose next action
//   - StateAct: Execute side-effects (destructive tools allowed)
//   - StateValidate: Verify outcomes
//   - StateDone: Terminal success state
//   - StateFailed: Terminal failure state
//
// # Tools
//
// Tools are capabilities the agent can invoke. Each tool has annotations that
// describe its behavior:
//
//   - ReadOnly: Tool does not modify external state
//   - Destructive: Tool performs irreversible operations
//   - Idempotent: Repeated calls produce same result
//   - Cacheable: Results can be cached
//   - RiskLevel: Potential impact (None, Low, Medium, High, Critical)
//
// # Planners
//
// Planners make decisions about what the agent should do next:
//
//   - ScriptedPlanner: Predefined sequence for testing
//   - MockPlanner: Returns specific decisions for testing
//   - LLMPlanner: Uses an LLM provider for intelligent planning
//
// # Policies
//
// Policies enforce constraints on agent behavior:
//
//   - ToolEligibility: Which tools can run in which states
//   - StateTransitions: Which state transitions are allowed
//   - Approvers: Human approval for destructive operations
//   - Budgets: Limits on tool calls, tokens, etc.
package api

import (
	"context"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/artifact"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
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

// Re-export run status.
type RunStatus = agent.RunStatus

const (
	StatusPending   = agent.RunStatusPending
	StatusRunning   = agent.RunStatusRunning
	StatusPaused    = agent.RunStatusPaused
	StatusCompleted = agent.RunStatusCompleted
	StatusFailed    = agent.RunStatusFailed
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
		Middleware:   config.middleware,
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
	middleware  *middleware.Registry
}

// Option configures the Engine.
type Option func(*engineConfig)

// WithRegistry sets the tool registry.
func WithRegistry(r tool.Registry) Option {
	return func(c *engineConfig) {
		c.registry = r
	}
}

// WithTool registers a tool with the engine's registry.
// Can be called multiple times to register multiple tools.
func WithTool(t tool.Tool) Option {
	return func(c *engineConfig) {
		c.registry.Register(t)
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

// WithBudget sets a single budget limit.
// This is a convenience function that can be called multiple times.
func WithBudget(name string, limit int) Option {
	return func(c *engineConfig) {
		if c.budgets == nil {
			c.budgets = make(map[string]int)
		}
		c.budgets[name] = limit
	}
}

// WithMaxSteps sets the maximum number of execution steps.
func WithMaxSteps(n int) Option {
	return func(c *engineConfig) {
		c.maxSteps = n
	}
}

// WithMiddleware sets a custom middleware registry for tool execution.
// If not set, the engine uses a default middleware chain with:
// - Eligibility middleware (tool access control per state)
// - Approval middleware (human approval for destructive tools)
// - Logging middleware (execution timing and results)
func WithMiddleware(middlewares ...middleware.Middleware) Option {
	return func(c *engineConfig) {
		if c.middleware == nil {
			c.middleware = middleware.NewRegistry()
		}
		c.middleware.UseMany(middlewares...)
	}
}
