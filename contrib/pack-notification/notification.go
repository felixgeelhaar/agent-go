// Package notification provides notification tools for agent-go.
//
// This pack includes tools for sending notifications:
//   - notify_slack: Send a Slack message
//   - notify_discord: Send a Discord message
//   - notify_teams: Send a Microsoft Teams message
//   - notify_webhook: Send a generic webhook notification
//   - notify_sms: Send an SMS message
//   - notify_push: Send a push notification
//   - notify_pagerduty: Create a PagerDuty incident
//
// Supports templating and variable substitution in messages.
package notification

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the notification tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("notification").
		WithDescription("Notification tools for alerts and messaging").
		WithVersion("0.1.0").
		AddTools(
			notifySlack(),
			notifyDiscord(),
			notifyTeams(),
			notifyWebhook(),
			notifySMS(),
			notifyPush(),
			notifyPagerDuty(),
		).
		AllowInState(agent.StateAct, "notify_slack", "notify_discord", "notify_teams", "notify_webhook", "notify_sms", "notify_push", "notify_pagerduty").
		Build()
}

func notifySlack() tool.Tool {
	return tool.NewBuilder("notify_slack").
		WithDescription("Send a message to a Slack channel").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func notifyDiscord() tool.Tool {
	return tool.NewBuilder("notify_discord").
		WithDescription("Send a message to a Discord channel").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func notifyTeams() tool.Tool {
	return tool.NewBuilder("notify_teams").
		WithDescription("Send a message to a Microsoft Teams channel").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func notifyWebhook() tool.Tool {
	return tool.NewBuilder("notify_webhook").
		WithDescription("Send a notification via generic webhook").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func notifySMS() tool.Tool {
	return tool.NewBuilder("notify_sms").
		WithDescription("Send an SMS message via Twilio or similar").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func notifyPush() tool.Tool {
	return tool.NewBuilder("notify_push").
		WithDescription("Send a push notification to mobile devices").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func notifyPagerDuty() tool.Tool {
	return tool.NewBuilder("notify_pagerduty").
		WithDescription("Create or update a PagerDuty incident").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}
