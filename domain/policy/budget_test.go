package policy

import (
	"sync"
	"testing"
)

func TestNewBudget(t *testing.T) {
	budget := NewBudget(map[string]int{
		"tool_calls": 100,
		"tokens":     50000,
	})

	if budget.Remaining("tool_calls") != 100 {
		t.Errorf("NewBudget() tool_calls remaining = %d, want 100", budget.Remaining("tool_calls"))
	}
	if budget.Remaining("tokens") != 50000 {
		t.Errorf("NewBudget() tokens remaining = %d, want 50000", budget.Remaining("tokens"))
	}
}

func TestUnlimitedBudget(t *testing.T) {
	budget := UnlimitedBudget()

	// Should always allow consumption
	if !budget.CanConsume("anything", 1000000) {
		t.Error("UnlimitedBudget() should allow any consumption")
	}

	// Remaining should be -1 for unlimited
	if budget.Remaining("anything") != -1 {
		t.Errorf("UnlimitedBudget() Remaining = %d, want -1", budget.Remaining("anything"))
	}
}

func TestBudget_CanConsume(t *testing.T) {
	budget := NewBudget(map[string]int{
		"tool_calls": 10,
	})

	tests := []struct {
		name     string
		resource string
		amount   int
		expected bool
	}{
		{"within limit", "tool_calls", 5, true},
		{"at limit", "tool_calls", 10, true},
		{"over limit", "tool_calls", 11, false},
		{"unknown resource", "unknown", 100, true}, // No limit defined
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := budget.CanConsume(tt.resource, tt.amount); got != tt.expected {
				t.Errorf("Budget.CanConsume(%q, %d) = %v, want %v", tt.resource, tt.amount, got, tt.expected)
			}
		})
	}
}

func TestBudget_Consume(t *testing.T) {
	t.Run("successful consumption", func(t *testing.T) {
		budget := NewBudget(map[string]int{"calls": 10})

		err := budget.Consume("calls", 3)
		if err != nil {
			t.Errorf("Budget.Consume() error = %v, want nil", err)
		}

		if budget.Remaining("calls") != 7 {
			t.Errorf("Budget.Remaining() = %d, want 7", budget.Remaining("calls"))
		}
	})

	t.Run("consumption exceeds limit", func(t *testing.T) {
		budget := NewBudget(map[string]int{"calls": 10})

		err := budget.Consume("calls", 11)
		if err != ErrBudgetExceeded {
			t.Errorf("Budget.Consume() error = %v, want ErrBudgetExceeded", err)
		}
	})

	t.Run("consumption of unknown resource", func(t *testing.T) {
		budget := NewBudget(map[string]int{"calls": 10})

		err := budget.Consume("unknown", 100)
		if err != nil {
			t.Errorf("Budget.Consume() for unknown resource error = %v, want nil", err)
		}
	})

	t.Run("multiple consumptions", func(t *testing.T) {
		budget := NewBudget(map[string]int{"calls": 10})

		_ = budget.Consume("calls", 3)
		_ = budget.Consume("calls", 4)
		_ = budget.Consume("calls", 2)

		if budget.Remaining("calls") != 1 {
			t.Errorf("Budget.Remaining() = %d, want 1", budget.Remaining("calls"))
		}

		err := budget.Consume("calls", 2)
		if err != ErrBudgetExceeded {
			t.Errorf("Budget.Consume() should fail when exceeding limit")
		}
	})
}

func TestBudget_Snapshot(t *testing.T) {
	budget := NewBudget(map[string]int{
		"calls":  10,
		"tokens": 100,
	})

	_ = budget.Consume("calls", 3)
	_ = budget.Consume("tokens", 40)

	snapshot := budget.Snapshot()

	if snapshot.Limits["calls"] != 10 {
		t.Errorf("Snapshot.Limits[calls] = %d, want 10", snapshot.Limits["calls"])
	}
	if snapshot.Consumed["calls"] != 3 {
		t.Errorf("Snapshot.Consumed[calls] = %d, want 3", snapshot.Consumed["calls"])
	}
	if snapshot.Remaining["calls"] != 7 {
		t.Errorf("Snapshot.Remaining[calls] = %d, want 7", snapshot.Remaining["calls"])
	}
	if snapshot.Remaining["tokens"] != 60 {
		t.Errorf("Snapshot.Remaining[tokens] = %d, want 60", snapshot.Remaining["tokens"])
	}
}

func TestBudget_Reset(t *testing.T) {
	budget := NewBudget(map[string]int{"calls": 10})
	_ = budget.Consume("calls", 7)

	budget.Reset()

	if budget.Remaining("calls") != 10 {
		t.Errorf("Budget.Reset() Remaining = %d, want 10", budget.Remaining("calls"))
	}
}

func TestBudget_SetLimit(t *testing.T) {
	budget := NewBudget(map[string]int{"calls": 10})

	budget.SetLimit("calls", 20)
	if budget.Remaining("calls") != 20 {
		t.Errorf("Budget.SetLimit() calls remaining = %d, want 20", budget.Remaining("calls"))
	}

	budget.SetLimit("new_resource", 50)
	if budget.Remaining("new_resource") != 50 {
		t.Errorf("Budget.SetLimit() new_resource remaining = %d, want 50", budget.Remaining("new_resource"))
	}
}

func TestBudget_IsExhausted(t *testing.T) {
	budget := NewBudget(map[string]int{"calls": 3})

	if budget.IsExhausted() {
		t.Error("Budget.IsExhausted() should be false initially")
	}

	_ = budget.Consume("calls", 3)
	if !budget.IsExhausted() {
		t.Error("Budget.IsExhausted() should be true after full consumption")
	}
}

func TestBudget_ExhaustedBudgets(t *testing.T) {
	budget := NewBudget(map[string]int{
		"calls":  3,
		"tokens": 100,
	})

	_ = budget.Consume("calls", 3)

	exhausted := budget.ExhaustedBudgets()
	if len(exhausted) != 1 || exhausted[0] != "calls" {
		t.Errorf("Budget.ExhaustedBudgets() = %v, want [calls]", exhausted)
	}
}

func TestBudget_Concurrency(t *testing.T) {
	budget := NewBudget(map[string]int{"calls": 1000})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				budget.CanConsume("calls", 1)
				_ = budget.Consume("calls", 1)
				budget.Remaining("calls")
				budget.Snapshot()
			}
		}()
	}
	wg.Wait()

	// All 1000 consumptions should have succeeded
	if budget.Remaining("calls") != 0 {
		t.Errorf("Budget concurrent access: Remaining = %d, want 0", budget.Remaining("calls"))
	}
}
