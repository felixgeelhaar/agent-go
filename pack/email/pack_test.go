package email

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider:    provider,
		DefaultFrom: "default@test.com",
	})

	if p.Name != "email" {
		t.Errorf("expected pack name 'email', got %s", p.Name)
	}

	if len(p.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"email_send", "email_parse", "email_template"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestSend(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		defaultFrom string
		wantErr     bool
		errContains string
	}{
		{
			name: "successful send",
			input: map[string]interface{}{
				"from":    "sender@test.com",
				"to":      []string{"recipient@test.com"},
				"subject": "Test Subject",
				"body":    "Test body content",
			},
			wantErr: false,
		},
		{
			name: "send with all options",
			input: map[string]interface{}{
				"from":      "sender@test.com",
				"to":        []string{"recipient@test.com"},
				"cc":        []string{"cc@test.com"},
				"bcc":       []string{"bcc@test.com"},
				"reply_to":  "reply@test.com",
				"subject":   "Test Subject",
				"body":      "Plain text body",
				"html_body": "<p>HTML body</p>",
				"priority":  "high",
			},
			wantErr: false,
		},
		{
			name: "uses default from",
			input: map[string]interface{}{
				"to":      []string{"recipient@test.com"},
				"subject": "Test Subject",
				"body":    "Test body",
			},
			defaultFrom: "default@test.com",
			wantErr:     false,
		},
		{
			name: "missing to returns error",
			input: map[string]interface{}{
				"from":    "sender@test.com",
				"subject": "Test",
				"body":    "Test",
			},
			wantErr: true,
		},
		{
			name: "missing from without default returns error",
			input: map[string]interface{}{
				"to":      []string{"recipient@test.com"},
				"subject": "Test",
				"body":    "Test",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"from":    "sender@test.com",
				"to":      []string{"recipient@test.com"},
				"subject": "Test",
				"body":    "Test",
			},
			setupFunc: func(p *MockProvider) {
				p.SendFunc = func(context.Context, SendRequest) (SendResponse, error) {
					return SendResponse{}, errors.New("send error")
				}
			},
			wantErr:     true,
			errContains: "send error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:    provider,
				DefaultFrom: tt.defaultFrom,
			})

			var sendTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "email_send" {
					sendTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := sendTool.Execute(context.Background(), input)

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

			var resp SendResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}

			if resp.MessageID == "" {
				t.Error("expected non-empty message ID")
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful parse",
			input: map[string]interface{}{
				"raw_content": "From: sender@test.com\nTo: recipient@test.com\nSubject: Test\n\nBody content",
			},
			wantErr: false,
		},
		{
			name: "parse with attachment extraction",
			input: map[string]interface{}{
				"raw_content":         "From: sender@test.com\nTo: recipient@test.com\nSubject: Test\n\nBody",
				"extract_attachments": true,
			},
			wantErr: false,
		},
		{
			name: "missing raw_content returns error",
			input: map[string]interface{}{
				"extract_attachments": true,
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"raw_content": "test",
			},
			setupFunc: func(p *MockProvider) {
				p.ParseFunc = func(context.Context, ParseRequest) (ParseResponse, error) {
					return ParseResponse{}, errors.New("parse error")
				}
			},
			wantErr:     true,
			errContains: "parse error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var parseTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "email_parse" {
					parseTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := parseTool.Execute(context.Background(), input)

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

			var resp ParseResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if resp.From.Address == "" {
				t.Error("expected non-empty from address")
			}
		})
	}
}

func TestTemplate(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful inline template",
			input: map[string]interface{}{
				"template_content": "Hello {{name}}, your order {{order_id}} is ready.",
				"variables": map[string]interface{}{
					"name":     "John",
					"order_id": "12345",
				},
			},
			wantErr: false,
		},
		{
			name: "successful named template",
			input: map[string]interface{}{
				"template_name": "welcome",
				"variables": map[string]interface{}{
					"name": "Jane",
				},
			},
			setupFunc: func(p *MockProvider) {
				p.AddTemplate("welcome", "Welcome {{name}}!", "text")
			},
			wantErr: false,
		},
		{
			name: "html format",
			input: map[string]interface{}{
				"template_content": "<h1>Hello {{name}}</h1>",
				"variables": map[string]interface{}{
					"name": "World",
				},
				"format": "html",
			},
			wantErr: false,
		},
		{
			name: "missing template returns error",
			input: map[string]interface{}{
				"variables": map[string]interface{}{"name": "test"},
			},
			wantErr: true,
		},
		{
			name: "template not found returns error",
			input: map[string]interface{}{
				"template_name": "nonexistent",
				"variables":     map[string]interface{}{},
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"template_content": "test",
				"variables":        map[string]interface{}{},
			},
			setupFunc: func(p *MockProvider) {
				p.RenderTemplateFunc = func(context.Context, RenderTemplateRequest) (RenderTemplateResponse, error) {
					return RenderTemplateResponse{}, errors.New("render error")
				}
			},
			wantErr:     true,
			errContains: "render error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{Provider: provider})

			var templateTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "email_template" {
					templateTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := templateTool.Execute(context.Background(), input)

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

			var resp RenderTemplateResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}

			// Template tool should be cached
			if !result.Cached {
				t.Error("expected template results to be cached")
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"from":             "test@test.com",
			"to":               []string{"recipient@test.com"},
			"subject":          "test",
			"body":             "test",
			"raw_content":      "test",
			"template_content": "test",
			"variables":        map[string]interface{}{},
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

	// Test Send
	sendResp, err := provider.Send(ctx, SendRequest{
		From:    "sender@test.com",
		To:      []string{"recipient@test.com"},
		Subject: "Test Subject",
		Body:    "Test body",
	})
	if err != nil {
		t.Errorf("Send error: %v", err)
	}
	if !sendResp.Success {
		t.Error("expected success")
	}
	if sendResp.MessageID == "" {
		t.Error("expected message ID")
	}

	// Test Parse
	parseResp, err := provider.Parse(ctx, ParseRequest{
		RawContent: "From: test@test.com\nTo: recipient@test.com\nSubject: Hello\n\nBody here",
	})
	if err != nil {
		t.Errorf("Parse error: %v", err)
	}
	if parseResp.From.Address == "" {
		t.Error("expected from address")
	}

	// Test RenderTemplate
	provider.AddTemplate("test", "Hello {{name}}!", "text")
	renderResp, err := provider.RenderTemplate(ctx, RenderTemplateRequest{
		TemplateName: "test",
		Variables:    map[string]interface{}{"name": "World"},
	})
	if err != nil {
		t.Errorf("RenderTemplate error: %v", err)
	}
	if !renderResp.Success {
		t.Error("expected success")
	}
	if renderResp.Body != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %s", renderResp.Body)
	}

	// Verify email count
	if provider.EmailCount() != 1 {
		t.Errorf("expected 1 email, got %d", provider.EmailCount())
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.DefaultFrom != "" {
		t.Errorf("expected empty DefaultFrom, got %s", cfg.DefaultFrom)
	}

	if cfg.DefaultReplyTo != "" {
		t.Errorf("expected empty DefaultReplyTo, got %s", cfg.DefaultReplyTo)
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
