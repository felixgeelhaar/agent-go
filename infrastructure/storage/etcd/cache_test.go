package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
)

func TestNewCache(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Client:    NewMockClient(),
				KeyPrefix: "test/",
			},
			wantErr: false,
		},
		{
			name: "missing client",
			cfg: Config{
				KeyPrefix: "test/",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCache(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCache() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// Set a value first
	_ = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})

	tests := []struct {
		name      string
		key       string
		wantValue []byte
		wantFound bool
		wantErr   bool
	}{
		{
			name:      "get existing key",
			key:       "key1",
			wantValue: []byte("value1"),
			wantFound: true,
			wantErr:   false,
		},
		{
			name:      "get non-existing key",
			key:       "key2",
			wantValue: nil,
			wantFound: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := c.Get(ctx, tt.key)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if found != tt.wantFound {
				t.Errorf("expected found %v, got %v", tt.wantFound, found)
			}

			if string(value) != string(tt.wantValue) {
				t.Errorf("expected value %q, got %q", tt.wantValue, value)
			}
		})
	}
}

func TestSet(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	tests := []struct {
		name    string
		key     string
		value   []byte
		opts    cache.SetOptions
		wantErr bool
	}{
		{
			name:    "set simple value",
			key:     "key1",
			value:   []byte("value1"),
			opts:    cache.SetOptions{},
			wantErr: false,
		},
		{
			name:    "set with TTL",
			key:     "key2",
			value:   []byte("value2"),
			opts:    cache.SetOptions{TTL: time.Hour},
			wantErr: false,
		},
		{
			name:    "set empty key",
			key:     "",
			value:   []byte("value"),
			opts:    cache.SetOptions{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Set(ctx, tt.key, tt.value, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the value was set
			value, found, _ := c.Get(ctx, tt.key)
			if !found {
				t.Error("expected value to be found")
			}
			if string(value) != string(tt.value) {
				t.Errorf("expected value %q, got %q", tt.value, value)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// Set values first
	_ = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "delete existing key",
			key:     "key1",
			wantErr: false,
		},
		{
			name:    "delete non-existing key",
			key:     "key2",
			wantErr: false, // etcd doesn't error on delete non-existing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Delete(ctx, tt.key)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify deletion
			_, found, _ := c.Get(ctx, tt.key)
			if found {
				t.Error("expected key to be deleted")
			}
		})
	}
}

func TestExists(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// Set a value first
	_ = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})

	tests := []struct {
		name       string
		key        string
		wantExists bool
	}{
		{
			name:       "existing key",
			key:        "key1",
			wantExists: true,
		},
		{
			name:       "non-existing key",
			key:        "key2",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := c.Exists(ctx, tt.key)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if exists != tt.wantExists {
				t.Errorf("expected exists %v, got %v", tt.wantExists, exists)
			}
		})
	}
}

func TestClear(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// Set multiple values
	_ = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
	_ = c.Set(ctx, "key2", []byte("value2"), cache.SetOptions{})
	_ = c.Set(ctx, "key3", []byte("value3"), cache.SetOptions{})

	// Clear the cache
	err := c.Clear(ctx)
	if err != nil {
		t.Errorf("Clear error: %v", err)
	}

	// Verify all keys are deleted
	for _, key := range []string{"key1", "key2", "key3"} {
		exists, _ := c.Exists(ctx, key)
		if exists {
			t.Errorf("expected key %q to be cleared", key)
		}
	}
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// Initial stats
	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("expected initial stats to be zero")
	}

	// Set a value
	_ = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})

	// Hit
	_, _, _ = c.Get(ctx, "key1")
	stats = c.Stats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}

	// Miss
	_, _, _ = c.Get(ctx, "key2")
	stats = c.Stats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestTTLExpiration(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// Set a value with very short TTL
	_ = c.Set(ctx, "expiring", []byte("value"), cache.SetOptions{TTL: time.Millisecond})

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Should be expired
	_, found, err := c.Get(ctx, "expiring")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected expired key to not be found")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewMockClient()
	c, _ := NewCache(Config{
		Client:    client,
		KeyPrefix: "test/",
	})

	// All operations should return context error
	_, _, err := c.Get(ctx, "key")
	if err == nil {
		t.Error("expected context cancelled error for Get")
	}

	err = c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
	if err == nil {
		t.Error("expected context cancelled error for Set")
	}

	err = c.Delete(ctx, "key")
	if err == nil {
		t.Error("expected context cancelled error for Delete")
	}

	_, err = c.Exists(ctx, "key")
	if err == nil {
		t.Error("expected context cancelled error for Exists")
	}

	err = c.Clear(ctx)
	if err == nil {
		t.Error("expected context cancelled error for Clear")
	}
}

func TestMockClient(t *testing.T) {
	ctx := context.Background()
	client := NewMockClient()

	// Test Put
	err := client.Put(ctx, "key1", []byte("value1"), 0)
	if err != nil {
		t.Errorf("Put error: %v", err)
	}

	// Test Get
	value, found, err := client.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get error: %v", err)
	}
	if !found {
		t.Error("expected key to be found")
	}
	if string(value) != "value1" {
		t.Errorf("expected 'value1', got %s", string(value))
	}

	// Test HasKey
	if !client.HasKey("key1") {
		t.Error("expected HasKey to return true")
	}

	// Test KeyCount
	_ = client.Put(ctx, "key2", []byte("value2"), 0)
	if client.KeyCount() != 2 {
		t.Errorf("expected 2 keys, got %d", client.KeyCount())
	}

	// Test GetValue
	v, ok := client.GetValue("key1")
	if !ok {
		t.Error("expected GetValue to return value")
	}
	if string(v) != "value1" {
		t.Errorf("expected 'value1', got %s", string(v))
	}

	// Test Delete
	err = client.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete error: %v", err)
	}
	if client.HasKey("key1") {
		t.Error("expected key to be deleted")
	}

	// Test DeletePrefix
	_ = client.Put(ctx, "prefix/a", []byte("a"), 0)
	_ = client.Put(ctx, "prefix/b", []byte("b"), 0)
	_ = client.Put(ctx, "other/c", []byte("c"), 0)

	err = client.DeletePrefix(ctx, "prefix/")
	if err != nil {
		t.Errorf("DeletePrefix error: %v", err)
	}
	if client.HasKey("prefix/a") || client.HasKey("prefix/b") {
		t.Error("expected prefix keys to be deleted")
	}
	if !client.HasKey("other/c") {
		t.Error("expected other key to remain")
	}

	// Test Close
	if err := client.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}
