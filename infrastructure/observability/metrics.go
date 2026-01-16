package observability

import (
	"context"

	"github.com/felixgeelhaar/agent-go/domain/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelMeter wraps an OpenTelemetry meter.
type OTelMeter struct {
	meter metric.Meter
}

// NewOTelMeter creates a new OpenTelemetry meter.
func NewOTelMeter(name string) *OTelMeter {
	return &OTelMeter{
		meter: otel.Meter(name),
	}
}

// Counter implements telemetry.Meter.
func (m *OTelMeter) Counter(name string, opts ...telemetry.MetricOption) telemetry.Counter {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	otelOpts := make([]metric.Int64CounterOption, 0, 2)
	if cfg.Description != "" {
		otelOpts = append(otelOpts, metric.WithDescription(cfg.Description))
	}
	if cfg.Unit != "" {
		otelOpts = append(otelOpts, metric.WithUnit(cfg.Unit))
	}

	counter, err := m.meter.Int64Counter(name, otelOpts...)
	if err != nil {
		// Return noop counter on error
		return &noopCounter{}
	}
	return &otelCounter{counter: counter}
}

// Histogram implements telemetry.Meter.
func (m *OTelMeter) Histogram(name string, opts ...telemetry.MetricOption) telemetry.Histogram {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	otelOpts := make([]metric.Float64HistogramOption, 0, 2)
	if cfg.Description != "" {
		otelOpts = append(otelOpts, metric.WithDescription(cfg.Description))
	}
	if cfg.Unit != "" {
		otelOpts = append(otelOpts, metric.WithUnit(cfg.Unit))
	}

	histogram, err := m.meter.Float64Histogram(name, otelOpts...)
	if err != nil {
		return &noopHistogram{}
	}
	return &otelHistogram{histogram: histogram}
}

// Gauge implements telemetry.Meter.
func (m *OTelMeter) Gauge(name string, opts ...telemetry.MetricOption) telemetry.Gauge {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	otelOpts := make([]metric.Float64GaugeOption, 0, 2)
	if cfg.Description != "" {
		otelOpts = append(otelOpts, metric.WithDescription(cfg.Description))
	}
	if cfg.Unit != "" {
		otelOpts = append(otelOpts, metric.WithUnit(cfg.Unit))
	}

	gauge, err := m.meter.Float64Gauge(name, otelOpts...)
	if err != nil {
		return &noopGauge{}
	}
	return &otelGauge{gauge: gauge}
}

var _ telemetry.Meter = (*OTelMeter)(nil)

// otelCounter wraps an OpenTelemetry counter.
type otelCounter struct {
	counter metric.Int64Counter
}

// Add implements telemetry.Counter.
func (c *otelCounter) Add(ctx context.Context, value int64, attrs ...telemetry.Attribute) {
	c.counter.Add(ctx, value, metric.WithAttributes(convertMetricAttributes(attrs)...))
}

var _ telemetry.Counter = (*otelCounter)(nil)

// otelHistogram wraps an OpenTelemetry histogram.
type otelHistogram struct {
	histogram metric.Float64Histogram
}

// Record implements telemetry.Histogram.
func (h *otelHistogram) Record(ctx context.Context, value float64, attrs ...telemetry.Attribute) {
	h.histogram.Record(ctx, value, metric.WithAttributes(convertMetricAttributes(attrs)...))
}

var _ telemetry.Histogram = (*otelHistogram)(nil)

// otelGauge wraps an OpenTelemetry gauge.
type otelGauge struct {
	gauge metric.Float64Gauge
}

// Record implements telemetry.Gauge.
func (g *otelGauge) Record(ctx context.Context, value float64, attrs ...telemetry.Attribute) {
	g.gauge.Record(ctx, value, metric.WithAttributes(convertMetricAttributes(attrs)...))
}

var _ telemetry.Gauge = (*otelGauge)(nil)

// convertMetricAttributes converts domain attributes to OTel attributes.
func convertMetricAttributes(attrs []telemetry.Attribute) []attribute.KeyValue {
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
