package policy

import "errors"

// Domain errors for policy enforcement.
var (
	// ErrBudgetExceeded indicates the budget limit has been exceeded.
	ErrBudgetExceeded = errors.New("budget exceeded")

	// ErrApprovalRequired indicates approval is required but not obtained.
	ErrApprovalRequired = errors.New("approval required")

	// ErrApprovalDenied indicates the approval request was denied.
	ErrApprovalDenied = errors.New("approval denied")

	// ErrApprovalTimeout indicates the approval request timed out.
	ErrApprovalTimeout = errors.New("approval request timed out")

	// ErrConstraintViolation indicates a policy constraint was violated.
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrTransitionNotAllowed indicates the state transition is not permitted.
	ErrTransitionNotAllowed = errors.New("state transition not allowed")

	// ErrToolNotEligible indicates the tool is not eligible in the current state.
	ErrToolNotEligible = errors.New("tool not eligible in current state")
)
