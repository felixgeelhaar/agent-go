package agent

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecisionType_Constants(t *testing.T) {
	t.Parallel()

	// Verify decision types are distinct
	types := []DecisionType{
		DecisionCallTool,
		DecisionTransition,
		DecisionAskHuman,
		DecisionFinish,
		DecisionFail,
	}

	seen := make(map[DecisionType]bool)
	for _, dt := range types {
		if seen[dt] {
			t.Errorf("Duplicate decision type: %s", dt)
		}
		seen[dt] = true
	}
}

func TestNewCallToolDecision(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"path": "/tmp/test"}`)
	d := NewCallToolDecision("read_file", input, "Need to read file")

	if d.Type != DecisionCallTool {
		t.Errorf("Type = %v, want call_tool", d.Type)
	}
	if d.CallTool == nil {
		t.Fatal("CallTool is nil")
	}
	if d.CallTool.ToolName != "read_file" {
		t.Errorf("ToolName = %v, want read_file", d.CallTool.ToolName)
	}
	if string(d.CallTool.Input) != string(input) {
		t.Errorf("Input = %s, want %s", d.CallTool.Input, input)
	}
	if d.CallTool.Reason != "Need to read file" {
		t.Errorf("Reason = %v, want 'Need to read file'", d.CallTool.Reason)
	}

	// Other fields should be nil
	if d.Transition != nil {
		t.Error("Transition should be nil")
	}
	if d.AskHuman != nil {
		t.Error("AskHuman should be nil")
	}
	if d.Finish != nil {
		t.Error("Finish should be nil")
	}
	if d.Fail != nil {
		t.Error("Fail should be nil")
	}
}

func TestNewTransitionDecision(t *testing.T) {
	t.Parallel()

	d := NewTransitionDecision(StateExplore, "Gathering information")

	if d.Type != DecisionTransition {
		t.Errorf("Type = %v, want transition", d.Type)
	}
	if d.Transition == nil {
		t.Fatal("Transition is nil")
	}
	if d.Transition.ToState != StateExplore {
		t.Errorf("ToState = %v, want explore", d.Transition.ToState)
	}
	if d.Transition.Reason != "Gathering information" {
		t.Errorf("Reason = %v, want 'Gathering information'", d.Transition.Reason)
	}

	// Other fields should be nil
	if d.CallTool != nil {
		t.Error("CallTool should be nil")
	}
}

func TestNewAskHumanDecision(t *testing.T) {
	t.Parallel()

	t.Run("without options", func(t *testing.T) {
		t.Parallel()

		d := NewAskHumanDecision("What should I do next?")

		if d.Type != DecisionAskHuman {
			t.Errorf("Type = %v, want ask_human", d.Type)
		}
		if d.AskHuman == nil {
			t.Fatal("AskHuman is nil")
		}
		if d.AskHuman.Question != "What should I do next?" {
			t.Errorf("Question = %v, want 'What should I do next?'", d.AskHuman.Question)
		}
		if len(d.AskHuman.Options) != 0 {
			t.Errorf("Options = %v, want empty", d.AskHuman.Options)
		}
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()

		d := NewAskHumanDecision("Choose an action:", "option1", "option2", "option3")

		if d.AskHuman.Question != "Choose an action:" {
			t.Errorf("Question = %v, want 'Choose an action:'", d.AskHuman.Question)
		}
		if len(d.AskHuman.Options) != 3 {
			t.Errorf("Options count = %d, want 3", len(d.AskHuman.Options))
		}
		if d.AskHuman.Options[0] != "option1" {
			t.Errorf("Options[0] = %v, want option1", d.AskHuman.Options[0])
		}
	})
}

func TestNewFinishDecision(t *testing.T) {
	t.Parallel()

	t.Run("with result", func(t *testing.T) {
		t.Parallel()

		result := json.RawMessage(`{"success": true, "count": 42}`)
		d := NewFinishDecision("Task completed successfully", result)

		if d.Type != DecisionFinish {
			t.Errorf("Type = %v, want finish", d.Type)
		}
		if d.Finish == nil {
			t.Fatal("Finish is nil")
		}
		if d.Finish.Summary != "Task completed successfully" {
			t.Errorf("Summary = %v, want 'Task completed successfully'", d.Finish.Summary)
		}
		if string(d.Finish.Result) != string(result) {
			t.Errorf("Result = %s, want %s", d.Finish.Result, result)
		}
	})

	t.Run("without result", func(t *testing.T) {
		t.Parallel()

		d := NewFinishDecision("Done", nil)

		if d.Finish.Summary != "Done" {
			t.Errorf("Summary = %v, want 'Done'", d.Finish.Summary)
		}
		if d.Finish.Result != nil {
			t.Errorf("Result = %v, want nil", d.Finish.Result)
		}
	})
}

func TestNewFailDecision(t *testing.T) {
	t.Parallel()

	t.Run("with error", func(t *testing.T) {
		t.Parallel()

		err := errors.New("something went wrong")
		d := NewFailDecision("Operation failed", err)

		if d.Type != DecisionFail {
			t.Errorf("Type = %v, want fail", d.Type)
		}
		if d.Fail == nil {
			t.Fatal("Fail is nil")
		}
		if d.Fail.Reason != "Operation failed" {
			t.Errorf("Reason = %v, want 'Operation failed'", d.Fail.Reason)
		}
		if d.Fail.Err != err {
			t.Errorf("Err = %v, want %v", d.Fail.Err, err)
		}
	})

	t.Run("without error", func(t *testing.T) {
		t.Parallel()

		d := NewFailDecision("Operation failed", nil)

		if d.Fail.Reason != "Operation failed" {
			t.Errorf("Reason = %v, want 'Operation failed'", d.Fail.Reason)
		}
		if d.Fail.Err != nil {
			t.Errorf("Err = %v, want nil", d.Fail.Err)
		}
	})
}

func TestDecision_IsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision Decision
		expected bool
	}{
		{
			name:     "call_tool",
			decision: NewCallToolDecision("tool", nil, "reason"),
			expected: false,
		},
		{
			name:     "transition",
			decision: NewTransitionDecision(StateExplore, "reason"),
			expected: false,
		},
		{
			name:     "ask_human",
			decision: NewAskHumanDecision("question"),
			expected: false,
		},
		{
			name:     "finish",
			decision: NewFinishDecision("done", nil),
			expected: true,
		},
		{
			name:     "fail",
			decision: NewFailDecision("error", nil),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.decision.IsTerminal(); got != tt.expected {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDecision_JSONSerialization(t *testing.T) {
	t.Parallel()

	t.Run("CallToolDecision", func(t *testing.T) {
		t.Parallel()

		ctd := &CallToolDecision{
			ToolName: "test_tool",
			Input:    json.RawMessage(`{"key": "value"}`),
			Reason:   "testing",
		}

		data, err := json.Marshal(ctd)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var unmarshaled CallToolDecision
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if unmarshaled.ToolName != ctd.ToolName {
			t.Errorf("ToolName = %v, want %v", unmarshaled.ToolName, ctd.ToolName)
		}
	})

	t.Run("TransitionDecision", func(t *testing.T) {
		t.Parallel()

		td := &TransitionDecision{
			ToState: StateExplore,
			Reason:  "exploring",
		}

		data, err := json.Marshal(td)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var unmarshaled TransitionDecision
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if unmarshaled.ToState != td.ToState {
			t.Errorf("ToState = %v, want %v", unmarshaled.ToState, td.ToState)
		}
	})

	t.Run("FinishDecision", func(t *testing.T) {
		t.Parallel()

		fd := &FinishDecision{
			Summary: "completed",
			Result:  json.RawMessage(`{"status": "ok"}`),
		}

		data, err := json.Marshal(fd)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var unmarshaled FinishDecision
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if unmarshaled.Summary != fd.Summary {
			t.Errorf("Summary = %v, want %v", unmarshaled.Summary, fd.Summary)
		}
	})
}
