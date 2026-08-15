// Package monitoring provides monitoring tools for agent-go.
//
// MVP tools:
//   - health_check: HTTP GET against an allowlisted URL
//   - metrics_query: Prometheus-compatible /api/v1/query against an allowlisted base
//
// Other observability surfaces (alerts, logs, traces, dashboards) remain out of
// scope for this MVP and are not registered.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/sandbox"
)

const (
	defaultTimeout    = 10 * time.Second
	maxResponseBody   = 1 << 20 // 1MB
	maxRedirects      = 5
)

// Config configures the monitoring pack.
type Config struct {
	// AllowedHosts is a list of hostnames (exact or "*.example.com") permitted
	// for health_check and metrics_query. Required — empty means deny-all.
	AllowedHosts []string

	// MetricsBaseURL is the Prometheus-compatible API base, e.g.
	// "https://prometheus.example.com". Query path /api/v1/query is appended.
	MetricsBaseURL string

	// HTTPClient overrides the default client (tests).
	HTTPClient *http.Client
}

// Pack returns MVP monitoring tools.
func Pack(cfg Config) *pack.Pack {
	if len(cfg.AllowedHosts) == 0 {
		panic("monitoring.Pack: AllowedHosts is required")
	}
	p := &monPack{cfg: cfg}
	if p.cfg.HTTPClient == nil {
		p.cfg.HTTPClient = &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("stopped after %d redirects", maxRedirects)
				}
				return nil
			},
		}
	}
	return pack.NewBuilder("monitoring").
		WithDescription("Monitoring tools (health check + Prometheus metrics query)").
		WithVersion("0.1.0").
		AddTools(
			p.healthCheck(),
			p.metricsQuery(),
		).
		AllowInState(agent.StateExplore, "health_check", "metrics_query").
		AllowInState(agent.StateAct, "health_check", "metrics_query").
		AllowInState(agent.StateValidate, "health_check", "metrics_query").
		Build()
}

type monPack struct {
	cfg Config
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *monPack) hostAllowed(host string) bool {
	return sandbox.HostAllowed(host, p.cfg.AllowedHosts)
}

func (p *monPack) validateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid url", tool.ErrInvalidInput)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: only http/https allowed", tool.ErrInvalidInput)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("%w: host required", tool.ErrInvalidInput)
	}
	if !p.hostAllowed(u.Host) {
		return nil, fmt.Errorf("%w: host %q not in AllowedHosts", tool.ErrInvalidInput, u.Hostname())
	}
	return u, nil
}

func (p *monPack) doGET(ctx context.Context, rawURL string, timeout time.Duration) (status int, body string, contentType string, dur time.Duration, err error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", "", 0, err
	}
	client := p.cfg.HTTPClient
	if timeout > 0 {
		c := *client
		c.Timeout = timeout
		client = &c
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", time.Since(start), err
	}
	defer func() { _ = resp.Body.Close() }()
	limited := io.LimitReader(resp.Body, maxResponseBody+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return resp.StatusCode, "", resp.Header.Get("Content-Type"), time.Since(start), err
	}
	if len(b) > maxResponseBody {
		b = b[:maxResponseBody]
	}
	return resp.StatusCode, string(b), resp.Header.Get("Content-Type"), time.Since(start), nil
}

func (p *monPack) healthCheck() tool.Tool {
	return tool.NewBuilder("health_check").
		WithDescription("Check a service health endpoint (allowlisted hosts only)").
		ReadOnly().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"url":          json.RawMessage(`{"type":"string"}`),
			"timeout_secs": json.RawMessage(`{"type":"number"}`),
		}, []string{"url"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				URL         string   `json:"url"`
				TimeoutSecs *float64 `json:"timeout_secs"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			u, err := p.validateURL(in.URL)
			if err != nil {
				return tool.Result{}, err
			}
			timeout := defaultTimeout
			if in.TimeoutSecs != nil && *in.TimeoutSecs > 0 {
				timeout = time.Duration(*in.TimeoutSecs * float64(time.Second))
			}
			status, body, ct, dur, err := p.doGET(ctx, u.String(), timeout)
			healthy := err == nil && status >= 200 && status < 300
			out := map[string]any{
				"url":          u.String(),
				"healthy":      healthy,
				"status_code":  status,
				"content_type": ct,
				"duration_ms":  float64(dur.Microseconds()) / 1000,
			}
			if err != nil {
				out["error"] = err.Error()
			} else if len(body) > 512 {
				out["body"] = body[:512]
				out["body_truncated"] = true
			} else {
				out["body"] = body
			}
			return resultOK(out)
		}).
		MustBuild()
}

func (p *monPack) metricsQuery() tool.Tool {
	return tool.NewBuilder("metrics_query").
		WithDescription("Query Prometheus-compatible /api/v1/query (allowlisted MetricsBaseURL host)").
		ReadOnly().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"query": json.RawMessage(`{"type":"string"}`),
			"time":  json.RawMessage(`{"type":"string"}`),
		}, []string{"query"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if p.cfg.MetricsBaseURL == "" {
				return tool.Result{}, fmt.Errorf("%w: MetricsBaseURL not configured", tool.ErrInvalidInput)
			}
			base, err := p.validateURL(p.cfg.MetricsBaseURL)
			if err != nil {
				return tool.Result{}, err
			}
			in, err := decode[struct {
				Query string `json:"query"`
				Time  string `json:"time"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if strings.TrimSpace(in.Query) == "" {
				return tool.Result{}, fmt.Errorf("%w: query is required", tool.ErrInvalidInput)
			}
			endpoint := strings.TrimRight(base.String(), "/") + "/api/v1/query"
			q := url.Values{}
			q.Set("query", in.Query)
			if in.Time != "" {
				q.Set("time", in.Time)
			}
			full := endpoint + "?" + q.Encode()
			status, body, ct, dur, err := p.doGET(ctx, full, defaultTimeout)
			out := map[string]any{
				"status_code":  status,
				"content_type": ct,
				"duration_ms":  float64(dur.Microseconds()) / 1000,
				"query":        in.Query,
			}
			if err != nil {
				out["error"] = err.Error()
				return resultOK(out)
			}
			var parsed any
			if json.Unmarshal([]byte(body), &parsed) == nil {
				out["result"] = parsed
			} else {
				out["body"] = body
			}
			return resultOK(out)
		}).
		MustBuild()
}
