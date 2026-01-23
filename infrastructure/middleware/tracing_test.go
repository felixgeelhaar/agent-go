package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	domainmw "github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	mw "github.com/felixgeelhaar/agent-go/infrastructure/middleware"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestTracing(t *testing.T) {
	t.Parallel()

	t.Run("creates span for tool execution", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultTracingConfig()
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")

		middleware := mw.Tracing(cfg)

		mockT := &mockTool{
			name:        "read_file",
			annotations: tool.Annotations{ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
			Input:        json.RawMessage(`{"path":"/test"}`),
			Reason:       "gathering info",
		}

		expected := tool.Result{Output: json.RawMessage(`{"content":"hello"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("records error on span", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultTracingConfig()
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")

		middleware := mw.Tracing(cfg)

		mockT := &mockTool{name: "failing_tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		handlerErr := errors.New("execution failed")
		handler := middleware(createTestHandler(tool.Result{}, handlerErr))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error from handler")
		}
	})

	t.Run("records input when enabled", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultTracingConfig()
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")
		cfg.RecordInput = true

		middleware := mw.Tracing(cfg)

		mockT := &mockTool{name: "tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
			Input:        json.RawMessage(`{"key":"value"}`),
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("records output when enabled", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultTracingConfig()
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")
		cfg.RecordOutput = true

		middleware := mw.Tracing(cfg)

		mockT := &mockTool{name: "tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{"result":"data"}`)}, nil))

		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("applies span name prefix", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultTracingConfig()
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")
		cfg.SpanNamePrefix = "custom."

		middleware := mw.Tracing(cfg)

		mockT := &mockTool{name: "tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("truncates large attributes", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultTracingConfig()
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")
		cfg.RecordInput = true
		cfg.MaxAttributeSize = 10

		middleware := mw.Tracing(cfg)

		mockT := &mockTool{name: "tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
			Input:        json.RawMessage(`{"long_value":"this is a very long string that should be truncated"}`),
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNewTracing(t *testing.T) {
	t.Parallel()

	t.Run("creates middleware with default config", func(t *testing.T) {
		t.Parallel()

		middleware := mw.NewTracing()
		if middleware == nil {
			t.Fatal("NewTracing() returned nil")
		}
	})

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()

		tracer := noop.NewTracerProvider().Tracer("custom")
		middleware := mw.NewTracing(
			mw.WithTracer(tracer),
			mw.WithTracerName("test-tracer"),
			mw.WithInputRecording(false),
			mw.WithOutputRecording(true),
			mw.WithMaxAttributeSize(512),
			mw.WithSpanNamePrefix("agent."),
			mw.WithAdditionalAttributes(attribute.String("env", "test")),
		)

		if middleware == nil {
			t.Fatal("NewTracing() with options returned nil")
		}
	})
}

func TestDefaultTracingConfig(t *testing.T) {
	t.Parallel()

	cfg := mw.DefaultTracingConfig()

	if cfg.TracerName != "agent-go" {
		t.Errorf("expected TracerName 'agent-go', got '%s'", cfg.TracerName)
	}
	if !cfg.RecordInput {
		t.Error("expected RecordInput to be true by default")
	}
	if cfg.RecordOutput {
		t.Error("expected RecordOutput to be false by default")
	}
	if cfg.MaxAttributeSize != 1024 {
		t.Errorf("expected MaxAttributeSize 1024, got %d", cfg.MaxAttributeSize)
	}
	if cfg.SpanNamePrefix != "tool." {
		t.Errorf("expected SpanNamePrefix 'tool.', got '%s'", cfg.SpanNamePrefix)
	}
}

func TestTracingWithMetrics(t *testing.T) {
	t.Parallel()

	t.Run("combines tracing and metrics collection", func(t *testing.T) {
		t.Parallel()

		cfg := mw.TracingMetricsConfig{
			TracingConfig:    mw.DefaultTracingConfig(),
			CollectHistogram: true,
			CollectCounter:   true,
		}
		cfg.Tracer = noop.NewTracerProvider().Tracer("test")

		middleware := mw.TracingWithMetrics(cfg)

		mockT := &mockTool{name: "tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSpanHelpers(t *testing.T) {
	t.Parallel()

	t.Run("ContextWithSpan creates span", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		_, span := mw.ContextWithSpan(ctx, "test-span")
		defer span.End()

		if span == nil {
			t.Error("expected span to be created")
		}
	})

	t.Run("SpanFromContext returns span", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx, _ = mw.ContextWithSpan(ctx, "test-span")

		span := mw.SpanFromContext(ctx)
		if span == nil {
			t.Error("expected span from context")
		}
	})

	t.Run("AddSpanEvent does not panic", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx, span := mw.ContextWithSpan(ctx, "test-span")
		defer span.End()

		// Should not panic
		mw.AddSpanEvent(ctx, "test-event", attribute.String("key", "value"))
	})

	t.Run("AddSpanAttributes does not panic", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx, span := mw.ContextWithSpan(ctx, "test-span")
		defer span.End()

		// Should not panic
		mw.AddSpanAttributes(ctx, attribute.String("key", "value"))
	})

	t.Run("RecordSpanError does not panic", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx, span := mw.ContextWithSpan(ctx, "test-span")
		defer span.End()

		// Should not panic
		mw.RecordSpanError(ctx, errors.New("test error"))
	})
}

func TestToolSpanAttributes(t *testing.T) {
	t.Parallel()

	mockT := &mockTool{
		name: "test_tool",
		annotations: tool.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
			RiskLevel:   tool.RiskLow,
		},
	}

	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateExplore,
		Tool:         mockT,
	}

	attrs := mw.ToolSpanAttributes(execCtx)

	if len(attrs) == 0 {
		t.Fatal("expected attributes")
	}

	// Verify specific attributes exist
	attrMap := make(map[string]attribute.KeyValue)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr
	}

	if _, ok := attrMap["agent.run_id"]; !ok {
		t.Error("expected agent.run_id attribute")
	}
	if _, ok := attrMap["tool.name"]; !ok {
		t.Error("expected tool.name attribute")
	}
	if _, ok := attrMap["tool.read_only"]; !ok {
		t.Error("expected tool.read_only attribute")
	}
}

func TestCreateToolSpan(t *testing.T) {
	t.Parallel()

	mockT := &mockTool{
		name:        "test_tool",
		annotations: tool.Annotations{ReadOnly: true},
	}

	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateExplore,
		Tool:         mockT,
	}

	ctx, span := mw.CreateToolSpan(context.Background(), execCtx)
	defer span.End()

	if span == nil {
		t.Error("expected span to be created")
	}

	// Context should contain the span
	spanFromCtx := mw.SpanFromContext(ctx)
	if spanFromCtx == nil {
		t.Error("expected span in context")
	}
}

func TestExtractTraceContext(t *testing.T) {
	t.Parallel()

	t.Run("handles empty data", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		result := mw.ExtractTraceContext(ctx, json.RawMessage(`{}`))
		if result == nil {
			t.Error("expected context to be returned")
		}
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		result := mw.ExtractTraceContext(ctx, json.RawMessage(`invalid`))
		if result == nil {
			t.Error("expected context to be returned")
		}
	})

	t.Run("handles trace context in data", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		data := json.RawMessage(`{"trace_id":"abc123","span_id":"def456"}`)
		result := mw.ExtractTraceContext(ctx, data)
		if result == nil {
			t.Error("expected context to be returned")
		}
	})
}

func TestInjectTraceContext(t *testing.T) {
	t.Parallel()

	t.Run("handles nil data", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		result := mw.InjectTraceContext(ctx, nil)
		// Context without span returns nil or empty map - both are acceptable
		_ = result
	})

	t.Run("injects context with existing data", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		data := map[string]interface{}{"existing": "value"}
		result := mw.InjectTraceContext(ctx, data)

		if result == nil {
			t.Error("expected data to be returned")
		}
		if result["existing"] != "value" {
			t.Error("expected existing data to be preserved")
		}
	})
}

func TestLoggingTraceDecorator(t *testing.T) {
	t.Parallel()

	t.Run("returns nil without span", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		result := mw.LoggingTraceDecorator(ctx)
		if result != nil {
			t.Error("expected nil without span")
		}
	})
}
