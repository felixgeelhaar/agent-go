// Package telemetry provides observability interfaces for tracing and metrics.
package telemetry

import (
	"context"
)

// Tracer creates spans for distributed tracing.
type Tracer interface {
	// StartSpan starts a new span and returns a new context containing the span.
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
}

// Span represents a unit of work in a trace.
type Span interface {
	// End completes the span.
	End()

	// SetAttributes sets attributes on the span.
	SetAttributes(attrs ...Attribute)

	// RecordError records an error on the span.
	RecordError(err error)

	// SetStatus sets the span status.
	SetStatus(code StatusCode, description string)

	// AddEvent adds an event to the span.
	AddEvent(name string, attrs ...Attribute)
}

// SpanOption configures a span.
type SpanOption interface {
	ApplySpan(*SpanConfig)
}

// SpanConfig holds span configuration.
type SpanConfig struct {
	Attributes []Attribute
	Kind       SpanKind
}

// WithAttributes sets span attributes at creation.
func WithAttributes(attrs ...Attribute) SpanOption {
	return SpanOptionFunc(func(c *SpanConfig) {
		c.Attributes = append(c.Attributes, attrs...)
	})
}

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return SpanOptionFunc(func(c *SpanConfig) {
		c.Kind = kind
	})
}

// SpanOptionFunc is a function that implements SpanOption.
type SpanOptionFunc func(*SpanConfig)

// ApplySpan implements SpanOption.
func (f SpanOptionFunc) ApplySpan(c *SpanConfig) { f(c) }

// SpanKind represents the role of a span.
type SpanKind int

const (
	SpanKindUnspecified SpanKind = iota
	SpanKindInternal
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// StatusCode represents the status of a span.
type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOK
	StatusCodeError
)

// Attribute represents a key-value pair.
type Attribute struct {
	Key   string
	Value any
}

// String creates a string attribute.
func String(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int creates an integer attribute.
func Int(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int64 creates an int64 attribute.
func Int64(key string, value int64) Attribute {
	return Attribute{Key: key, Value: value}
}

// Float64 creates a float64 attribute.
func Float64(key string, value float64) Attribute {
	return Attribute{Key: key, Value: value}
}

// Bool creates a boolean attribute.
func Bool(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}

// Meter creates metrics instruments.
type Meter interface {
	// Counter creates a new counter.
	Counter(name string, opts ...MetricOption) Counter

	// Histogram creates a new histogram.
	Histogram(name string, opts ...MetricOption) Histogram

	// Gauge creates a new gauge.
	Gauge(name string, opts ...MetricOption) Gauge
}

// Counter is a monotonically increasing value.
type Counter interface {
	// Add adds a value to the counter.
	Add(ctx context.Context, value int64, attrs ...Attribute)
}

// Histogram records a distribution of values.
type Histogram interface {
	// Record records a value.
	Record(ctx context.Context, value float64, attrs ...Attribute)
}

// Gauge records a current value.
type Gauge interface {
	// Record records the current value.
	Record(ctx context.Context, value float64, attrs ...Attribute)
}

// MetricOption configures a metric.
type MetricOption interface {
	ApplyMetric(*MetricConfig)
}

// MetricConfig holds metric configuration.
type MetricConfig struct {
	Description string
	Unit        string
}

// WithDescription sets the metric description.
func WithDescription(desc string) MetricOption {
	return MetricOptionFunc(func(c *MetricConfig) {
		c.Description = desc
	})
}

// WithUnit sets the metric unit.
func WithUnit(unit string) MetricOption {
	return MetricOptionFunc(func(c *MetricConfig) {
		c.Unit = unit
	})
}

// MetricOptionFunc is a function that implements MetricOption.
type MetricOptionFunc func(*MetricConfig)

// ApplyMetric implements MetricOption.
func (f MetricOptionFunc) ApplyMetric(c *MetricConfig) { f(c) }
