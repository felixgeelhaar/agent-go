package planner

import (
	"errors"
	"io"
	"sync"
	"testing"
)

func TestCopilotConfig_Defaults(t *testing.T) {
	t.Parallel()

	config := CopilotConfig{}

	provider, err := NewCopilotProvider(config)
	if err != nil {
		t.Fatalf("NewCopilotProvider() error = %v", err)
	}

	if provider.config.Model != "gpt-4.1" {
		t.Errorf("config.Model = %s, want gpt-4.1", provider.config.Model)
	}
	if provider.config.Timeout != 120 {
		t.Errorf("config.Timeout = %d, want 120", provider.config.Timeout)
	}
	if provider.config.MaxTokens != 1024 {
		t.Errorf("config.MaxTokens = %d, want 1024", provider.config.MaxTokens)
	}
	if provider.config.LogLevel != "error" {
		t.Errorf("config.LogLevel = %s, want error", provider.config.LogLevel)
	}
}

func TestCopilotConfig_Custom(t *testing.T) {
	t.Parallel()

	config := CopilotConfig{
		Model:        "gpt-5",
		Streaming:    true,
		Timeout:      60,
		MaxTokens:    2048,
		SystemPrompt: "You are a helpful assistant.",
		LogLevel:     "debug",
	}

	provider, err := NewCopilotProvider(config)
	if err != nil {
		t.Fatalf("NewCopilotProvider() error = %v", err)
	}

	if provider.config.Model != "gpt-5" {
		t.Errorf("config.Model = %s, want gpt-5", provider.config.Model)
	}
	if provider.config.Streaming != true {
		t.Errorf("config.Streaming = %v, want true", provider.config.Streaming)
	}
	if provider.config.Timeout != 60 {
		t.Errorf("config.Timeout = %d, want 60", provider.config.Timeout)
	}
	if provider.config.MaxTokens != 2048 {
		t.Errorf("config.MaxTokens = %d, want 2048", provider.config.MaxTokens)
	}
	if provider.config.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("config.SystemPrompt = %s, want 'You are a helpful assistant.'", provider.config.SystemPrompt)
	}
	if provider.config.LogLevel != "debug" {
		t.Errorf("config.LogLevel = %s, want debug", provider.config.LogLevel)
	}
}

func TestCopilotProvider_Name(t *testing.T) {
	t.Parallel()

	provider, err := NewCopilotProvider(CopilotConfig{})
	if err != nil {
		t.Fatalf("NewCopilotProvider() error = %v", err)
	}

	if provider.Name() != "copilot" {
		t.Errorf("Name() = %s, want copilot", provider.Name())
	}
}

func TestCopilotProvider_GetModel(t *testing.T) {
	t.Parallel()

	provider, err := NewCopilotProvider(CopilotConfig{Model: "gpt-4.1"})
	if err != nil {
		t.Fatalf("NewCopilotProvider() error = %v", err)
	}

	t.Run("uses request model when provided", func(t *testing.T) {
		t.Parallel()
		model := provider.getModel("custom-model")
		if model != "custom-model" {
			t.Errorf("getModel() = %s, want custom-model", model)
		}
	})

	t.Run("falls back to config model", func(t *testing.T) {
		t.Parallel()
		model := provider.getModel("")
		if model != "gpt-4.1" {
			t.Errorf("getModel() = %s, want gpt-4.1", model)
		}
	})
}

func TestCopilotProvider_BuildPrompt(t *testing.T) {
	t.Parallel()

	t.Run("empty messages", func(t *testing.T) {
		t.Parallel()

		provider, _ := NewCopilotProvider(CopilotConfig{})
		_, err := provider.buildPrompt([]Message{})
		if err == nil {
			t.Error("buildPrompt() expected error for empty messages")
		}
	})

	t.Run("single user message", func(t *testing.T) {
		t.Parallel()

		provider, _ := NewCopilotProvider(CopilotConfig{})
		prompt, err := provider.buildPrompt([]Message{
			{Role: "user", Content: "Hello"},
		})
		if err != nil {
			t.Fatalf("buildPrompt() error = %v", err)
		}
		expected := "User: Hello\n\n"
		if prompt != expected {
			t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
		}
	})

	t.Run("system message", func(t *testing.T) {
		t.Parallel()

		provider, _ := NewCopilotProvider(CopilotConfig{})
		prompt, err := provider.buildPrompt([]Message{
			{Role: "system", Content: "Be helpful"},
			{Role: "user", Content: "Hello"},
		})
		if err != nil {
			t.Fatalf("buildPrompt() error = %v", err)
		}
		expected := "System: Be helpful\n\nUser: Hello\n\n"
		if prompt != expected {
			t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
		}
	})

	t.Run("with config system prompt", func(t *testing.T) {
		t.Parallel()

		provider, _ := NewCopilotProvider(CopilotConfig{
			SystemPrompt: "Default system prompt",
		})
		prompt, err := provider.buildPrompt([]Message{
			{Role: "user", Content: "Hello"},
		})
		if err != nil {
			t.Fatalf("buildPrompt() error = %v", err)
		}
		expected := "System: Default system prompt\n\nUser: Hello\n\n"
		if prompt != expected {
			t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
		}
	})

	t.Run("multi-turn conversation", func(t *testing.T) {
		t.Parallel()

		provider, _ := NewCopilotProvider(CopilotConfig{})
		prompt, err := provider.buildPrompt([]Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		})
		if err != nil {
			t.Fatalf("buildPrompt() error = %v", err)
		}
		expected := "User: Hello\n\nAssistant: Hi there!\n\nUser: How are you?\n\n"
		if prompt != expected {
			t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
		}
	})

	t.Run("tool result message", func(t *testing.T) {
		t.Parallel()

		provider, _ := NewCopilotProvider(CopilotConfig{})
		prompt, err := provider.buildPrompt([]Message{
			{Role: "user", Content: "Read file"},
			{Role: "tool", Name: "read_file", Content: "file contents"},
		})
		if err != nil {
			t.Fatalf("buildPrompt() error = %v", err)
		}
		expected := "User: Read file\n\nTool Result (read_file): file contents\n\n"
		if prompt != expected {
			t.Errorf("buildPrompt() = %q, want %q", prompt, expected)
		}
	})
}

func TestStreamingCopilotProvider_SupportsStreaming(t *testing.T) {
	t.Parallel()

	provider, err := NewStreamingCopilotProvider(CopilotConfig{})
	if err != nil {
		t.Fatalf("NewStreamingCopilotProvider() error = %v", err)
	}

	if !provider.SupportsStreaming() {
		t.Error("SupportsStreaming() = false, want true")
	}
}

func TestStreamingCopilotProvider_ForcesStreaming(t *testing.T) {
	t.Parallel()

	// Even if streaming is false in config, it should be forced to true
	provider, err := NewStreamingCopilotProvider(CopilotConfig{
		Streaming: false,
	})
	if err != nil {
		t.Fatalf("NewStreamingCopilotProvider() error = %v", err)
	}

	if !provider.config.Streaming {
		t.Error("config.Streaming = false, want true (should be forced)")
	}
}

func TestCopilotStream_ClosedState(t *testing.T) {
	t.Parallel()

	stream := &CopilotStream{
		chunks:    make(chan StreamChunk, 10),
		done:      make(chan struct{}),
		closeOnce: sync.Once{},
	}

	// Close the stream
	err := stream.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify stream is closed
	_, err = stream.Next()
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("Next() after Close() error = %v, want ErrStreamClosed", err)
	}

	// Close again should be idempotent
	err = stream.Close()
	if err != nil {
		t.Errorf("Close() second time error = %v, want nil", err)
	}
}

func TestCopilotStream_ChunkDelivery(t *testing.T) {
	t.Parallel()

	stream := &CopilotStream{
		chunks:    make(chan StreamChunk, 10),
		done:      make(chan struct{}),
		closeOnce: sync.Once{},
	}

	// Send a chunk
	stream.chunks <- StreamChunk{
		Delta: StreamDelta{Content: "Hello"},
	}

	// Receive the chunk
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if chunk.Delta.Content != "Hello" {
		t.Errorf("chunk.Delta.Content = %s, want Hello", chunk.Delta.Content)
	}
}

func TestCopilotStream_EOFOnDone(t *testing.T) {
	t.Parallel()

	stream := &CopilotStream{
		chunks:    make(chan StreamChunk, 10),
		done:      make(chan struct{}),
		closeOnce: sync.Once{},
	}

	// Signal done
	close(stream.done)

	// Next should return EOF
	_, err := stream.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("Next() after done error = %v, want io.EOF", err)
	}
}

func TestCopilotStream_ErrorPropagation(t *testing.T) {
	t.Parallel()

	stream := &CopilotStream{
		chunks:    make(chan StreamChunk, 10),
		done:      make(chan struct{}),
		closeOnce: sync.Once{},
		err:       errors.New("test error"),
	}

	// Signal done
	close(stream.done)

	// Next should return the error
	_, err := stream.Next()
	if err == nil || err.Error() != "test error" {
		t.Errorf("Next() error = %v, want 'test error'", err)
	}
}

func TestProcessCopilotStream(t *testing.T) {
	t.Parallel()

	// Create a mock stream
	chunks := make(chan StreamChunk, 10)
	done := make(chan struct{})
	stream := &CopilotStream{
		chunks:    chunks,
		done:      done,
		closeOnce: sync.Once{},
	}

	// Send chunks
	go func() {
		chunks <- StreamChunk{Delta: StreamDelta{Content: "Hello"}}
		chunks <- StreamChunk{Delta: StreamDelta{Content: " World"}}
		chunks <- StreamChunk{Delta: StreamDelta{Content: "!"}, FinishReason: "stop"}
		close(done)
	}()

	var collectedContent string
	var completeCalled bool

	response, err := ProcessCopilotStream(stream, &CopilotStreamHandler{
		OnContent: func(content string) {
			collectedContent += content
		},
		OnComplete: func() {
			completeCalled = true
		},
	})

	if err != nil {
		t.Fatalf("ProcessCopilotStream() error = %v", err)
	}

	if collectedContent != "Hello World!" {
		t.Errorf("collected content = %q, want 'Hello World!'", collectedContent)
	}

	if !completeCalled {
		t.Error("OnComplete was not called")
	}

	if response.Message.Content != "Hello World!" {
		t.Errorf("response.Message.Content = %q, want 'Hello World!'", response.Message.Content)
	}

	if response.Message.Role != "assistant" {
		t.Errorf("response.Message.Role = %s, want assistant", response.Message.Role)
	}
}

func TestCopilotInteractiveSession_GetMessages(t *testing.T) {
	t.Parallel()

	// Create a mock session to test the message accumulation logic
	session := &CopilotInteractiveSession{
		messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}

	messages := session.GetMessages()

	if len(messages) != 2 {
		t.Fatalf("len(GetMessages()) = %d, want 2", len(messages))
	}

	if messages[0].Role != "user" || messages[0].Content != "Hello" {
		t.Errorf("messages[0] = %v, want {Role: user, Content: Hello}", messages[0])
	}

	if messages[1].Role != "assistant" || messages[1].Content != "Hi there!" {
		t.Errorf("messages[1] = %v, want {Role: assistant, Content: Hi there!}", messages[1])
	}

	// Verify returned slice is a copy
	messages[0].Content = "Modified"
	if session.messages[0].Content == "Modified" {
		t.Error("GetMessages() returned reference instead of copy")
	}
}

func TestCopilotConfig_CLIOptions(t *testing.T) {
	t.Parallel()

	t.Run("with CLIPath", func(t *testing.T) {
		t.Parallel()

		provider, err := NewCopilotProvider(CopilotConfig{
			CLIPath: "/usr/local/bin/copilot",
		})
		if err != nil {
			t.Fatalf("NewCopilotProvider() error = %v", err)
		}

		if provider.config.CLIPath != "/usr/local/bin/copilot" {
			t.Errorf("config.CLIPath = %s, want /usr/local/bin/copilot", provider.config.CLIPath)
		}
	})

	t.Run("with CLIUrl", func(t *testing.T) {
		t.Parallel()

		provider, err := NewCopilotProvider(CopilotConfig{
			CLIUrl: "localhost:8080",
		})
		if err != nil {
			t.Fatalf("NewCopilotProvider() error = %v", err)
		}

		if provider.config.CLIUrl != "localhost:8080" {
			t.Errorf("config.CLIUrl = %s, want localhost:8080", provider.config.CLIUrl)
		}
	})

	t.Run("with Cwd", func(t *testing.T) {
		t.Parallel()

		provider, err := NewCopilotProvider(CopilotConfig{
			Cwd: "/home/user/project",
		})
		if err != nil {
			t.Fatalf("NewCopilotProvider() error = %v", err)
		}

		if provider.config.Cwd != "/home/user/project" {
			t.Errorf("config.Cwd = %s, want /home/user/project", provider.config.Cwd)
		}
	})
}
