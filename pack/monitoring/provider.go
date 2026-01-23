// Package monitoring provides tools for metrics and alerting.
package monitoring

import (
	"context"
	"errors"
	"time"
)

// Common errors for monitoring operations.
var (
	ErrProviderNotConfigured = errors.New("monitoring provider not configured")
	ErrInvalidInput          = errors.New("invalid input")
	ErrMetricNotFound        = errors.New("metric not found")
	ErrAlertNotFound         = errors.New("alert not found")
	ErrProviderUnavailable   = errors.New("provider unavailable")
)

// Provider defines the interface for monitoring providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string

	// EmitMetric emits a metric data point.
	EmitMetric(ctx context.Context, req EmitMetricRequest) (EmitMetricResponse, error)

	// CreateAlert creates or updates an alert rule.
	CreateAlert(ctx context.Context, req CreateAlertRequest) (CreateAlertResponse, error)

	// QueryMetrics queries metric data.
	QueryMetrics(ctx context.Context, req QueryMetricsRequest) (QueryMetricsResponse, error)

	// Available checks if the provider is available.
	Available(ctx context.Context) bool
}

// MetricType defines the type of metric.
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// EmitMetricRequest represents a request to emit a metric.
type EmitMetricRequest struct {
	// Name is the metric name.
	Name string `json:"name"`

	// Value is the metric value.
	Value float64 `json:"value"`

	// Type is the metric type.
	Type MetricType `json:"type,omitempty"`

	// Tags are key-value labels.
	Tags map[string]string `json:"tags,omitempty"`

	// Timestamp is the metric timestamp (defaults to now).
	Timestamp time.Time `json:"timestamp,omitempty"`

	// Unit is the metric unit (e.g., "seconds", "bytes").
	Unit string `json:"unit,omitempty"`

	// Description is a human-readable description.
	Description string `json:"description,omitempty"`
}

// EmitMetricResponse represents the result of emitting a metric.
type EmitMetricResponse struct {
	// Success indicates if the metric was emitted.
	Success bool `json:"success"`

	// MetricID is an optional identifier for the emitted metric.
	MetricID string `json:"metric_id,omitempty"`

	// Timestamp is when the metric was recorded.
	Timestamp time.Time `json:"timestamp"`
}

// AlertSeverity defines alert severity levels.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

// CreateAlertRequest represents a request to create an alert.
type CreateAlertRequest struct {
	// Name is the alert name.
	Name string `json:"name"`

	// Description describes the alert.
	Description string `json:"description,omitempty"`

	// Condition is the alert condition (provider-specific format).
	Condition string `json:"condition"`

	// Severity is the alert severity.
	Severity AlertSeverity `json:"severity,omitempty"`

	// Tags are alert labels.
	Tags map[string]string `json:"tags,omitempty"`

	// NotificationChannels are where to send alerts.
	NotificationChannels []string `json:"notification_channels,omitempty"`

	// EvaluationInterval is how often to check the condition.
	EvaluationInterval time.Duration `json:"evaluation_interval,omitempty"`

	// Enabled indicates if the alert is enabled.
	Enabled bool `json:"enabled"`
}

// CreateAlertResponse represents the result of creating an alert.
type CreateAlertResponse struct {
	// AlertID is the created alert identifier.
	AlertID string `json:"alert_id"`

	// Created indicates if the alert was created (vs updated).
	Created bool `json:"created"`

	// Name is the alert name.
	Name string `json:"name"`
}

// QueryMetricsRequest represents a request to query metrics.
type QueryMetricsRequest struct {
	// Query is the query string (provider-specific format).
	Query string `json:"query"`

	// StartTime is the query start time.
	StartTime time.Time `json:"start_time"`

	// EndTime is the query end time.
	EndTime time.Time `json:"end_time"`

	// Step is the query resolution (for time series).
	Step time.Duration `json:"step,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`
}

// QueryMetricsResponse represents the result of a metrics query.
type QueryMetricsResponse struct {
	// Series are the returned time series.
	Series []TimeSeries `json:"series"`

	// QueryDuration is how long the query took.
	QueryDuration time.Duration `json:"query_duration"`
}

// TimeSeries represents a time series of metric data.
type TimeSeries struct {
	// Name is the metric name.
	Name string `json:"name"`

	// Tags are the metric labels.
	Tags map[string]string `json:"tags,omitempty"`

	// DataPoints are the time series values.
	DataPoints []DataPoint `json:"data_points"`
}

// DataPoint represents a single metric data point.
type DataPoint struct {
	// Timestamp is when the data point was recorded.
	Timestamp time.Time `json:"timestamp"`

	// Value is the metric value.
	Value float64 `json:"value"`
}
