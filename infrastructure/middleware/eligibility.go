// Package middleware provides pre-built middleware implementations.
package middleware

import (
	"context"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// EligibilityConfig configures the eligibility middleware.
type EligibilityConfig struct {
	// Eligibility defines which tools are allowed in which states.
	Eligibility *policy.ToolEligibility
}

// Eligibility returns middleware that enforces tool eligibility per state.
// If a tool is not allowed in the current state, execution is blocked.
func Eligibility(cfg EligibilityConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if no eligibility configured
			if cfg.Eligibility == nil {
				return next(ctx, execCtx)
			}

			// Check if tool is allowed in current state
			if !cfg.Eligibility.IsAllowed(execCtx.CurrentState, execCtx.Tool.Name()) {
				return tool.Result{}, fmt.Errorf("%w: %s in state %s",
					tool.ErrToolNotAllowed, execCtx.Tool.Name(), execCtx.CurrentState)
			}

			return next(ctx, execCtx)
		}
	}
}
