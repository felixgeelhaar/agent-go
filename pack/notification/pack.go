package notification

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PackConfig configures the notification pack.
type PackConfig struct {
	// Provider is the notification provider to use.
	Provider Provider

	// DefaultChannel is the default channel for notifications.
	DefaultChannel string

	// DefaultLevel is the default notification level.
	DefaultLevel string
}

// DefaultPackConfig returns default pack configuration.
func DefaultPackConfig() PackConfig {
	return PackConfig{
		DefaultLevel: "info",
	}
}

// New creates a new notification pack with the given configuration.
func New(cfg PackConfig) *pack.Pack {
	if cfg.DefaultLevel == "" {
		cfg.DefaultLevel = "info"
	}

	return pack.NewBuilder("notification").
		WithDescription("Tools for sending notifications").
		WithVersion("1.0.0").
		AddTools(
			sendTool(cfg),
			updateTool(cfg),
		).
		AllowInState(agent.StateAct, "notify_send", "notify_update").
		AllowInState(agent.StateValidate, "notify_send").
		Build()
}

// sendTool creates the notify_send tool.
func sendTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("notify_send").
		WithDescription("Send a notification to a channel").
		WithTags("notification", "alert", "messaging").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				Channel     string                 `json:"channel"`
				Message     string                 `json:"message"`
				Title       string                 `json:"title"`
				Level       string                 `json:"level"`
				ThreadID    string                 `json:"thread_id"`
				Attachments []Attachment           `json:"attachments"`
				Metadata    map[string]interface{} `json:"metadata"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			// Apply defaults
			channel := req.Channel
			if channel == "" {
				channel = cfg.DefaultChannel
			}

			if channel == "" || req.Message == "" {
				return tool.Result{}, ErrInvalidInput
			}

			level := req.Level
			if level == "" {
				level = cfg.DefaultLevel
			}

			resp, err := cfg.Provider.Send(ctx, SendRequest{
				Channel:     channel,
				Message:     req.Message,
				Title:       req.Title,
				Level:       level,
				ThreadID:    req.ThreadID,
				Attachments: req.Attachments,
				Metadata:    req.Metadata,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// updateTool creates the notify_update tool.
func updateTool(cfg PackConfig) tool.Tool {
	return tool.NewBuilder("notify_update").
		WithDescription("Update an existing notification").
		WithTags("notification", "update", "messaging").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			if cfg.Provider == nil {
				return tool.Result{}, ErrProviderNotConfigured
			}

			var req struct {
				MessageID string                 `json:"message_id"`
				Channel   string                 `json:"channel"`
				Message   string                 `json:"message"`
				Title     string                 `json:"title"`
				Metadata  map[string]interface{} `json:"metadata"`
			}

			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}

			// Apply defaults
			channel := req.Channel
			if channel == "" {
				channel = cfg.DefaultChannel
			}

			if req.MessageID == "" || channel == "" {
				return tool.Result{}, ErrInvalidInput
			}

			resp, err := cfg.Provider.Update(ctx, UpdateRequest{
				MessageID: req.MessageID,
				Channel:   channel,
				Message:   req.Message,
				Title:     req.Title,
				Metadata:  req.Metadata,
			})
			if err != nil {
				return tool.Result{}, err
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
