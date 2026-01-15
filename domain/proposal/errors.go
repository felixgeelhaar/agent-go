// Package proposal provides proposal types for policy evolution.
package proposal

import "errors"

var (
	// ErrProposalNotFound indicates the proposal was not found.
	ErrProposalNotFound = errors.New("proposal not found")

	// ErrProposalExists indicates a proposal with this ID already exists.
	ErrProposalExists = errors.New("proposal already exists")

	// ErrInvalidProposal indicates the proposal is invalid.
	ErrInvalidProposal = errors.New("invalid proposal")

	// ErrInvalidStatusTransition indicates an invalid status transition.
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrCannotModifyNonDraft indicates the proposal cannot be modified because it's not in draft status.
	ErrCannotModifyNonDraft = errors.New("cannot modify proposal that is not in draft status")

	// ErrNoChanges indicates the proposal has no changes.
	ErrNoChanges = errors.New("proposal has no changes")

	// ErrHumanActorRequired indicates a human actor is required for this action.
	ErrHumanActorRequired = errors.New("human actor required")

	// ErrApplyFailed indicates applying the proposal changes failed.
	ErrApplyFailed = errors.New("failed to apply proposal changes")

	// ErrRollbackFailed indicates rolling back the proposal changes failed.
	ErrRollbackFailed = errors.New("failed to rollback proposal changes")
)
