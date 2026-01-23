package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockProvider is a mock monitoring provider for testing.
type MockProvider struct {
	name string

	// EmitMetricFunc is called when EmitMetric is invoked.
	EmitMetricFunc func(ctx context.Context, req EmitMetricRequest) (EmitMetricResponse, error)

	// CreateAlertFunc is called when CreateAlert is invoked.
	CreateAlertFunc func(ctx context.Context, req CreateAlertRequest) (CreateAlertResponse, error)

	// QueryMetricsFunc is called when QueryMetrics is invoked.
	QueryMetricsFunc func(ctx context.Context, req QueryMetricsRequest) (QueryMetricsResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool

	// Internal state
	mu          sync.RWMutex
	metrics     []storedMetric
	alerts      map[string]storedAlert
	alertCount  int
	metricCount int
}

type storedMetric struct {
	name      string
	value     float64
	metricType MetricType
	tags      map[string]string
	timestamp time.Time
}

type storedAlert struct {
	id          string
	name        string
	description string
	condition   string
	severity    AlertSeverity
	enabled     bool
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	p := &MockProvider{
		name:    name,
		metrics: make([]storedMetric, 0),
		alerts:  make(map[string]storedAlert),
	}

	p.EmitMetricFunc = p.defaultEmitMetric
	p.CreateAlertFunc = p.defaultCreateAlert
	p.QueryMetricsFunc = p.defaultQueryMetrics
	p.AvailableFunc = func(_ context.Context) bool { return true }

	return p
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// EmitMetric emits a metric.
func (p *MockProvider) EmitMetric(ctx context.Context, req EmitMetricRequest) (EmitMetricResponse, error) {
	return p.EmitMetricFunc(ctx, req)
}

// CreateAlert creates an alert.
func (p *MockProvider) CreateAlert(ctx context.Context, req CreateAlertRequest) (CreateAlertResponse, error) {
	return p.CreateAlertFunc(ctx, req)
}

// QueryMetrics queries metrics.
func (p *MockProvider) QueryMetrics(ctx context.Context, req QueryMetricsRequest) (QueryMetricsResponse, error) {
	return p.QueryMetricsFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

func (p *MockProvider) defaultEmitMetric(_ context.Context, req EmitMetricRequest) (EmitMetricResponse, error) {
	if req.Name == "" {
		return EmitMetricResponse{}, ErrInvalidInput
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	timestamp := req.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	metricType := req.Type
	if metricType == "" {
		metricType = MetricTypeGauge
	}

	p.metricCount++
	p.metrics = append(p.metrics, storedMetric{
		name:       req.Name,
		value:      req.Value,
		metricType: metricType,
		tags:       req.Tags,
		timestamp:  timestamp,
	})

	return EmitMetricResponse{
		Success:   true,
		MetricID:  fmt.Sprintf("metric-%d", p.metricCount),
		Timestamp: timestamp,
	}, nil
}

func (p *MockProvider) defaultCreateAlert(_ context.Context, req CreateAlertRequest) (CreateAlertResponse, error) {
	if req.Name == "" || req.Condition == "" {
		return CreateAlertResponse{}, ErrInvalidInput
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if alert already exists
	var alertID string
	created := true
	for id, alert := range p.alerts {
		if alert.name == req.Name {
			alertID = id
			created = false
			break
		}
	}

	if alertID == "" {
		p.alertCount++
		alertID = fmt.Sprintf("alert-%d", p.alertCount)
	}

	severity := req.Severity
	if severity == "" {
		severity = AlertSeverityWarning
	}

	p.alerts[alertID] = storedAlert{
		id:          alertID,
		name:        req.Name,
		description: req.Description,
		condition:   req.Condition,
		severity:    severity,
		enabled:     req.Enabled,
	}

	return CreateAlertResponse{
		AlertID: alertID,
		Created: created,
		Name:    req.Name,
	}, nil
}

func (p *MockProvider) defaultQueryMetrics(_ context.Context, req QueryMetricsRequest) (QueryMetricsResponse, error) {
	if req.Query == "" {
		return QueryMetricsResponse{}, ErrInvalidInput
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	startTime := time.Now()

	// Filter metrics by time range
	var matchingMetrics []storedMetric
	for _, m := range p.metrics {
		if !req.StartTime.IsZero() && m.timestamp.Before(req.StartTime) {
			continue
		}
		if !req.EndTime.IsZero() && m.timestamp.After(req.EndTime) {
			continue
		}
		matchingMetrics = append(matchingMetrics, m)
	}

	// Group by metric name
	seriesByName := make(map[string][]DataPoint)
	tagsByName := make(map[string]map[string]string)
	for _, m := range matchingMetrics {
		seriesByName[m.name] = append(seriesByName[m.name], DataPoint{
			Timestamp: m.timestamp,
			Value:     m.value,
		})
		if tagsByName[m.name] == nil {
			tagsByName[m.name] = m.tags
		}
	}

	// Convert to response
	var series []TimeSeries
	for name, dataPoints := range seriesByName {
		series = append(series, TimeSeries{
			Name:       name,
			Tags:       tagsByName[name],
			DataPoints: dataPoints,
		})
	}

	// Apply limit
	if req.Limit > 0 && len(series) > req.Limit {
		series = series[:req.Limit]
	}

	return QueryMetricsResponse{
		Series:        series,
		QueryDuration: time.Since(startTime),
	}, nil
}

// MetricCount returns the number of emitted metrics (for testing).
func (p *MockProvider) MetricCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.metrics)
}

// AlertCount returns the number of created alerts (for testing).
func (p *MockProvider) AlertCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.alerts)
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
