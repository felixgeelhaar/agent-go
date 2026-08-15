package agent

import (
	"encoding/json"
	"time"
)

// EvidenceType classifies the source of evidence.
type EvidenceType string

const (
	EvidenceToolResult EvidenceType = "tool_result" // Result from tool execution
	EvidenceHumanInput EvidenceType = "human_input" // Input from human
	EvidenceSystemNote EvidenceType = "system_note" // System-generated observation
)

// Evidence represents an observation or result accumulated during a run.
type Evidence struct {
	Type      EvidenceType    `json:"type"`
	Source    string          `json:"source"` // Tool name or "system"
	Content   json.RawMessage `json:"content"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewToolEvidence creates evidence from a tool result, stamping the wall clock.
// The engine uses NewToolEvidenceAt with an injected clock for determinism.
func NewToolEvidence(toolName string, content json.RawMessage) Evidence {
	return NewToolEvidenceAt(toolName, content, time.Now())
}

// NewToolEvidenceAt creates evidence from a tool result at the given instant.
func NewToolEvidenceAt(toolName string, content json.RawMessage, t time.Time) Evidence {
	return Evidence{
		Type:      EvidenceToolResult,
		Source:    toolName,
		Content:   content,
		Timestamp: t,
	}
}

// NewHumanEvidence creates evidence from human input, stamping the wall clock.
// The engine uses NewHumanEvidenceAt with an injected clock for determinism.
func NewHumanEvidence(content json.RawMessage) Evidence {
	return NewHumanEvidenceAt(content, time.Now())
}

// NewHumanEvidenceAt creates evidence from human input at the given instant.
func NewHumanEvidenceAt(content json.RawMessage, t time.Time) Evidence {
	return Evidence{
		Type:      EvidenceHumanInput,
		Source:    "human",
		Content:   content,
		Timestamp: t,
	}
}

// NewSystemEvidence creates system-generated evidence, stamping the wall clock.
// The engine uses NewSystemEvidenceAt with an injected clock for determinism.
func NewSystemEvidence(note string) Evidence {
	return NewSystemEvidenceAt(note, time.Now())
}

// NewSystemEvidenceAt creates system-generated evidence at the given instant.
func NewSystemEvidenceAt(note string, t time.Time) Evidence {
	content, _ := json.Marshal(map[string]string{"note": note})
	return Evidence{
		Type:      EvidenceSystemNote,
		Source:    "system",
		Content:   content,
		Timestamp: t,
	}
}
