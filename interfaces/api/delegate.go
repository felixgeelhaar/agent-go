package api

import (
	"go.klarlabs.de/agent/domain/artifact"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/run"
	infraagent "go.klarlabs.de/agent/infrastructure/agent"
	"go.klarlabs.de/agent/infrastructure/storage/filesystem"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// In-memory stores (prefer these over importing infrastructure/storage/*).

// NewEventStore creates an in-memory event store for Stream/Replay/Fork.
func NewEventStore() event.Store {
	return memory.NewEventStore()
}

// NewRunStore creates an in-memory run store for persisting run snapshots.
func NewRunStore() run.Store {
	return memory.NewRunStore()
}

// NewArtifactStore creates a filesystem artifact store under basePath.
func NewArtifactStore(basePath string) (artifact.Store, error) {
	return filesystem.NewArtifactStore(basePath)
}

// Multi-agent delegation

// DelegateTool wraps a child Engine as a tool.
type DelegateTool = infraagent.DelegateTool

// DelegateOption configures a DelegateTool.
type DelegateOption = infraagent.DelegateOption

// WithDelegateTaskContext shares a TaskContext across parent/child runs.
func WithDelegateTaskContext(tc *TaskContext) DelegateOption {
	return infraagent.WithDelegateTaskContext(tc)
}

// WithDelegateRiskLevel sets the risk annotation on the delegate tool.
func WithDelegateRiskLevel(level RiskLevel) DelegateOption {
	return infraagent.WithRiskLevel(level)
}

// NewDelegateTool wraps child as a tool named name for agent composition.
// Callers pass *api.Engine; no need to import infrastructure/agent.
func NewDelegateTool(name, description string, child *Engine, opts ...DelegateOption) *DelegateTool {
	return infraagent.NewDelegateTool(name, description, child.engine, opts...)
}
