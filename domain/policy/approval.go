package policy

import (
	"context"
	"encoding/json"
	"time"
)

// ApprovalRequest contains information for an approval decision.
type ApprovalRequest struct {
	RunID     string          `json:"run_id"`
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	Reason    string          `json:"reason"`
	RiskLevel string          `json:"risk_level"`
	Timestamp time.Time       `json:"timestamp"`
}

// ApprovalResponse contains the result of an approval request.
type ApprovalResponse struct {
	Approved  bool      `json:"approved"`
	Approver  string    `json:"approver,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Approver is the interface for approval handlers.
type Approver interface {
	// Approve processes an approval request and returns the decision.
	Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}

// AutoApprover automatically approves all requests.
type AutoApprover struct {
	approverName string
}

// NewAutoApprover creates an approver that automatically approves all requests.
func NewAutoApprover(name string) *AutoApprover {
	return &AutoApprover{approverName: name}
}

// Approve automatically approves the request.
func (a *AutoApprover) Approve(_ context.Context, _ ApprovalRequest) (ApprovalResponse, error) {
	return ApprovalResponse{
		Approved:  true,
		Approver:  a.approverName,
		Reason:    "auto-approved",
		Timestamp: time.Now(),
	}, nil
}

// DenyApprover automatically denies all requests.
type DenyApprover struct {
	reason string
}

// NewDenyApprover creates an approver that automatically denies all requests.
func NewDenyApprover(reason string) *DenyApprover {
	return &DenyApprover{reason: reason}
}

// Approve automatically denies the request.
func (d *DenyApprover) Approve(_ context.Context, _ ApprovalRequest) (ApprovalResponse, error) {
	return ApprovalResponse{
		Approved:  false,
		Reason:    d.reason,
		Timestamp: time.Now(),
	}, nil
}

// ApprovalPolicy determines which actions require approval.
type ApprovalPolicy struct {
	// RequireForDestructive requires approval for destructive tools.
	RequireForDestructive bool

	// RequireForHighRisk requires approval for high-risk tools.
	RequireForHighRisk bool

	// RequireForTools lists specific tools that always require approval.
	RequireForTools []string

	// ExemptTools lists tools that never require approval.
	ExemptTools []string
}

// DefaultApprovalPolicy returns a policy requiring approval for destructive actions.
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		RequireForDestructive: true,
		RequireForHighRisk:    true,
	}
}

// RequiresApproval checks if the given tool requires approval under this policy.
func (p ApprovalPolicy) RequiresApproval(toolName string, isDestructive, isHighRisk bool) bool {
	// Check exemptions first
	for _, exempt := range p.ExemptTools {
		if exempt == toolName {
			return false
		}
	}

	// Check explicit requirements
	for _, required := range p.RequireForTools {
		if required == toolName {
			return true
		}
	}

	// Check destructive requirement
	if p.RequireForDestructive && isDestructive {
		return true
	}

	// Check high-risk requirement
	if p.RequireForHighRisk && isHighRisk {
		return true
	}

	return false
}
