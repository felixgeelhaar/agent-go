package monitoring

import (
	"context"
	"encoding/json"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the monitoring pack.
type PackConfig struct {
	// Provider is the monitoring provider to use.
	Provider Provider

	// DefaultMetricType is the default type for metrics.
	DefaultMetricType MetricType

	// DefaultAlertSeverity is the default alert severity.
	DefaultAlertSeverity AlertSeverity

	// DefaultQueryDuration is the default query time range.
	DefaultQueryDuration time.Duration
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{
		DefaultMetricType:    MetricTypeGauge,
		DefaultAlertSeverity: AlertSeverityWarning,
		DefaultQueryDuration: time.Hour,
	}
}

// New creates a new monitoring pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	if cfg.DefaultMetricType == "" {
		cfg.DefaultMetricType = MetricTypeGauge
	}
	if cfg.DefaultAlertSeverity == "" {
		cfg.DefaultAlertSeverity = AlertSeverityWarning
	}
	if cfg.DefaultQueryDuration == 0 {
		cfg.DefaultQueryDuration = time.Hour
	}

	return pack.NewBuilder("monitoring").
		WithDescription("Tools for metrics and alerting").
		WithVersion("1.0.0").
		AddTools(
			emitMetricTool(cfg),
			createAlertTool(cfg),
			queryMetricsTool(cfg),
		).
		AllowInState(agent.StateExplore, "monitoring_query_metrics").
		AllowInState(agent.StateAct, "monitoring_emit_metric", "monitoring_create_alert", "monitoring_query_metrics").
		AllowInState(agent.StateValidate, "monitoring_query_metrics").
		Build()
}

// emitMetricTool creates the monitoring_emit_metric tool.
func emitMetricTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("monitoring_emit_metric").
		WithDescription("Emit a metric data point").
		WithTags("monitoring", "metrics", "observability").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Name        string            `json:"name"`
				Value       float64           `json:"value"`
				Type        string            `json:"type"`
				Tags        map[string]string `json:"tags"`
				Unit        string            `json:"unit"`
				Description string            `json:"description"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Name == "" {
				return tool.Result{}, ErrInvalidInput
			}

			metricType := MetricType(req.Type)
			if metricType == "" {
				metricType = cfg.DefaultMetricType
			}

			resp, err := cfg.Provider.EmitMetric(ctx, EmitMetricRequest{
				Name:        req.Name,
				Value:       req.Value,
				Type:        metricType,
				Tags:        req.Tags,
				Unit:        req.Unit,
				Description: req.Description,
				Timestamp:   time.Now(),
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// createAlertTool creates the monitoring_create_alert tool.
func createAlertTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("monitoring_create_alert").
		WithDescription("Create or update an alert rule").
		WithTags("monitoring", "alerts", "observability").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Name                 string            `json:"name"`
				Description          string            `json:"description"`
				Condition            string            `json:"condition"`
				Severity             string            `json:"severity"`
				Tags                 map[string]string `json:"tags"`
				NotificationChannels []string          `json:"notification_channels"`
				EvaluationInterval   string            `json:"evaluation_interval"`
				Enabled              bool              `json:"enabled"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Name == "" || req.Condition == "" {
				return tool.Result{}, ErrInvalidInput
			}

			severity := AlertSeverity(req.Severity)
			if severity == "" {
				severity = cfg.DefaultAlertSeverity
			}

			var evalInterval time.Duration
			if req.EvaluationInterval != "" {
				var err error
				evalInterval, err = time.ParseDuration(req.EvaluationInterval)
				if err != nil {
					return tool.Result{}, ErrInvalidInput
				}
			}

			resp, err := cfg.Provider.CreateAlert(ctx, CreateAlertRequest{
				Name:                 req.Name,
				Description:          req.Description,
				Condition:            req.Condition,
				Severity:             severity,
				Tags:                 req.Tags,
				NotificationChannels: req.NotificationChannels,
				EvaluationInterval:   evalInterval,
				Enabled:              req.Enabled,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// queryMetricsTool creates the monitoring_query_metrics tool.
func queryMetricsTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("monitoring_query_metrics").
		WithDescription("Query metric data").
		ReadOnly().
		Cacheable().
		WithTags("monitoring", "metrics", "query").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Query     string `json:"query"`
				StartTime string `json:"start_time"`
				EndTime   string `json:"end_time"`
				Step      string `json:"step"`
				Limit     int    `json:"limit"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.Query == "" {
				return tool.Result{}, ErrInvalidInput
			}

			// Parse times
			var startTime, endTime time.Time
			now := time.Now()

			if req.StartTime != "" {
				var err error
				startTime, err = time.Parse(time.RFC3339, req.StartTime)
				if err != nil {
					return tool.Result{}, ErrInvalidInput
				}
			} else {
				startTime = now.Add(-cfg.DefaultQueryDuration)
			}

			if req.EndTime != "" {
				var err error
				endTime, err = time.Parse(time.RFC3339, req.EndTime)
				if err != nil {
					return tool.Result{}, ErrInvalidInput
				}
			} else {
				endTime = now
			}

			var step time.Duration
			if req.Step != "" {
				var err error
				step, err = time.ParseDuration(req.Step)
				if err != nil {
					return tool.Result{}, ErrInvalidInput
				}
			}

			resp, err := cfg.Provider.QueryMetrics(ctx, QueryMetricsRequest{
				Query:     req.Query,
				StartTime: startTime,
				EndTime:   endTime,
				Step:      step,
				Limit:     req.Limit,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{
				Output: output,
				Cached: true,
			}, nil
		}).
		MustBuild()
}
