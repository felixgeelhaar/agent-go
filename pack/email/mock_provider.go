package email

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockProvider is a mock email provider for testing.
type MockProvider struct {
	name string

	// SendFunc is called when Send is invoked.
	SendFunc func(ctx context.Context, req SendRequest) (SendResponse, error)

	// ParseFunc is called when Parse is invoked.
	ParseFunc func(ctx context.Context, req ParseRequest) (ParseResponse, error)

	// RenderTemplateFunc is called when RenderTemplate is invoked.
	RenderTemplateFunc func(ctx context.Context, req RenderTemplateRequest) (RenderTemplateResponse, error)

	// AvailableFunc is called when Available is invoked.
	AvailableFunc func(ctx context.Context) bool

	// Internal state
	mu           sync.RWMutex
	sentEmails   []storedEmail
	templates    map[string]storedTemplate
	emailCount   int
}

type storedEmail struct {
	id          string
	from        string
	to          []string
	subject     string
	body        string
	htmlBody    string
	attachments []Attachment
	sentAt      time.Time
}

type storedTemplate struct {
	name    string
	content string
	format  string
}

// NewMockProvider creates a new mock provider with default implementations.
func NewMockProvider(name string) *MockProvider {
	p := &MockProvider{
		name:       name,
		sentEmails: make([]storedEmail, 0),
		templates:  make(map[string]storedTemplate),
	}

	p.SendFunc = p.defaultSend
	p.ParseFunc = p.defaultParse
	p.RenderTemplateFunc = p.defaultRenderTemplate
	p.AvailableFunc = func(_ context.Context) bool { return true }

	return p
}

// Name returns the provider name.
func (p *MockProvider) Name() string {
	return p.name
}

// Send sends an email.
func (p *MockProvider) Send(ctx context.Context, req SendRequest) (SendResponse, error) {
	return p.SendFunc(ctx, req)
}

// Parse parses an email.
func (p *MockProvider) Parse(ctx context.Context, req ParseRequest) (ParseResponse, error) {
	return p.ParseFunc(ctx, req)
}

// RenderTemplate renders an email template.
func (p *MockProvider) RenderTemplate(ctx context.Context, req RenderTemplateRequest) (RenderTemplateResponse, error) {
	return p.RenderTemplateFunc(ctx, req)
}

// Available checks if the provider is available.
func (p *MockProvider) Available(ctx context.Context) bool {
	return p.AvailableFunc(ctx)
}

func (p *MockProvider) defaultSend(_ context.Context, req SendRequest) (SendResponse, error) {
	if len(req.To) == 0 {
		return SendResponse{}, ErrInvalidInput
	}
	if req.Subject == "" {
		return SendResponse{}, ErrInvalidInput
	}
	if req.Body == "" && req.HTMLBody == "" {
		return SendResponse{}, ErrInvalidInput
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.emailCount++
	now := time.Now()
	messageID := fmt.Sprintf("msg-%d@mock.local", p.emailCount)

	p.sentEmails = append(p.sentEmails, storedEmail{
		id:          messageID,
		from:        req.From,
		to:          req.To,
		subject:     req.Subject,
		body:        req.Body,
		htmlBody:    req.HTMLBody,
		attachments: req.Attachments,
		sentAt:      now,
	})

	recipients := make([]RecipientStatus, len(req.To))
	for i, email := range req.To {
		recipients[i] = RecipientStatus{
			Email:  email,
			Status: "delivered",
		}
	}

	return SendResponse{
		MessageID:  messageID,
		Success:    true,
		Timestamp:  now,
		Recipients: recipients,
	}, nil
}

func (p *MockProvider) defaultParse(_ context.Context, req ParseRequest) (ParseResponse, error) {
	if req.RawContent == "" {
		return ParseResponse{}, ErrInvalidInput
	}

	// Simple mock parsing - extract basic fields
	lines := strings.Split(req.RawContent, "\n")
	var from EmailAddress
	var to []EmailAddress
	var subject string
	var body strings.Builder
	var inBody bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" && !inBody {
			inBody = true
			continue
		}

		if !inBody {
			if strings.HasPrefix(line, "From:") {
				from = EmailAddress{Address: strings.TrimSpace(strings.TrimPrefix(line, "From:"))}
			} else if strings.HasPrefix(line, "To:") {
				addr := strings.TrimSpace(strings.TrimPrefix(line, "To:"))
				to = append(to, EmailAddress{Address: addr})
			} else if strings.HasPrefix(line, "Subject:") {
				subject = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
			}
		} else {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	return ParseResponse{
		From:    from,
		To:      to,
		Subject: subject,
		Body:    strings.TrimSpace(body.String()),
		Date:    time.Now(),
	}, nil
}

func (p *MockProvider) defaultRenderTemplate(_ context.Context, req RenderTemplateRequest) (RenderTemplateResponse, error) {
	var content string

	if req.TemplateContent != "" {
		content = req.TemplateContent
	} else if req.TemplateName != "" {
		p.mu.RLock()
		tmpl, ok := p.templates[req.TemplateName]
		p.mu.RUnlock()
		if !ok {
			return RenderTemplateResponse{}, ErrTemplateNotFound
		}
		content = tmpl.content
	} else {
		return RenderTemplateResponse{}, ErrInvalidInput
	}

	// Simple variable substitution
	rendered := content
	for key, value := range req.Variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		rendered = strings.ReplaceAll(rendered, placeholder, fmt.Sprintf("%v", value))
	}

	format := req.Format
	if format == "" {
		format = "text"
	}

	resp := RenderTemplateResponse{
		Success: true,
	}

	if format == "html" {
		resp.HTMLBody = rendered
	} else {
		resp.Body = rendered
	}

	return resp, nil
}

// AddTemplate adds a template for testing.
func (p *MockProvider) AddTemplate(name, content, format string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templates[name] = storedTemplate{
		name:    name,
		content: content,
		format:  format,
	}
}

// EmailCount returns the number of sent emails (for testing).
func (p *MockProvider) EmailCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.sentEmails)
}

// GetEmail returns a sent email by index (for testing).
func (p *MockProvider) GetEmail(index int) (storedEmail, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if index < 0 || index >= len(p.sentEmails) {
		return storedEmail{}, false
	}
	return p.sentEmails[index], true
}

// Ensure MockProvider implements Provider
var _ Provider = (*MockProvider)(nil)
