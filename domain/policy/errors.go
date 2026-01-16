package policy

import "errors"

// Domain errors for policy enforcement.
//
// Note: For approval-related errors (required, denied), use the canonical
// errors from the tool package (tool.ErrApprovalRequired, tool.ErrApprovalDenied).
// This avoids duplication and maintains clear ownership since approvals are
// fundamentally about tool execution authorization.
var (
	// ErrBudgetExceeded indicates the budget limit has been exceeded.
	ErrBudgetExceeded = errors.New("budget exceeded")

	// ErrApprovalTimeout indicates the approval request timed out.
	ErrApprovalTimeout = errors.New("approval request timed out")

	// ErrConstraintViolation indicates a policy constraint was violated.
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrTransitionNotAllowed indicates the state transition is not permitted.
	ErrTransitionNotAllowed = errors.New("state transition not allowed")

	// ErrToolNotEligible indicates the tool is not eligible in the current state.
	ErrToolNotEligible = errors.New("tool not eligible in current state")
)
