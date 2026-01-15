// Package proposal provides proposal types for policy evolution.
package proposal

import (
	"context"
	"time"
)

// Store persists proposals.
type Store interface {
	// Save persists a new proposal.
	Save(ctx context.Context, proposal *Proposal) error

	// Get retrieves a proposal by ID.
	Get(ctx context.Context, id string) (*Proposal, error)

	// List returns proposals matching the filter.
	List(ctx context.Context, filter ListFilter) ([]*Proposal, error)

	// Delete removes a proposal.
	Delete(ctx context.Context, id string) error

	// Update updates an existing proposal.
	Update(ctx context.Context, proposal *Proposal) error
}

// ListFilter filters proposal queries.
type ListFilter struct {
	// Status filters by proposal status.
	Status []ProposalStatus

	// CreatedBy filters by creator.
	CreatedBy string

	// SuggestionID filters by linked suggestion.
	SuggestionID string

	// FromTime filters proposals created after this time.
	FromTime time.Time

	// ToTime filters proposals created before this time.
	ToTime time.Time

	// Limit is the maximum number of results.
	Limit int

	// Offset is the number of results to skip.
	Offset int

	// OrderBy specifies the ordering.
	OrderBy OrderBy

	// Descending reverses the order.
	Descending bool
}

// OrderBy specifies how to order proposal results.
type OrderBy string

const (
	// OrderByCreatedAt orders by creation time.
	OrderByCreatedAt OrderBy = "created_at"

	// OrderBySubmittedAt orders by submission time.
	OrderBySubmittedAt OrderBy = "submitted_at"

	// OrderByApprovedAt orders by approval time.
	OrderByApprovedAt OrderBy = "approved_at"

	// OrderByStatus orders by status.
	OrderByStatus OrderBy = "status"
)

// Counter provides proposal counting.
type Counter interface {
	// Count returns the number of proposals matching the filter.
	Count(ctx context.Context, filter ListFilter) (int64, error)
}

// Summary provides proposal summary statistics.
type Summary struct {
	// TotalProposals is the total number of proposals.
	TotalProposals int64

	// ByStatus counts proposals by status.
	ByStatus map[ProposalStatus]int64

	// PendingReview is the count of proposals awaiting review.
	PendingReview int64

	// AppliedCount is the count of applied proposals.
	AppliedCount int64

	// RolledBackCount is the count of rolled back proposals.
	RolledBackCount int64
}

// Summarizer provides proposal summaries.
type Summarizer interface {
	// Summarize returns summary statistics.
	Summarize(ctx context.Context, filter ListFilter) (*Summary, error)
}
