package application

import (
	"github.com/felixgeelhaar/agent-go/domain/artifact"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/resilience"
)

// Option configures the engine.
type Option func(*EngineConfig)

// WithRegistry sets the tool registry.
func WithRegistry(r tool.Registry) Option {
	return func(c *EngineConfig) {
		c.Registry = r
	}
}

// WithPlanner sets the planner.
func WithPlanner(p planner.Planner) Option {
	return func(c *EngineConfig) {
		c.Planner = p
	}
}

// WithExecutor sets the resilient executor.
func WithExecutor(e *resilience.Executor) Option {
	return func(c *EngineConfig) {
		c.Executor = e
	}
}

// WithArtifactStore sets the artifact store.
func WithArtifactStore(s artifact.Store) Option {
	return func(c *EngineConfig) {
		c.Artifacts = s
	}
}

// WithEligibility sets the tool eligibility configuration.
func WithEligibility(e *policy.ToolEligibility) Option {
	return func(c *EngineConfig) {
		c.Eligibility = e
	}
}

// WithTransitions sets the state transitions configuration.
func WithTransitions(t *policy.StateTransitions) Option {
	return func(c *EngineConfig) {
		c.Transitions = t
	}
}

// WithApprover sets the approval handler.
func WithApprover(a policy.Approver) Option {
	return func(c *EngineConfig) {
		c.Approver = a
	}
}

// WithBudgets sets budget limits.
func WithBudgets(limits map[string]int) Option {
	return func(c *EngineConfig) {
		c.BudgetLimits = limits
	}
}

// WithMaxSteps sets the maximum number of steps.
func WithMaxSteps(n int) Option {
	return func(c *EngineConfig) {
		c.MaxSteps = n
	}
}

// WithMiddleware sets a custom middleware registry.
// If not set, the engine uses a default middleware chain with:
// - Eligibility middleware (tool access control per state)
// - Approval middleware (human approval for destructive tools)
// - Logging middleware (execution timing and results)
func WithMiddleware(m *middleware.Registry) Option {
	return func(c *EngineConfig) {
		c.Middleware = m
	}
}

// NewEngineWithOptions creates an engine with functional options.
func NewEngineWithOptions(opts ...Option) (*Engine, error) {
	config := EngineConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	return NewEngine(config)
}
