// Package proposal provides proposal types for policy evolution.
package proposal

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Proposal represents a proposed policy change requiring human approval.
type Proposal struct {
	// ID is the unique identifier.
	ID string `json:"id"`

	// Title is a human-readable summary.
	Title string `json:"title"`

	// Description explains the proposal in detail.
	Description string `json:"description"`

	// SuggestionID links to the suggestion that created this proposal (if any).
	SuggestionID string `json:"suggestion_id,omitempty"`

	// Changes are the policy changes in this proposal.
	Changes []PolicyChange `json:"changes"`

	// Evidence provides supporting data for the proposal.
	Evidence []ProposalEvidence `json:"evidence"`

	// Status is the current proposal status.
	Status ProposalStatus `json:"status"`

	// CreatedAt is when the proposal was created.
	CreatedAt time.Time `json:"created_at"`

	// CreatedBy identifies who created the proposal.
	CreatedBy string `json:"created_by"`

	// SubmittedAt is when the proposal was submitted for review.
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`

	// SubmittedBy identifies who submitted the proposal.
	SubmittedBy string `json:"submitted_by,omitempty"`

	// ApprovedAt is when the proposal was approved.
	ApprovedAt *time.Time `json:"approved_at,omitempty"`

	// ApprovedBy identifies who approved the proposal.
	ApprovedBy string `json:"approved_by,omitempty"`

	// ApprovalReason explains why the proposal was approved.
	ApprovalReason string `json:"approval_reason,omitempty"`

	// RejectedAt is when the proposal was rejected.
	RejectedAt *time.Time `json:"rejected_at,omitempty"`

	// RejectedBy identifies who rejected the proposal.
	RejectedBy string `json:"rejected_by,omitempty"`

	// RejectionReason explains why the proposal was rejected.
	RejectionReason string `json:"rejection_reason,omitempty"`

	// AppliedAt is when the changes were applied.
	AppliedAt *time.Time `json:"applied_at,omitempty"`

	// RolledBackAt is when the changes were rolled back.
	RolledBackAt *time.Time `json:"rolled_back_at,omitempty"`

	// RollbackReason explains why the proposal was rolled back.
	RollbackReason string `json:"rollback_reason,omitempty"`

	// PolicyVersionBefore is the policy version before applying changes.
	PolicyVersionBefore int `json:"policy_version_before,omitempty"`

	// PolicyVersionAfter is the policy version after applying changes.
	PolicyVersionAfter int `json:"policy_version_after,omitempty"`

	// Notes contains discussion and notes about the proposal.
	Notes []ProposalNote `json:"notes,omitempty"`

	// Metadata contains additional information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ProposalEvidence provides supporting data for a proposal.
type ProposalEvidence struct {
	// Type describes what kind of evidence this is.
	Type string `json:"type"`

	// Description explains the evidence.
	Description string `json:"description"`

	// Data contains the evidence data.
	Data json.RawMessage `json:"data,omitempty"`

	// AddedAt is when the evidence was added.
	AddedAt time.Time `json:"added_at"`
}

// ProposalNote is a note or comment on a proposal.
type ProposalNote struct {
	// Author is who wrote the note.
	Author string `json:"author"`

	// Content is the note text.
	Content string `json:"content"`

	// CreatedAt is when the note was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewProposal creates a new proposal with a generated ID.
func NewProposal(title, description, createdBy string) *Proposal {
	now := time.Now()
	return &Proposal{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		Status:      ProposalStatusDraft,
		Changes:     make([]PolicyChange, 0),
		Evidence:    make([]ProposalEvidence, 0),
		Notes:       make([]ProposalNote, 0),
		Metadata:    make(map[string]any),
	}
}

// AddChange adds a policy change to the proposal.
func (p *Proposal) AddChange(change PolicyChange) error {
	if p.Status != ProposalStatusDraft {
		return ErrCannotModifyNonDraft
	}
	p.Changes = append(p.Changes, change)
	return nil
}

// AddEvidence adds supporting evidence to the proposal.
func (p *Proposal) AddEvidence(evidenceType, description string, data any) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	p.Evidence = append(p.Evidence, ProposalEvidence{
		Type:        evidenceType,
		Description: description,
		Data:        dataJSON,
		AddedAt:     time.Now(),
	})
	return nil
}

// AddNote adds a note to the proposal.
func (p *Proposal) AddNote(author, content string) {
	p.Notes = append(p.Notes, ProposalNote{
		Author:    author,
		Content:   content,
		CreatedAt: time.Now(),
	})
}

// Submit submits the proposal for review.
func (p *Proposal) Submit(submitter string) error {
	if !p.Status.CanTransitionTo(ProposalStatusPendingReview) {
		return ErrInvalidStatusTransition
	}
	if len(p.Changes) == 0 {
		return ErrNoChanges
	}

	now := time.Now()
	p.Status = ProposalStatusPendingReview
	p.SubmittedAt = &now
	p.SubmittedBy = submitter
	return nil
}

// Approve approves the proposal.
func (p *Proposal) Approve(approver, reason string) error {
	if approver == "" {
		return ErrHumanActorRequired
	}
	if !p.Status.CanTransitionTo(ProposalStatusApproved) {
		return ErrInvalidStatusTransition
	}

	now := time.Now()
	p.Status = ProposalStatusApproved
	p.ApprovedAt = &now
	p.ApprovedBy = approver
	p.ApprovalReason = reason
	return nil
}

// Reject rejects the proposal.
func (p *Proposal) Reject(rejector, reason string) error {
	if rejector == "" {
		return ErrHumanActorRequired
	}
	if !p.Status.CanTransitionTo(ProposalStatusRejected) {
		return ErrInvalidStatusTransition
	}

	now := time.Now()
	p.Status = ProposalStatusRejected
	p.RejectedAt = &now
	p.RejectedBy = rejector
	p.RejectionReason = reason
	return nil
}

// Apply marks the proposal as applied.
func (p *Proposal) Apply(policyVersionBefore, policyVersionAfter int) error {
	if !p.Status.CanTransitionTo(ProposalStatusApplied) {
		return ErrInvalidStatusTransition
	}

	now := time.Now()
	p.Status = ProposalStatusApplied
	p.AppliedAt = &now
	p.PolicyVersionBefore = policyVersionBefore
	p.PolicyVersionAfter = policyVersionAfter
	return nil
}

// Rollback marks the proposal as rolled back.
func (p *Proposal) Rollback(reason string) error {
	if !p.Status.CanTransitionTo(ProposalStatusRolledBack) {
		return ErrInvalidStatusTransition
	}

	now := time.Now()
	p.Status = ProposalStatusRolledBack
	p.RolledBackAt = &now
	p.RollbackReason = reason
	return nil
}

// ReturnToDraft returns a rejected or rolled back proposal to draft.
func (p *Proposal) ReturnToDraft() error {
	if !p.Status.CanTransitionTo(ProposalStatusDraft) {
		return ErrInvalidStatusTransition
	}

	p.Status = ProposalStatusDraft
	return nil
}

// CanBeModified returns true if the proposal can be modified.
func (p *Proposal) CanBeModified() bool {
	return p.Status == ProposalStatusDraft
}

// IsApplied returns true if the proposal has been applied.
func (p *Proposal) IsApplied() bool {
	return p.Status == ProposalStatusApplied
}

// IsRolledBack returns true if the proposal has been rolled back.
func (p *Proposal) IsRolledBack() bool {
	return p.Status == ProposalStatusRolledBack
}
