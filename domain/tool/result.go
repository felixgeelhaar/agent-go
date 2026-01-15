package tool

import (
	"encoding/json"
	"time"
)

// Result contains the output of a tool execution.
type Result struct {
	// Output is the primary result data.
	Output json.RawMessage `json:"output"`

	// Artifacts are optional large outputs produced by the tool.
	Artifacts []ArtifactRef `json:"artifacts,omitempty"`

	// Duration is how long the execution took.
	Duration time.Duration `json:"duration"`

	// Cached indicates if this result was served from cache.
	Cached bool `json:"cached,omitempty"`

	// Error is a tool-level error (distinct from execution error).
	Error error `json:"-"`
}

// ArtifactRef is a reference to a stored artifact.
// This is a lightweight reference; the full artifact is in domain/artifact.
type ArtifactRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// NewResult creates a successful result with the given output.
func NewResult(output json.RawMessage) Result {
	return Result{Output: output}
}

// NewResultWithDuration creates a result with timing information.
func NewResultWithDuration(output json.RawMessage, duration time.Duration) Result {
	return Result{
		Output:   output,
		Duration: duration,
	}
}

// NewErrorResult creates a result representing an error.
func NewErrorResult(err error) Result {
	return Result{Error: err}
}

// NewCachedResult creates a result marked as cached.
func NewCachedResult(output json.RawMessage) Result {
	return Result{
		Output: output,
		Cached: true,
	}
}

// IsError returns true if the result represents an error.
func (r Result) IsError() bool {
	return r.Error != nil
}

// HasArtifacts returns true if the result includes artifacts.
func (r Result) HasArtifacts() bool {
	return len(r.Artifacts) > 0
}

// WithArtifact adds an artifact reference to the result.
func (r Result) WithArtifact(ref ArtifactRef) Result {
	r.Artifacts = append(r.Artifacts, ref)
	return r
}

// OutputString returns the output as a string for convenience.
func (r Result) OutputString() string {
	return string(r.Output)
}
