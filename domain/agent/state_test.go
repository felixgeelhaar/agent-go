package agent

import "testing"

func TestState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateIntake, false},
		{StateExplore, false},
		{StateDecide, false},
		{StateAct, false},
		{StateValidate, false},
		{StateDone, true},
		{StateFailed, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.expected {
				t.Errorf("State(%q).IsTerminal() = %v, want %v", tt.state, got, tt.expected)
			}
		})
	}
}

func TestState_AllowsSideEffects(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateIntake, false},
		{StateExplore, false},
		{StateDecide, false},
		{StateAct, true},
		{StateValidate, false},
		{StateDone, false},
		{StateFailed, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.AllowsSideEffects(); got != tt.expected {
				t.Errorf("State(%q).AllowsSideEffects() = %v, want %v", tt.state, got, tt.expected)
			}
		})
	}
}

func TestState_IsValid(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateIntake, true},
		{StateExplore, true},
		{StateDecide, true},
		{StateAct, true},
		{StateValidate, true},
		{StateDone, true},
		{StateFailed, true},
		{State("unknown"), false},
		{State(""), false},
		{State("INTAKE"), false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsValid(); got != tt.expected {
				t.Errorf("State(%q).IsValid() = %v, want %v", tt.state, got, tt.expected)
			}
		})
	}
}

func TestState_String(t *testing.T) {
	if got := StateIntake.String(); got != "intake" {
		t.Errorf("StateIntake.String() = %q, want %q", got, "intake")
	}
}

func TestAllStates(t *testing.T) {
	states := AllStates()
	if len(states) != 7 {
		t.Errorf("AllStates() returned %d states, want 7", len(states))
	}

	expected := map[State]bool{
		StateIntake:   true,
		StateExplore:  true,
		StateDecide:   true,
		StateAct:      true,
		StateValidate: true,
		StateDone:     true,
		StateFailed:   true,
	}

	for _, s := range states {
		if !expected[s] {
			t.Errorf("AllStates() contains unexpected state %q", s)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	states := TerminalStates()
	if len(states) != 2 {
		t.Errorf("TerminalStates() returned %d states, want 2", len(states))
	}

	for _, s := range states {
		if !s.IsTerminal() {
			t.Errorf("TerminalStates() contains non-terminal state %q", s)
		}
	}
}

func TestNonTerminalStates(t *testing.T) {
	states := NonTerminalStates()
	if len(states) != 5 {
		t.Errorf("NonTerminalStates() returned %d states, want 5", len(states))
	}

	for _, s := range states {
		if s.IsTerminal() {
			t.Errorf("NonTerminalStates() contains terminal state %q", s)
		}
	}
}
