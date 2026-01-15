// Package api provides the public API for the agent runtime.
package api

import (
	"github.com/felixgeelhaar/agent-go/domain/ledger"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	inframw "github.com/felixgeelhaar/agent-go/infrastructure/middleware"
)

// Re-export middleware types for convenience.
type (
	// Middleware wraps a Handler with additional behavior.
	Middleware = middleware.Middleware

	// Handler executes a tool and returns its result.
	Handler = middleware.Handler

	// ExecutionContext contains all information needed for middleware decisions.
	ExecutionContext = middleware.ExecutionContext

	// BudgetView provides read-only access to budget state.
	BudgetView = middleware.BudgetView

	// MiddlewareRegistry manages an ordered list of middleware.
	MiddlewareRegistry = middleware.Registry

	// Cache provides in-memory caching for tool results.
	MiddlewareCache = inframw.Cache
)

// NewMiddlewareRegistry creates a new middleware registry.
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return middleware.NewRegistry()
}

// NewMiddlewareCache creates a new cache with the specified maximum entries.
func NewMiddlewareCache(maxEntries int) *MiddlewareCache {
	return inframw.NewCache(maxEntries)
}

// ChainMiddleware composes multiple middleware into a single middleware.
// Middleware are executed in the order provided, with each wrapping the next.
func ChainMiddleware(middlewares ...Middleware) Middleware {
	return middleware.Chain(middlewares...)
}

// NoopMiddleware returns a middleware that does nothing, just passes through.
func NoopMiddleware() Middleware {
	return middleware.Noop()
}

// EligibilityMiddleware returns middleware that enforces tool eligibility per state.
// It checks if the tool is allowed in the current state before execution.
func EligibilityMiddleware(eligibility *policy.ToolEligibility) Middleware {
	return inframw.Eligibility(inframw.EligibilityConfig{
		Eligibility: eligibility,
	})
}

// ApprovalMiddleware returns middleware that enforces human approval for risky tools.
// Tools marked with ShouldRequireApproval() must be approved before execution.
func ApprovalMiddleware(approver policy.Approver) Middleware {
	return inframw.Approval(inframw.ApprovalConfig{
		Approver: approver,
	})
}

// BudgetMiddleware returns middleware that enforces budget limits.
// It checks budget availability before execution and consumes on success.
func BudgetMiddleware(budget *policy.Budget, budgetName string, amount int) Middleware {
	return inframw.Budget(inframw.BudgetConfig{
		Budget:     budget,
		BudgetName: budgetName,
		Amount:     amount,
	})
}

// BudgetFromContextMiddleware returns middleware that uses the budget from ExecutionContext.
// This is useful when budget needs to be determined at runtime.
func BudgetFromContextMiddleware(budgetName string, amount int) Middleware {
	return inframw.BudgetFromContext(budgetName, amount)
}

// LoggingMiddlewareConfig configures the logging middleware.
type LoggingMiddlewareConfig struct {
	// LogInput logs the tool input (may contain sensitive data).
	LogInput bool
	// LogOutput logs the tool output (may be large).
	LogOutput bool
}

// LoggingMiddleware returns middleware that logs tool execution.
// Pass nil config for default settings (no input/output logging).
func LoggingMiddleware(cfg *LoggingMiddlewareConfig) Middleware {
	if cfg == nil {
		cfg = &LoggingMiddlewareConfig{}
	}
	return inframw.Logging(inframw.LoggingConfig{
		LogInput:  cfg.LogInput,
		LogOutput: cfg.LogOutput,
	})
}

// CachingMiddleware returns middleware that caches cacheable tool results.
// Only tools marked as cacheable (via annotations) will be cached.
func CachingMiddleware(cache *MiddlewareCache) Middleware {
	return inframw.Caching(cache)
}

// LedgerRecordingMiddleware returns middleware that records tool calls to the ledger.
// This provides an audit trail of all tool executions.
func LedgerRecordingMiddleware(l *ledger.Ledger) Middleware {
	return inframw.LedgerRecording(inframw.LedgerConfig{
		Ledger: l,
	})
}

// Re-export types for convenience.
type (
	// ToolResult is returned by tool execution.
	ToolResult = tool.Result
)
