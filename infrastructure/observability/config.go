// Package observability provides OpenTelemetry integration for tracing and metrics.
package observability

import (
	"time"
)

// Config configures the observability infrastructure.
type Config struct {
	// ServiceName is the name of the service for telemetry.
	ServiceName string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// Environment is the deployment environment (e.g., "production", "staging").
	Environment string

	// Tracing configures distributed tracing.
	Tracing TracingConfig

	// Metrics configures metrics collection.
	Metrics MetricsConfig
}

// TracingConfig configures distributed tracing.
type TracingConfig struct {
	// Enabled enables tracing (default: false).
	Enabled bool

	// Exporter specifies the trace exporter type.
	Exporter ExporterType

	// Endpoint is the OTLP endpoint (e.g., "localhost:4317").
	Endpoint string

	// Insecure disables TLS for the exporter connection.
	Insecure bool

	// SampleRate is the sampling rate (0.0-1.0, default: 1.0).
	SampleRate float64

	// BatchTimeout is the batch export timeout.
	BatchTimeout time.Duration

	// MaxExportBatchSize is the maximum batch size.
	MaxExportBatchSize int
}

// MetricsConfig configures metrics collection.
type MetricsConfig struct {
	// Enabled enables metrics (default: false).
	Enabled bool

	// Exporter specifies the metrics exporter type.
	Exporter ExporterType

	// Endpoint is the OTLP endpoint (e.g., "localhost:4317").
	Endpoint string

	// Insecure disables TLS for the exporter connection.
	Insecure bool

	// ExportInterval is the metrics export interval.
	ExportInterval time.Duration
}

// ExporterType specifies the telemetry exporter.
type ExporterType string

const (
	// ExporterOTLP exports to OTLP endpoint (e.g., Jaeger, Tempo, Grafana).
	ExporterOTLP ExporterType = "otlp"

	// ExporterStdout exports to stdout (useful for development).
	ExporterStdout ExporterType = "stdout"

	// ExporterNoop disables export (no-op).
	ExporterNoop ExporterType = "noop"
)

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		ServiceName:    "agent-go",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Tracing: TracingConfig{
			Enabled:            false,
			Exporter:           ExporterNoop,
			SampleRate:         1.0,
			BatchTimeout:       5 * time.Second,
			MaxExportBatchSize: 512,
		},
		Metrics: MetricsConfig{
			Enabled:        false,
			Exporter:       ExporterNoop,
			ExportInterval: 60 * time.Second,
		},
	}
}

// Option configures the observability infrastructure.
type Option func(*Config)

// WithServiceName sets the service name.
func WithServiceName(name string) Option {
	return func(c *Config) {
		c.ServiceName = name
	}
}

// WithServiceVersion sets the service version.
func WithServiceVersion(version string) Option {
	return func(c *Config) {
		c.ServiceVersion = version
	}
}

// WithEnvironment sets the environment.
func WithEnvironment(env string) Option {
	return func(c *Config) {
		c.Environment = env
	}
}

// WithTracing enables tracing with the specified exporter.
func WithTracing(exporter ExporterType, endpoint string) Option {
	return func(c *Config) {
		c.Tracing.Enabled = true
		c.Tracing.Exporter = exporter
		c.Tracing.Endpoint = endpoint
	}
}

// WithTracingInsecure disables TLS for tracing.
func WithTracingInsecure() Option {
	return func(c *Config) {
		c.Tracing.Insecure = true
	}
}

// WithSampleRate sets the trace sampling rate.
func WithSampleRate(rate float64) Option {
	return func(c *Config) {
		c.Tracing.SampleRate = rate
	}
}

// WithMetrics enables metrics with the specified exporter.
func WithMetrics(exporter ExporterType, endpoint string) Option {
	return func(c *Config) {
		c.Metrics.Enabled = true
		c.Metrics.Exporter = exporter
		c.Metrics.Endpoint = endpoint
	}
}

// WithMetricsInsecure disables TLS for metrics.
func WithMetricsInsecure() Option {
	return func(c *Config) {
		c.Metrics.Insecure = true
	}
}

// WithMetricsInterval sets the metrics export interval.
func WithMetricsInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.Metrics.ExportInterval = interval
	}
}

// WithStdoutTracing enables stdout tracing (for development).
func WithStdoutTracing() Option {
	return func(c *Config) {
		c.Tracing.Enabled = true
		c.Tracing.Exporter = ExporterStdout
	}
}

// WithStdoutMetrics enables stdout metrics (for development).
func WithStdoutMetrics() Option {
	return func(c *Config) {
		c.Metrics.Enabled = true
		c.Metrics.Exporter = ExporterStdout
	}
}

// WithOTLP enables OTLP export for both tracing and metrics.
func WithOTLP(endpoint string) Option {
	return func(c *Config) {
		c.Tracing.Enabled = true
		c.Tracing.Exporter = ExporterOTLP
		c.Tracing.Endpoint = endpoint
		c.Metrics.Enabled = true
		c.Metrics.Exporter = ExporterOTLP
		c.Metrics.Endpoint = endpoint
	}
}

// WithNoopTracing enables tracing with a no-op exporter.
func WithNoopTracing() Option {
	return func(c *Config) {
		c.Tracing.Enabled = true
		c.Tracing.Exporter = ExporterNoop
	}
}

// WithNoopMetrics enables metrics with a no-op exporter.
func WithNoopMetrics() Option {
	return func(c *Config) {
		c.Metrics.Enabled = true
		c.Metrics.Exporter = ExporterNoop
	}
}
