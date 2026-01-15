// Package http provides HTTP client tools.
package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackOptions configures the HTTP pack.
type PackOptions struct {
	// Timeout for HTTP requests.
	Timeout time.Duration

	// MaxBodySize limits response body size (bytes).
	MaxBodySize int64
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() PackOptions {
	return PackOptions{
		Timeout:     30 * time.Second,
		MaxBodySize: 10 * 1024 * 1024, // 10MB
	}
}

// New creates the HTTP pack.
func New(opts ...func(*PackOptions)) *pack.Pack {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	client := &http.Client{
		Timeout: options.Timeout,
	}

	return pack.NewBuilder("http").
		WithDescription("HTTP client operations").
		WithVersion("1.0.0").
		AddTools(
			getTool(client, options),
			postTool(client, options),
			headTool(client, options),
		).
		AllowInState(agent.StateExplore, "http_get", "http_head").
		AllowInState(agent.StateAct, "http_get", "http_post", "http_head").
		Build()
}

type httpGetInput struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type httpResponse struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Size       int64             `json:"size"`
}

func getTool(client *http.Client, opts PackOptions) tool.Tool {
	return tool.NewBuilder("http_get").
		WithDescription("Perform HTTP GET request").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in httpGetInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
			if err != nil {
				return tool.Result{}, err
			}

			for k, v := range in.Headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			// Read body with size limit
			body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodySize))
			if err != nil {
				return tool.Result{}, err
			}

			headers := make(map[string]string)
			for k := range resp.Header {
				headers[k] = resp.Header.Get(k)
			}

			out := httpResponse{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Headers:    headers,
				Body:       string(body),
				Size:       int64(len(body)),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type httpPostInput struct {
	URL         string            `json:"url"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

func postTool(client *http.Client, opts PackOptions) tool.Tool {
	return tool.NewBuilder("http_post").
		WithDescription("Perform HTTP POST request").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in httpPostInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			var bodyReader io.Reader
			if in.Body != "" {
				bodyReader = strings.NewReader(in.Body)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.URL, bodyReader)
			if err != nil {
				return tool.Result{}, err
			}

			if in.ContentType != "" {
				req.Header.Set("Content-Type", in.ContentType)
			}

			for k, v := range in.Headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodySize))
			if err != nil {
				return tool.Result{}, err
			}

			headers := make(map[string]string)
			for k := range resp.Header {
				headers[k] = resp.Header.Get(k)
			}

			out := httpResponse{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Headers:    headers,
				Body:       string(body),
				Size:       int64(len(body)),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

type httpHeadInput struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

type httpHeadResponse struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
}

func headTool(client *http.Client, _ PackOptions) tool.Tool {
	return tool.NewBuilder("http_head").
		WithDescription("Perform HTTP HEAD request").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in httpHeadInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, in.URL, nil)
			if err != nil {
				return tool.Result{}, err
			}

			for k, v := range in.Headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				return tool.Result{}, err
			}
			defer resp.Body.Close()

			headers := make(map[string]string)
			for k := range resp.Header {
				headers[k] = resp.Header.Get(k)
			}

			out := httpHeadResponse{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Headers:    headers,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
