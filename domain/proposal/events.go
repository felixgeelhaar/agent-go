// Package proposal provides proposal types for policy evolution.
package proposal

import "time"

// EventType identifies proposal-related events.
type EventType string

const (
	// EventTypeProposalCreated is emitted when a proposal is created.
	EventTypeProposalCreated EventType = "proposal.created"

	// EventTypeProposalSubmitted is emitted when a proposal is submitted for review.
	EventTypeProposalSubmitted EventType = "proposal.submitted"

	// EventTypeProposalApproved is emitted when a proposal is approved.
	EventTypeProposalApproved EventType = "proposal.approved"

	// EventTypeProposalRejected is emitted when a proposal is rejected.
	EventTypeProposalRejected EventType = "proposal.rejected"

	// EventTypeProposalApplied is emitted when proposal changes are applied.
	EventTypeProposalApplied EventType = "proposal.applied"

	// EventTypeProposalRolledBack is emitted when proposal changes are rolled back.
	EventTypeProposalRolledBack EventType = "proposal.rolled_back"

	// EventTypeNoteAdded is emitted when a note is added to a proposal.
	EventTypeNoteAdded EventType = "proposal.note_added"
)

// ProposalEvent represents an event related to a proposal.
type ProposalEvent struct {
	// Type identifies the event.
	Type EventType `json:"type"`

	// ProposalID is the proposal this event relates to.
	ProposalID string `json:"proposal_id"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Actor is who triggered the event.
	Actor string `json:"actor"`

	// Data contains event-specific data.
	Data any `json:"data,omitempty"`
}

// ProposalCreatedData contains data for proposal.created events.
type ProposalCreatedData struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SuggestionID string `json:"suggestion_id,omitempty"`
}

// ProposalSubmittedData contains data for proposal.submitted events.
type ProposalSubmittedData struct {
	ChangeCount int `json:"change_count"`
}

// ProposalApprovedData contains data for proposal.approved events.
type ProposalApprovedData struct {
	Reason string `json:"reason"`
}

// ProposalRejectedData contains data for proposal.rejected events.
type ProposalRejectedData struct {
	Reason string `json:"reason"`
}

// ProposalAppliedData contains data for proposal.applied events.
type ProposalAppliedData struct {
	PolicyVersionBefore int `json:"policy_version_before"`
	PolicyVersionAfter  int `json:"policy_version_after"`
}

// ProposalRolledBackData contains data for proposal.rolled_back events.
type ProposalRolledBackData struct {
	Reason string `json:"reason"`
}

// NoteAddedData contains data for proposal.note_added events.
type NoteAddedData struct {
	Author  string `json:"author"`
	Content string `json:"content"`
}
