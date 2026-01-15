package pattern

import "errors"

var (
	// ErrPatternNotFound indicates the pattern was not found.
	ErrPatternNotFound = errors.New("pattern not found")

	// ErrPatternExists indicates a pattern with this ID already exists.
	ErrPatternExists = errors.New("pattern already exists")

	// ErrInvalidPattern indicates the pattern is invalid.
	ErrInvalidPattern = errors.New("invalid pattern")

	// ErrInvalidPatternType indicates an unknown pattern type.
	ErrInvalidPatternType = errors.New("invalid pattern type")

	// ErrInsufficientData indicates not enough data for detection.
	ErrInsufficientData = errors.New("insufficient data for pattern detection")

	// ErrDetectionFailed indicates pattern detection failed.
	ErrDetectionFailed = errors.New("pattern detection failed")
)
