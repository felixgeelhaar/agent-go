package agent

import (
	"encoding/json"
	"testing"
)

func TestNewToolEvidence(t *testing.T) {
	t.Parallel()

	content := json.RawMessage(`{"result": "success"}`)
	evidence := NewToolEvidence("my_tool", content)

	if evidence.Type != EvidenceToolResult {
		t.Errorf("Type = %s, want %s", evidence.Type, EvidenceToolResult)
	}
	if evidence.Source != "my_tool" {
		t.Errorf("Source = %s, want my_tool", evidence.Source)
	}
	if string(evidence.Content) != string(content) {
		t.Errorf("Content = %s, want %s", evidence.Content, content)
	}
	if evidence.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNewHumanEvidence(t *testing.T) {
	t.Parallel()

	content := json.RawMessage(`{"input": "user says hello"}`)
	evidence := NewHumanEvidence(content)

	if evidence.Type != EvidenceHumanInput {
		t.Errorf("Type = %s, want %s", evidence.Type, EvidenceHumanInput)
	}
	if evidence.Source != "human" {
		t.Errorf("Source = %s, want human", evidence.Source)
	}
	if string(evidence.Content) != string(content) {
		t.Errorf("Content = %s, want %s", evidence.Content, content)
	}
	if evidence.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNewSystemEvidence(t *testing.T) {
	t.Parallel()

	note := "system observation"
	evidence := NewSystemEvidence(note)

	if evidence.Type != EvidenceSystemNote {
		t.Errorf("Type = %s, want %s", evidence.Type, EvidenceSystemNote)
	}
	if evidence.Source != "system" {
		t.Errorf("Source = %s, want system", evidence.Source)
	}
	if evidence.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// Verify content contains the note
	var content map[string]string
	if err := json.Unmarshal(evidence.Content, &content); err != nil {
		t.Fatalf("failed to unmarshal content: %v", err)
	}
	if content["note"] != note {
		t.Errorf("note = %s, want %s", content["note"], note)
	}
}

func TestEvidenceTypeConstants(t *testing.T) {
	t.Parallel()

	// Verify constants have expected values
	if EvidenceToolResult != "tool_result" {
		t.Errorf("EvidenceToolResult = %s, want tool_result", EvidenceToolResult)
	}
	if EvidenceHumanInput != "human_input" {
		t.Errorf("EvidenceHumanInput = %s, want human_input", EvidenceHumanInput)
	}
	if EvidenceSystemNote != "system_note" {
		t.Errorf("EvidenceSystemNote = %s, want system_note", EvidenceSystemNote)
	}
}
