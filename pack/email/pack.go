package email

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the email pack.
type PackConfig struct {
	// Provider is the email provider to use.
	Provider Provider

	// DefaultFrom is the default sender address.
	DefaultFrom string

	// DefaultReplyTo is the default reply-to address.
	DefaultReplyTo string
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{}
}

// New creates a new email pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	return pack.NewBuilder("email").
		WithDescription("Tools for sending and parsing emails").
		WithVersion("1.0.0").
		AddTools(
			sendTool(cfg),
			parseTool(cfg),
			templateTool(cfg),
		).
		AllowInState(agent.StateExplore, "email_parse", "email_template").
		AllowInState(agent.StateAct, "email_send", "email_parse", "email_template").
		AllowInState(agent.StateValidate, "email_parse").
		Build()
}

// sendTool creates the email_send tool.
func sendTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("email_send").
		WithDescription("Send an email").
		WithTags("email", "communication", "notification").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				From        string            `json:"from"`
				To          []string          `json:"to"`
				CC          []string          `json:"cc"`
				BCC         []string          `json:"bcc"`
				ReplyTo     string            `json:"reply_to"`
				Subject     string            `json:"subject"`
				Body        string            `json:"body"`
				HTMLBody    string            `json:"html_body"`
				Attachments []Attachment      `json:"attachments"`
				Headers     map[string]string `json:"headers"`
				Priority    string            `json:"priority"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if len(req.To) == 0 {
				return tool.Result{}, ErrInvalidInput
			}

			from := req.From
			if from == "" {
				from = cfg.DefaultFrom
			}
			if from == "" {
				return tool.Result{}, ErrInvalidInput
			}

			replyTo := req.ReplyTo
			if replyTo == "" {
				replyTo = cfg.DefaultReplyTo
			}

			resp, err := cfg.Provider.Send(ctx, SendRequest{
				From:        from,
				To:          req.To,
				CC:          req.CC,
				BCC:         req.BCC,
				ReplyTo:     replyTo,
				Subject:     req.Subject,
				Body:        req.Body,
				HTMLBody:    req.HTMLBody,
				Attachments: req.Attachments,
				Headers:     req.Headers,
				Priority:    req.Priority,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseTool creates the email_parse tool.
func parseTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("email_parse").
		WithDescription("Parse an email from raw content").
		ReadOnly().
		WithTags("email", "parsing").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				RawContent         string `json:"raw_content"`
				ExtractAttachments bool   `json:"extract_attachments"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.RawContent == "" {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.Parse(ctx, ParseRequest{
				RawContent:         req.RawContent,
				ExtractAttachments: req.ExtractAttachments,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// templateTool creates the email_template tool.
func templateTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("email_template").
		WithDescription("Render an email template with variables").
		ReadOnly().
		Cacheable().
		WithTags("email", "template", "rendering").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				TemplateName    string                 `json:"template_name"`
				TemplateContent string                 `json:"template_content"`
				Variables       map[string]interface{} `json:"variables"`
				Format          string                 `json:"format"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			if req.TemplateName == "" && req.TemplateContent == "" {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.RenderTemplate(ctx, RenderTemplateRequest{
				TemplateName:    req.TemplateName,
				TemplateContent: req.TemplateContent,
				Variables:       req.Variables,
				Format:          req.Format,
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
