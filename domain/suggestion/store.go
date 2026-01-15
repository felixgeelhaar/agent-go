// Package suggestion provides suggestion types for policy evolution.
package suggestion

import (
	"context"
	"time"
)

// Store persists suggestions.
type Store interface {
	// Save persists a new suggestion.
	Save(ctx context.Context, suggestion *Suggestion) error

	// Get retrieves a suggestion by ID.
	Get(ctx context.Context, id string) (*Suggestion, error)

	// List returns suggestions matching the filter.
	List(ctx context.Context, filter ListFilter) ([]*Suggestion, error)

	// Delete removes a suggestion.
	Delete(ctx context.Context, id string) error

	// Update updates an existing suggestion.
	Update(ctx context.Context, suggestion *Suggestion) error
}

// ListFilter filters suggestion queries.
type ListFilter struct {
	// Types filters to specific suggestion types.
	Types []SuggestionType

	// Status filters by suggestion status.
	Status []SuggestionStatus

	// MinConfidence filters by minimum confidence.
	MinConfidence float64

	// Impact filters by impact level.
	Impact []ImpactLevel

	// PatternID filters suggestions linked to a pattern.
	PatternID string

	// FromTime filters suggestions created after this time.
	FromTime time.Time

	// ToTime filters suggestions created before this time.
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

// OrderBy specifies how to order suggestion results.
type OrderBy string

const (
	// OrderByCreatedAt orders by creation time.
	OrderByCreatedAt OrderBy = "created_at"

	// OrderByConfidence orders by confidence score.
	OrderByConfidence OrderBy = "confidence"

	// OrderByImpact orders by impact level.
	OrderByImpact OrderBy = "impact"

	// OrderByStatus orders by status.
	OrderByStatus OrderBy = "status"
)

// Counter provides suggestion counting.
type Counter interface {
	// Count returns the number of suggestions matching the filter.
	Count(ctx context.Context, filter ListFilter) (int64, error)
}

// Summary provides suggestion summary statistics.
type Summary struct {
	// TotalSuggestions is the total number of suggestions.
	TotalSuggestions int64

	// ByType counts suggestions by type.
	ByType map[SuggestionType]int64

	// ByStatus counts suggestions by status.
	ByStatus map[SuggestionStatus]int64

	// ByImpact counts suggestions by impact level.
	ByImpact map[ImpactLevel]int64

	// AverageConfidence is the average confidence across suggestions.
	AverageConfidence float64
}

// Summarizer provides suggestion summaries.
type Summarizer interface {
	// Summarize returns summary statistics.
	Summarize(ctx context.Context, filter ListFilter) (*Summary, error)
}
