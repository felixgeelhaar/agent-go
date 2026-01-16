// Package tool provides the domain model for agent tools.
package tool

// RiskLevel indicates the potential impact of a tool execution.
type RiskLevel int

const (
	RiskNone     RiskLevel = iota // No risk - purely informational
	RiskLow                       // Low risk - reversible changes
	RiskMedium                    // Medium risk - may require cleanup
	RiskHigh                      // High risk - difficult to reverse
	RiskCritical                  // Critical risk - irreversible or destructive
)

// String returns the string representation of the risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskNone:
		return "none"
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Annotations describe tool behavior for policy enforcement, caching, and planning.
type Annotations struct {
	// ReadOnly indicates the tool has no side effects.
	ReadOnly bool `json:"read_only"`

	// Destructive indicates the tool may cause irreversible changes.
	Destructive bool `json:"destructive"`

	// Idempotent indicates multiple calls with same input yield same result.
	Idempotent bool `json:"idempotent"`

	// Cacheable indicates results can be cached.
	Cacheable bool `json:"cacheable"`

	// RiskLevel indicates the potential impact of execution.
	RiskLevel RiskLevel `json:"risk_level"`

	// RequiresApproval indicates human approval is required.
	RequiresApproval bool `json:"requires_approval"`

	// Timeout is the maximum execution time in seconds (0 = default).
	Timeout int `json:"timeout,omitempty"`

	// Sandboxed indicates the tool should execute in an isolated sandbox.
	Sandboxed bool `json:"sandboxed"`

	// Tags are arbitrary labels for categorization.
	Tags []string `json:"tags,omitempty"`
}

// DefaultAnnotations returns annotations with safe defaults.
func DefaultAnnotations() Annotations {
	return Annotations{
		ReadOnly:         false,
		Destructive:      false,
		Idempotent:       false,
		Cacheable:        false,
		RiskLevel:        RiskLow,
		RequiresApproval: false,
	}
}

// ReadOnlyAnnotations returns annotations for a read-only tool.
func ReadOnlyAnnotations() Annotations {
	return Annotations{
		ReadOnly:   true,
		Idempotent: true,
		Cacheable:  true,
		RiskLevel:  RiskNone,
	}
}

// DestructiveAnnotations returns annotations for a destructive tool.
func DestructiveAnnotations() Annotations {
	return Annotations{
		Destructive:      true,
		RiskLevel:        RiskHigh,
		RequiresApproval: true,
	}
}

// ShouldRequireApproval returns true if the tool should require approval.
func (a Annotations) ShouldRequireApproval() bool {
	return a.RequiresApproval || a.Destructive || a.RiskLevel >= RiskHigh
}

// CanCache returns true if the tool result can be cached.
func (a Annotations) CanCache() bool {
	return a.Cacheable && (a.ReadOnly || a.Idempotent)
}

// CanRetry returns true if the tool can be safely retried on failure.
func (a Annotations) CanRetry() bool {
	return a.Idempotent || a.ReadOnly
}
