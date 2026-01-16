package observability

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/telemetry"
)

// NoopTracer is a no-op tracer implementation.
type NoopTracer struct{}

// NewNoopTracer creates a new no-op tracer.
func NewNoopTracer() *NoopTracer {
	return &NoopTracer{}
}

// StartSpan implements telemetry.Tracer.
func (t *NoopTracer) StartSpan(ctx context.Context, _ string, _ ...telemetry.SpanOption) (context.Context, telemetry.Span) {
	return ctx, &noopSpan{}
}

var _ telemetry.Tracer = (*NoopTracer)(nil)

type noopSpan struct{}

func (s *noopSpan) End()                                               {}
func (s *noopSpan) SetAttributes(_ ...telemetry.Attribute)             {}
func (s *noopSpan) RecordError(_ error)                                {}
func (s *noopSpan) SetStatus(_ telemetry.StatusCode, _ string)         {}
func (s *noopSpan) AddEvent(_ string, _ ...telemetry.Attribute)        {}

var _ telemetry.Span = (*noopSpan)(nil)

// NoopMeter is a no-op meter implementation.
type NoopMeter struct{}

// NewNoopMeter creates a new no-op meter.
func NewNoopMeter() *NoopMeter {
	return &NoopMeter{}
}

// Counter implements telemetry.Meter.
func (m *NoopMeter) Counter(_ string, _ ...telemetry.MetricOption) telemetry.Counter {
	return &noopCounter{}
}

// Histogram implements telemetry.Meter.
func (m *NoopMeter) Histogram(_ string, _ ...telemetry.MetricOption) telemetry.Histogram {
	return &noopHistogram{}
}

// Gauge implements telemetry.Meter.
func (m *NoopMeter) Gauge(_ string, _ ...telemetry.MetricOption) telemetry.Gauge {
	return &noopGauge{}
}

var _ telemetry.Meter = (*NoopMeter)(nil)

type noopCounter struct{}

func (c *noopCounter) Add(_ context.Context, _ int64, _ ...telemetry.Attribute) {}

var _ telemetry.Counter = (*noopCounter)(nil)

type noopHistogram struct{}

func (h *noopHistogram) Record(_ context.Context, _ float64, _ ...telemetry.Attribute) {}

var _ telemetry.Histogram = (*noopHistogram)(nil)

type noopGauge struct{}

func (g *noopGauge) Record(_ context.Context, _ float64, _ ...telemetry.Attribute) {}

var _ telemetry.Gauge = (*noopGauge)(nil)
