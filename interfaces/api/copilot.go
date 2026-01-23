package api

import (
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
)

// CopilotConfig configures the Copilot provider.
type CopilotConfig = planner.CopilotConfig

// CopilotProvider implements the Provider interface using GitHub Copilot SDK.
type CopilotProvider = planner.CopilotProvider

// StreamingCopilotProvider extends CopilotProvider with streaming capabilities.
type StreamingCopilotProvider = planner.StreamingCopilotProvider

// CopilotStream implements the Stream interface for Copilot responses.
type CopilotStream = planner.CopilotStream

// CopilotStreamHandler provides callback-based stream processing.
type CopilotStreamHandler = planner.CopilotStreamHandler

// CopilotInteractiveSession provides an interactive session with Copilot.
type CopilotInteractiveSession = planner.CopilotInteractiveSession

// NewCopilotProvider creates a new Copilot provider.
//
// Example:
//
//	provider, err := api.NewCopilotProvider(api.CopilotConfig{
//	    Model:        "gpt-4.1",
//	    Timeout:      120,
//	    MaxTokens:    4096,
//	    SystemPrompt: "You are a helpful coding assistant.",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Stop()
//
//	if err := provider.Start(); err != nil {
//	    log.Fatal(err)
//	}
func NewCopilotProvider(config CopilotConfig) (*CopilotProvider, error) {
	return planner.NewCopilotProvider(config)
}

// NewStreamingCopilotProvider creates a new streaming Copilot provider.
// The provider forces streaming mode regardless of the config setting.
//
// Example:
//
//	provider, err := api.NewStreamingCopilotProvider(api.CopilotConfig{
//	    Model: "gpt-4.1",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Stop()
//
//	stream, err := provider.CompleteStream(ctx, req)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
//
//	for {
//	    chunk, err := stream.Next()
//	    if err == io.EOF {
//	        break
//	    }
//	    fmt.Print(chunk.Delta.Content)
//	}
func NewStreamingCopilotProvider(config CopilotConfig) (*StreamingCopilotProvider, error) {
	return planner.NewStreamingCopilotProvider(config)
}

// NewCopilotInteractiveSession creates a new interactive session with Copilot.
// This is useful for multi-turn conversations where context needs to be preserved.
//
// Example:
//
//	provider, _ := api.NewCopilotProvider(config)
//	defer provider.Stop()
//	provider.Start()
//
//	session, err := api.NewCopilotInteractiveSession(provider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Close()
//
//	response, err := session.Send(ctx, "Hello, how are you?")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(response)
//
//	// Continue the conversation
//	response, err = session.Send(ctx, "What did I just ask you?")
func NewCopilotInteractiveSession(provider *CopilotProvider) (*CopilotInteractiveSession, error) {
	return planner.NewCopilotInteractiveSession(provider)
}

// ProcessCopilotStream processes a Copilot stream with callbacks.
// This is a convenience function for handling streaming responses.
//
// Example:
//
//	stream, _ := provider.CompleteStream(ctx, req)
//	response, err := api.ProcessCopilotStream(stream, &api.CopilotStreamHandler{
//	    OnContent: func(content string) {
//	        fmt.Print(content)
//	    },
//	    OnComplete: func() {
//	        fmt.Println("\n--- Done ---")
//	    },
//	    OnError: func(err error) {
//	        log.Printf("Error: %v", err)
//	    },
//	})
func ProcessCopilotStream(stream planner.Stream, handler *CopilotStreamHandler) (planner.CompletionResponse, error) {
	return planner.ProcessCopilotStream(stream, handler)
}
