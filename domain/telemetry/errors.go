package telemetry

import "errors"

var (
	// ErrTracerNotConfigured indicates the tracer is not configured.
	ErrTracerNotConfigured = errors.New("tracer not configured")

	// ErrMeterNotConfigured indicates the meter is not configured.
	ErrMeterNotConfigured = errors.New("meter not configured")

	// ErrExporterFailed indicates the exporter failed.
	ErrExporterFailed = errors.New("exporter failed")

	// ErrShutdownFailed indicates shutdown failed.
	ErrShutdownFailed = errors.New("shutdown failed")
)
