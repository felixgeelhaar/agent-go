package badger_test

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/badger"
)

func TestNewCache(t *testing.T) {
	cfg := badger.Config{
		InMemory: true,
	}

	c, err := badger.NewCache(cfg)
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

	// Set with 2 second TTL (longer to avoid timing issues)
	err := c.Set(ctx, "expiring", []byte("value"), cache.SetOptions{TTL: 2 * time.Second})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	val, found, err := c.Get(ctx, "expiring")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatalf("expected key to be found immediately, got found=%v val=%v", found, val)
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
	cfg := badger.Config{
		InMemory:  true,
		KeyPrefix: "prefix:",
	}

	c, err := badger.NewCache(cfg)
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

func TestCache_GetWithMeta(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set with TTL
	ttl := 10 * time.Second
	err := c.Set(ctx, "key", []byte("value"), cache.SetOptions{TTL: ttl})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get with metadata
	val, expiresAt, err := c.GetWithMeta(ctx, "key")
	if err != nil {
		t.Fatalf("GetWithMeta failed: %v", err)
	}
	if string(val) != "value" {
		t.Errorf("expected 'value', got '%s'", string(val))
	}

	// Check expiration time is in the future
	if expiresAt.Before(time.Now()) {
		t.Error("expected expiresAt to be in the future")
	}
}

func TestCache_GetWithMeta_NotFound(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	_, _, err := c.GetWithMeta(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if err != cache.ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestCache_SetNX(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// First SetNX should succeed
	set, err := c.SetNX(ctx, "key", []byte("value1"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("SetNX failed: %v", err)
	}
	if !set {
		t.Error("expected first SetNX to succeed")
	}

	// Second SetNX should not set
	set, err = c.SetNX(ctx, "key", []byte("value2"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("SetNX failed: %v", err)
	}
	if set {
		t.Error("expected second SetNX to not set")
	}

	// Value should be original
	val, _, _ := c.Get(ctx, "key")
	if string(val) != "value1" {
		t.Errorf("expected 'value1', got '%s'", string(val))
	}
}

func TestCache_SetNX_InvalidKey(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	_, err := c.SetNX(ctx, "", []byte("value"), cache.SetOptions{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCache_Increment(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// First increment initializes to delta
	val, err := c.Increment(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}

	// Second increment adds to existing
	val, err = c.Increment(ctx, "counter", 3)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 8 {
		t.Errorf("expected 8, got %d", val)
	}

	// Negative delta
	val, err = c.Increment(ctx, "counter", -2)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if val != 6 {
		t.Errorf("expected 6, got %d", val)
	}
}

func TestCache_Increment_InvalidKey(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	_, err := c.Increment(ctx, "", 1)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCache_Keys(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set some keys
	_ = c.Set(ctx, "user:1", []byte("a"), cache.SetOptions{})
	_ = c.Set(ctx, "user:2", []byte("b"), cache.SetOptions{})
	_ = c.Set(ctx, "user:3", []byte("c"), cache.SetOptions{})
	_ = c.Set(ctx, "item:1", []byte("d"), cache.SetOptions{})

	// Get keys with prefix
	keys, err := c.Keys(ctx, "user:")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// Verify all start with "user:"
	for _, key := range keys {
		if len(key) < 5 || key[:5] != "user:" {
			t.Errorf("key %s does not start with 'user:'", key)
		}
	}
}

func TestCache_Keys_EmptyPrefix(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	ctx := context.Background()

	// Set some keys
	_ = c.Set(ctx, "a", []byte("1"), cache.SetOptions{})
	_ = c.Set(ctx, "b", []byte("2"), cache.SetOptions{})
	_ = c.Set(ctx, "c", []byte("3"), cache.SetOptions{})

	// Get all keys
	keys, err := c.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func newTestCache(t *testing.T) *badger.Cache {
	t.Helper()

	cfg := badger.Config{
		InMemory: true,
	}

	c, err := badger.NewCache(cfg)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	return c
}
