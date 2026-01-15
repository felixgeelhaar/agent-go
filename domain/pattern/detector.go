package pattern

import (
	"context"
	"time"
)

// Detector detects patterns from run data.
type Detector interface {
	// Detect analyzes runs and returns detected patterns.
	Detect(ctx context.Context, opts DetectionOptions) ([]Pattern, error)

	// Types returns the pattern types this detector can find.
	Types() []PatternType
}

// DetectionOptions configures pattern detection.
type DetectionOptions struct {
	// RunIDs filters to specific runs (empty means all).
	RunIDs []string

	// FromTime filters runs after this time.
	FromTime time.Time

	// ToTime filters runs before this time.
	ToTime time.Time

	// MinConfidence is the minimum confidence threshold (0.0-1.0).
	MinConfidence float64

	// MinFrequency is the minimum occurrence count.
	MinFrequency int

	// PatternTypes filters to specific pattern types (empty means all).
	PatternTypes []PatternType

	// Limit is the maximum number of patterns to return (0 = no limit).
	Limit int
}

// DefaultDetectionOptions returns sensible defaults.
func DefaultDetectionOptions() DetectionOptions {
	return DetectionOptions{
		MinConfidence: 0.5,
		MinFrequency:  2,
		Limit:         100,
	}
}

// DetectorFunc is a function that implements Detector.
type DetectorFunc func(ctx context.Context, opts DetectionOptions) ([]Pattern, error)

// Detect implements Detector.
func (f DetectorFunc) Detect(ctx context.Context, opts DetectionOptions) ([]Pattern, error) {
	return f(ctx, opts)
}

// Types implements Detector (returns empty slice).
func (f DetectorFunc) Types() []PatternType {
	return nil
}
