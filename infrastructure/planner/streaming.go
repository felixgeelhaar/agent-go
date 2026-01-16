package planner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

var (
	// ErrStreamClosed indicates the stream has been closed.
	ErrStreamClosed = errors.New("stream closed")

	// ErrStreamingNotSupported indicates the provider doesn't support streaming.
	ErrStreamingNotSupported = errors.New("streaming not supported")
)

// StreamingProvider extends Provider with streaming capabilities.
type StreamingProvider interface {
	Provider

	// CompleteStream streams a chat completion response.
	CompleteStream(ctx context.Context, req CompletionRequest) (Stream, error)

	// SupportsStreaming returns whether the provider supports streaming.
	SupportsStreaming() bool
}

// Stream represents a streaming response from an LLM provider.
type Stream interface {
	// Next returns the next chunk of the response.
	// Returns io.EOF when the stream is complete.
	Next() (StreamChunk, error)

	// Close closes the stream and releases resources.
	Close() error
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	// ID is the chunk ID (may be same across chunks).
	ID string `json:"id,omitempty"`

	// Delta contains the incremental content.
	Delta StreamDelta `json:"delta"`

	// FinishReason indicates why the stream finished (if applicable).
	FinishReason string `json:"finish_reason,omitempty"`

	// Usage contains token usage (typically only in the final chunk).
	Usage *Usage `json:"usage,omitempty"`
}

// StreamDelta contains the incremental content in a chunk.
type StreamDelta struct {
	// Role is set in the first chunk.
	Role string `json:"role,omitempty"`

	// Content is the incremental text content.
	Content string `json:"content,omitempty"`

	// ToolCalls contains incremental tool call information.
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCallDelta represents an incremental tool call update.
type ToolCallDelta struct {
	// Index identifies which tool call this delta belongs to.
	Index int `json:"index"`

	// ID is set in the first delta for this tool call.
	ID string `json:"id,omitempty"`

	// Type is set in the first delta for this tool call.
	Type string `json:"type,omitempty"`

	// Function contains incremental function information.
	Function *FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta represents an incremental function call update.
type FunctionCallDelta struct {
	// Name is set in the first delta for this function call.
	Name string `json:"name,omitempty"`

	// Arguments contains incremental JSON arguments.
	Arguments string `json:"arguments,omitempty"`
}

// StreamHandler processes streaming chunks.
type StreamHandler interface {
	// OnChunk is called for each chunk received.
	OnChunk(chunk StreamChunk) error

	// OnComplete is called when the stream completes successfully.
	OnComplete(response CompletionResponse) error

	// OnError is called if an error occurs during streaming.
	OnError(err error)
}

// StreamCollector collects streaming chunks into a complete response.
type StreamCollector struct {
	chunks      []StreamChunk
	content     string
	toolCalls   map[int]*ToolCall
	usage       *Usage
	finishReason string
}

// NewStreamCollector creates a new stream collector.
func NewStreamCollector() *StreamCollector {
	return &StreamCollector{
		chunks:    make([]StreamChunk, 0),
		toolCalls: make(map[int]*ToolCall),
	}
}

// OnChunk implements StreamHandler.
func (c *StreamCollector) OnChunk(chunk StreamChunk) error {
	c.chunks = append(c.chunks, chunk)

	// Accumulate content
	c.content += chunk.Delta.Content

	// Accumulate tool calls
	for _, tcDelta := range chunk.Delta.ToolCalls {
		tc, exists := c.toolCalls[tcDelta.Index]
		if !exists {
			tc = &ToolCall{
				ID:   tcDelta.ID,
				Type: tcDelta.Type,
			}
			c.toolCalls[tcDelta.Index] = tc
		}

		if tcDelta.Function != nil {
			if tc.Function.Name == "" && tcDelta.Function.Name != "" {
				tc.Function.Name = tcDelta.Function.Name
			}
			tc.Function.Arguments += tcDelta.Function.Arguments
		}
	}

	// Capture finish reason and usage from final chunk
	if chunk.FinishReason != "" {
		c.finishReason = chunk.FinishReason
	}
	if chunk.Usage != nil {
		c.usage = chunk.Usage
	}

	return nil
}

// OnComplete implements StreamHandler.
func (c *StreamCollector) OnComplete(response CompletionResponse) error {
	return nil
}

// OnError implements StreamHandler.
func (c *StreamCollector) OnError(err error) {}

// ToResponse converts collected chunks to a CompletionResponse.
func (c *StreamCollector) ToResponse() CompletionResponse {
	// Build tool calls slice
	var toolCalls []ToolCall
	if len(c.toolCalls) > 0 {
		toolCalls = make([]ToolCall, len(c.toolCalls))
		for idx, tc := range c.toolCalls {
			if idx < len(toolCalls) {
				toolCalls[idx] = *tc
			}
		}
	}

	resp := CompletionResponse{
		Message: Message{
			Role:      "assistant",
			Content:   c.content,
			ToolCalls: toolCalls,
		},
	}

	if c.usage != nil {
		resp.Usage = *c.usage
	}

	return resp
}

// CollectStream reads all chunks from a stream into a complete response.
func CollectStream(stream Stream) (CompletionResponse, error) {
	collector := NewStreamCollector()

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return CompletionResponse{}, err
		}
		if err := collector.OnChunk(chunk); err != nil {
			return CompletionResponse{}, err
		}
	}

	return collector.ToResponse(), nil
}

// StreamToChannel converts a Stream to a channel for async processing.
func StreamToChannel(ctx context.Context, stream Stream) <-chan StreamChunk {
	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)
		defer func() { _ = stream.Close() }()

		for {
			chunk, err := stream.Next()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// StreamCallback is a function called for each chunk.
type StreamCallback func(chunk StreamChunk) error

// ProcessStream reads a stream and calls the callback for each chunk.
func ProcessStream(stream Stream, callback StreamCallback) (CompletionResponse, error) {
	collector := NewStreamCollector()

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return CompletionResponse{}, err
		}

		if err := callback(chunk); err != nil {
			return CompletionResponse{}, err
		}

		if err := collector.OnChunk(chunk); err != nil {
			return CompletionResponse{}, err
		}
	}

	return collector.ToResponse(), nil
}

// JSONLineStream implements Stream for newline-delimited JSON responses.
type JSONLineStream struct {
	reader  io.ReadCloser
	decoder *json.Decoder
	closed  bool
}

// NewJSONLineStream creates a stream from a reader of newline-delimited JSON.
func NewJSONLineStream(r io.ReadCloser) *JSONLineStream {
	return &JSONLineStream{
		reader:  r,
		decoder: json.NewDecoder(r),
	}
}

// Next implements Stream.
func (s *JSONLineStream) Next() (StreamChunk, error) {
	if s.closed {
		return StreamChunk{}, ErrStreamClosed
	}

	var chunk StreamChunk
	if err := s.decoder.Decode(&chunk); err != nil {
		if err == io.EOF {
			return StreamChunk{}, io.EOF
		}
		return StreamChunk{}, err
	}

	return chunk, nil
}

// Close implements Stream.
func (s *JSONLineStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.reader.Close()
}

// SSEStream implements Stream for Server-Sent Events responses.
type SSEStream struct {
	reader   io.ReadCloser
	closed   bool
	eventBuf []byte
}

// NewSSEStream creates a stream from a Server-Sent Events reader.
func NewSSEStream(r io.ReadCloser) *SSEStream {
	return &SSEStream{
		reader:   r,
		eventBuf: make([]byte, 0, 4096),
	}
}

// Next implements Stream by parsing SSE events.
func (s *SSEStream) Next() (StreamChunk, error) {
	if s.closed {
		return StreamChunk{}, ErrStreamClosed
	}

	// Read until we get a complete event
	buf := make([]byte, 4096)
	for {
		n, err := s.reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return StreamChunk{}, io.EOF
			}
			return StreamChunk{}, err
		}

		s.eventBuf = append(s.eventBuf, buf[:n]...)

		// Look for complete event (double newline)
		if idx := findEventEnd(s.eventBuf); idx >= 0 {
			event := s.eventBuf[:idx]
			s.eventBuf = s.eventBuf[idx+2:] // Skip past the double newline

			// Parse the event
			chunk, err := parseSSEEvent(event)
			if err != nil {
				continue // Skip malformed events
			}

			// Check for [DONE] marker
			if chunk.FinishReason == "[DONE]" {
				return StreamChunk{}, io.EOF
			}

			return chunk, nil
		}
	}
}

// Close implements Stream.
func (s *SSEStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.reader.Close()
}

// findEventEnd finds the index of event terminator (\n\n).
func findEventEnd(data []byte) int {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			return i
		}
	}
	return -1
}

// parseSSEEvent parses a Server-Sent Event into a StreamChunk.
func parseSSEEvent(event []byte) (StreamChunk, error) {
	var chunk StreamChunk
	var dataLine []byte

	lines := splitLines(event)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Check for "data:" prefix
		if len(line) > 5 && string(line[:5]) == "data:" {
			dataLine = line[5:]
			// Trim leading space if present
			if len(dataLine) > 0 && dataLine[0] == ' ' {
				dataLine = dataLine[1:]
			}
		}
	}

	if len(dataLine) == 0 {
		return chunk, errors.New("no data in event")
	}

	// Check for [DONE] marker
	if string(dataLine) == "[DONE]" {
		chunk.FinishReason = "[DONE]"
		return chunk, nil
	}

	// Parse JSON data
	if err := json.Unmarshal(dataLine, &chunk); err != nil {
		return chunk, err
	}

	return chunk, nil
}

// splitLines splits data by newlines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
