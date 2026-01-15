// Package policy provides policy constraint types.
package policy

import (
	"context"
	"time"
)

// PolicyVersion represents an immutable snapshot of policy configuration.
type PolicyVersion struct {
	// Version is the version number (monotonically increasing).
	Version int `json:"version"`

	// CreatedAt is when this version was created.
	CreatedAt time.Time `json:"created_at"`

	// ProposalID links to the proposal that created this version (if any).
	ProposalID string `json:"proposal_id,omitempty"`

	// Description explains what changed in this version.
	Description string `json:"description,omitempty"`

	// Eligibility contains the tool eligibility snapshot.
	Eligibility EligibilitySnapshot `json:"eligibility"`

	// Transitions contains the state transitions snapshot.
	Transitions TransitionSnapshot `json:"transitions"`

	// Budgets contains the budget limits snapshot.
	Budgets BudgetLimitsSnapshot `json:"budgets"`

	// Approvals contains the approval requirements snapshot.
	Approvals ApprovalSnapshot `json:"approvals"`
}

// VersionStore persists policy versions.
type VersionStore interface {
	// Save persists a new policy version.
	Save(ctx context.Context, version *PolicyVersion) error

	// GetCurrent retrieves the current (latest) policy version.
	GetCurrent(ctx context.Context) (*PolicyVersion, error)

	// Get retrieves a specific policy version.
	Get(ctx context.Context, version int) (*PolicyVersion, error)

	// List returns all policy versions.
	List(ctx context.Context) ([]*PolicyVersion, error)

	// GetByProposal retrieves the policy version created by a proposal.
	GetByProposal(ctx context.Context, proposalID string) (*PolicyVersion, error)
}

// VersionDiff represents the differences between two policy versions.
type VersionDiff struct {
	// FromVersion is the starting version.
	FromVersion int `json:"from_version"`

	// ToVersion is the ending version.
	ToVersion int `json:"to_version"`

	// EligibilityChanges lists eligibility differences.
	EligibilityChanges []EligibilityDiff `json:"eligibility_changes,omitempty"`

	// TransitionChanges lists transition differences.
	TransitionChanges []TransitionDiff `json:"transition_changes,omitempty"`

	// BudgetChanges lists budget differences.
	BudgetChanges []BudgetDiff `json:"budget_changes,omitempty"`

	// ApprovalChanges lists approval requirement differences.
	ApprovalChanges []ApprovalDiff `json:"approval_changes,omitempty"`
}

// EligibilityDiff represents an eligibility change between versions.
type EligibilityDiff struct {
	State    string `json:"state"`
	ToolName string `json:"tool_name"`
	Before   bool   `json:"before"`
	After    bool   `json:"after"`
}

// TransitionDiff represents a transition change between versions.
type TransitionDiff struct {
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
	Before    bool   `json:"before"`
	After     bool   `json:"after"`
}

// BudgetDiff represents a budget change between versions.
type BudgetDiff struct {
	BudgetName string `json:"budget_name"`
	Before     int    `json:"before"`
	After      int    `json:"after"`
}

// ApprovalDiff represents an approval requirement change between versions.
type ApprovalDiff struct {
	ToolName string `json:"tool_name"`
	Before   bool   `json:"before"`
	After    bool   `json:"after"`
}
