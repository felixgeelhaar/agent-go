package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	provider := NewMockProvider("test")
	p := New(PackConfig{
		Provider:       provider,
		DefaultChannel: "#general",
	})

	if p.Name != "notification" {
		t.Errorf("expected pack name 'notification', got %s", p.Name)
	}

	if len(p.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(p.Tools))
	}

	// Verify tool names
	names := make(map[string]bool)
	for _, tool := range p.Tools {
		names[tool.Name()] = true
	}

	expectedNames := []string{"notify_send", "notify_update"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestSend(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful send",
			input: map[string]interface{}{
				"channel": "#general",
				"message": "Hello, world!",
			},
			wantErr: false,
		},
		{
			name: "send with title and level",
			input: map[string]interface{}{
				"channel": "#alerts",
				"message": "System alert",
				"title":   "Warning",
				"level":   "warning",
			},
			wantErr: false,
		},
		{
			name: "send with thread",
			input: map[string]interface{}{
				"channel":   "#general",
				"message":   "Reply message",
				"thread_id": "thread-123",
			},
			wantErr: false,
		},
		{
			name: "uses default channel",
			input: map[string]interface{}{
				"message": "Hello!",
			},
			wantErr: false,
		},
		{
			name: "missing message returns error",
			input: map[string]interface{}{
				"channel": "#general",
			},
			wantErr: true,
		},
		{
			name: "missing channel without default returns error",
			input: map[string]interface{}{
				"message": "Hello",
			},
			setupFunc: func(p *MockProvider) {
				// Will use a config without default channel
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"channel": "#general",
				"message": "Test",
			},
			setupFunc: func(p *MockProvider) {
				p.SendFunc = func(context.Context, SendRequest) (SendResponse, error) {
					return SendResponse{}, errors.New("send error")
				}
			},
			wantErr:     true,
			errContains: "send error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")
			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			defaultChannel := "#default"
			if tt.name == "missing channel without default returns error" {
				defaultChannel = ""
			}

			p := New(PackConfig{
				Provider:       provider,
				DefaultChannel: defaultChannel,
			})

			var sendTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "notify_send" {
					sendTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := sendTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp SendResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}

			if resp.MessageID == "" {
				t.Error("expected non-empty message ID")
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		setupFunc   func(*MockProvider)
		preCreate   bool // Create a message first
		wantErr     bool
		errContains string
	}{
		{
			name: "successful update",
			input: map[string]interface{}{
				"message_id": "msg-1",
				"channel":    "#general",
				"message":    "Updated message",
			},
			preCreate: true,
			wantErr:   false,
		},
		{
			name: "update title only",
			input: map[string]interface{}{
				"message_id": "msg-1",
				"channel":    "#general",
				"title":      "New Title",
			},
			preCreate: true,
			wantErr:   false,
		},
		{
			name: "update non-existent message",
			input: map[string]interface{}{
				"message_id": "msg-999",
				"channel":    "#general",
				"message":    "Updated",
			},
			preCreate: false,
			wantErr:   false, // Not an error, just updated=false
		},
		{
			name: "uses default channel",
			input: map[string]interface{}{
				"message_id": "msg-1",
				"message":    "Updated",
			},
			preCreate: true,
			wantErr:   false,
		},
		{
			name: "missing message_id returns error",
			input: map[string]interface{}{
				"channel": "#general",
				"message": "Updated",
			},
			wantErr: true,
		},
		{
			name: "provider error is propagated",
			input: map[string]interface{}{
				"message_id": "msg-1",
				"channel":    "#general",
				"message":    "Updated",
			},
			setupFunc: func(p *MockProvider) {
				p.UpdateFunc = func(context.Context, UpdateRequest) (UpdateResponse, error) {
					return UpdateResponse{}, errors.New("update error")
				}
			},
			wantErr:     true,
			errContains: "update error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockProvider("test")

			if tt.preCreate {
				// Pre-create a message
				_, _ = provider.Send(context.Background(), SendRequest{
					Channel: "#general",
					Message: "Original message",
				})
			}

			if tt.setupFunc != nil {
				tt.setupFunc(provider)
			}

			p := New(PackConfig{
				Provider:       provider,
				DefaultChannel: "#general",
			})

			var updateTool = p.Tools[0]
			for _, tool := range p.Tools {
				if tool.Name() == "notify_update" {
					updateTool = tool
					break
				}
			}

			input, _ := json.Marshal(tt.input)
			result, err := updateTool.Execute(context.Background(), input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var resp UpdateResponse
			if err := json.Unmarshal(result.Output, &resp); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
			}

			if tt.preCreate && !resp.Updated {
				t.Error("expected updated to be true for pre-created message")
			}
		})
	}
}

func TestNoProvider(t *testing.T) {
	p := New(PackConfig{})

	for _, tool := range p.Tools {
		input, _ := json.Marshal(map[string]interface{}{
			"channel":    "#general",
			"message":    "test",
			"message_id": "msg-1",
		})

		_, err := tool.Execute(context.Background(), input)
		if !errors.Is(err, ErrProviderNotConfigured) {
			t.Errorf("tool %s: expected ErrProviderNotConfigured, got %v", tool.Name(), err)
		}
	}
}

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider("test-mock")

	if provider.Name() != "test-mock" {
		t.Errorf("expected name 'test-mock', got %s", provider.Name())
	}

	ctx := context.Background()

	// Test Available
	if !provider.Available(ctx) {
		t.Error("expected provider to be available")
	}

	// Test Send
	sendResp, err := provider.Send(ctx, SendRequest{
		Channel: "#general",
		Message: "Hello!",
		Title:   "Test",
	})
	if err != nil {
		t.Errorf("Send error: %v", err)
	}
	if !sendResp.Success {
		t.Error("expected success")
	}
	if sendResp.MessageID == "" {
		t.Error("expected message ID")
	}

	// Test Update
	updateResp, err := provider.Update(ctx, UpdateRequest{
		MessageID: sendResp.MessageID,
		Channel:   "#general",
		Message:   "Updated!",
	})
	if err != nil {
		t.Errorf("Update error: %v", err)
	}
	if !updateResp.Updated {
		t.Error("expected updated")
	}

	// Verify message count
	if provider.MessageCount() != 1 {
		t.Errorf("expected 1 message, got %d", provider.MessageCount())
	}
}

func TestDefaultPackConfig(t *testing.T) {
	cfg := DefaultPackConfig()

	if cfg.DefaultLevel != "info" {
		t.Errorf("expected DefaultLevel 'info', got %s", cfg.DefaultLevel)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
