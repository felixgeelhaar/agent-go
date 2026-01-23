package planner

import (
	"context"
	"errors"
	"io"
	"sync"

	copilot "github.com/github/copilot-sdk/go"
)

// StreamingCopilotProvider extends CopilotProvider with streaming capabilities.
type StreamingCopilotProvider struct {
	*CopilotProvider
}

// NewStreamingCopilotProvider creates a new streaming Copilot provider.
func NewStreamingCopilotProvider(config CopilotConfig) (*StreamingCopilotProvider, error) {
	// Force streaming mode
	config.Streaming = true

	provider, err := NewCopilotProvider(config)
	if err != nil {
		return nil, err
	}

	return &StreamingCopilotProvider{
		CopilotProvider: provider,
	}, nil
}

// SupportsStreaming returns true as this provider supports streaming.
func (p *StreamingCopilotProvider) SupportsStreaming() bool {
	return true
}

// CompleteStream streams a chat completion response.
func (p *StreamingCopilotProvider) CompleteStream(ctx context.Context, req CompletionRequest) (Stream, error) {
	// Create session with streaming enabled
	sessionConfig := &copilot.SessionConfig{
		Model:     p.getModel(req.Model),
		Streaming: true,
	}

	session, err := p.client.CreateSession(sessionConfig)
	if err != nil {
		return nil, err
	}

	// Build the prompt
	prompt, err := p.buildPrompt(req.Messages)
	if err != nil {
		_ = session.Destroy()
		return nil, err
	}

	// Create the stream
	stream := &CopilotStream{
		session:   session,
		chunks:    make(chan StreamChunk, 100),
		done:      make(chan struct{}),
		closeOnce: sync.Once{},
	}

	// Subscribe to events and forward to chunk channel
	unsubscribe := session.On(func(event copilot.SessionEvent) {
		select {
		case <-stream.done:
			return
		default:
		}

		switch event.Type {
		case copilot.AssistantMessageDelta:
			if event.Data.DeltaContent != nil {
				select {
				case stream.chunks <- StreamChunk{
					Delta: StreamDelta{
						Content: *event.Data.DeltaContent,
					},
				}:
				case <-stream.done:
				}
			}
		case copilot.AssistantReasoningDelta:
			// Forward reasoning deltas as well
			if event.Data.DeltaContent != nil {
				select {
				case stream.chunks <- StreamChunk{
					Delta: StreamDelta{
						Content: "[Reasoning] " + *event.Data.DeltaContent,
					},
				}:
				case <-stream.done:
				}
			}
		case copilot.AssistantMessage:
			// Final message - send finish marker
			if event.Data.Content != nil {
				select {
				case stream.chunks <- StreamChunk{
					Delta: StreamDelta{
						Content: *event.Data.Content,
					},
					FinishReason: "stop",
				}:
				case <-stream.done:
				}
			}
		case copilot.SessionIdle:
			stream.closeOnce.Do(func() {
				close(stream.done)
			})
		case copilot.SessionError:
			if event.Data.Message != nil {
				stream.err = errors.New(*event.Data.Message)
			} else {
				stream.err = errors.New("unknown session error")
			}
			stream.closeOnce.Do(func() {
				close(stream.done)
			})
		}
	})
	stream.unsubscribe = unsubscribe

	// Send the message to start streaming
	go func() {
		_, err := session.Send(copilot.MessageOptions{
			Prompt: prompt,
		})
		if err != nil {
			stream.err = err
			stream.closeOnce.Do(func() {
				close(stream.done)
			})
		}
	}()

	return stream, nil
}

// CopilotStream implements the Stream interface for Copilot responses.
type CopilotStream struct {
	session     *copilot.Session
	chunks      chan StreamChunk
	done        chan struct{}
	closeOnce   sync.Once
	unsubscribe func()
	err         error
	closed      bool
	mu          sync.Mutex
}

// Next returns the next chunk from the stream.
func (s *CopilotStream) Next() (StreamChunk, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return StreamChunk{}, ErrStreamClosed
	}
	s.mu.Unlock()

	select {
	case chunk, ok := <-s.chunks:
		if !ok {
			if s.err != nil {
				return StreamChunk{}, s.err
			}
			return StreamChunk{}, io.EOF
		}
		return chunk, nil
	case <-s.done:
		// Check if there are remaining chunks
		select {
		case chunk, ok := <-s.chunks:
			if ok {
				return chunk, nil
			}
		default:
		}
		if s.err != nil {
			return StreamChunk{}, s.err
		}
		return StreamChunk{}, io.EOF
	}
}

// Close closes the stream and releases resources.
func (s *CopilotStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	s.closeOnce.Do(func() {
		close(s.done)
	})

	if s.unsubscribe != nil {
		s.unsubscribe()
	}

	if s.session != nil {
		return s.session.Destroy()
	}

	return nil
}

// CopilotStreamHandler provides callback-based stream processing.
type CopilotStreamHandler struct {
	OnContent  func(content string)
	OnToolCall func(id, name, arguments string)
	OnComplete func()
	OnError    func(err error)
}

// ProcessCopilotStream processes a Copilot stream with callbacks.
func ProcessCopilotStream(stream Stream, handler *CopilotStreamHandler) (CompletionResponse, error) {
	var content string

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			if handler.OnComplete != nil {
				handler.OnComplete()
			}
			break
		}
		if err != nil {
			if handler.OnError != nil {
				handler.OnError(err)
			}
			return CompletionResponse{}, err
		}

		content += chunk.Delta.Content

		if handler.OnContent != nil && chunk.Delta.Content != "" {
			handler.OnContent(chunk.Delta.Content)
		}

		for _, tc := range chunk.Delta.ToolCalls {
			if handler.OnToolCall != nil && tc.Function != nil {
				handler.OnToolCall(tc.ID, tc.Function.Name, tc.Function.Arguments)
			}
		}
	}

	return CompletionResponse{
		Message: Message{
			Role:    "assistant",
			Content: content,
		},
	}, nil
}

// CopilotInteractiveSession provides an interactive session with Copilot.
// This is useful for multi-turn conversations where context needs to be preserved.
type CopilotInteractiveSession struct {
	provider *CopilotProvider
	session  *copilot.Session
	messages []Message
	mu       sync.Mutex
}

// NewCopilotInteractiveSession creates a new interactive session.
func NewCopilotInteractiveSession(provider *CopilotProvider) (*CopilotInteractiveSession, error) {
	session, err := provider.CreateSession(nil)
	if err != nil {
		return nil, err
	}

	return &CopilotInteractiveSession{
		provider: provider,
		session:  session,
		messages: make([]Message, 0),
	}, nil
}

// Send sends a message and returns the response.
func (s *CopilotInteractiveSession) Send(ctx context.Context, content string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add user message to history
	s.messages = append(s.messages, Message{
		Role:    "user",
		Content: content,
	})

	// Wait for response
	var response string
	done := make(chan struct{})
	var responseErr error

	unsubscribe := s.session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case copilot.AssistantMessage:
			if event.Data.Content != nil {
				response = *event.Data.Content
			}
		case copilot.AssistantMessageDelta:
			if event.Data.DeltaContent != nil {
				response += *event.Data.DeltaContent
			}
		case copilot.SessionIdle:
			close(done)
		case copilot.SessionError:
			if event.Data.Message != nil {
				responseErr = errors.New(*event.Data.Message)
			} else {
				responseErr = errors.New("unknown session error")
			}
			close(done)
		}
	})
	defer unsubscribe()

	_, err := s.session.Send(copilot.MessageOptions{
		Prompt: content,
	})
	if err != nil {
		return "", err
	}

	select {
	case <-done:
		if responseErr != nil {
			return "", responseErr
		}
	case <-ctx.Done():
		_ = s.session.Abort()
		return "", ctx.Err()
	}

	// Add assistant response to history
	s.messages = append(s.messages, Message{
		Role:    "assistant",
		Content: response,
	})

	return response, nil
}

// GetMessages returns the conversation history.
func (s *CopilotInteractiveSession) GetMessages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Close closes the interactive session.
func (s *CopilotInteractiveSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		return s.session.Destroy()
	}
	return nil
}
