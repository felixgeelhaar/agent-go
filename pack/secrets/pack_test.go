package secrets_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/pack/secrets"
)

func TestNew(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider)
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

	expectedTools := []string{"secret_list", "secret_get", "secret_metadata"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}

	// Write tools should not be present
	if toolNames["secret_put"] {
		t.Error("secret_put should not be present in read-only mode")
	}
	if toolNames["secret_delete"] {
		t.Error("secret_delete should not be present in read-only mode")
	}
	if toolNames["secret_rotate"] {
		t.Error("secret_rotate should not be present without rotation access")
	}
}

func TestNew_WithWriteAccess(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["secret_put"] {
		t.Error("secret_put should be present with write access")
	}
}

func TestNew_WithDeleteAccess(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithDeleteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["secret_delete"] {
		t.Error("secret_delete should be present with delete access")
	}
}

func TestNew_WithRotationAccess(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithRotationAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tl := range pack.Tools {
		toolNames[tl.Name()] = true
	}

	if !toolNames["secret_rotate"] {
		t.Error("secret_rotate should be present with rotation access")
	}
}

func TestNew_NilProvider(t *testing.T) {
	_, err := secrets.New(nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestListSecrets(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Create some secrets
	_ = provider.PutSecret(ctx, "app/db/password", map[string]interface{}{"value": "secret1"}, secrets.SecretOptions{})
	_ = provider.PutSecret(ctx, "app/api/key", map[string]interface{}{"value": "secret2"}, secrets.SecretOptions{})
	_ = provider.PutSecret(ctx, "other/secret", map[string]interface{}{"value": "secret3"}, secrets.SecretOptions{})

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find secret_list tool
	var listTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_list" {
			listTool = tl
			break
		}
	}

	if listTool == nil {
		t.Fatal("secret_list tool not found")
	}

	// List all secrets
	result, err := listTool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("secret_list failed: %v", err)
	}

	var output struct {
		Provider string   `json:"provider"`
		Paths    []string `json:"paths"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Provider != "memory" {
		t.Errorf("expected provider 'memory', got '%s'", output.Provider)
	}

	if output.Count != 3 {
		t.Errorf("expected 3 secrets, got %d", output.Count)
	}
}

func TestListSecrets_WithPrefix(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Create some secrets
	_ = provider.PutSecret(ctx, "app/db/password", map[string]interface{}{"value": "secret1"}, secrets.SecretOptions{})
	_ = provider.PutSecret(ctx, "app/api/key", map[string]interface{}{"value": "secret2"}, secrets.SecretOptions{})
	_ = provider.PutSecret(ctx, "other/secret", map[string]interface{}{"value": "secret3"}, secrets.SecretOptions{})

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var listTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_list" {
			listTool = tl
			break
		}
	}

	// List secrets with prefix
	result, err := listTool.Execute(ctx, json.RawMessage(`{"prefix": "app/"}`))
	if err != nil {
		t.Fatalf("secret_list failed: %v", err)
	}

	var output struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Count != 2 {
		t.Errorf("expected 2 secrets with prefix 'app/', got %d", output.Count)
	}
}

func TestGetSecret(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Create a secret
	secretData := map[string]interface{}{
		"username": "admin",
		"password": "secret123",
	}
	_ = provider.PutSecret(ctx, "app/db/credentials", secretData, secrets.SecretOptions{})

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find secret_get tool
	var getTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_get" {
			getTool = tl
			break
		}
	}

	if getTool == nil {
		t.Fatal("secret_get tool not found")
	}

	// Get the secret
	result, err := getTool.Execute(ctx, json.RawMessage(`{"path": "app/db/credentials"}`))
	if err != nil {
		t.Fatalf("secret_get failed: %v", err)
	}

	var output struct {
		Path    string                 `json:"path"`
		Data    map[string]interface{} `json:"data"`
		Version int                    `json:"version"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Path != "app/db/credentials" {
		t.Errorf("expected path 'app/db/credentials', got '%s'", output.Path)
	}

	if output.Data["username"] != "admin" {
		t.Errorf("expected username 'admin', got '%v'", output.Data["username"])
	}

	if output.Version != 1 {
		t.Errorf("expected version 1, got %d", output.Version)
	}
}

func TestGetSecret_NotFound(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var getTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_get" {
			getTool = tl
			break
		}
	}

	_, err = getTool.Execute(context.Background(), json.RawMessage(`{"path": "nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent secret")
	}
}

func TestGetSecret_MissingPath(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var getTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_get" {
			getTool = tl
			break
		}
	}

	_, err = getTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestGetSecretMetadata(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Create a secret
	_ = provider.PutSecret(ctx, "app/db/password", map[string]interface{}{"value": "secret"}, secrets.SecretOptions{})

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find secret_metadata tool
	var metadataTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_metadata" {
			metadataTool = tl
			break
		}
	}

	if metadataTool == nil {
		t.Fatal("secret_metadata tool not found")
	}

	// Get metadata
	result, err := metadataTool.Execute(ctx, json.RawMessage(`{"path": "app/db/password"}`))
	if err != nil {
		t.Fatalf("secret_metadata failed: %v", err)
	}

	var output struct {
		Path     string `json:"path"`
		Metadata struct {
			Version int `json:"version"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if output.Path != "app/db/password" {
		t.Errorf("expected path 'app/db/password', got '%s'", output.Path)
	}

	if output.Metadata.Version != 1 {
		t.Errorf("expected version 1, got %d", output.Metadata.Version)
	}
}

func TestPutSecret(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find secret_put tool
	var putTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_put" {
			putTool = tl
			break
		}
	}

	if putTool == nil {
		t.Fatal("secret_put tool not found")
	}

	// Put a secret
	input := json.RawMessage(`{"path": "app/db/password", "data": {"value": "secret123"}}`)
	result, err := putTool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("secret_put failed: %v", err)
	}

	var output struct {
		Path    string `json:"path"`
		Created bool   `json:"created"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Created {
		t.Error("expected created to be true")
	}

	// Verify secret was stored
	if provider.SecretCount() != 1 {
		t.Errorf("expected 1 secret, got %d", provider.SecretCount())
	}
}

func TestPutSecret_MissingPath(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var putTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_put" {
			putTool = tl
			break
		}
	}

	input := json.RawMessage(`{"data": {"value": "secret123"}}`)
	_, err = putTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestPutSecret_MissingData(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithWriteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var putTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_put" {
			putTool = tl
			break
		}
	}

	input := json.RawMessage(`{"path": "app/db/password"}`)
	_, err = putTool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing data")
	}
}

func TestDeleteSecret(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Create a secret
	_ = provider.PutSecret(ctx, "app/db/password", map[string]interface{}{"value": "secret"}, secrets.SecretOptions{})

	pack, err := secrets.New(provider, secrets.WithDeleteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find secret_delete tool
	var deleteTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_delete" {
			deleteTool = tl
			break
		}
	}

	if deleteTool == nil {
		t.Fatal("secret_delete tool not found")
	}

	// Delete the secret
	result, err := deleteTool.Execute(ctx, json.RawMessage(`{"path": "app/db/password"}`))
	if err != nil {
		t.Fatalf("secret_delete failed: %v", err)
	}

	var output struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Deleted {
		t.Error("expected deleted to be true")
	}

	// Verify secret was deleted
	if provider.SecretCount() != 0 {
		t.Errorf("expected 0 secrets, got %d", provider.SecretCount())
	}
}

func TestDeleteSecret_NotFound(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithDeleteAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var deleteTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_delete" {
			deleteTool = tl
			break
		}
	}

	_, err = deleteTool.Execute(context.Background(), json.RawMessage(`{"path": "nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent secret")
	}
}

func TestRotateSecret(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	ctx := context.Background()

	// Create a secret
	_ = provider.PutSecret(ctx, "app/db/password", map[string]interface{}{"password": "oldpassword"}, secrets.SecretOptions{})

	pack, err := secrets.New(provider, secrets.WithRotationAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Find secret_rotate tool
	var rotateTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_rotate" {
			rotateTool = tl
			break
		}
	}

	if rotateTool == nil {
		t.Fatal("secret_rotate tool not found")
	}

	// Rotate the secret
	result, err := rotateTool.Execute(ctx, json.RawMessage(`{"path": "app/db/password"}`))
	if err != nil {
		t.Fatalf("secret_rotate failed: %v", err)
	}

	var output struct {
		Path       string `json:"path"`
		NewVersion int    `json:"new_version"`
		Rotated    bool   `json:"rotated"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !output.Rotated {
		t.Error("expected rotated to be true")
	}

	if output.NewVersion != 2 {
		t.Errorf("expected new version 2, got %d", output.NewVersion)
	}
}

func TestRotateSecret_NotFound(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithRotationAccess())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var rotateTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_rotate" {
			rotateTool = tl
			break
		}
	}

	_, err = rotateTool.Execute(context.Background(), json.RawMessage(`{"path": "nonexistent"}`))
	if err == nil {
		t.Fatal("expected error for nonexistent secret")
	}
}

func TestContextCancelled(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var listTool tool.Tool
	for _, tl := range pack.Tools {
		if tl.Name() == "secret_list" {
			listTool = tl
			break
		}
	}

	_, err = listTool.Execute(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWithTimeout(t *testing.T) {
	provider := secrets.NewMemoryProvider()
	defer provider.Close()

	pack, err := secrets.New(provider, secrets.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if pack == nil {
		t.Fatal("expected pack, got nil")
	}
}
