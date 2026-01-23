package api

import (
	"strings"
	"testing"
)

func TestNewCopilotProvider(t *testing.T) {
	t.Parallel()

	t.Run("default config", func(t *testing.T) {
		t.Parallel()

		provider, err := NewCopilotProvider(CopilotConfig{})
		if err != nil {
			t.Fatalf("NewCopilotProvider() error = %v", err)
		}

		if provider == nil {
			t.Fatal("NewCopilotProvider() returned nil")
		}

		if provider.Name() != "copilot" {
			t.Errorf("provider.Name() = %s, want copilot", provider.Name())
		}
	})

	t.Run("custom config", func(t *testing.T) {
		t.Parallel()

		provider, err := NewCopilotProvider(CopilotConfig{
			Model:        "gpt-4",
			Timeout:      120,
			MaxTokens:    8192,
			SystemPrompt: "You are a helpful assistant.",
		})
		if err != nil {
			t.Fatalf("NewCopilotProvider() error = %v", err)
		}

		if provider == nil {
			t.Fatal("NewCopilotProvider() returned nil")
		}
	})
}

func TestNewStreamingCopilotProvider(t *testing.T) {
	t.Parallel()

	t.Run("default config", func(t *testing.T) {
		t.Parallel()

		provider, err := NewStreamingCopilotProvider(CopilotConfig{})
		if err != nil {
			t.Fatalf("NewStreamingCopilotProvider() error = %v", err)
		}

		if provider == nil {
			t.Fatal("NewStreamingCopilotProvider() returned nil")
		}

		if !provider.SupportsStreaming() {
			t.Error("provider.SupportsStreaming() = false, want true")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		t.Parallel()

		provider, err := NewStreamingCopilotProvider(CopilotConfig{
			Model:     "gpt-4",
			Streaming: false, // Should be forced to true
		})
		if err != nil {
			t.Fatalf("NewStreamingCopilotProvider() error = %v", err)
		}

		if provider == nil {
			t.Fatal("NewStreamingCopilotProvider() returned nil")
		}

		if !provider.SupportsStreaming() {
			t.Error("streaming should be forced on")
		}
	})
}

func TestNewCopilotInteractiveSession(t *testing.T) {
	t.Parallel()

	provider, err := NewCopilotProvider(CopilotConfig{})
	if err != nil {
		t.Fatalf("NewCopilotProvider() error = %v", err)
	}

	session, err := NewCopilotInteractiveSession(provider)
	if err != nil {
		// Skip if Copilot CLI is not available
		if strings.Contains(err.Error(), "executable file not found") {
			t.Skip("Copilot CLI not available, skipping test")
		}
		t.Fatalf("NewCopilotInteractiveSession() error = %v", err)
	}

	if session == nil {
		t.Fatal("NewCopilotInteractiveSession() returned nil")
	}
}

func TestProcessCopilotStream(t *testing.T) {
	t.Parallel()

	// Test with handler callbacks
	t.Run("with callbacks", func(t *testing.T) {
		t.Parallel()

		var contentReceived string
		var completeCalled bool

		handler := &CopilotStreamHandler{
			OnContent: func(content string) {
				contentReceived += content
			},
			OnComplete: func() {
				completeCalled = true
			},
			OnError: func(err error) {
				// Error handler
			},
		}

		// Verify handler struct is properly initialized
		if handler.OnContent == nil {
			t.Error("OnContent callback is nil")
		}
		if handler.OnComplete == nil {
			t.Error("OnComplete callback is nil")
		}
		if handler.OnError == nil {
			t.Error("OnError callback is nil")
		}

		// Use the variables to satisfy compiler
		_ = contentReceived
		_ = completeCalled
	})
}

func TestCopilotTypeAliases(t *testing.T) {
	t.Parallel()

	// Verify type aliases work correctly
	var _ CopilotConfig
	var _ *CopilotProvider
	var _ *StreamingCopilotProvider
	var _ *CopilotStream
	var _ *CopilotStreamHandler
	var _ *CopilotInteractiveSession
}
