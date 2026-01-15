// Package suggestion provides suggestion types for policy evolution.
package suggestion

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Suggestion represents a policy improvement suggestion generated from patterns.
type Suggestion struct {
	// ID is the unique identifier.
	ID string `json:"id"`

	// Type classifies the suggestion.
	Type SuggestionType `json:"type"`

	// Title is a human-readable summary.
	Title string `json:"title"`

	// Description explains the suggestion in detail.
	Description string `json:"description"`

	// Rationale explains why this suggestion was made.
	Rationale string `json:"rationale"`

	// Confidence indicates how certain we are (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Impact indicates the potential impact level.
	Impact ImpactLevel `json:"impact"`

	// PatternIDs are the patterns that led to this suggestion.
	PatternIDs []string `json:"pattern_ids"`

	// Change is the proposed policy change.
	Change PolicyChange `json:"change"`

	// ChangeData contains type-specific change details.
	ChangeData json.RawMessage `json:"change_data,omitempty"`

	// CreatedAt is when the suggestion was generated.
	CreatedAt time.Time `json:"created_at"`

	// Status is the current suggestion status.
	Status SuggestionStatus `json:"status"`

	// StatusChangedAt is when the status last changed.
	StatusChangedAt time.Time `json:"status_changed_at,omitempty"`

	// StatusChangedBy is who changed the status.
	StatusChangedBy string `json:"status_changed_by,omitempty"`

	// ProposalID is set when converted to a proposal.
	ProposalID string `json:"proposal_id,omitempty"`

	// Metadata contains additional information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewSuggestion creates a new suggestion with a generated ID.
func NewSuggestion(typ SuggestionType, title, description string) *Suggestion {
	now := time.Now()
	return &Suggestion{
		ID:          uuid.New().String(),
		Type:        typ,
		Title:       title,
		Description: description,
		CreatedAt:   now,
		Status:      SuggestionStatusPending,
		PatternIDs:  make([]string, 0),
		Metadata:    make(map[string]any),
	}
}

// SetChangeData sets the type-specific change data.
func (s *Suggestion) SetChangeData(data any) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.ChangeData = dataJSON
	return nil
}

// GetChangeData unmarshals the type-specific change data.
func (s *Suggestion) GetChangeData(v any) error {
	if s.ChangeData == nil {
		return nil
	}
	return json.Unmarshal(s.ChangeData, v)
}

// Accept marks the suggestion as accepted and links to a proposal.
func (s *Suggestion) Accept(proposalID, actor string) error {
	if s.Status != SuggestionStatusPending {
		return ErrInvalidStatusTransition
	}
	s.Status = SuggestionStatusAccepted
	s.ProposalID = proposalID
	s.StatusChangedAt = time.Now()
	s.StatusChangedBy = actor
	return nil
}

// Reject marks the suggestion as rejected.
func (s *Suggestion) Reject(actor, reason string) error {
	if s.Status != SuggestionStatusPending {
		return ErrInvalidStatusTransition
	}
	s.Status = SuggestionStatusRejected
	s.StatusChangedAt = time.Now()
	s.StatusChangedBy = actor
	s.Metadata["rejection_reason"] = reason
	return nil
}

// Supersede marks the suggestion as superseded by a newer one.
func (s *Suggestion) Supersede(newSuggestionID string) error {
	if s.Status != SuggestionStatusPending {
		return ErrInvalidStatusTransition
	}
	s.Status = SuggestionStatusSuperseded
	s.StatusChangedAt = time.Now()
	s.Metadata["superseded_by"] = newSuggestionID
	return nil
}

// IsSignificant returns true if the suggestion meets significance thresholds.
func (s *Suggestion) IsSignificant(minConfidence float64) bool {
	return s.Confidence >= minConfidence
}

// AddPatternID adds a pattern ID if not already present.
func (s *Suggestion) AddPatternID(patternID string) {
	for _, id := range s.PatternIDs {
		if id == patternID {
			return
		}
	}
	s.PatternIDs = append(s.PatternIDs, patternID)
}
