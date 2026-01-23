package messaging_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/pack/messaging"
)

func TestNew(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}

	tools := pack.Tools
	if len(tools) == 0 {
		t.Fatal("expected tools, got none")
	}

	// Verify expected tools (read-only mode)
	toolNames := make(map[string]bool)
	for _, tl := range tools {
		toolNames[tl.Name()] = true
	}

	expectedTools := []string{"msg_list_topics", "msg_peek", "msg_acknowledge"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}

	// msg_publish should not be present in read-only mode
	if toolNames["msg_publish"] {
		t.Error("msg_publish should not be present in read-only mode")
	}
}

func TestNew_WithWriteAccess(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider, messaging.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["msg_publish"] {
		t.Error("msg_publish should be present with write access")
	}
}

func TestNew_NilProvider(t *testing.T) {
	_, err := messaging.New(nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestListTopics(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	// Create some topics
	provider.CreateTopic("topic-1")
	provider.CreateTopic("topic-2")
	provider.CreateTopic("topic-3")

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find msg_list_topics tool
	var listTopicsTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_list_topics" {
			listTopicsTool = tl
			break
		}
	}

	if listTopicsTool == nil {
		t.Fatal("msg_list_topics tool not found")
	}

	// Execute tool
	result, err := listTopicsTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("msg_list_topics failed: %v", err)
	}

	var output struct {
		Provider string               `json:"provider"`
		Topics   []messaging.TopicInfo `json:"topics"`
		Count    int                  `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Provider != "memory" {
		t.Errorf("expected provider 'memory', got '%s'", output.Provider)
	}

	if output.Count != 3 {
		t.Errorf("expected 3 topics, got %d", output.Count)
	}
}

func TestPublish(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider, messaging.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find msg_publish tool
	var publishTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_publish" {
			publishTool = tl
			break
		}
	}

	if publishTool == nil {
		t.Fatal("msg_publish tool not found")
	}

	// Execute tool
	input := json.RawMessage(`{"topic": "test-topic", "message": "hello world", "key": "key1"}`)
	result, err := publishTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("msg_publish failed: %v", err)
	}

	var output struct {
		MessageID string    `json:"message_id"`
		Topic     string    `json:"topic"`
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.MessageID == "" {
		t.Error("expected message_id to be set")
	}

	if output.Topic != "test-topic" {
		t.Errorf("expected topic 'test-topic', got '%s'", output.Topic)
	}

	// Verify message was stored
	if provider.MessageCount("test-topic") != 1 {
		t.Errorf("expected 1 message in topic, got %d", provider.MessageCount("test-topic"))
	}
}

func TestPublish_MissingTopic(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider, messaging.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var publishTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_publish" {
			publishTool = tl
			break
		}
	}

	input := json.RawMessage(`{"message": "hello world"}`)
	_, err = publishTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing topic")
	}
}

func TestPublish_MissingMessage(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider, messaging.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var publishTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_publish" {
			publishTool = tl
			break
		}
	}

	input := json.RawMessage(`{"topic": "test-topic"}`)
	_, err = publishTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing message")
	}
}

func TestPublish_MessageTooLarge(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	// Set a small max message size
	pack, err := messaging.New(provider,
		messaging.WithWriteAccess(),
		messaging.WithMaxMessageSize(10),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var publishTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_publish" {
			publishTool = tl
			break
		}
	}

	input := json.RawMessage(`{"topic": "test-topic", "message": "this message is way too long"}`)
	_, err = publishTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for message too large")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected size error, got: %v", err)
	}
}

func TestPeek(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	// Publish some messages first
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := provider.Publish(ctx, "test-topic", []byte("message"), messaging.PublishOptions{})
		if err != nil {
			t.Fatalf("failed to publish: %v", err)
		}
	}

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find msg_peek tool
	var peekTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_peek" {
			peekTool = tl
			break
		}
	}

	if peekTool == nil {
		t.Fatal("msg_peek tool not found")
	}

	// Peek at messages
	input := json.RawMessage(`{"topic": "test-topic", "limit": 3}`)
	result, err := peekTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("msg_peek failed: %v", err)
	}

	var output struct {
		Topic    string              `json:"topic"`
		Messages []messaging.Message `json:"messages"`
		Count    int                 `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Count != 3 {
		t.Errorf("expected 3 messages, got %d", output.Count)
	}

	// Original messages should still be there
	if provider.MessageCount("test-topic") != 5 {
		t.Errorf("expected 5 messages still in topic, got %d", provider.MessageCount("test-topic"))
	}
}

func TestPeek_MissingTopic(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var peekTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_peek" {
			peekTool = tl
			break
		}
	}

	input := json.RawMessage(`{}`)
	_, err = peekTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing topic")
	}
}

func TestPeek_EmptyTopic(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	provider.CreateTopic("empty-topic")

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var peekTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_peek" {
			peekTool = tl
			break
		}
	}

	input := json.RawMessage(`{"topic": "empty-topic"}`)
	result, err := peekTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("msg_peek failed: %v", err)
	}

	var output struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Count != 0 {
		t.Errorf("expected 0 messages, got %d", output.Count)
	}
}

func TestAcknowledge(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Publish a message
	result, err := provider.Publish(ctx, "test-topic", []byte("message"), messaging.PublishOptions{})
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find msg_acknowledge tool
	var ackTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_acknowledge" {
			ackTool = tl
			break
		}
	}

	if ackTool == nil {
		t.Fatal("msg_acknowledge tool not found")
	}

	// Acknowledge the message
	input := json.RawMessage(`{"message_id": "` + result.MessageID + `"}`)
	ackResult, err := ackTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("msg_acknowledge failed: %v", err)
	}

	var output struct {
		MessageID    string `json:"message_id"`
		Acknowledged bool   `json:"acknowledged"`
	}
	if err := json.Unmarshal(ackResult.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Acknowledged {
		t.Error("expected acknowledged to be true")
	}

	if output.MessageID != result.MessageID {
		t.Errorf("expected message_id '%s', got '%s'", result.MessageID, output.MessageID)
	}
}

func TestAcknowledge_MissingMessageID(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var ackTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_acknowledge" {
			ackTool = tl
			break
		}
	}

	input := json.RawMessage(`{}`)
	_, err = ackTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing message_id")
	}
}

func TestAcknowledge_NonexistentMessage(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var ackTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_acknowledge" {
			ackTool = tl
			break
		}
	}

	input := json.RawMessage(`{"message_id": "nonexistent-id"}`)
	_, err = ackTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

func TestContextCancelled(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Find msg_list_topics tool
	var listTopicsTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_list_topics" {
			listTopicsTool = tl
			break
		}
	}

	_, err = listTopicsTool.Execute(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestMemoryProvider_Subscribe(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Publish some messages
	for i := 0; i < 3; i++ {
		_, err := provider.Publish(ctx, "test-topic", []byte("message"), messaging.PublishOptions{})
		if err != nil {
			t.Fatalf("failed to publish: %v", err)
		}
	}

	// Subscribe
	ch, err := provider.Subscribe(ctx, "test-topic", messaging.SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Read messages
	count := 0
	timeout := time.After(1 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				// Channel closed
				goto done
			}
			count++
		case <-timeout:
			t.Fatal("timeout waiting for messages")
		}
	}

done:
	if count != 3 {
		t.Errorf("expected 3 messages, got %d", count)
	}
}

func TestMemoryProvider_TopicExists(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Topic doesn't exist
	exists, err := provider.TopicExists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("TopicExists failed: %v", err)
	}
	if exists {
		t.Error("expected topic not to exist")
	}

	// Create topic
	provider.CreateTopic("test-topic")

	// Topic exists
	exists, err = provider.TopicExists(ctx, "test-topic")
	if err != nil {
		t.Fatalf("TopicExists failed: %v", err)
	}
	if !exists {
		t.Error("expected topic to exist")
	}
}

func TestWithTimeout(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	pack, err := messaging.New(provider, messaging.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}
}

func TestWithMaxPeekMessages(t *testing.T) {
	provider := messaging.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Publish many messages
	for i := 0; i < 20; i++ {
		_, err := provider.Publish(ctx, "test-topic", []byte("message"), messaging.PublishOptions{})
		if err != nil {
			t.Fatalf("failed to publish: %v", err)
		}
	}

	// Create pack with limited peek
	pack, err := messaging.New(provider, messaging.WithMaxPeekMessages(5))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var peekTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "msg_peek" {
			peekTool = tl
			break
		}
	}

	// Request more than max
	input := json.RawMessage(`{"topic": "test-topic", "limit": 100}`)
	result, err := peekTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("msg_peek failed: %v", err)
	}

	var output struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Should be limited to max
	if output.Count != 5 {
		t.Errorf("expected 5 messages (max), got %d", output.Count)
	}
}
