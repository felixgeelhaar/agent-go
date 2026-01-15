// Package suggestion provides suggestion types for policy evolution.
package suggestion

import "errors"

var (
	// ErrSuggestionNotFound indicates the suggestion was not found.
	ErrSuggestionNotFound = errors.New("suggestion not found")

	// ErrSuggestionExists indicates a suggestion with this ID already exists.
	ErrSuggestionExists = errors.New("suggestion already exists")

	// ErrInvalidSuggestion indicates the suggestion is invalid.
	ErrInvalidSuggestion = errors.New("invalid suggestion")

	// ErrInvalidSuggestionType indicates an unknown suggestion type.
	ErrInvalidSuggestionType = errors.New("invalid suggestion type")

	// ErrInvalidStatusTransition indicates an invalid status transition.
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrGenerationFailed indicates suggestion generation failed.
	ErrGenerationFailed = errors.New("suggestion generation failed")

	// ErrNoPatterns indicates no patterns were provided for generation.
	ErrNoPatterns = errors.New("no patterns provided for suggestion generation")
)
