package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ApprovalConfig configures the approval middleware.
type ApprovalConfig struct {
	// Approver handles approval requests.
	Approver policy.Approver
}

// Approval returns middleware that enforces approval for high-risk tools.
// Tools that require approval (destructive, high-risk, or explicitly marked)
// must be approved before execution.
func Approval(cfg ApprovalConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			t := execCtx.Tool
			annotations := t.Annotations()

			// Check if approval is required
			if !annotations.ShouldRequireApproval() {
				return next(ctx, execCtx)
			}

			// No approver configured - fail if approval required
			if cfg.Approver == nil {
				return tool.Result{}, fmt.Errorf("%w: no approver configured for tool %s",
					tool.ErrApprovalRequired, t.Name())
			}

			// Build approval request
			req := policy.ApprovalRequest{
				RunID:     execCtx.RunID,
				ToolName:  t.Name(),
				Input:     execCtx.Input,
				Reason:    execCtx.Reason,
				RiskLevel: annotations.RiskLevel.String(),
				Timestamp: time.Now(),
			}

			// Request approval
			resp, err := cfg.Approver.Approve(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("approval error: %w", err)
			}

			if !resp.Approved {
				reason := "approval denied"
				if resp.Reason != "" {
					reason = resp.Reason
				}
				return tool.Result{}, fmt.Errorf("%w: %s", tool.ErrApprovalDenied, reason)
			}

			return next(ctx, execCtx)
		}
	}
}
