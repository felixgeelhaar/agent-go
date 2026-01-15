// Package policy provides domain models for policy enforcement.
package policy

import (
	"sync"
)

// Budget tracks consumption against configured limits.
type Budget struct {
	limits   map[string]int
	consumed map[string]int
	mu       sync.RWMutex
}

// BudgetSnapshot is an immutable view of budget state.
type BudgetSnapshot struct {
	Limits    map[string]int `json:"limits"`
	Consumed  map[string]int `json:"consumed"`
	Remaining map[string]int `json:"remaining"`
}

// NewBudget creates a budget with the given limits.
func NewBudget(limits map[string]int) *Budget {
	b := &Budget{
		limits:   make(map[string]int),
		consumed: make(map[string]int),
	}
	for k, v := range limits {
		b.limits[k] = v
		b.consumed[k] = 0
	}
	return b
}

// UnlimitedBudget creates a budget with no limits.
func UnlimitedBudget() *Budget {
	return &Budget{
		limits:   make(map[string]int),
		consumed: make(map[string]int),
	}
}

// CanConsume checks if the budget allows consuming the given amount.
func (b *Budget) CanConsume(name string, amount int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	limit, hasLimit := b.limits[name]
	if !hasLimit {
		return true // No limit defined
	}

	consumed := b.consumed[name]
	return consumed+amount <= limit
}

// Consume deducts from the budget if allowed.
func (b *Budget) Consume(name string, amount int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	limit, hasLimit := b.limits[name]
	if !hasLimit {
		b.consumed[name] += amount
		return nil
	}

	consumed := b.consumed[name]
	if consumed+amount > limit {
		return ErrBudgetExceeded
	}

	b.consumed[name] = consumed + amount
	return nil
}

// Remaining returns the remaining budget for a given name.
func (b *Budget) Remaining(name string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	limit, hasLimit := b.limits[name]
	if !hasLimit {
		return -1 // Unlimited
	}

	return limit - b.consumed[name]
}

// Snapshot returns an immutable view of the current budget state.
func (b *Budget) Snapshot() BudgetSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	snapshot := BudgetSnapshot{
		Limits:    make(map[string]int),
		Consumed:  make(map[string]int),
		Remaining: make(map[string]int),
	}

	for k, v := range b.limits {
		snapshot.Limits[k] = v
		snapshot.Consumed[k] = b.consumed[k]
		snapshot.Remaining[k] = v - b.consumed[k]
	}

	// Include consumed items without limits
	for k, v := range b.consumed {
		if _, hasLimit := b.limits[k]; !hasLimit {
			snapshot.Consumed[k] = v
		}
	}

	return snapshot
}

// Reset resets all consumed values to zero.
func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for k := range b.consumed {
		b.consumed[k] = 0
	}
}

// SetLimit sets or updates a budget limit.
func (b *Budget) SetLimit(name string, limit int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.limits[name] = limit
	if _, exists := b.consumed[name]; !exists {
		b.consumed[name] = 0
	}
}

// IsExhausted returns true if any budget is fully consumed.
func (b *Budget) IsExhausted() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for name, limit := range b.limits {
		if b.consumed[name] >= limit {
			return true
		}
	}
	return false
}

// ExhaustedBudgets returns the names of all exhausted budgets.
func (b *Budget) ExhaustedBudgets() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var exhausted []string
	for name, limit := range b.limits {
		if b.consumed[name] >= limit {
			exhausted = append(exhausted, name)
		}
	}
	return exhausted
}
