package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider: provider,
	})

	if p.Name != "monitoring" {
		t.Errorf("expected pack name 'monitoring', got %s", p.Name)
	}

	if len(p.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"monitoring_emit_metric", "monitoring_create_alert", "monitoring_query_metrics"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestEmitMetric(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful emit",
			input: map[string]interface{}{
				"name":  "cpu_usage",
				"value": 75.5,
			},
			wantErr: false,
		},
		{
			name: "emit with all options",
			input: map[string]interface{}{
				"name":        "request_count",
				"value":       100,
				"type":        "counter",
				"tags":        map[string]interface{}{"service": "api", "env": "prod"},
				"unit":        "count",
				"description": "Total request count",
			},
			wantErr: false,
		},
		{
			name: "missing name returns error",
			input: map[string]interface{}{
				"value": 50,
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"name":  "test",
				"value": 1,
			},
			setupFunc: func(p *MockProvider) {
				p.EmitMetricFunc = func(context.Context, EmitMetricRequest) (EmitMetricResponse, error) {
					return EmitMetricResponse{}, errors.New("emit error")
				}
			},
			wantErr:     true,
			errContains: "emit error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var emitTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "monitoring_emit_metric" {
					emitTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := emitTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp EmitMetricResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}
		})
	}
}

func TestCreateAlert(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful create",
			input: map[string]interface{}{
				"name":      "high_cpu",
				"condition": "cpu_usage > 90",
				"enabled":   true,
			},
			wantErr: false,
		},
		{
			name: "create with all options",
			input: map[string]interface{}{
				"name":                  "error_rate_high",
				"description":           "Alert when error rate exceeds threshold",
				"condition":             "error_rate > 0.05",
				"severity":              "critical",
				"tags":                  map[string]interface{}{"team": "platform"},
				"notification_channels": []string{"#alerts", "oncall@example.com"},
				"evaluation_interval":   "1m",
				"enabled":               true,
			},
			wantErr: false,
		},
		{
			name: "missing name returns error",
			input: map[string]interface{}{
				"condition": "test > 0",
			},
			wantErr: true,
		},
		{
			name: "missing condition returns error",
			input: map[string]interface{}{
				"name": "test_alert",
			},
			wantErr: true,
		},
		{
			name: "invalid evaluation interval returns error",
			input: map[string]interface{}{
				"name":                "test",
				"condition":           "test > 0",
				"evaluation_interval": "invalid",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"name":      "test",
				"condition": "test > 0",
			},
			setupFunc: func(p *MockProvider) {
				p.CreateAlertFunc = func(context.Context, CreateAlertRequest) (CreateAlertResponse, error) {
					return CreateAlertResponse{}, errors.New("alert error")
				}
			},
			wantErr:     true,
			errContains: "alert error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var alertTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "monitoring_create_alert" {
					alertTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := alertTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp CreateAlertResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.AlertID == "" {
				t.Error("expected non-empty alert ID")
			}
		})
	}
}

func TestQueryMetrics(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		preEmit     bool
		wantErr     bool
		errContains string
	}{
		{
			name: "successful query",
			input: map[string]interface{}{
				"query": "cpu_usage",
			},
			preEmit: true,
			wantErr: false,
		},
		{
			name: "query with time range",
			input: map[string]interface{}{
				"query":      "cpu_usage",
				"start_time": time.Now().Add(-time.Hour).Format(time.RFC3339),
				"end_time":   time.Now().Format(time.RFC3339),
			},
			preEmit: true,
			wantErr: false,
		},
		{
			name: "query with step and limit",
			input: map[string]interface{}{
				"query": "cpu_usage",
				"step":  "1m",
				"limit": 10,
			},
			preEmit: true,
			wantErr: false,
		},
		{
			name: "missing query returns error",
			input: map[string]interface{}{
				"limit": 10,
			},
			wantErr: true,
		},
		{
			name: "invalid start_time returns error",
			input: map[string]interface{}{
				"query":      "test",
				"start_time": "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid step returns error",
			input: map[string]interface{}{
				"query": "test",
				"step":  "invalid",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"query": "test",
			},
			setupFunc: func(p *MockProvider) {
				p.QueryMetricsFunc = func(context.Context, QueryMetricsRequest) (QueryMetricsResponse, error) {
					return QueryMetricsResponse{}, errors.New("query error")
				}
			},
			wantErr:     true,
			errContains: "query error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")

			if tt.preEmit {
				// Pre-emit some metrics
				_, _ = provider.EmitMetric(context.Background(), EmitMetricRequest{
					Name:  "cpu_usage",
					Value: 75.5,
				})
			}

			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var queryTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "monitoring_query_metrics" {
					queryTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := queryTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp QueryMetricsResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			// Query should be cacheable
			if !result.Cached {
				t.Error("expected query results to be cached")
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"name":      "test",
			"value":     1,
			"condition": "test > 0",
			"query":     "test",
		})

		_, err := tool.Execute(context.Background(), input)
		if !errors.Is(err, ErrProviderNotConfigured) {
			t.Errorf("tool %s: expected ErrProviderNotConfigured, got %v", tool.Name(), err)
		}
	}
}

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider("test-mock")

	if provider.Name() != "test-mock" {
		t.Errorf("expected name 'test-mock', got %s", provider.Name())
	}

	ctx := context.Background()

	// Test Available
	if !provider.Available(ctx) {
		t.Error("expected provider to be available")
	}

	// Test EmitMetric
	emitResp, err := provider.EmitMetric(ctx, EmitMetricRequest{
		Name:  "test_metric",
		Value: 42.0,
		Type:  MetricTypeGauge,
		Tags:  map[string]string{"env": "test"},
	})
	if err != nil {
		t.Errorf("EmitMetric error: %v", err)
	}
	if !emitResp.Success {
		t.Error("expected success")
	}

	// Test CreateAlert
	alertResp, err := provider.CreateAlert(ctx, CreateAlertRequest{
		Name:      "test_alert",
		Condition: "test_metric > 50",
		Severity:  AlertSeverityWarning,
		Enabled:   true,
	})
	if err != nil {
		t.Errorf("CreateAlert error: %v", err)
	}
	if !alertResp.Created {
		t.Error("expected alert to be created")
	}

	// Test QueryMetrics
	queryResp, err := provider.QueryMetrics(ctx, QueryMetricsRequest{
		Query: "test_metric",
	})
	if err != nil {
		t.Errorf("QueryMetrics error: %v", err)
	}
	if len(queryResp.Series) == 0 {
		t.Error("expected at least one series")
	}

	// Verify counts
	if provider.MetricCount() != 1 {
		t.Errorf("expected 1 metric, got %d", provider.MetricCount())
	}
	if provider.AlertCount() != 1 {
		t.Errorf("expected 1 alert, got %d", provider.AlertCount())
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.DefaultMetricType != MetricTypeGauge {
		t.Errorf("expected DefaultMetricType 'gauge', got %s", cfg.DefaultMetricType)
	}

	if cfg.DefaultAlertSeverity != AlertSeverityWarning {
		t.Errorf("expected DefaultAlertSeverity 'warning', got %s", cfg.DefaultAlertSeverity)
	}

	if cfg.DefaultQueryDuration != time.Hour {
		t.Errorf("expected DefaultQueryDuration 1h, got %v", cfg.DefaultQueryDuration)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
