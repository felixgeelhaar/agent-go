package observability

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Provider manages the observability infrastructure.
type Provider struct {
	config         Config
	tracerProvider *sdktrace.TracerProvider
	tracer         telemetry.Tracer
	meter          telemetry.Meter
	shutdownFuncs  []func(context.Context) error
}

// New creates a new observability provider.
func New(opts ...Option) (*Provider, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	p := &Provider{
		config:        cfg,
		shutdownFuncs: make([]func(context.Context) error, 0),
	}

	// Setup tracing
	if cfg.Tracing.Enabled {
		if err := p.setupTracing(); err != nil {
			return nil, err
		}
	} else {
		p.tracer = NewNoopTracer()
	}

	// Setup metrics
	if cfg.Metrics.Enabled {
		if err := p.setupMetrics(); err != nil {
			return nil, err
		}
	} else {
		p.meter = NewNoopMeter()
	}

	return p, nil
}

// setupTracing initializes the tracing infrastructure.
func (p *Provider) setupTracing() error {
	ctx := context.Background()

	// Create resource with service attributes
	// We don't merge with Default() to avoid schema URL conflicts
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(p.config.ServiceName),
		semconv.ServiceVersion(p.config.ServiceVersion),
		semconv.DeploymentEnvironment(p.config.Environment),
	)

	// Create exporter based on config
	var exporter sdktrace.SpanExporter

	switch p.config.Tracing.Exporter {
	case ExporterOTLP:
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(p.config.Tracing.Endpoint),
		}
		if p.config.Tracing.Insecure {
			opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exp, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return err
		}
		exporter = exp

	case ExporterStdout:
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return err
		}
		exporter = exp

	case ExporterNoop:
		p.tracer = NewNoopTracer()
		return nil

	default:
		return errors.New("unknown trace exporter type")
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if p.config.Tracing.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if p.config.Tracing.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(p.config.Tracing.SampleRate)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(p.config.Tracing.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(p.config.Tracing.MaxExportBatchSize),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	p.tracerProvider = tp
	p.tracer = NewOTelTracer(p.config.ServiceName)
	p.shutdownFuncs = append(p.shutdownFuncs, tp.Shutdown)

	return nil
}

// setupMetrics initializes the metrics infrastructure.
func (p *Provider) setupMetrics() error {
	// For now, use OTel meter directly
	// Full metric export setup would require metric SDK
	p.meter = NewOTelMeter(p.config.ServiceName)
	return nil
}

// Tracer returns the tracer.
func (p *Provider) Tracer() telemetry.Tracer {
	return p.tracer
}

// Meter returns the meter.
func (p *Provider) Meter() telemetry.Meter {
	return p.meter
}

// Shutdown gracefully shuts down the provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	for _, fn := range p.shutdownFuncs {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// TracingMiddlewareFromProvider creates tracing middleware from a provider.
func TracingMiddlewareFromProvider(p *Provider) middleware.Middleware {
	return TracingMiddleware(p.Tracer())
}

// MetricsMiddlewareFromProvider creates metrics middleware from a provider.
func MetricsMiddlewareFromProvider(p *Provider) middleware.Middleware {
	return MetricsMiddleware(p.Meter())
}

// NewStdoutProvider creates a provider with stdout exporters (for development).
func NewStdoutProvider(serviceName string) (*Provider, error) {
	return New(
		WithServiceName(serviceName),
		WithStdoutTracing(),
	)
}

// NewOTLPProvider creates a provider with OTLP exporters.
func NewOTLPProvider(serviceName, endpoint string) (*Provider, error) {
	return New(
		WithServiceName(serviceName),
		WithOTLP(endpoint),
		WithTracingInsecure(),
		WithMetricsInsecure(),
	)
}

// NewNoopProvider creates a provider with no-op tracer and meter.
func NewNoopProvider() *Provider {
	return &Provider{
		config:        DefaultConfig(),
		tracer:        NewNoopTracer(),
		meter:         NewNoopMeter(),
		shutdownFuncs: nil,
	}
}

// WriteTraceToFile creates a stdout trace exporter that writes to a file.
func WriteTraceToFile(path string) (io.Closer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}
