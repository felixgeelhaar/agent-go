package monitoring_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	monitoring "go.klarlabs.de/agent/contrib/pack-monitoring"
	"go.klarlabs.de/agent/domain/tool"
)

func TestPackNotStub(t *testing.T) {
	p := monitoring.Pack(monitoring.Config{AllowedHosts: []string{"example.com"}})
	if p.Metadata["status"] == "stub" {
		t.Fatal("should not be stub")
	}
	if len(p.Tools) != 2 {
		t.Fatalf("expected 2 MVP tools, got %d", len(p.Tools))
	}
}

func TestHealthCheckAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	p := monitoring.Pack(monitoring.Config{
		AllowedHosts: []string{"127.0.0.1"},
		HTTPClient:   srv.Client(),
	})
	var health tool.Tool
	for _, tt := range p.Tools {
		if tt.Name() == "health_check" {
			health = tt
		}
	}

	res, err := health.Execute(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`/health"}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Healthy    bool `json:"healthy"`
		StatusCode int  `json:"status_code"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Healthy || out.StatusCode != 200 {
		t.Fatalf("unexpected: %+v", out)
	}

	_, err = health.Execute(context.Background(), json.RawMessage(`{"url":"https://evil.example/x"}`))
	if err == nil {
		t.Fatal("expected allowlist rejection")
	}
}

func TestMetricsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	p := monitoring.Pack(monitoring.Config{
		AllowedHosts:   []string{"127.0.0.1"},
		MetricsBaseURL: srv.URL,
		HTTPClient:     srv.Client(),
	})
	var metrics tool.Tool
	for _, tt := range p.Tools {
		if tt.Name() == "metrics_query" {
			metrics = tt
		}
	}
	res, err := metrics.Execute(context.Background(), json.RawMessage(`{"query":"up"}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		StatusCode int `json:"status_code"`
		Result     any `json:"result"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.StatusCode != 200 || out.Result == nil {
		t.Fatalf("unexpected: %+v", out)
	}
}
