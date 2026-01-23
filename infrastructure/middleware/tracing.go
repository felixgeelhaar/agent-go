package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// TracingConfig configures the tracing middleware.
type TracingConfig struct {
	// TracerName is the name of the tracer to use.
	TracerName string

	// Tracer is a custom tracer to use. If nil, a new tracer is created.
	Tracer trace.Tracer

	// RecordInput determines if tool input should be recorded as span attributes.
	RecordInput bool

	// RecordOutput determines if tool output should be recorded as span attributes.
	RecordOutput bool

	// MaxAttributeSize limits the size of recorded attributes.
	MaxAttributeSize int

	// SpanNamePrefix is prepended to span names.
	SpanNamePrefix string

	// AdditionalAttributes are added to all spans.
	AdditionalAttributes []attribute.KeyValue
}

// DefaultTracingConfig returns a sensible default configuration.
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		TracerName:       "agent-go",
		RecordInput:      true,
		RecordOutput:     false, // Output can be large
		MaxAttributeSize: 1024,
		SpanNamePrefix:   "tool.",
	}
}

// Tracing returns middleware that creates OpenTelemetry spans for tool executions.
func Tracing(cfg TracingConfig) middleware.Middleware {
	// Get or create tracer
	tracer := cfg.Tracer
	if tracer == nil {
		tracerName := cfg.TracerName
		if tracerName == "" {
			tracerName = "agent-go"
		}
		tracer = otel.Tracer(tracerName)
	}

	maxSize := cfg.MaxAttributeSize
	if maxSize <= 0 {
		maxSize = 1024
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Build span name
			spanName := execCtx.Tool.Name()
			if cfg.SpanNamePrefix != "" {
				spanName = cfg.SpanNamePrefix + spanName
			}

			// Build span options
			opts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindInternal),
			}

			// Start span
			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer span.End()

			// Set base attributes
			attrs := []attribute.KeyValue{
				attribute.String("agent.run_id", execCtx.RunID),
				attribute.String("agent.state", string(execCtx.CurrentState)),
				attribute.String("tool.name", execCtx.Tool.Name()),
				attribute.String("tool.description", execCtx.Tool.Description()),
			}

			// Add tool annotations
			annotations := execCtx.Tool.Annotations()
			attrs = append(attrs,
				attribute.Bool("tool.read_only", annotations.ReadOnly),
				attribute.Bool("tool.destructive", annotations.Destructive),
				attribute.Bool("tool.idempotent", annotations.Idempotent),
				attribute.Bool("tool.cacheable", annotations.Cacheable),
				attribute.Int("tool.risk_level", int(annotations.RiskLevel)),
			)

			// Add reason if present
			if execCtx.Reason != "" {
				attrs = append(attrs, attribute.String("tool.reason", truncate(execCtx.Reason, maxSize)))
			}

			// Record input if enabled
			if cfg.RecordInput && len(execCtx.Input) > 0 {
				inputStr := string(execCtx.Input)
				attrs = append(attrs, attribute.String("tool.input", truncate(inputStr, maxSize)))
			}

			// Add additional attributes
			attrs = append(attrs, cfg.AdditionalAttributes...)

			span.SetAttributes(attrs...)

			// Execute the handler
			result, err := next(ctx, execCtx)

			// Record result
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetStatus(codes.Ok, "")

				// Record output if enabled
				if cfg.RecordOutput && len(result.Output) > 0 {
					outputStr := string(result.Output)
					span.SetAttributes(attribute.String("tool.output", truncate(outputStr, maxSize)))
				}

				// Record duration
				span.SetAttributes(
					attribute.Int64("tool.duration_ms", result.Duration.Milliseconds()),
					attribute.Bool("tool.cached", result.Cached),
				)

				// Record artifacts count
				if len(result.Artifacts) > 0 {
					span.SetAttributes(attribute.Int("tool.artifacts_count", len(result.Artifacts)))
				}
			}

			return result, err
		}
	}
}

// TracingOption configures the tracing middleware.
type TracingOption func(*TracingConfig)

// WithTracerName sets the tracer name.
func WithTracerName(name string) TracingOption {
	return func(c *TracingConfig) {
		c.TracerName = name
	}
}

// WithTracer sets a custom tracer.
func WithTracer(tracer trace.Tracer) TracingOption {
	return func(c *TracingConfig) {
		c.Tracer = tracer
	}
}

// WithInputRecording enables or disables input recording.
func WithInputRecording(enabled bool) TracingOption {
	return func(c *TracingConfig) {
		c.RecordInput = enabled
	}
}

// WithOutputRecording enables or disables output recording.
func WithOutputRecording(enabled bool) TracingOption {
	return func(c *TracingConfig) {
		c.RecordOutput = enabled
	}
}

// WithMaxAttributeSize sets the maximum attribute size.
func WithMaxAttributeSize(size int) TracingOption {
	return func(c *TracingConfig) {
		c.MaxAttributeSize = size
	}
}

// WithSpanNamePrefix sets the span name prefix.
func WithSpanNamePrefix(prefix string) TracingOption {
	return func(c *TracingConfig) {
		c.SpanNamePrefix = prefix
	}
}

// WithAdditionalAttributes adds extra attributes to all spans.
func WithAdditionalAttributes(attrs ...attribute.KeyValue) TracingOption {
	return func(c *TracingConfig) {
		c.AdditionalAttributes = append(c.AdditionalAttributes, attrs...)
	}
}

// NewTracing creates tracing middleware with the given options.
func NewTracing(opts ...TracingOption) middleware.Middleware {
	cfg := DefaultTracingConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return Tracing(cfg)
}

// ContextWithSpan creates a context with a span for tool execution.
// Useful for creating child spans outside of middleware.
func ContextWithSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	tracer := otel.Tracer("agent-go")
	return tracer.Start(ctx, name)
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanEvent adds an event to the current span.
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// AddSpanAttributes adds attributes to the current span.
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordSpanError records an error on the current span.
func RecordSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// TracingMetricsConfig configures combined tracing and metrics collection.
type TracingMetricsConfig struct {
	TracingConfig

	// CollectHistogram enables latency histogram collection.
	CollectHistogram bool

	// CollectCounter enables call counter collection.
	CollectCounter bool
}

// TracingWithMetrics returns middleware that collects both traces and metrics.
func TracingWithMetrics(cfg TracingMetricsConfig) middleware.Middleware {
	// Start with base tracing
	tracingMiddleware := Tracing(cfg.TracingConfig)

	return func(next middleware.Handler) middleware.Handler {
		// Wrap with tracing first
		handler := tracingMiddleware(next)

		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Execute with tracing
			result, err := handler(ctx, execCtx)

			// Metrics would be collected here if enabled
			// This is a placeholder for actual metrics implementation
			if cfg.CollectHistogram || cfg.CollectCounter {
				span := trace.SpanFromContext(ctx)
				if cfg.CollectHistogram {
					span.AddEvent("metrics.histogram",
						trace.WithAttributes(
							attribute.String("tool.name", execCtx.Tool.Name()),
							attribute.Int64("duration_ms", result.Duration.Milliseconds()),
						),
					)
				}
				if cfg.CollectCounter {
					success := "true"
					if err != nil {
						success = "false"
					}
					span.AddEvent("metrics.counter",
						trace.WithAttributes(
							attribute.String("tool.name", execCtx.Tool.Name()),
							attribute.String("success", success),
						),
					)
				}
			}

			return result, err
		}
	}
}

// ExtractTraceContext extracts trace context from a JSON payload.
// Useful for distributed tracing across agent boundaries.
func ExtractTraceContext(ctx context.Context, data json.RawMessage) context.Context {
	// Parse trace context from data if present
	var payload struct {
		TraceID string `json:"trace_id,omitempty"`
		SpanID  string `json:"span_id,omitempty"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return ctx
	}

	// If trace context is present, create a linked span
	if payload.TraceID != "" && payload.SpanID != "" {
		tracer := otel.Tracer("agent-go")
		_, span := tracer.Start(ctx, "linked-trace",
			trace.WithAttributes(
				attribute.String("linked.trace_id", payload.TraceID),
				attribute.String("linked.span_id", payload.SpanID),
			),
		)
		defer span.End()
	}

	return ctx
}

// InjectTraceContext injects trace context into output data.
// Useful for propagating trace context to downstream agents.
func InjectTraceContext(ctx context.Context, data map[string]interface{}) map[string]interface{} {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return data
	}

	if data == nil {
		data = make(map[string]interface{})
	}

	data["trace_id"] = span.SpanContext().TraceID().String()
	data["span_id"] = span.SpanContext().SpanID().String()

	return data
}

// LoggingTraceDecorator creates a trace-aware logging decorator.
// It adds trace and span IDs to log context.
func LoggingTraceDecorator(ctx context.Context) map[string]string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}

	return map[string]string{
		"trace_id": span.SpanContext().TraceID().String(),
		"span_id":  span.SpanContext().SpanID().String(),
	}
}

// ToolSpanAttributes returns standard attributes for a tool span.
func ToolSpanAttributes(execCtx *middleware.ExecutionContext) []attribute.KeyValue {
	annotations := execCtx.Tool.Annotations()
	return []attribute.KeyValue{
		attribute.String("agent.run_id", execCtx.RunID),
		attribute.String("agent.state", string(execCtx.CurrentState)),
		attribute.String("tool.name", execCtx.Tool.Name()),
		attribute.String("tool.description", execCtx.Tool.Description()),
		attribute.Bool("tool.read_only", annotations.ReadOnly),
		attribute.Bool("tool.destructive", annotations.Destructive),
		attribute.Bool("tool.idempotent", annotations.Idempotent),
		attribute.Int("tool.risk_level", int(annotations.RiskLevel)),
	}
}

// CreateToolSpan creates a new span for tool execution.
func CreateToolSpan(ctx context.Context, execCtx *middleware.ExecutionContext) (context.Context, trace.Span) {
	tracer := otel.Tracer("agent-go")
	spanName := fmt.Sprintf("tool.%s", execCtx.Tool.Name())

	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(ToolSpanAttributes(execCtx)...),
	)

	return ctx, span
}
