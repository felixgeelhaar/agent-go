package observability

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OTelTracer wraps an OpenTelemetry tracer.
type OTelTracer struct {
	tracer trace.Tracer
}

// NewOTelTracer creates a new OpenTelemetry tracer.
func NewOTelTracer(name string) *OTelTracer {
	return &OTelTracer{
		tracer: otel.Tracer(name),
	}
}

// StartSpan implements telemetry.Tracer.
func (t *OTelTracer) StartSpan(ctx context.Context, name string, opts ...telemetry.SpanOption) (context.Context, telemetry.Span) {
	// Apply options
	cfg := &telemetry.SpanConfig{}
	for _, opt := range opts {
		opt.ApplySpan(cfg)
	}

	// Convert to OTel options
	otelOpts := make([]trace.SpanStartOption, 0, len(cfg.Attributes)+1)
	if len(cfg.Attributes) > 0 {
		otelOpts = append(otelOpts, trace.WithAttributes(convertAttributes(cfg.Attributes)...))
	}
	if cfg.Kind != telemetry.SpanKindUnspecified {
		otelOpts = append(otelOpts, trace.WithSpanKind(convertSpanKind(cfg.Kind)))
	}

	ctx, span := t.tracer.Start(ctx, name, otelOpts...)
	return ctx, &otelSpan{span: span}
}

var _ telemetry.Tracer = (*OTelTracer)(nil)

// otelSpan wraps an OpenTelemetry span.
type otelSpan struct {
	span trace.Span
}

// End implements telemetry.Span.
func (s *otelSpan) End() {
	s.span.End()
}

// SetAttributes implements telemetry.Span.
func (s *otelSpan) SetAttributes(attrs ...telemetry.Attribute) {
	s.span.SetAttributes(convertAttributes(attrs)...)
}

// RecordError implements telemetry.Span.
func (s *otelSpan) RecordError(err error) {
	s.span.RecordError(err)
}

// SetStatus implements telemetry.Span.
func (s *otelSpan) SetStatus(code telemetry.StatusCode, description string) {
	s.span.SetStatus(convertStatusCode(code), description)
}

// AddEvent implements telemetry.Span.
func (s *otelSpan) AddEvent(name string, attrs ...telemetry.Attribute) {
	s.span.AddEvent(name, trace.WithAttributes(convertAttributes(attrs)...))
}

var _ telemetry.Span = (*otelSpan)(nil)

// convertAttributes converts domain attributes to OTel attributes.
func convertAttributes(attrs []telemetry.Attribute) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		switch v := attr.Value.(type) {
		case string:
			result = append(result, attribute.String(attr.Key, v))
		case int:
			result = append(result, attribute.Int(attr.Key, v))
		case int64:
			result = append(result, attribute.Int64(attr.Key, v))
		case float64:
			result = append(result, attribute.Float64(attr.Key, v))
		case bool:
			result = append(result, attribute.Bool(attr.Key, v))
		}
	}
	return result
}

// convertSpanKind converts domain span kind to OTel span kind.
func convertSpanKind(kind telemetry.SpanKind) trace.SpanKind {
	switch kind {
	case telemetry.SpanKindInternal:
		return trace.SpanKindInternal
	case telemetry.SpanKindServer:
		return trace.SpanKindServer
	case telemetry.SpanKindClient:
		return trace.SpanKindClient
	case telemetry.SpanKindProducer:
		return trace.SpanKindProducer
	case telemetry.SpanKindConsumer:
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindUnspecified
	}
}

// convertStatusCode converts domain status code to OTel status code.
func convertStatusCode(code telemetry.StatusCode) codes.Code {
	switch code {
	case telemetry.StatusCodeOK:
		return codes.Ok
	case telemetry.StatusCodeError:
		return codes.Error
	default:
		return codes.Unset
	}
}

// SpanFromContext extracts the span from context.
func SpanFromContext(ctx context.Context) telemetry.Span {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return &noopSpan{}
	}
	return &otelSpan{span: span}
}
