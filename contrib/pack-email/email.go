// Package email provides email sending tools for agent-go.
//
// This pack includes tools for email operations:
//   - email_send: Send an email message
//   - email_send_template: Send an email using a template
//   - email_send_bulk: Send bulk emails
//   - email_validate: Validate an email address
//   - email_list_templates: List available email templates
//
// Supports SMTP, SendGrid, Mailgun, AWS SES, and Postmark.
// Templates support variable substitution and HTML/plain text formats.
package email

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the email tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("email").
		WithDescription("Email sending and template tools").
		WithVersion("0.1.0").
		AddTools(
			emailSend(),
			emailSendTemplate(),
			emailSendBulk(),
			emailValidate(),
			emailListTemplates(),
		).
		AllowInState(agent.StateExplore, "email_validate", "email_list_templates").
		AllowInState(agent.StateAct, "email_send", "email_send_template", "email_send_bulk", "email_validate", "email_list_templates").
		Build()
}

func emailSend() tool.Tool {
	return tool.NewBuilder("email_send").
		WithDescription("Send an email message").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func emailSendTemplate() tool.Tool {
	return tool.NewBuilder("email_send_template").
		WithDescription("Send an email using a predefined template").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func emailSendBulk() tool.Tool {
	return tool.NewBuilder("email_send_bulk").
		WithDescription("Send bulk emails to multiple recipients").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func emailValidate() tool.Tool {
	return tool.NewBuilder("email_validate").
		WithDescription("Validate an email address format and deliverability").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func emailListTemplates() tool.Tool {
	return tool.NewBuilder("email_list_templates").
		WithDescription("List available email templates").
		ReadOnly().
		Cacheable().
		MustBuild()
}
