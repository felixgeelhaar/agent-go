package pattern

import (
	"context"
	"time"
)

// Store persists detected patterns.
type Store interface {
	// Save persists a pattern.
	Save(ctx context.Context, pattern *Pattern) error

	// Get retrieves a pattern by ID.
	Get(ctx context.Context, id string) (*Pattern, error)

	// List returns patterns matching the filter.
	List(ctx context.Context, filter ListFilter) ([]*Pattern, error)

	// Delete removes a pattern.
	Delete(ctx context.Context, id string) error

	// Update updates an existing pattern.
	Update(ctx context.Context, pattern *Pattern) error
}

// ListFilter filters pattern queries.
type ListFilter struct {
	// Types filters to specific pattern types.
	Types []PatternType

	// MinConfidence filters by minimum confidence.
	MinConfidence float64

	// MinFrequency filters by minimum frequency.
	MinFrequency int

	// FromTime filters patterns first seen after this time.
	FromTime time.Time

	// ToTime filters patterns last seen before this time.
	ToTime time.Time

	// RunID filters patterns that include this run.
	RunID string

	// Limit is the maximum number of results.
	Limit int

	// Offset is the number of results to skip.
	Offset int

	// OrderBy specifies the ordering.
	OrderBy OrderBy

	// Descending reverses the order.
	Descending bool
}

// OrderBy specifies how to order pattern results.
type OrderBy string

const (
	// OrderByFirstSeen orders by first detection time.
	OrderByFirstSeen OrderBy = "first_seen"

	// OrderByLastSeen orders by last detection time.
	OrderByLastSeen OrderBy = "last_seen"

	// OrderByFrequency orders by occurrence count.
	OrderByFrequency OrderBy = "frequency"

	// OrderByConfidence orders by confidence score.
	OrderByConfidence OrderBy = "confidence"
)

// Counter provides pattern counting.
type Counter interface {
	// Count returns the number of patterns matching the filter.
	Count(ctx context.Context, filter ListFilter) (int64, error)
}

// Summary provides pattern summary statistics.
type Summary struct {
	// TotalPatterns is the total number of patterns.
	TotalPatterns int64

	// ByType counts patterns by type.
	ByType map[PatternType]int64

	// AverageConfidence is the average confidence across patterns.
	AverageConfidence float64

	// AverageFrequency is the average frequency across patterns.
	AverageFrequency float64
}

// Summarizer provides pattern summaries.
type Summarizer interface {
	// Summarize returns summary statistics.
	Summarize(ctx context.Context, filter ListFilter) (*Summary, error)
}
