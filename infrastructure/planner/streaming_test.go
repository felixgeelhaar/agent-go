package planner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// mockStream implements Stream for testing.
type mockStream struct {
	chunks []StreamChunk
	index  int
	closed bool
}

func newMockStream(chunks ...StreamChunk) *mockStream {
	return &mockStream{
		chunks: chunks,
	}
}

func (s *mockStream) Next() (StreamChunk, error) {
	if s.closed {
		return StreamChunk{}, ErrStreamClosed
	}
	if s.index >= len(s.chunks) {
		return StreamChunk{}, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *mockStream) Close() error {
	s.closed = true
	return nil
}

// errorStream is a stream that returns an error.
type errorStream struct {
	err error
}

func (s *errorStream) Next() (StreamChunk, error) {
	return StreamChunk{}, s.err
}

func (s *errorStream) Close() error {
	return nil
}

func TestNewStreamCollector(t *testing.T) {
	t.Parallel()

	collector := NewStreamCollector()

	if collector == nil {
		t.Fatal("NewStreamCollector() returned nil")
	}
	if collector.chunks == nil {
		t.Error("chunks slice not initialized")
	}
	if collector.toolCalls == nil {
		t.Error("toolCalls map not initialized")
	}
	if collector.content != "" {
		t.Error("content should be empty initially")
	}
}

func TestStreamCollector_OnChunk(t *testing.T) {
	t.Parallel()

	t.Run("accumulates content", func(t *testing.T) {
		t.Parallel()

		collector := NewStreamCollector()

		err := collector.OnChunk(StreamChunk{
			Delta: StreamDelta{Content: "Hello "},
		})
		if err != nil {
			t.Fatalf("OnChunk() error = %v", err)
		}

		err = collector.OnChunk(StreamChunk{
			Delta: StreamDelta{Content: "World!"},
		})
		if err != nil {
			t.Fatalf("OnChunk() error = %v", err)
		}

		if collector.content != "Hello World!" {
			t.Errorf("content = %q, want 'Hello World!'", collector.content)
		}
	})

	t.Run("accumulates tool calls", func(t *testing.T) {
		t.Parallel()

		collector := NewStreamCollector()

		// First chunk with tool call start
		err := collector.OnChunk(StreamChunk{
			Delta: StreamDelta{
				ToolCalls: []ToolCallDelta{
					{
						Index: 0,
						ID:    "call-123",
						Type:  "function",
						Function: &FunctionCallDelta{
							Name: "read_file",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("OnChunk() error = %v", err)
		}

		// Second chunk with arguments
		err = collector.OnChunk(StreamChunk{
			Delta: StreamDelta{
				ToolCalls: []ToolCallDelta{
					{
						Index: 0,
						Function: &FunctionCallDelta{
							Arguments: `{"path":`,
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("OnChunk() error = %v", err)
		}

		// Third chunk with more arguments
		err = collector.OnChunk(StreamChunk{
			Delta: StreamDelta{
				ToolCalls: []ToolCallDelta{
					{
						Index: 0,
						Function: &FunctionCallDelta{
							Arguments: `"/test.txt"}`,
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("OnChunk() error = %v", err)
		}

		tc, exists := collector.toolCalls[0]
		if !exists {
			t.Fatal("tool call not accumulated")
		}
		if tc.ID != "call-123" {
			t.Errorf("tool call ID = %s, want call-123", tc.ID)
		}
		if tc.Function.Name != "read_file" {
			t.Errorf("function name = %s, want read_file", tc.Function.Name)
		}
		if tc.Function.Arguments != `{"path":"/test.txt"}` {
			t.Errorf("arguments = %s, want {\"path\":\"/test.txt\"}", tc.Function.Arguments)
		}
	})

	t.Run("captures finish reason and usage", func(t *testing.T) {
		t.Parallel()

		collector := NewStreamCollector()

		err := collector.OnChunk(StreamChunk{
			FinishReason: "stop",
			Usage: &Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		})
		if err != nil {
			t.Fatalf("OnChunk() error = %v", err)
		}

		if collector.finishReason != "stop" {
			t.Errorf("finishReason = %s, want stop", collector.finishReason)
		}
		if collector.usage == nil {
			t.Fatal("usage not captured")
		}
		if collector.usage.TotalTokens != 30 {
			t.Errorf("usage.TotalTokens = %d, want 30", collector.usage.TotalTokens)
		}
	})
}

func TestStreamCollector_OnComplete(t *testing.T) {
	t.Parallel()

	collector := NewStreamCollector()
	err := collector.OnComplete(CompletionResponse{})
	if err != nil {
		t.Errorf("OnComplete() error = %v, want nil", err)
	}
}

func TestStreamCollector_OnError(t *testing.T) {
	t.Parallel()

	collector := NewStreamCollector()
	// OnError should not panic
	collector.OnError(errors.New("test error"))
}

func TestStreamCollector_ToResponse(t *testing.T) {
	t.Parallel()

	t.Run("builds response with content only", func(t *testing.T) {
		t.Parallel()

		collector := NewStreamCollector()
		_ = collector.OnChunk(StreamChunk{
			Delta: StreamDelta{Content: "Hello World!"},
		})

		resp := collector.ToResponse()

		if resp.Message.Role != "assistant" {
			t.Errorf("Message.Role = %s, want assistant", resp.Message.Role)
		}
		if resp.Message.Content != "Hello World!" {
			t.Errorf("Message.Content = %s, want 'Hello World!'", resp.Message.Content)
		}
	})

	t.Run("builds response with usage", func(t *testing.T) {
		t.Parallel()

		collector := NewStreamCollector()
		_ = collector.OnChunk(StreamChunk{
			Usage: &Usage{TotalTokens: 50},
		})

		resp := collector.ToResponse()

		if resp.Usage.TotalTokens != 50 {
			t.Errorf("Usage.TotalTokens = %d, want 50", resp.Usage.TotalTokens)
		}
	})

	t.Run("builds response with tool calls", func(t *testing.T) {
		t.Parallel()

		collector := NewStreamCollector()
		_ = collector.OnChunk(StreamChunk{
			Delta: StreamDelta{
				ToolCalls: []ToolCallDelta{
					{
						Index: 0,
						ID:    "call-1",
						Type:  "function",
						Function: &FunctionCallDelta{
							Name:      "test_func",
							Arguments: `{"arg": "value"}`,
						},
					},
				},
			},
		})

		resp := collector.ToResponse()

		if len(resp.Message.ToolCalls) != 1 {
			t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.Message.ToolCalls))
		}
		if resp.Message.ToolCalls[0].ID != "call-1" {
			t.Errorf("ToolCalls[0].ID = %s, want call-1", resp.Message.ToolCalls[0].ID)
		}
	})
}

func TestCollectStream(t *testing.T) {
	t.Parallel()

	t.Run("collects all chunks", func(t *testing.T) {
		t.Parallel()

		stream := newMockStream(
			StreamChunk{Delta: StreamDelta{Content: "Hello "}},
			StreamChunk{Delta: StreamDelta{Content: "World!"}},
		)

		resp, err := CollectStream(stream)
		if err != nil {
			t.Fatalf("CollectStream() error = %v", err)
		}

		if resp.Message.Content != "Hello World!" {
			t.Errorf("content = %s, want 'Hello World!'", resp.Message.Content)
		}
	})

	t.Run("handles stream error", func(t *testing.T) {
		t.Parallel()

		stream := &errorStream{err: errors.New("stream error")}

		_, err := CollectStream(stream)
		if err == nil {
			t.Error("CollectStream() expected error, got nil")
		}
	})
}

func TestStreamToChannel(t *testing.T) {
	t.Parallel()

	t.Run("sends chunks to channel", func(t *testing.T) {
		t.Parallel()

		stream := newMockStream(
			StreamChunk{Delta: StreamDelta{Content: "A"}},
			StreamChunk{Delta: StreamDelta{Content: "B"}},
			StreamChunk{Delta: StreamDelta{Content: "C"}},
		)

		ctx := context.Background()
		ch := StreamToChannel(ctx, stream)

		var content string
		for chunk := range ch {
			content += chunk.Delta.Content
		}

		if content != "ABC" {
			t.Errorf("content = %s, want ABC", content)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		// Create a stream with many chunks
		chunks := make([]StreamChunk, 100)
		for i := range chunks {
			chunks[i] = StreamChunk{Delta: StreamDelta{Content: "X"}}
		}
		stream := newMockStream(chunks...)

		ctx, cancel := context.WithCancel(context.Background())

		ch := StreamToChannel(ctx, stream)

		// Read one chunk then cancel
		<-ch
		cancel()

		// Give goroutine time to exit
		time.Sleep(10 * time.Millisecond)

		// Channel should be closed eventually
		timeout := time.After(100 * time.Millisecond)
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					// Channel closed, test passes
					return
				}
			case <-timeout:
				t.Error("channel not closed after context cancellation")
				return
			}
		}
	})
}

func TestProcessStream(t *testing.T) {
	t.Parallel()

	t.Run("calls callback for each chunk", func(t *testing.T) {
		t.Parallel()

		stream := newMockStream(
			StreamChunk{Delta: StreamDelta{Content: "A"}},
			StreamChunk{Delta: StreamDelta{Content: "B"}},
		)

		var callCount int
		callback := func(chunk StreamChunk) error {
			callCount++
			return nil
		}

		resp, err := ProcessStream(stream, callback)
		if err != nil {
			t.Fatalf("ProcessStream() error = %v", err)
		}

		if callCount != 2 {
			t.Errorf("callback called %d times, want 2", callCount)
		}
		if resp.Message.Content != "AB" {
			t.Errorf("content = %s, want AB", resp.Message.Content)
		}
	})

	t.Run("stops on callback error", func(t *testing.T) {
		t.Parallel()

		stream := newMockStream(
			StreamChunk{Delta: StreamDelta{Content: "A"}},
			StreamChunk{Delta: StreamDelta{Content: "B"}},
		)

		testErr := errors.New("callback error")
		callback := func(chunk StreamChunk) error {
			return testErr
		}

		_, err := ProcessStream(stream, callback)
		if !errors.Is(err, testErr) {
			t.Errorf("error = %v, want %v", err, testErr)
		}
	})

	t.Run("handles stream error", func(t *testing.T) {
		t.Parallel()

		streamErr := errors.New("stream error")
		stream := &errorStream{err: streamErr}

		callback := func(chunk StreamChunk) error {
			return nil
		}

		_, err := ProcessStream(stream, callback)
		if !errors.Is(err, streamErr) {
			t.Errorf("error = %v, want %v", err, streamErr)
		}
	})
}

func TestJSONLineStream(t *testing.T) {
	t.Parallel()

	t.Run("parses newline-delimited JSON", func(t *testing.T) {
		t.Parallel()

		data := `{"delta":{"content":"Hello "}}
{"delta":{"content":"World!"}}
`
		reader := io.NopCloser(strings.NewReader(data))
		stream := NewJSONLineStream(reader)

		chunk1, err := stream.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if chunk1.Delta.Content != "Hello " {
			t.Errorf("chunk1.content = %s, want 'Hello '", chunk1.Delta.Content)
		}

		chunk2, err := stream.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if chunk2.Delta.Content != "World!" {
			t.Errorf("chunk2.content = %s, want 'World!'", chunk2.Delta.Content)
		}

		_, err = stream.Next()
		if err != io.EOF {
			t.Errorf("Next() error = %v, want io.EOF", err)
		}
	})

	t.Run("returns error when closed", func(t *testing.T) {
		t.Parallel()

		data := `{"delta":{"content":"Test"}}`
		reader := io.NopCloser(strings.NewReader(data))
		stream := NewJSONLineStream(reader)

		_ = stream.Close()

		_, err := stream.Next()
		if !errors.Is(err, ErrStreamClosed) {
			t.Errorf("Next() error = %v, want ErrStreamClosed", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		t.Parallel()

		reader := io.NopCloser(strings.NewReader(""))
		stream := NewJSONLineStream(reader)

		err1 := stream.Close()
		err2 := stream.Close()

		if err1 != nil {
			t.Errorf("first Close() error = %v", err1)
		}
		if err2 != nil {
			t.Errorf("second Close() error = %v", err2)
		}
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		t.Parallel()

		data := `{invalid json}`
		reader := io.NopCloser(strings.NewReader(data))
		stream := NewJSONLineStream(reader)

		_, err := stream.Next()
		if err == nil {
			t.Error("Next() expected error for invalid JSON")
		}
	})
}

// slowReader returns data one byte at a time to simulate streaming.
type slowReader struct {
	data []byte
	pos  int
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	// Return one byte at a time
	p[0] = r.data[r.pos]
	r.pos++
	return 1, nil
}

func (r *slowReader) Close() error {
	return nil
}

func TestSSEStream(t *testing.T) {
	t.Parallel()

	t.Run("parses SSE events", func(t *testing.T) {
		t.Parallel()

		// Use slowReader to simulate byte-by-byte streaming
		data := "data: {\"delta\":{\"content\":\"Hello \"}}\n\ndata: {\"delta\":{\"content\":\"World!\"}}\n\n"
		reader := &slowReader{data: []byte(data)}
		stream := NewSSEStream(reader)

		chunk1, err := stream.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if chunk1.Delta.Content != "Hello " {
			t.Errorf("chunk1.content = %s, want 'Hello '", chunk1.Delta.Content)
		}

		chunk2, err := stream.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if chunk2.Delta.Content != "World!" {
			t.Errorf("chunk2.content = %s, want 'World!'", chunk2.Delta.Content)
		}
	})

	t.Run("handles [DONE] marker", func(t *testing.T) {
		t.Parallel()

		data := "data: {\"delta\":{\"content\":\"Test\"}}\n\ndata: [DONE]\n\n"
		reader := &slowReader{data: []byte(data)}
		stream := NewSSEStream(reader)

		_, err := stream.Next()
		if err != nil {
			t.Fatalf("first Next() error = %v", err)
		}

		_, err = stream.Next()
		if err != io.EOF {
			t.Errorf("Next() after [DONE] error = %v, want io.EOF", err)
		}
	})

	t.Run("returns error when closed", func(t *testing.T) {
		t.Parallel()

		data := "data: {\"delta\":{\"content\":\"Test\"}}\n\n"
		reader := &slowReader{data: []byte(data)}
		stream := NewSSEStream(reader)

		_ = stream.Close()

		_, err := stream.Next()
		if !errors.Is(err, ErrStreamClosed) {
			t.Errorf("Next() error = %v, want ErrStreamClosed", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		t.Parallel()

		reader := io.NopCloser(strings.NewReader(""))
		stream := NewSSEStream(reader)

		err1 := stream.Close()
		err2 := stream.Close()

		if err1 != nil {
			t.Errorf("first Close() error = %v", err1)
		}
		if err2 != nil {
			t.Errorf("second Close() error = %v", err2)
		}
	})

	t.Run("handles data with leading space", func(t *testing.T) {
		t.Parallel()

		// Some SSE implementations use "data: " with a space
		data := "data: {\"delta\":{\"content\":\"Spaced\"}}\n\n"
		reader := &slowReader{data: []byte(data)}
		stream := NewSSEStream(reader)

		chunk, err := stream.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if chunk.Delta.Content != "Spaced" {
			t.Errorf("content = %s, want Spaced", chunk.Delta.Content)
		}
	})
}

func TestFindEventEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     []byte
		expected int
	}{
		{
			name:     "finds double newline",
			data:     []byte("data: test\n\nmore"),
			expected: 10,
		},
		{
			name:     "no double newline",
			data:     []byte("data: test\nmore"),
			expected: -1,
		},
		{
			name:     "at start",
			data:     []byte("\n\ndata"),
			expected: 0,
		},
		{
			name:     "empty data",
			data:     []byte(""),
			expected: -1,
		},
		{
			name:     "single byte",
			data:     []byte("\n"),
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := findEventEnd(tt.data)
			if result != tt.expected {
				t.Errorf("findEventEnd() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestParseSSEEvent(t *testing.T) {
	t.Parallel()

	t.Run("parses valid event", func(t *testing.T) {
		t.Parallel()

		event := []byte(`data: {"delta":{"content":"Test"}}`)
		chunk, err := parseSSEEvent(event)
		if err != nil {
			t.Fatalf("parseSSEEvent() error = %v", err)
		}
		if chunk.Delta.Content != "Test" {
			t.Errorf("content = %s, want Test", chunk.Delta.Content)
		}
	})

	t.Run("handles [DONE] marker", func(t *testing.T) {
		t.Parallel()

		event := []byte("data: [DONE]")
		chunk, err := parseSSEEvent(event)
		if err != nil {
			t.Fatalf("parseSSEEvent() error = %v", err)
		}
		if chunk.FinishReason != "[DONE]" {
			t.Errorf("finishReason = %s, want [DONE]", chunk.FinishReason)
		}
	})

	t.Run("returns error for no data", func(t *testing.T) {
		t.Parallel()

		event := []byte("event: test\n")
		_, err := parseSSEEvent(event)
		if err == nil {
			t.Error("parseSSEEvent() expected error for event without data")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		event := []byte("data: {invalid}")
		_, err := parseSSEEvent(event)
		if err == nil {
			t.Error("parseSSEEvent() expected error for invalid JSON")
		}
	})

	t.Run("skips empty lines", func(t *testing.T) {
		t.Parallel()

		event := []byte("\n\ndata: {\"delta\":{\"content\":\"Test\"}}\n")
		chunk, err := parseSSEEvent(event)
		if err != nil {
			t.Fatalf("parseSSEEvent() error = %v", err)
		}
		if chunk.Delta.Content != "Test" {
			t.Errorf("content = %s, want Test", chunk.Delta.Content)
		}
	})
}

func TestSplitLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     []byte
		expected int
	}{
		{
			name:     "multiple lines",
			data:     []byte("line1\nline2\nline3"),
			expected: 3,
		},
		{
			name:     "single line no newline",
			data:     []byte("single"),
			expected: 1,
		},
		{
			name:     "trailing newline",
			data:     []byte("line1\nline2\n"),
			expected: 2, // lines without trailing empty element
		},
		{
			name:     "empty",
			data:     []byte(""),
			expected: 0, // no lines for empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := splitLines(tt.data)
			if len(result) != tt.expected {
				t.Errorf("splitLines() returned %d lines, want %d", len(result), tt.expected)
			}
		})
	}
}

// TestStreamChunkFields tests that StreamChunk fields work correctly.
func TestStreamChunkFields(t *testing.T) {
	t.Parallel()

	chunk := StreamChunk{
		ID: "chunk-123",
		Delta: StreamDelta{
			Role:    "assistant",
			Content: "Hello",
			ToolCalls: []ToolCallDelta{
				{
					Index: 0,
					ID:    "tc-1",
					Type:  "function",
					Function: &FunctionCallDelta{
						Name:      "test",
						Arguments: "{}",
					},
				},
			},
		},
		FinishReason: "stop",
		Usage: &Usage{
			TotalTokens: 10,
		},
	}

	if chunk.ID != "chunk-123" {
		t.Errorf("ID = %s, want chunk-123", chunk.ID)
	}
	if chunk.Delta.Role != "assistant" {
		t.Errorf("Delta.Role = %s, want assistant", chunk.Delta.Role)
	}
	if chunk.Delta.Content != "Hello" {
		t.Errorf("Delta.Content = %s, want Hello", chunk.Delta.Content)
	}
	if len(chunk.Delta.ToolCalls) != 1 {
		t.Fatalf("len(Delta.ToolCalls) = %d, want 1", len(chunk.Delta.ToolCalls))
	}
	if chunk.Delta.ToolCalls[0].Function.Name != "test" {
		t.Errorf("ToolCalls[0].Function.Name = %s, want test", chunk.Delta.ToolCalls[0].Function.Name)
	}
	if chunk.FinishReason != "stop" {
		t.Errorf("FinishReason = %s, want stop", chunk.FinishReason)
	}
	if chunk.Usage.TotalTokens != 10 {
		t.Errorf("Usage.TotalTokens = %d, want 10", chunk.Usage.TotalTokens)
	}
}

// mockReadCloser helps test error handling in streams.
type mockReadCloser struct {
	*bytes.Reader
	closeErr error
}

func (m *mockReadCloser) Close() error {
	return m.closeErr
}

func TestJSONLineStream_CloseError(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close error")
	reader := &mockReadCloser{
		Reader:   bytes.NewReader([]byte{}),
		closeErr: closeErr,
	}
	stream := NewJSONLineStream(reader)

	err := stream.Close()
	if !errors.Is(err, closeErr) {
		t.Errorf("Close() error = %v, want %v", err, closeErr)
	}
}

func TestSSEStream_CloseError(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close error")
	reader := &mockReadCloser{
		Reader:   bytes.NewReader([]byte{}),
		closeErr: closeErr,
	}
	stream := NewSSEStream(reader)

	err := stream.Close()
	if !errors.Is(err, closeErr) {
		t.Errorf("Close() error = %v, want %v", err, closeErr)
	}
}
