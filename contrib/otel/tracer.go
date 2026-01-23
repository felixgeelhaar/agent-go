// Package otel provides OpenTelemetry integration for agent-go.
//
// This package implements the telemetry interfaces from the domain layer using
// OpenTelemetry SDK, enabling distributed tracing and metrics collection for
// agent runs.
//
// # Usage
//
//	// Initialize tracer
//	tp, err := otel.NewTracerProvider(otel.TracerConfig{
//		ServiceName: "my-agent",
//		Endpoint:    "localhost:4317",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
//
//	// Create tracer
//	tracer := otel.NewTracer(tp)
//
//	// Use with agent engine
//	engine, err := api.New(
//		api.WithTracer(tracer),
//	)
//
// # Exported Spans
//
// The tracer creates spans for:
//   - Run lifecycle (start, complete, fail)
//   - State transitions
//   - Tool executions
//   - Planner decisions
//
// All spans include relevant attributes like run ID, state, tool name, etc.
package otel

import (
	"context"
	"errors"

	"github.com/felixgeelhaar/agent-go/domain/telemetry"
)

// Common errors for OpenTelemetry operations.
var (
	ErrShutdown         = errors.New("tracer provider shutdown")
	ErrInvalidEndpoint  = errors.New("invalid endpoint")
	ErrExporterFailed   = errors.New("exporter initialization failed")
)

// TracerConfig configures the OpenTelemetry tracer provider.
type TracerConfig struct {
	// ServiceName is the name of the service for tracing.
	ServiceName string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317").
	Endpoint string

	// Insecure disables TLS for the exporter connection.
	Insecure bool

	// SampleRate controls the trace sampling rate (0.0 to 1.0).
	SampleRate float64

	// ExporterType specifies the exporter ("otlp", "stdout", "none").
	ExporterType string

	// Headers are additional headers for the OTLP exporter.
	Headers map[string]string

	// BatchSize is the maximum number of spans per batch.
	BatchSize int

	// BatchTimeout is the maximum time to wait before sending a batch.
	BatchTimeout int // milliseconds

	// ResourceAttributes are additional resource attributes.
	ResourceAttributes map[string]string
}

// TracerProvider wraps the OpenTelemetry tracer provider.
type TracerProvider struct {
	config   TracerConfig
	shutdown bool
}

// NewTracerProvider creates a new OpenTelemetry tracer provider.
func NewTracerProvider(cfg TracerConfig) (*TracerProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "agent-go"
	}
	if cfg.ExporterType == "" {
		cfg.ExporterType = "otlp"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0 // Sample all traces by default
	}

	// TODO: Initialize actual OpenTelemetry tracer provider
	// 1. Create resource with service name and version
	// 2. Create exporter based on ExporterType
	// 3. Create span processor (batch or simple)
	// 4. Create and register tracer provider

	return &TracerProvider{
		config: cfg,
	}, nil
}

// Shutdown gracefully shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.shutdown {
		return nil
	}
	tp.shutdown = true

	// TODO: Implement actual shutdown
	_ = ctx
	return nil
}

// Tracer returns a new tracer from this provider.
func (tp *TracerProvider) Tracer(name string) *Tracer {
	return &Tracer{
		provider: tp,
		name:     name,
	}
}

// Tracer implements the telemetry.Tracer interface using OpenTelemetry.
type Tracer struct {
	provider *TracerProvider
	name     string
}

// NewTracer creates a tracer from the global provider.
// Use TracerProvider.Tracer() for explicit provider control.
func NewTracer(name string) *Tracer {
	return &Tracer{
		name: name,
	}
}

// StartSpan starts a new span and returns a new context containing the span.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...telemetry.SpanOption) (context.Context, telemetry.Span) {
	// Apply options
	cfg := &telemetry.SpanConfig{}
	for _, opt := range opts {
		opt.ApplySpan(cfg)
	}

	// TODO: Create actual OpenTelemetry span
	// 1. Get tracer from provider
	// 2. Start span with options
	// 3. Set initial attributes

	span := &Span{
		name:   name,
		tracer: t,
	}

	return ctx, span
}

// Span implements the telemetry.Span interface using OpenTelemetry.
type Span struct {
	name   string
	tracer *Tracer
}

// End completes the span.
func (s *Span) End() {
	// TODO: End the actual OpenTelemetry span
}

// SetAttributes sets attributes on the span.
func (s *Span) SetAttributes(attrs ...telemetry.Attribute) {
	// TODO: Convert and set attributes on the OpenTelemetry span
	_ = attrs
}

// RecordError records an error on the span.
func (s *Span) RecordError(err error) {
	// TODO: Record error on the OpenTelemetry span
	_ = err
}

// SetStatus sets the span status.
func (s *Span) SetStatus(code telemetry.StatusCode, description string) {
	// TODO: Set status on the OpenTelemetry span
	_ = code
	_ = description
}

// AddEvent adds an event to the span.
func (s *Span) AddEvent(name string, attrs ...telemetry.Attribute) {
	// TODO: Add event to the OpenTelemetry span
	_ = name
	_ = attrs
}

// MeterConfig configures the OpenTelemetry meter provider.
type MeterConfig struct {
	// ServiceName is the name of the service for metrics.
	ServiceName string

	// Endpoint is the OTLP collector endpoint.
	Endpoint string

	// Insecure disables TLS for the exporter connection.
	Insecure bool

	// ExportInterval is how often to export metrics.
	ExportInterval int // seconds

	// ExporterType specifies the exporter ("otlp", "stdout", "none").
	ExporterType string
}

// MeterProvider wraps the OpenTelemetry meter provider.
type MeterProvider struct {
	config MeterConfig
}

// NewMeterProvider creates a new OpenTelemetry meter provider.
func NewMeterProvider(cfg MeterConfig) (*MeterProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "agent-go"
	}
	if cfg.ExporterType == "" {
		cfg.ExporterType = "otlp"
	}
	if cfg.ExportInterval == 0 {
		cfg.ExportInterval = 60
	}

	// TODO: Initialize actual OpenTelemetry meter provider

	return &MeterProvider{
		config: cfg,
	}, nil
}

// Shutdown gracefully shuts down the meter provider.
func (mp *MeterProvider) Shutdown(ctx context.Context) error {
	// TODO: Implement actual shutdown
	_ = ctx
	return nil
}

// Meter returns a new meter from this provider.
func (mp *MeterProvider) Meter(name string) *Meter {
	return &Meter{
		provider: mp,
		name:     name,
	}
}

// Meter implements the telemetry.Meter interface using OpenTelemetry.
type Meter struct {
	provider *MeterProvider
	name     string
}

// NewMeter creates a meter from the global provider.
func NewMeter(name string) *Meter {
	return &Meter{
		name: name,
	}
}

// Counter creates a new counter metric.
func (m *Meter) Counter(name string, opts ...telemetry.MetricOption) telemetry.Counter {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	// TODO: Create actual OpenTelemetry counter
	return &Counter{name: name, meter: m}
}

// Histogram creates a new histogram metric.
func (m *Meter) Histogram(name string, opts ...telemetry.MetricOption) telemetry.Histogram {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	// TODO: Create actual OpenTelemetry histogram
	return &Histogram{name: name, meter: m}
}

// Gauge creates a new gauge metric.
func (m *Meter) Gauge(name string, opts ...telemetry.MetricOption) telemetry.Gauge {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	// TODO: Create actual OpenTelemetry gauge
	return &Gauge{name: name, meter: m}
}

// Counter implements telemetry.Counter using OpenTelemetry.
type Counter struct {
	name  string
	meter *Meter
}

// Add adds a value to the counter.
func (c *Counter) Add(ctx context.Context, value int64, attrs ...telemetry.Attribute) {
	// TODO: Record value to OpenTelemetry counter
	_ = ctx
	_ = value
	_ = attrs
}

// Histogram implements telemetry.Histogram using OpenTelemetry.
type Histogram struct {
	name  string
	meter *Meter
}

// Record records a value to the histogram.
func (h *Histogram) Record(ctx context.Context, value float64, attrs ...telemetry.Attribute) {
	// TODO: Record value to OpenTelemetry histogram
	_ = ctx
	_ = value
	_ = attrs
}

// Gauge implements telemetry.Gauge using OpenTelemetry.
type Gauge struct {
	name  string
	meter *Meter
}

// Record records the current value.
func (g *Gauge) Record(ctx context.Context, value float64, attrs ...telemetry.Attribute) {
	// TODO: Record value to OpenTelemetry gauge
	_ = ctx
	_ = value
	_ = attrs
}

// Predefined span and metric names for agent operations.
const (
	// Span names
	SpanRunStart      = "agent.run.start"
	SpanRunComplete   = "agent.run.complete"
	SpanStateTransition = "agent.state.transition"
	SpanToolExecute   = "agent.tool.execute"
	SpanPlannerDecide = "agent.planner.decide"
	SpanApprovalWait  = "agent.approval.wait"

	// Metric names
	MetricRunDuration   = "agent.run.duration"
	MetricRunCount      = "agent.run.count"
	MetricToolCalls     = "agent.tool.calls"
	MetricToolDuration  = "agent.tool.duration"
	MetricToolErrors    = "agent.tool.errors"
	MetricStateChanges  = "agent.state.changes"
	MetricApprovalWait  = "agent.approval.wait_time"
)

// Attribute key constants for consistent labeling.
const (
	AttrRunID     = "agent.run.id"
	AttrGoal      = "agent.run.goal"
	AttrState     = "agent.state"
	AttrToolName  = "agent.tool.name"
	AttrDecision  = "agent.decision.type"
	AttrError     = "agent.error"
	AttrApproved  = "agent.approval.approved"
)

// Ensure interfaces are satisfied.
var (
	_ telemetry.Tracer    = (*Tracer)(nil)
	_ telemetry.Span      = (*Span)(nil)
	_ telemetry.Meter     = (*Meter)(nil)
	_ telemetry.Counter   = (*Counter)(nil)
	_ telemetry.Histogram = (*Histogram)(nil)
	_ telemetry.Gauge     = (*Gauge)(nil)
)
