// Package pattern provides pattern detection types.
package pattern

import (
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// PatternType classifies patterns.
type PatternType string

const (
	// Behavioral patterns
	PatternTypeToolSequence PatternType = "tool_sequence"  // Repeated tool call sequences
	PatternTypeStateLoop    PatternType = "state_loop"     // Repeated state transitions
	PatternTypeToolAffinity PatternType = "tool_affinity"  // Tools frequently used together

	// Failure patterns
	PatternTypeRecurringFailure PatternType = "recurring_failure" // Same failure mode
	PatternTypeToolFailure      PatternType = "tool_failure"      // Tool consistently fails
	PatternTypeBudgetExhaustion PatternType = "budget_exhaustion" // Runs hitting budget limits

	// Performance patterns
	PatternTypeSlowTool PatternType = "slow_tool"  // Tool performance degradation
	PatternTypeLongRuns PatternType = "long_runs"  // Runs taking longer than expected
)

// ToolSequenceData captures a repeated sequence of tool calls.
type ToolSequenceData struct {
	Sequence   []string        `json:"sequence"`    // Ordered tool names
	AverageGap time.Duration   `json:"average_gap"` // Average time between calls
	States     []agent.State   `json:"states"`      // States where sequence occurs
}

// StateLoopData captures repeated state transitions.
type StateLoopData struct {
	Loop       []agent.State `json:"loop"`        // State sequence forming loop
	Iterations int           `json:"iterations"`  // Average iterations before exit
	ExitState  agent.State   `json:"exit_state"`  // How loop typically exits
}

// ToolAffinityData captures tools frequently used together.
type ToolAffinityData struct {
	Tools       []string `json:"tools"`       // Tool names that appear together
	Correlation float64  `json:"correlation"` // How often they co-occur (0.0-1.0)
}

// FailureData captures recurring failure information.
type FailureData struct {
	FailureType  string      `json:"failure_type"`
	ToolName     string      `json:"tool_name,omitempty"`
	State        agent.State `json:"state"`
	ErrorPattern string      `json:"error_pattern"` // Common substring in errors
}

// PerformanceData captures performance-related pattern data.
type PerformanceData struct {
	ToolName        string        `json:"tool_name,omitempty"`
	AverageDuration time.Duration `json:"average_duration"`
	Threshold       time.Duration `json:"threshold"`       // Normal expected duration
	Deviation       float64       `json:"deviation"`       // How much above threshold
}

// ToolFailureData captures tool failure pattern data.
type ToolFailureData struct {
	ToolName   string `json:"tool_name"`
	ErrorType  string `json:"error_type"`  // Classified error type
	ErrorCount int    `json:"error_count"` // Number of failures
}

// BudgetExhaustionData captures budget exhaustion pattern data.
type BudgetExhaustionData struct {
	BudgetName      string `json:"budget_name,omitempty"` // Which budget was exhausted
	ExhaustionCount int    `json:"exhaustion_count"`      // Number of exhaustions
}

// SlowToolData captures slow tool performance pattern data.
type SlowToolData struct {
	ToolName        string        `json:"tool_name"`
	AverageDuration time.Duration `json:"average_duration"`
	P90Duration     time.Duration `json:"p90_duration"` // 90th percentile
	SlowCount       int           `json:"slow_count"`   // Number of slow executions
}

// LongRunsData captures long-running runs pattern data.
type LongRunsData struct {
	AverageDuration time.Duration `json:"average_duration"`
	Threshold       time.Duration `json:"threshold"`
	LongRunCount    int           `json:"long_run_count"`
}
