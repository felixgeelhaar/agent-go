package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/sqlite"
)

func TestNewCache(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
	}

	c, err := sqlite.NewCache(cfg)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer c.Close()

	if c == nil {
		t.Fatal("expected cache, got nil")
	}
}

func TestCache_SetAndGet(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set a value
	err := c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	val, found, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found")
	}
	if string(val) != "value1" {
		t.Errorf("expected 'value1', got '%s'", string(val))
	}
}

func TestCache_GetNonexistent(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	val, found, err := c.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("expected key not to be found")
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set with short TTL (2 seconds to avoid Unix timestamp edge cases)
	err := c.Set(ctx, "expiring", []byte("value"), cache.SetOptions{TTL: 2 * time.Second})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	_, found, err := c.Get(ctx, "expiring")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found immediately")
	}

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// Should be expired
	_, found, err = c.Get(ctx, "expiring")
	if err != nil {
		t.Fatalf("Get after expiration failed: %v", err)
	}
	if found {
		t.Fatal("expected key to be expired")
	}
}

func TestCache_Delete(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set a value
	err := c.Set(ctx, "to_delete", []byte("value"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete it
	err = c.Delete(ctx, "to_delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist
	_, found, err := c.Get(ctx, "to_delete")
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if found {
		t.Fatal("expected key to be deleted")
	}
}

func TestCache_Exists(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Key doesn't exist
	exists, err := c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected key not to exist")
	}

	// Set it
	err = c.Set(ctx, "key1", []byte("value"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Now it exists
	exists, err = c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected key to exist")
	}
}

func TestCache_Clear(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set multiple keys
	for i := 0; i < 5; i++ {
		err := c.Set(ctx, "key"+string(rune('0'+i)), []byte("value"), cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Clear all
	err := c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// None should exist
	for i := 0; i < 5; i++ {
		exists, err := c.Exists(ctx, "key"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Errorf("expected key%d to be cleared", i)
		}
	}
}

func TestCache_Stats(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Initial stats
	stats := c.Stats()
	if stats.Hits != 0 {
		t.Errorf("expected 0 hits, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("expected 0 misses, got %d", stats.Misses)
	}

	// Cause a miss
	_, _, _ = c.Get(ctx, "nonexistent")

	// Cause a hit
	_ = c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
	_, _, _ = c.Get(ctx, "key")

	stats = c.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestCache_Cleanup(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set keys with short TTL
	for i := 0; i < 3; i++ {
		err := c.Set(ctx, "exp"+string(rune('0'+i)), []byte("value"), cache.SetOptions{TTL: 10 * time.Millisecond})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Set a key without TTL
	err := c.Set(ctx, "permanent", []byte("value"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(50 * time.Millisecond)

	// Cleanup
	cleaned, err := c.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	if cleaned != 3 {
		t.Errorf("expected 3 cleaned entries, got %d", cleaned)
	}

	// Permanent key should still exist
	exists, err := c.Exists(ctx, "permanent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected permanent key to exist")
	}
}

func TestCache_SetInvalidKey(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	err := c.Set(ctx, "", []byte("value"), cache.SetOptions{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCache_ContextCancelled(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should fail with cancelled context
	_, _, err := c.Get(ctx, "key")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	err = c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestCache_WithKeyPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
		KeyPrefix:   "prefix:",
	}

	c, err := sqlite.NewCache(cfg)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set a value
	err = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get should work with the same key
	val, found, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found")
	}
	if string(val) != "value1" {
		t.Errorf("expected 'value1', got '%s'", string(val))
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set initial value
	err := c.Set(ctx, "key", []byte("value1"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Overwrite
	err = c.Set(ctx, "key", []byte("value2"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Overwrite failed: %v", err)
	}

	// Should get new value
	val, _, err := c.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "value2" {
		t.Errorf("expected 'value2', got '%s'", string(val))
	}
}

func newTestCache(t *testing.T) *sqlite.Cache {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := sqlite.Config{
		DSN:         "file:" + tmpDir + "/test.db?mode=rwc",
		AutoMigrate: true,
	}

	c, err := sqlite.NewCache(cfg)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	return c
}
