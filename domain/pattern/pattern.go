// Package pattern provides pattern detection types.
package pattern

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Pattern represents a detected behavioral pattern across runs.
type Pattern struct {
	// ID is the unique identifier.
	ID string `json:"id"`

	// Type classifies the pattern.
	Type PatternType `json:"type"`

	// Name is a human-readable name for the pattern.
	Name string `json:"name"`

	// Description explains what the pattern represents.
	Description string `json:"description"`

	// Confidence indicates how certain we are (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Frequency is the number of occurrences.
	Frequency int `json:"frequency"`

	// FirstSeen is when the pattern was first detected.
	FirstSeen time.Time `json:"first_seen"`

	// LastSeen is when the pattern was last detected.
	LastSeen time.Time `json:"last_seen"`

	// RunIDs are the runs where this pattern was observed.
	RunIDs []string `json:"run_ids"`

	// Evidence captures specific instances of the pattern.
	Evidence []PatternEvidence `json:"evidence"`

	// Data contains type-specific pattern data.
	Data json.RawMessage `json:"data,omitempty"`

	// Metadata contains additional information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PatternEvidence captures a specific instance of a pattern.
type PatternEvidence struct {
	// RunID is the run where this evidence was found.
	RunID string `json:"run_id"`

	// Timestamp is when the evidence was observed.
	Timestamp time.Time `json:"timestamp"`

	// Details contains evidence-specific information.
	Details json.RawMessage `json:"details,omitempty"`
}

// NewPattern creates a new pattern with a generated ID.
func NewPattern(typ PatternType, name, description string) *Pattern {
	now := time.Now()
	return &Pattern{
		ID:          uuid.New().String(),
		Type:        typ,
		Name:        name,
		Description: description,
		FirstSeen:   now,
		LastSeen:    now,
		RunIDs:      make([]string, 0),
		Evidence:    make([]PatternEvidence, 0),
		Metadata:    make(map[string]any),
	}
}

// AddEvidence adds evidence to the pattern.
func (p *Pattern) AddEvidence(runID string, details any) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return err
	}

	evidence := PatternEvidence{
		RunID:     runID,
		Timestamp: time.Now(),
		Details:   detailsJSON,
	}

	p.Evidence = append(p.Evidence, evidence)
	p.Frequency++
	p.LastSeen = evidence.Timestamp

	// Add run ID if not already present
	for _, id := range p.RunIDs {
		if id == runID {
			return nil
		}
	}
	p.RunIDs = append(p.RunIDs, runID)

	return nil
}

// SetData sets the type-specific pattern data.
func (p *Pattern) SetData(data any) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	p.Data = dataJSON
	return nil
}

// GetData unmarshals the type-specific pattern data.
func (p *Pattern) GetData(v any) error {
	if p.Data == nil {
		return nil
	}
	return json.Unmarshal(p.Data, v)
}

// IsSignificant returns true if the pattern meets significance thresholds.
func (p *Pattern) IsSignificant(minConfidence float64, minFrequency int) bool {
	return p.Confidence >= minConfidence && p.Frequency >= minFrequency
}
