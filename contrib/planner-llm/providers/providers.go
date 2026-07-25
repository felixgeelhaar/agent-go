// Package providers contains LLM provider implementations for the planner-llm module.
//
// This package provides ready-to-use implementations for various LLM providers:
//
//   - OpenAI (GPT-4o, GPT-4, GPT-3.5-turbo) — also supports Azure OpenAI via BaseURL
//   - Anthropic (Claude 4, Claude 3.5)
//   - Google Gemini (Gemini Pro, Gemini Ultra)
//   - Cohere (Command-R, Command-R+)
//   - AWS Bedrock (Claude, Llama, Mistral via SigV4)
//   - GitHub Copilot (OpenAI-compatible endpoint)
//   - Ollama (local models via native /api/chat)
//
// Each provider implements the plannerllm.Provider interface.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
)

// Common errors for providers.
var (
	ErrMissingAPIKey    = errors.New("missing API key")
	ErrInvalidModel     = errors.New("invalid model")
	ErrRateLimited      = errors.New("rate limited")
	ErrContextCanceled  = errors.New("context canceled")
	ErrConnectionFailed = errors.New("connection failed")
)

// doRequest performs an HTTP request with standard error handling.
// It marshals the body, sets headers, executes the request, and returns the raw response bytes.
func doRequest(ctx context.Context, method, url string, headers map[string]string, body any, timeoutSec int) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", redactURLError(err))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrContextCanceled
		}
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, redactURLError(err))
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// redactURLError strips credential-bearing parts of the URL from a *url.Error
// before it is formatted into a message a caller is likely to log.
//
// net/url's own redaction covers userinfo only: a *url.Error returned by
// http.Client.Do renders the full request URL, query string included. Providers
// should not put secrets in query strings at all (see gemini.go), but transport
// errors are the one place where a URL escapes into an error string verbatim, so
// the query is dropped here as well — belt and braces for any future endpoint
// that carries a token, signature or session ID as a parameter.
//
// Errors that are not *url.Error are returned unchanged.
func redactURLError(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}

	redacted := *urlErr
	redacted.URL = redactURL(urlErr.URL)
	return &redacted
}

// redactURL returns raw with any userinfo and query string removed.
func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		// Unparseable URLs may still contain a secret; reveal nothing.
		return "[redacted]"
	}

	parsed.User = nil
	if parsed.RawQuery != "" {
		parsed.RawQuery = "[redacted]"
	}
	parsed.Fragment = ""

	return parsed.String()
}

// resolveModel returns the first non-empty model string.
func resolveModel(requestModel, configModel, defaultModel string) string {
	if requestModel != "" {
		return requestModel
	}
	if configModel != "" {
		return configModel
	}
	return defaultModel
}

// Ensure all providers implement the Provider interface.
var (
	_ plannerllm.Provider = (*OpenAIProvider)(nil)
	_ plannerllm.Provider = (*AnthropicProvider)(nil)
	_ plannerllm.Provider = (*GeminiProvider)(nil)
	_ plannerllm.Provider = (*CohereProvider)(nil)
	_ plannerllm.Provider = (*BedrockProvider)(nil)
	_ plannerllm.Provider = (*OllamaProvider)(nil)
	_ plannerllm.Provider = (*CopilotProvider)(nil)
)
