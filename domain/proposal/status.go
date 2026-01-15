// Package proposal provides proposal types for policy evolution.
package proposal

// ProposalStatus represents the lifecycle state of a proposal.
type ProposalStatus string

const (
	// ProposalStatusDraft is the initial state for new proposals.
	ProposalStatusDraft ProposalStatus = "draft"

	// ProposalStatusPendingReview indicates the proposal is awaiting human review.
	ProposalStatusPendingReview ProposalStatus = "pending_review"

	// ProposalStatusApproved indicates the proposal has been approved for application.
	ProposalStatusApproved ProposalStatus = "approved"

	// ProposalStatusRejected indicates the proposal was rejected.
	ProposalStatusRejected ProposalStatus = "rejected"

	// ProposalStatusApplied indicates the proposal changes have been applied.
	ProposalStatusApplied ProposalStatus = "applied"

	// ProposalStatusRolledBack indicates the applied changes have been rolled back.
	ProposalStatusRolledBack ProposalStatus = "rolled_back"
)

// StatusTransitions defines valid status transitions.
var StatusTransitions = map[ProposalStatus][]ProposalStatus{
	ProposalStatusDraft:         {ProposalStatusPendingReview, ProposalStatusRejected},
	ProposalStatusPendingReview: {ProposalStatusApproved, ProposalStatusRejected, ProposalStatusDraft},
	ProposalStatusApproved:      {ProposalStatusApplied, ProposalStatusRejected},
	ProposalStatusApplied:       {ProposalStatusRolledBack},
	ProposalStatusRejected:      {ProposalStatusDraft},
	ProposalStatusRolledBack:    {ProposalStatusDraft},
}

// CanTransitionTo returns true if the transition from current status to target is valid.
func (s ProposalStatus) CanTransitionTo(target ProposalStatus) bool {
	validTargets, ok := StatusTransitions[s]
	if !ok {
		return false
	}
	for _, valid := range validTargets {
		if valid == target {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the status is a terminal state.
func (s ProposalStatus) IsTerminal() bool {
	return s == ProposalStatusRejected || s == ProposalStatusRolledBack
}

// IsActive returns true if the status represents an active proposal.
func (s ProposalStatus) IsActive() bool {
	return s == ProposalStatusDraft || s == ProposalStatusPendingReview || s == ProposalStatusApproved
}

// RequiresHumanAction returns true if the status requires human intervention.
func (s ProposalStatus) RequiresHumanAction() bool {
	return s == ProposalStatusPendingReview
}
