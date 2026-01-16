package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultOptions()

	if opts.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", opts.Timeout, 30*time.Second)
	}
	if opts.MaxBodySize != 10*1024*1024 {
		t.Errorf("MaxBodySize = %d, want %d", opts.MaxBodySize, 10*1024*1024)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}

	if p.Name != "http" {
		t.Errorf("Name = %s, want http", p.Name)
	}

	// Check that expected tools exist
	expectedTools := []string{"http_get", "http_post", "http_head"}
	for _, name := range expectedTools {
		if _, ok := p.GetTool(name); !ok {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestNewWithOptions(t *testing.T) {
	t.Parallel()

	p := New(func(o *PackOptions) {
		o.Timeout = 60 * time.Second
		o.MaxBodySize = 5 * 1024 * 1024
	})

	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHTTPGetTool(t *testing.T) {
	t.Parallel()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "hello"}`))
	}))
	defer server.Close()

	p := New()
	tool, ok := p.GetTool("http_get")
	if !ok {
		t.Fatal("http_get tool not found")
	}

	input, _ := json.Marshal(httpGetInput{
		URL: server.URL,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out httpResponse
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", out.StatusCode)
	}
	if out.Body != `{"message": "hello"}` {
		t.Errorf("Body = %s, want {\"message\": \"hello\"}", out.Body)
	}
	if out.Headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("X-Custom-Header = %s, want custom-value", out.Headers["X-Custom-Header"])
	}
}

func TestHTTPGetToolWithHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Authorization header = %s, want Bearer token123", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	p := New()
	tool, _ := p.GetTool("http_get")

	input, _ := json.Marshal(httpGetInput{
		URL: server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer token123",
		},
	})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestHTTPGetToolInvalidURL(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("http_get")

	input, _ := json.Marshal(httpGetInput{
		URL: "not-a-valid-url",
	})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestHTTPGetToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("http_get")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHTTPPostTool(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 123}`))
	}))
	defer server.Close()

	p := New()
	tool, ok := p.GetTool("http_post")
	if !ok {
		t.Fatal("http_post tool not found")
	}

	input, _ := json.Marshal(httpPostInput{
		URL:         server.URL,
		Body:        `{"name": "test"}`,
		ContentType: "application/json",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out httpResponse
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", out.StatusCode)
	}
}

func TestHTTPPostToolWithHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("X-Api-Key = %s, want secret", r.Header.Get("X-Api-Key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New()
	tool, _ := p.GetTool("http_post")

	input, _ := json.Marshal(httpPostInput{
		URL: server.URL,
		Headers: map[string]string{
			"X-Api-Key": "secret",
		},
	})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestHTTPPostToolWithoutBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New()
	tool, _ := p.GetTool("http_post")

	input, _ := json.Marshal(httpPostInput{
		URL: server.URL,
	})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestHTTPPostToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("http_post")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHTTPHeadTool(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.Header().Set("Content-Length", "1234")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New()
	tool, ok := p.GetTool("http_head")
	if !ok {
		t.Fatal("http_head tool not found")
	}

	input, _ := json.Marshal(httpHeadInput{
		URL: server.URL,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out httpHeadResponse
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", out.StatusCode)
	}
	if out.Headers["Content-Length"] != "1234" {
		t.Errorf("Content-Length = %s, want 1234", out.Headers["Content-Length"])
	}
}

func TestHTTPHeadToolWithHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "TestAgent" {
			t.Errorf("User-Agent = %s, want TestAgent", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := New()
	tool, _ := p.GetTool("http_head")

	input, _ := json.Marshal(httpHeadInput{
		URL: server.URL,
		Headers: map[string]string{
			"User-Agent": "TestAgent",
		},
	})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestHTTPHeadToolInvalidJSON(t *testing.T) {
	t.Parallel()

	p := New()
	tool, _ := p.GetTool("http_head")

	_, err := tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestToolAnnotations(t *testing.T) {
	t.Parallel()

	p := New()

	// http_get should be read-only and cacheable
	if tool, ok := p.GetTool("http_get"); ok {
		a := tool.Annotations()
		if !a.ReadOnly {
			t.Error("http_get should be ReadOnly")
		}
		if !a.Cacheable {
			t.Error("http_get should be Cacheable")
		}
	}

	// http_post should not be read-only
	if tool, ok := p.GetTool("http_post"); ok {
		a := tool.Annotations()
		if a.ReadOnly {
			t.Error("http_post should not be ReadOnly")
		}
	}

	// http_head should be read-only and cacheable
	if tool, ok := p.GetTool("http_head"); ok {
		a := tool.Annotations()
		if !a.ReadOnly {
			t.Error("http_head should be ReadOnly")
		}
		if !a.Cacheable {
			t.Error("http_head should be Cacheable")
		}
	}
}
