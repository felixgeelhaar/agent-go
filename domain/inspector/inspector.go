// Package inspector provides types for inspecting and exporting agent data.
package inspector

import (
	"context"
	"time"
)

// Inspector exports agent data for visualization and analysis.
type Inspector interface {
	// ExportRun exports data for a single run.
	ExportRun(ctx context.Context, runID string, format ExportFormat) ([]byte, error)

	// ExportStateMachine exports the state machine graph.
	ExportStateMachine(ctx context.Context, format ExportFormat) ([]byte, error)

	// ExportMetrics exports aggregated metrics.
	ExportMetrics(ctx context.Context, filter MetricsFilter, format ExportFormat) ([]byte, error)
}

// MetricsFilter configures metrics export.
type MetricsFilter struct {
	// FromTime filters metrics after this time.
	FromTime time.Time

	// ToTime filters metrics before this time.
	ToTime time.Time

	// IncludeToolMetrics includes per-tool metrics.
	IncludeToolMetrics bool

	// IncludeStateMetrics includes per-state metrics.
	IncludeStateMetrics bool
}

// DefaultMetricsFilter returns sensible defaults for metrics export.
func DefaultMetricsFilter() MetricsFilter {
	return MetricsFilter{
		IncludeToolMetrics:  true,
		IncludeStateMetrics: true,
	}
}

// RunExporter exports run data.
type RunExporter interface {
	// Export exports run data in the specified format.
	Export(ctx context.Context, runID string) (*RunExport, error)
}

// StateMachineExporter exports state machine data.
type StateMachineExporter interface {
	// Export exports the state machine in the specified format.
	Export(ctx context.Context) (*StateMachineExport, error)
}

// MetricsExporter exports metrics data.
type MetricsExporter interface {
	// Export exports metrics in the specified format.
	Export(ctx context.Context, filter MetricsFilter) (*MetricsExport, error)
}

// Formatter formats export data to a specific format.
type Formatter interface {
	// Format formats the data.
	Format(data any) ([]byte, error)

	// FormatType returns the format type.
	FormatType() ExportFormat
}
