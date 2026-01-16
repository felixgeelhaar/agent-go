package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/telemetry"
)

func TestNoopTracer(t *testing.T) {
	tracer := NewNoopTracer()

	ctx := context.Background()
	newCtx, span := tracer.StartSpan(ctx, "test-span")

	if newCtx == nil {
		t.Error("expected non-nil context")
	}
	if span == nil {
		t.Error("expected non-nil span")
	}

	// These should not panic
	span.SetAttributes(telemetry.String("key", "value"))
	span.RecordError(errors.New("test error"))
	span.SetStatus(telemetry.StatusCodeOK, "ok")
	span.AddEvent("test-event")
	span.End()
}

func TestNoopMeter(t *testing.T) {
	meter := NewNoopMeter()

	ctx := context.Background()

	// Test counter
	counter := meter.Counter("test_counter",
		telemetry.WithDescription("test counter"),
		telemetry.WithUnit("{count}"),
	)
	if counter == nil {
		t.Error("expected non-nil counter")
	}
	// Should not panic
	counter.Add(ctx, 1)
	counter.Add(ctx, 5, telemetry.String("label", "value"))

	// Test histogram
	histogram := meter.Histogram("test_histogram",
		telemetry.WithDescription("test histogram"),
		telemetry.WithUnit("ms"),
	)
	if histogram == nil {
		t.Error("expected non-nil histogram")
	}
	// Should not panic
	histogram.Record(ctx, 1.5)
	histogram.Record(ctx, 2.5, telemetry.String("label", "value"))

	// Test gauge
	gauge := meter.Gauge("test_gauge",
		telemetry.WithDescription("test gauge"),
		telemetry.WithUnit("{item}"),
	)
	if gauge == nil {
		t.Error("expected non-nil gauge")
	}
	// Should not panic
	gauge.Record(ctx, 10.0)
	gauge.Record(ctx, 20.0, telemetry.String("label", "value"))
}

func TestNoopProvider(t *testing.T) {
	provider := NewNoopProvider()

	if provider.Tracer() == nil {
		t.Error("expected non-nil tracer")
	}
	if provider.Meter() == nil {
		t.Error("expected non-nil meter")
	}

	// Shutdown should not error
	err := provider.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServiceName != "agent-go" {
		t.Errorf("expected default service name, got: %s", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "1.0.0" {
		t.Errorf("expected default version, got: %s", cfg.ServiceVersion)
	}
	if cfg.Environment != "development" {
		t.Errorf("expected development environment, got: %s", cfg.Environment)
	}
	if cfg.Tracing.Enabled {
		t.Error("expected tracing disabled by default")
	}
	if cfg.Metrics.Enabled {
		t.Error("expected metrics disabled by default")
	}
}

func TestConfigOptions(t *testing.T) {
	tests := []struct {
		name   string
		opts   []Option
		verify func(*testing.T, Config)
	}{
		{
			name: "WithServiceName",
			opts: []Option{WithServiceName("my-service")},
			verify: func(t *testing.T, cfg Config) {
				if cfg.ServiceName != "my-service" {
					t.Errorf("expected my-service, got: %s", cfg.ServiceName)
				}
			},
		},
		{
			name: "WithServiceVersion",
			opts: []Option{WithServiceVersion("1.2.3")},
			verify: func(t *testing.T, cfg Config) {
				if cfg.ServiceVersion != "1.2.3" {
					t.Errorf("expected 1.2.3, got: %s", cfg.ServiceVersion)
				}
			},
		},
		{
			name: "WithEnvironment",
			opts: []Option{WithEnvironment("production")},
			verify: func(t *testing.T, cfg Config) {
				if cfg.Environment != "production" {
					t.Errorf("expected production, got: %s", cfg.Environment)
				}
			},
		},
		{
			name: "WithOTLP",
			opts: []Option{WithOTLP("localhost:4317")},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Tracing.Enabled {
					t.Error("expected tracing enabled")
				}
				if cfg.Tracing.Exporter != ExporterOTLP {
					t.Errorf("expected OTLP exporter, got: %s", cfg.Tracing.Exporter)
				}
				if cfg.Tracing.Endpoint != "localhost:4317" {
					t.Errorf("expected localhost:4317, got: %s", cfg.Tracing.Endpoint)
				}
				if !cfg.Metrics.Enabled {
					t.Error("expected metrics enabled")
				}
			},
		},
		{
			name: "WithStdoutTracing",
			opts: []Option{WithStdoutTracing()},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Tracing.Enabled {
					t.Error("expected tracing enabled")
				}
				if cfg.Tracing.Exporter != ExporterStdout {
					t.Errorf("expected stdout exporter, got: %s", cfg.Tracing.Exporter)
				}
			},
		},
		{
			name: "WithSampleRate",
			opts: []Option{WithSampleRate(0.5)},
			verify: func(t *testing.T, cfg Config) {
				if cfg.Tracing.SampleRate != 0.5 {
					t.Errorf("expected 0.5 sample rate, got: %f", cfg.Tracing.SampleRate)
				}
			},
		},
		{
			name: "WithTracingInsecure",
			opts: []Option{WithTracingInsecure()},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Tracing.Insecure {
					t.Error("expected tracing insecure")
				}
			},
		},
		{
			name: "WithMetricsInsecure",
			opts: []Option{WithMetricsInsecure()},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Metrics.Insecure {
					t.Error("expected metrics insecure")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			for _, opt := range tt.opts {
				opt(&cfg)
			}
			tt.verify(t, cfg)
		})
	}
}

func TestProviderWithNoopExporter(t *testing.T) {
	provider, err := New(
		WithServiceName("test-service"),
		WithNoopTracing(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider.Tracer() == nil {
		t.Error("expected non-nil tracer")
	}
	if provider.Meter() == nil {
		t.Error("expected non-nil meter")
	}
}

func TestConvertAttributes(t *testing.T) {
	attrs := []telemetry.Attribute{
		telemetry.String("string_key", "string_value"),
		telemetry.Int("int_key", 42),
		telemetry.Int64("int64_key", int64(123)),
		telemetry.Float64("float64_key", 3.14),
		telemetry.Bool("bool_key", true),
	}

	result := convertAttributes(attrs)

	if len(result) != 5 {
		t.Errorf("expected 5 attributes, got: %d", len(result))
	}
}

func TestConvertMetricAttributes(t *testing.T) {
	attrs := []telemetry.Attribute{
		telemetry.String("string_key", "string_value"),
		telemetry.Int("int_key", 42),
		telemetry.Int64("int64_key", int64(123)),
		telemetry.Float64("float64_key", 3.14),
		telemetry.Bool("bool_key", true),
	}

	result := convertMetricAttributes(attrs)

	if len(result) != 5 {
		t.Errorf("expected 5 attributes, got: %d", len(result))
	}
}

func TestConvertSpanKind(t *testing.T) {
	tests := []struct {
		input  telemetry.SpanKind
		expect string
	}{
		{telemetry.SpanKindInternal, "internal"},
		{telemetry.SpanKindServer, "server"},
		{telemetry.SpanKindClient, "client"},
		{telemetry.SpanKindProducer, "producer"},
		{telemetry.SpanKindConsumer, "consumer"},
		{telemetry.SpanKindUnspecified, "unspecified"},
	}

	for _, tt := range tests {
		result := convertSpanKind(tt.input)
		// Just verify no panic and result is valid
		if result.String() == "" {
			t.Errorf("expected non-empty span kind string for %v", tt.input)
		}
	}
}

func TestConvertStatusCode(t *testing.T) {
	tests := []struct {
		input telemetry.StatusCode
	}{
		{telemetry.StatusCodeUnset},
		{telemetry.StatusCodeOK},
		{telemetry.StatusCodeError},
	}

	for _, tt := range tests {
		result := convertStatusCode(tt.input)
		// Just verify no panic and result is valid
		if result.String() == "" {
			t.Errorf("expected non-empty status code string for %v", tt.input)
		}
	}
}

func TestAgentMetrics(t *testing.T) {
	meter := NewNoopMeter()
	metrics := NewAgentMetrics(meter)

	ctx := context.Background()

	// These should not panic
	metrics.RecordRunStart(ctx, "run-123")
	metrics.RecordRunEnd(ctx, "success", 5*time.Second)
	metrics.RecordDecision(ctx, "call_tool", "explore", 100*time.Millisecond)
	metrics.RecordBudget(ctx, "tool_calls", 50)
}

func TestSpanFromContext(t *testing.T) {
	ctx := context.Background()

	// Should return noop span when no span in context
	span := SpanFromContext(ctx)
	if span == nil {
		t.Error("expected non-nil span")
	}

	// Should not panic
	span.SetAttributes(telemetry.String("key", "value"))
	span.End()
}

func TestOTelTracerStartSpan(t *testing.T) {
	tracer := NewOTelTracer("test-tracer")

	ctx := context.Background()
	newCtx, span := tracer.StartSpan(ctx, "test-span",
		telemetry.WithAttributes(
			telemetry.String("key", "value"),
			telemetry.Int("count", 42),
		),
		telemetry.WithSpanKind(telemetry.SpanKindInternal),
	)

	if newCtx == nil {
		t.Error("expected non-nil context")
	}
	if span == nil {
		t.Error("expected non-nil span")
	}

	// Should not panic
	span.SetAttributes(telemetry.String("another", "attr"))
	span.RecordError(errors.New("test error"))
	span.SetStatus(telemetry.StatusCodeOK, "success")
	span.AddEvent("test-event", telemetry.String("event_key", "event_value"))
	span.End()
}

func TestOTelMeter(t *testing.T) {
	meter := NewOTelMeter("test-meter")

	ctx := context.Background()

	// Test counter
	counter := meter.Counter("test_counter",
		telemetry.WithDescription("A test counter"),
		telemetry.WithUnit("{count}"),
	)
	counter.Add(ctx, 1)
	counter.Add(ctx, 5, telemetry.String("label", "value"))

	// Test histogram
	histogram := meter.Histogram("test_histogram",
		telemetry.WithDescription("A test histogram"),
		telemetry.WithUnit("ms"),
	)
	histogram.Record(ctx, 1.5)
	histogram.Record(ctx, 2.5, telemetry.String("label", "value"))

	// Test gauge
	gauge := meter.Gauge("test_gauge",
		telemetry.WithDescription("A test gauge"),
		telemetry.WithUnit("{item}"),
	)
	gauge.Record(ctx, 10.0)
	gauge.Record(ctx, 20.0, telemetry.String("label", "value"))
}

func TestTracingMiddlewareFromProvider(t *testing.T) {
	provider := NewNoopProvider()

	mw := TracingMiddlewareFromProvider(provider)
	if mw == nil {
		t.Error("expected non-nil middleware")
	}
}

func TestMetricsMiddlewareFromProvider(t *testing.T) {
	provider := NewNoopProvider()

	mw := MetricsMiddlewareFromProvider(provider)
	if mw == nil {
		t.Error("expected non-nil middleware")
	}
}

func TestConfigOptionsAdditional(t *testing.T) {
	tests := []struct {
		name   string
		opts   []Option
		verify func(*testing.T, Config)
	}{
		{
			name: "WithTracing",
			opts: []Option{WithTracing(ExporterOTLP, "localhost:4317")},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Tracing.Enabled {
					t.Error("expected tracing enabled")
				}
				if cfg.Tracing.Exporter != ExporterOTLP {
					t.Errorf("expected OTLP exporter, got: %s", cfg.Tracing.Exporter)
				}
				if cfg.Tracing.Endpoint != "localhost:4317" {
					t.Errorf("expected localhost:4317, got: %s", cfg.Tracing.Endpoint)
				}
			},
		},
		{
			name: "WithMetrics",
			opts: []Option{WithMetrics(ExporterOTLP, "localhost:4317")},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Metrics.Enabled {
					t.Error("expected metrics enabled")
				}
				if cfg.Metrics.Exporter != ExporterOTLP {
					t.Errorf("expected OTLP exporter, got: %s", cfg.Metrics.Exporter)
				}
				if cfg.Metrics.Endpoint != "localhost:4317" {
					t.Errorf("expected localhost:4317, got: %s", cfg.Metrics.Endpoint)
				}
			},
		},
		{
			name: "WithStdoutMetrics",
			opts: []Option{WithStdoutMetrics()},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Metrics.Enabled {
					t.Error("expected metrics enabled")
				}
				if cfg.Metrics.Exporter != ExporterStdout {
					t.Errorf("expected stdout exporter, got: %s", cfg.Metrics.Exporter)
				}
			},
		},
		{
			name: "WithNoopMetrics",
			opts: []Option{WithNoopMetrics()},
			verify: func(t *testing.T, cfg Config) {
				if !cfg.Metrics.Enabled {
					t.Error("expected metrics enabled")
				}
				if cfg.Metrics.Exporter != ExporterNoop {
					t.Errorf("expected noop exporter, got: %s", cfg.Metrics.Exporter)
				}
			},
		},
		{
			name: "WithMetricsInterval",
			opts: []Option{WithMetricsInterval(30 * time.Second)},
			verify: func(t *testing.T, cfg Config) {
				if cfg.Metrics.ExportInterval != 30*time.Second {
					t.Errorf("expected 30s interval, got: %v", cfg.Metrics.ExportInterval)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			for _, opt := range tt.opts {
				opt(&cfg)
			}
			tt.verify(t, cfg)
		})
	}
}

func TestNoopSpanMethods(t *testing.T) {
	tracer := NewNoopTracer()
	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test")

	// Test all span methods individually
	span.End()
	span.SetAttributes(telemetry.String("key", "value"))
	span.RecordError(errors.New("test error"))
	span.SetStatus(telemetry.StatusCodeOK, "ok")
	span.AddEvent("test-event", telemetry.String("attr", "value"))
}

func TestNoopMetricMethods(t *testing.T) {
	meter := NewNoopMeter()
	ctx := context.Background()

	counter := meter.Counter("test_counter")
	counter.Add(ctx, 1)

	histogram := meter.Histogram("test_histogram")
	histogram.Record(ctx, 1.5)

	gauge := meter.Gauge("test_gauge")
	gauge.Record(ctx, 10.0)
}

func TestProviderWithStdoutTracing(t *testing.T) {
	// Note: This creates actual stdout output
	provider, err := New(
		WithServiceName("test-service"),
		WithStdoutTracing(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider.Tracer() == nil {
		t.Error("expected non-nil tracer")
	}
}

func TestCombinedMiddleware(t *testing.T) {
	tracer := NewNoopTracer()
	meter := NewNoopMeter()

	mw := CombinedMiddleware(tracer, meter)
	if mw == nil {
		t.Error("expected non-nil middleware")
	}
}

func TestProviderShutdownWithError(t *testing.T) {
	// Create a provider with a shutdown function that returns an error
	provider := &Provider{
		config:        DefaultConfig(),
		tracer:        NewNoopTracer(),
		meter:         NewNoopMeter(),
		shutdownFuncs: []func(context.Context) error{
			func(ctx context.Context) error {
				return errors.New("shutdown error")
			},
		},
	}

	err := provider.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error from shutdown")
	}
}

func TestProviderShutdownMultipleErrors(t *testing.T) {
	provider := &Provider{
		config:        DefaultConfig(),
		tracer:        NewNoopTracer(),
		meter:         NewNoopMeter(),
		shutdownFuncs: []func(context.Context) error{
			func(ctx context.Context) error {
				return errors.New("error 1")
			},
			func(ctx context.Context) error {
				return errors.New("error 2")
			},
		},
	}

	err := provider.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error from shutdown")
	}
}

func TestNewStdoutProvider(t *testing.T) {
	provider, err := NewStdoutProvider("test-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider.Tracer() == nil {
		t.Error("expected non-nil tracer")
	}
	if provider.Meter() == nil {
		t.Error("expected non-nil meter")
	}
}

func TestProviderWithMetricsEnabled(t *testing.T) {
	provider, err := New(
		WithServiceName("test-service"),
		WithNoopMetrics(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider.Meter() == nil {
		t.Error("expected non-nil meter")
	}
}

func TestProviderTracingUnknownExporter(t *testing.T) {
	provider := &Provider{
		config: Config{
			ServiceName:    "test",
			ServiceVersion: "1.0.0",
			Tracing: TracingConfig{
				Enabled:  true,
				Exporter: ExporterType("invalid"),
			},
		},
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	err := provider.setupTracing()
	if err == nil {
		t.Error("expected error for unknown exporter")
	}
}

func TestProviderTracingSamplers(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"ratio sample", 0.5},
		{"ratio sample high", 1.5},
		{"ratio sample negative", -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(
				WithServiceName("test-service"),
				WithStdoutTracing(),
				WithSampleRate(tt.sampleRate),
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			provider.Shutdown(context.Background())
		})
	}
}

func TestDirectNoopSpan(t *testing.T) {
	span := &noopSpan{}
	span.End()
	span.SetAttributes(telemetry.String("key", "value"))
	span.RecordError(errors.New("test"))
	span.SetStatus(telemetry.StatusCodeOK, "ok")
	span.AddEvent("event")
}

func TestDirectNoopCounter(t *testing.T) {
	counter := &noopCounter{}
	counter.Add(context.Background(), 1)
}

func TestDirectNoopHistogram(t *testing.T) {
	histogram := &noopHistogram{}
	histogram.Record(context.Background(), 1.5)
}

func TestDirectNoopGauge(t *testing.T) {
	gauge := &noopGauge{}
	gauge.Record(context.Background(), 10.0)
}
