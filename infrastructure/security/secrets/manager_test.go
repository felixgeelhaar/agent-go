package secrets

import (
	"context"
	"os"
	"testing"
)

func TestMemoryManager(t *testing.T) {
	ctx := context.Background()
	manager := NewMemoryManager()

	// Test Set and Get
	err := manager.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, err := manager.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value1" {
		t.Errorf("expected value1, got %s", value)
	}

	// Test Exists
	exists, err := manager.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key1 to exist")
	}

	exists, err = manager.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected nonexistent to not exist")
	}

	// Test Delete
	err = manager.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = manager.Get(ctx, "key1")
	if err != ErrSecretNotFound {
		t.Error("expected ErrSecretNotFound after delete")
	}
}

func TestMemoryManagerList(t *testing.T) {
	ctx := context.Background()
	manager := NewMemoryManager()

	manager.Set(ctx, "db_user", "admin")
	manager.Set(ctx, "db_pass", "secret")
	manager.Set(ctx, "api_key", "xyz")

	keys, err := manager.List(ctx, "db_")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys with db_ prefix, got %d", len(keys))
	}
}

func TestMemoryManagerReadOnly(t *testing.T) {
	ctx := context.Background()
	manager := NewMemoryManager(
		WithInitialSecrets(map[string]string{"key": "value"}),
		WithMemoryReadOnly(),
	)

	// Should be able to read
	value, err := manager.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value" {
		t.Errorf("expected value, got %s", value)
	}

	// Should not be able to write
	err = manager.Set(ctx, "new", "value")
	if err != ErrSecretReadOnly {
		t.Errorf("expected ErrSecretReadOnly, got %v", err)
	}

	// Should not be able to delete
	err = manager.Delete(ctx, "key")
	if err != ErrSecretReadOnly {
		t.Errorf("expected ErrSecretReadOnly, got %v", err)
	}
}

func TestMemoryManagerDeleteNonexistent(t *testing.T) {
	ctx := context.Background()
	manager := NewMemoryManager()

	err := manager.Delete(ctx, "nonexistent")
	if err != ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestEnvManager(t *testing.T) {
	ctx := context.Background()

	// Set up test environment variable
	testKey := "TEST_SECRET_KEY"
	testValue := "test_secret_value"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	manager := NewEnvManager()

	// Test Get
	value, err := manager.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != testValue {
		t.Errorf("expected %s, got %s", testValue, value)
	}

	// Test Get nonexistent
	_, err = manager.Get(ctx, "NONEXISTENT_KEY")
	if err != ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}

	// Test Exists
	exists, err := manager.Exists(ctx, testKey)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}
}

func TestEnvManagerWithPrefix(t *testing.T) {
	ctx := context.Background()

	testValue := "secret_value"
	os.Setenv("MY_APP_SECRET", testValue)
	defer os.Unsetenv("MY_APP_SECRET")

	manager := NewEnvManager(WithPrefix("MY_APP_"))

	// Get with prefix
	value, err := manager.Get(ctx, "SECRET")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != testValue {
		t.Errorf("expected %s, got %s", testValue, value)
	}
}

func TestEnvManagerReadOnly(t *testing.T) {
	ctx := context.Background()
	manager := NewEnvManager(WithEnvReadOnly())

	err := manager.Set(ctx, "KEY", "value")
	if err != ErrSecretReadOnly {
		t.Errorf("expected ErrSecretReadOnly, got %v", err)
	}

	err = manager.Delete(ctx, "KEY")
	if err != ErrSecretReadOnly {
		t.Errorf("expected ErrSecretReadOnly, got %v", err)
	}
}

func TestChainedManager(t *testing.T) {
	ctx := context.Background()

	primary := NewMemoryManager(WithInitialSecrets(map[string]string{
		"primary_key": "primary_value",
	}))

	secondary := NewMemoryManager(WithInitialSecrets(map[string]string{
		"secondary_key": "secondary_value",
	}))

	manager := NewChainedManager(primary, secondary)

	// Should find key in primary
	value, err := manager.Get(ctx, "primary_key")
	if err != nil {
		t.Fatalf("Get primary_key failed: %v", err)
	}
	if value != "primary_value" {
		t.Errorf("expected primary_value, got %s", value)
	}

	// Should find key in secondary
	value, err = manager.Get(ctx, "secondary_key")
	if err != nil {
		t.Fatalf("Get secondary_key failed: %v", err)
	}
	if value != "secondary_value" {
		t.Errorf("expected secondary_value, got %s", value)
	}

	// Should not find nonexistent
	_, err = manager.Get(ctx, "nonexistent")
	if err != ErrSecretNotFound {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}

	// Set should go to primary
	err = manager.Set(ctx, "new_key", "new_value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it's in primary
	value, err = primary.Get(ctx, "new_key")
	if err != nil {
		t.Fatalf("Get from primary failed: %v", err)
	}
	if value != "new_value" {
		t.Errorf("expected new_value, got %s", value)
	}
}

func TestChainedManagerList(t *testing.T) {
	ctx := context.Background()

	primary := NewMemoryManager(WithInitialSecrets(map[string]string{
		"db_user": "admin",
	}))

	secondary := NewMemoryManager(WithInitialSecrets(map[string]string{
		"db_pass": "secret",
		"api_key": "xyz",
	}))

	manager := NewChainedManager(primary, secondary)

	keys, err := manager.List(ctx, "db_")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("expected 2 keys with db_ prefix, got %d", len(keys))
	}
}

func TestChainedManagerExists(t *testing.T) {
	ctx := context.Background()

	primary := NewMemoryManager(WithInitialSecrets(map[string]string{
		"key1": "value1",
	}))

	secondary := NewMemoryManager(WithInitialSecrets(map[string]string{
		"key2": "value2",
	}))

	manager := NewChainedManager(primary, secondary)

	// Should find in primary
	exists, err := manager.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key1 to exist")
	}

	// Should find in secondary
	exists, err = manager.Exists(ctx, "key2")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key2 to exist")
	}

	// Should not find nonexistent
	exists, err = manager.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected nonexistent to not exist")
	}
}

func TestRedactedManager(t *testing.T) {
	ctx := context.Background()

	inner := NewMemoryManager(WithInitialSecrets(map[string]string{
		"password": "secret123",
		"username": "admin",
	}))

	manager := NewRedactedManager(inner, "password")

	// Regular Get should return actual value
	value, err := manager.Get(ctx, "password")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "secret123" {
		t.Errorf("expected secret123, got %s", value)
	}

	// GetRedacted should return [REDACTED] for sensitive keys
	value, err = manager.GetRedacted(ctx, "password")
	if err != nil {
		t.Fatalf("GetRedacted failed: %v", err)
	}
	if value != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %s", value)
	}

	// GetRedacted should return actual value for non-sensitive keys
	value, err = manager.GetRedacted(ctx, "username")
	if err != nil {
		t.Fatalf("GetRedacted failed: %v", err)
	}
	if value != "admin" {
		t.Errorf("expected admin, got %s", value)
	}

	// IsRedacted should work correctly
	if !manager.IsRedacted("password") {
		t.Error("expected password to be redacted")
	}
	if manager.IsRedacted("username") {
		t.Error("expected username to not be redacted")
	}
}

func TestRedactedManagerOperations(t *testing.T) {
	ctx := context.Background()

	inner := NewMemoryManager()
	manager := NewRedactedManager(inner, "secret")

	// Set should work
	err := manager.Set(ctx, "key", "value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Exists should work
	exists, err := manager.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}

	// List should work
	keys, err := manager.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}

	// Delete should work
	err = manager.Delete(ctx, "key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}
