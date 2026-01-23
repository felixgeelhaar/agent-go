package notification

import (
	"context"
)

// Notifier defines the interface for sending notifications.
type Notifier interface {
	// Notify sends a notification event.
	Notify(ctx context.Context, event *Event) error

	// NotifyBatch sends multiple notification events.
	NotifyBatch(ctx context.Context, events []*Event) error

	// Close releases any resources held by the notifier.
	Close() error
}

// EventFilter defines a function that filters events.
// Returns true if the event should be sent, false to skip it.
type EventFilter func(event *Event) bool

// FilterByType returns a filter that only allows specified event types.
func FilterByType(types ...EventType) EventFilter {
	typeSet := make(map[EventType]bool)
	for _, t := range types {
		typeSet[t] = true
	}
	return func(event *Event) bool {
		return typeSet[event.Type]
	}
}

// FilterByRunID returns a filter that only allows events for specific run IDs.
func FilterByRunID(runIDs ...string) EventFilter {
	idSet := make(map[string]bool)
	for _, id := range runIDs {
		idSet[id] = true
	}
	return func(event *Event) bool {
		return idSet[event.RunID]
	}
}

// CombineFilters returns a filter that requires all provided filters to pass.
func CombineFilters(filters ...EventFilter) EventFilter {
	return func(event *Event) bool {
		for _, f := range filters {
			if !f(event) {
				return false
			}
		}
		return true
	}
}

// Endpoint represents a webhook endpoint configuration.
type Endpoint struct {
	// URL is the webhook endpoint URL.
	URL string `json:"url"`
	// Secret is the shared secret for HMAC signing.
	Secret string `json:"secret,omitempty"`
	// Headers are additional HTTP headers to include.
	Headers map[string]string `json:"headers,omitempty"`
	// Filter is an optional event filter for this endpoint.
	Filter EventFilter `json:"-"`
	// Enabled indicates if this endpoint is active.
	Enabled bool `json:"enabled"`
	// Name is an optional friendly name for the endpoint.
	Name string `json:"name,omitempty"`
}
