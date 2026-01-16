package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/cache"
)

func TestNewCacheFromClient(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with nil client", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "test:")

		if c == nil {
			t.Fatal("NewCacheFromClient() returned nil")
		}
		if c.keyPrefix != "test:" {
			t.Errorf("keyPrefix = %s, want test:", c.keyPrefix)
		}
		if c.client != nil {
			t.Error("client should be nil")
		}
	})

	t.Run("creates cache with empty prefix", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "")

		if c == nil {
			t.Fatal("NewCacheFromClient() returned nil")
		}
		if c.keyPrefix != "" {
			t.Errorf("keyPrefix = %s, want empty", c.keyPrefix)
		}
	})

	t.Run("creates cache with custom prefix", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "myapp:cache:")

		if c == nil {
			t.Fatal("NewCacheFromClient() returned nil")
		}
		if c.keyPrefix != "myapp:cache:" {
			t.Errorf("keyPrefix = %s, want myapp:cache:", c.keyPrefix)
		}
	})
}

func TestCache_prefixKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		keyPrefix string
		key       string
		expected  string
	}{
		{
			name:      "default prefix",
			keyPrefix: "agent:",
			key:       "user:123",
			expected:  "agent:cache:user:123",
		},
		{
			name:      "empty prefix",
			keyPrefix: "",
			key:       "session:abc",
			expected:  "cache:session:abc",
		},
		{
			name:      "custom prefix",
			keyPrefix: "myapp:",
			key:       "data",
			expected:  "myapp:cache:data",
		},
		{
			name:      "nested key",
			keyPrefix: "prod:",
			key:       "api:v1:users:list",
			expected:  "prod:cache:api:v1:users:list",
		},
		{
			name:      "empty key",
			keyPrefix: "agent:",
			key:       "",
			expected:  "agent:cache:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := NewCacheFromClient(nil, tt.keyPrefix)
			result := c.prefixKey(tt.key)

			if result != tt.expected {
				t.Errorf("prefixKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestCache_Stats(t *testing.T) {
	t.Parallel()

	t.Run("initial stats are zero", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "test:")

		stats := c.Stats()

		if stats.Hits != 0 {
			t.Errorf("Hits = %d, want 0", stats.Hits)
		}
		if stats.Misses != 0 {
			t.Errorf("Misses = %d, want 0", stats.Misses)
		}
	})

	t.Run("stats track hits", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "test:")

		// Manually increment hits (simulating what happens on successful Get)
		c.hits.Add(5)

		stats := c.Stats()
		if stats.Hits != 5 {
			t.Errorf("Hits = %d, want 5", stats.Hits)
		}
	})

	t.Run("stats track misses", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "test:")

		// Manually increment misses (simulating what happens on cache miss)
		c.misses.Add(3)

		stats := c.Stats()
		if stats.Misses != 3 {
			t.Errorf("Misses = %d, want 3", stats.Misses)
		}
	})

	t.Run("stats are concurrent-safe", func(t *testing.T) {
		t.Parallel()
		c := NewCacheFromClient(nil, "test:")

		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					c.hits.Add(1)
					c.misses.Add(1)
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		stats := c.Stats()
		if stats.Hits != 1000 {
			t.Errorf("Hits = %d, want 1000", stats.Hits)
		}
		if stats.Misses != 1000 {
			t.Errorf("Misses = %d, want 1000", stats.Misses)
		}
	})
}

func TestCache_wrapError(t *testing.T) {
	t.Parallel()

	c := NewCacheFromClient(nil, "test:")

	t.Run("returns nil for nil error", func(t *testing.T) {
		t.Parallel()
		err := c.wrapError(nil)
		if err != nil {
			t.Errorf("wrapError(nil) = %v, want nil", err)
		}
	})

	t.Run("wraps deadline exceeded as timeout", func(t *testing.T) {
		t.Parallel()
		err := c.wrapError(context.DeadlineExceeded)
		if !errors.Is(err, cache.ErrOperationTimeout) {
			t.Errorf("wrapError(DeadlineExceeded) should wrap as ErrOperationTimeout")
		}
		// Should also contain the original error
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error("wrapped error should contain original error")
		}
	})

	t.Run("returns other errors unchanged", func(t *testing.T) {
		t.Parallel()
		originalErr := errors.New("some redis error")
		err := c.wrapError(originalErr)
		if err != originalErr {
			t.Errorf("wrapError() should return original error for non-timeout errors")
		}
	})
}

func TestCache_ContextCancellation(t *testing.T) {
	t.Parallel()

	c := NewCacheFromClient(nil, "test:")

	t.Run("Get returns error on cancelled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, _, err := c.Get(ctx, "key")
		if err == nil {
			t.Error("Get() should return error on cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Get() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Set returns error on cancelled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
		if err == nil {
			t.Error("Set() should return error on cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Set() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Delete returns error on cancelled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := c.Delete(ctx, "key")
		if err == nil {
			t.Error("Delete() should return error on cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Delete() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Exists returns error on cancelled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := c.Exists(ctx, "key")
		if err == nil {
			t.Error("Exists() should return error on cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Exists() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Clear returns error on cancelled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := c.Clear(ctx)
		if err == nil {
			t.Error("Clear() should return error on cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Clear() error = %v, want context.Canceled", err)
		}
	})
}

func TestCache_Set_EmptyKeyValidation(t *testing.T) {
	t.Parallel()
	// Empty key validation happens after context check and before Redis call.
	// Since we can't test with a real Redis client, we verify the constant exists.
	_ = cache.ErrInvalidKey
}

func TestCache_Client(t *testing.T) {
	t.Parallel()

	c := NewCacheFromClient(nil, "test:")

	client := c.Client()
	if client != nil {
		t.Error("Client() should return nil for cache created with nil client")
	}
}

// TestCache_InterfaceCompliance verifies that Cache implements the required interfaces.
func TestCache_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// These compile-time checks verify interface compliance
	var _ cache.Cache = (*Cache)(nil)
	var _ cache.StatsProvider = (*Cache)(nil)
}
