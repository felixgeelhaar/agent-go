package lock

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLockAcquireRelease(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	acquired, err := l.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired")
	}

	err = l.Release(ctx, "test-key")
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}
}

func TestMemoryLockReacquire(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	// Acquire first time
	_, _ = l.Acquire(ctx, "test-key", 10*time.Second)

	// Same holder should be able to reacquire
	acquired, err := l.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("reacquire failed: %v", err)
	}
	if !acquired {
		t.Error("expected same holder to reacquire lock")
	}
}

func TestMemoryLockDifferentHolder(t *testing.T) {
	ctx := context.Background()
	// Use shared store for multi-holder testing
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	// First holder acquires
	_, _ = l1.Acquire(ctx, "test-key", 10*time.Second)

	// Second holder should not be able to acquire
	acquired, err := l2.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if acquired {
		t.Error("expected different holder not to acquire held lock")
	}
}

func TestMemoryLockExpiration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	// First holder acquires with short TTL
	_, _ = l1.Acquire(ctx, "test-key", 10*time.Millisecond)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Second holder should now be able to acquire
	acquired, err := l2.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire expired lock")
	}
}

func TestMemoryLockExtend(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	_, _ = l.Acquire(ctx, "test-key", 100*time.Millisecond)

	// Extend the lock
	err := l.Extend(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("extend failed: %v", err)
	}

	// Check info shows extended expiry
	info, _ := l.Info("test-key")
	if info.ExpiresAt.Before(time.Now().Add(5 * time.Second)) {
		t.Error("expected lock to be extended")
	}
}

func TestMemoryLockExtendNotHeld(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	err := l.Extend(ctx, "test-key", 10*time.Second)
	if err != ErrLockNotHeld {
		t.Errorf("expected ErrLockNotHeld, got %v", err)
	}
}

func TestMemoryLockExtendDifferentHolder(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	_, _ = l1.Acquire(ctx, "test-key", 10*time.Second)

	err := l2.Extend(ctx, "test-key", 10*time.Second)
	if err != ErrLockNotHeld {
		t.Errorf("expected ErrLockNotHeld, got %v", err)
	}
}

func TestMemoryLockExtendExpired(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	_, _ = l.Acquire(ctx, "test-key", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	err := l.Extend(ctx, "test-key", 10*time.Second)
	if err != ErrLockExpired {
		t.Errorf("expected ErrLockExpired, got %v", err)
	}
}

func TestMemoryLockIsHeld(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	held, err := l.IsHeld(ctx, "test-key")
	if err != nil {
		t.Fatalf("isHeld failed: %v", err)
	}
	if held {
		t.Error("expected lock not to be held")
	}

	_, _ = l.Acquire(ctx, "test-key", 10*time.Second)

	held, err = l.IsHeld(ctx, "test-key")
	if err != nil {
		t.Fatalf("isHeld failed: %v", err)
	}
	if !held {
		t.Error("expected lock to be held")
	}
}

func TestMemoryLockIsHeldExpired(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	_, _ = l.Acquire(ctx, "test-key", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	held, err := l.IsHeld(ctx, "test-key")
	if err != nil {
		t.Fatalf("isHeld failed: %v", err)
	}
	if held {
		t.Error("expected expired lock not to be held")
	}
}

func TestMemoryLockReleaseNotHeld(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	err := l.Release(ctx, "test-key")
	if err != ErrLockNotHeld {
		t.Errorf("expected ErrLockNotHeld, got %v", err)
	}
}

func TestMemoryLockReleaseDifferentHolder(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	_, _ = l1.Acquire(ctx, "test-key", 10*time.Second)

	err := l2.Release(ctx, "test-key")
	if err != ErrLockNotHeld {
		t.Errorf("expected ErrLockNotHeld, got %v", err)
	}
}

func TestMemoryLockInvalidTTL(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	_, err := l.Acquire(ctx, "test-key", 0)
	if err != ErrInvalidTTL {
		t.Errorf("expected ErrInvalidTTL for zero TTL, got %v", err)
	}

	_, err = l.Acquire(ctx, "test-key", -1*time.Second)
	if err != ErrInvalidTTL {
		t.Errorf("expected ErrInvalidTTL for negative TTL, got %v", err)
	}
}

func TestMemoryLockWithLock(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	executed := false
	err := l.WithLock(ctx, "test-key", 10*time.Second, func(ctx context.Context) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("withLock failed: %v", err)
	}
	if !executed {
		t.Error("expected function to be executed")
	}

	// Lock should be released
	held, _ := l.IsHeld(ctx, "test-key")
	if held {
		t.Error("expected lock to be released after WithLock")
	}
}

func TestMemoryLockWithLockConflict(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	_, _ = l1.Acquire(ctx, "test-key", 10*time.Second)

	err := l2.WithLock(ctx, "test-key", 10*time.Second, func(ctx context.Context) error {
		return nil
	})

	if err != ErrLockHeld {
		t.Errorf("expected ErrLockHeld, got %v", err)
	}
}

func TestMemoryLockCleanup(t *testing.T) {
	l := NewMemoryLock()
	ctx := context.Background()

	// Acquire with short TTL
	_, _ = l.Acquire(ctx, "key1", 1*time.Millisecond)
	_, _ = l.Acquire(ctx, "key2", 10*time.Second)

	time.Sleep(5 * time.Millisecond)

	removed := l.Cleanup()
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// key2 should still exist
	held, _ := l.IsHeld(ctx, "key2")
	if !held {
		t.Error("expected key2 to still be held")
	}
}

func TestMemoryLockInfo(t *testing.T) {
	l := NewMemoryLock(WithHolderID("test-holder"))
	ctx := context.Background()

	_, exists := l.Info("test-key")
	if exists {
		t.Error("expected no info for unheld lock")
	}

	_, _ = l.Acquire(ctx, "test-key", 10*time.Second)

	info, exists := l.Info("test-key")
	if !exists {
		t.Error("expected info to exist")
	}
	if info.HolderID != "test-holder" {
		t.Errorf("expected holder ID test-holder, got %s", info.HolderID)
	}
	if info.Key != "test-key" {
		t.Errorf("expected key test-key, got %s", info.Key)
	}
}

func TestMemoryLockID(t *testing.T) {
	l1 := NewMemoryLock()
	l2 := NewMemoryLock(WithHolderID("custom-id"))

	if l1.ID() == "" {
		t.Error("expected auto-generated ID")
	}
	if l2.ID() != "custom-id" {
		t.Errorf("expected custom-id, got %s", l2.ID())
	}
}

func TestAcquireWithRetry(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	// First holder acquires with short TTL
	_, _ = l1.Acquire(ctx, "test-key", 50*time.Millisecond)

	// Second holder tries with retry
	acquired, err := AcquireWithRetry(ctx, l2, "test-key", 10*time.Second,
		WithRetryInterval(20*time.Millisecond),
		WithMaxRetries(5),
	)

	if err != nil {
		t.Fatalf("acquire with retry failed: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock after retry")
	}
}

func TestAcquireWithRetryFails(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	// First holder acquires with long TTL
	_, _ = l1.Acquire(ctx, "test-key", 10*time.Second)

	// Second holder tries with retry (should fail)
	acquired, err := AcquireWithRetry(ctx, l2, "test-key", 10*time.Second,
		WithRetryInterval(5*time.Millisecond),
		WithMaxRetries(2),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Error("expected not to acquire held lock")
	}
}

func TestAcquireWithRetryContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := NewMemoryLockStore()
	l1 := NewMemoryLock(WithStore(store), WithHolderID("holder-1"))
	l2 := NewMemoryLock(WithStore(store), WithHolderID("holder-2"))

	_, _ = l1.Acquire(ctx, "test-key", 10*time.Second)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := AcquireWithRetry(ctx, l2, "test-key", 10*time.Second,
		WithRetryInterval(10*time.Millisecond),
		WithMaxRetries(100),
	)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestAcquireWithRetryCallback(t *testing.T) {
	ctx := context.Background()
	l := NewMemoryLock()

	acquireCalled := false
	acquired, err := AcquireWithRetry(ctx, l, "test-key", 10*time.Second,
		WithOnAcquire(func(key string) {
			acquireCalled = true
			if key != "test-key" {
				t.Errorf("expected key test-key, got %s", key)
			}
		}),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}
	if !acquireCalled {
		t.Error("expected onAcquire callback to be called")
	}
}

func TestMemoryLockStore(t *testing.T) {
	store := NewMemoryLockStore()
	if store.locks == nil {
		t.Error("expected locks map to be initialized")
	}
}
