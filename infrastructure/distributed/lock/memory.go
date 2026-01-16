package lock

import (
	"context"
	"sync"
	"time"
)

// MemoryLockStore provides shared lock storage for testing distributed scenarios.
type MemoryLockStore struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

// NewMemoryLockStore creates a new shared lock store.
func NewMemoryLockStore() *MemoryLockStore {
	return &MemoryLockStore{
		locks: make(map[string]*lockEntry),
	}
}

type lockEntry struct {
	holderID   string
	acquiredAt time.Time
	expiresAt  time.Time
}

// MemoryLock implements Lock using in-memory storage.
// Useful for testing and single-node deployments.
type MemoryLock struct {
	store    *MemoryLockStore
	holderID string
}

// MemoryLockOption configures the memory lock.
type MemoryLockOption func(*MemoryLock)

// WithHolderID sets the holder ID for this locker.
func WithHolderID(id string) MemoryLockOption {
	return func(l *MemoryLock) {
		l.holderID = id
	}
}

// WithStore sets a shared lock store.
func WithStore(store *MemoryLockStore) MemoryLockOption {
	return func(l *MemoryLock) {
		l.store = store
	}
}

// NewMemoryLock creates a new in-memory lock.
func NewMemoryLock(opts ...MemoryLockOption) *MemoryLock {
	l := &MemoryLock{
		store:    NewMemoryLockStore(), // Default to own store
		holderID: generateLockID(),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// ID returns the unique identifier for this locker.
func (l *MemoryLock) ID() string {
	return l.holderID
}

// Acquire attempts to acquire the lock.
func (l *MemoryLock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, ErrInvalidTTL
	}

	l.store.mu.Lock()
	defer l.store.mu.Unlock()

	now := time.Now()

	// Check if lock exists and is still valid
	if entry, exists := l.store.locks[key]; exists {
		if entry.expiresAt.After(now) && entry.holderID != l.holderID {
			return false, nil // Lock held by someone else
		}
	}

	// Acquire the lock
	l.store.locks[key] = &lockEntry{
		holderID:   l.holderID,
		acquiredAt: now,
		expiresAt:  now.Add(ttl),
	}
	return true, nil
}

// Release releases the lock.
func (l *MemoryLock) Release(ctx context.Context, key string) error {
	l.store.mu.Lock()
	defer l.store.mu.Unlock()

	entry, exists := l.store.locks[key]
	if !exists {
		return ErrLockNotHeld
	}

	if entry.holderID != l.holderID {
		return ErrLockNotHeld
	}

	delete(l.store.locks, key)
	return nil
}

// Extend extends the TTL of a held lock.
func (l *MemoryLock) Extend(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrInvalidTTL
	}

	l.store.mu.Lock()
	defer l.store.mu.Unlock()

	entry, exists := l.store.locks[key]
	if !exists {
		return ErrLockNotHeld
	}

	if entry.holderID != l.holderID {
		return ErrLockNotHeld
	}

	now := time.Now()
	if entry.expiresAt.Before(now) {
		return ErrLockExpired
	}

	entry.expiresAt = now.Add(ttl)
	return nil
}

// IsHeld checks if the lock is currently held.
func (l *MemoryLock) IsHeld(ctx context.Context, key string) (bool, error) {
	l.store.mu.Lock()
	defer l.store.mu.Unlock()

	entry, exists := l.store.locks[key]
	if !exists {
		return false, nil
	}

	return entry.expiresAt.After(time.Now()), nil
}

// WithLock executes a function while holding the lock.
func (l *MemoryLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	acquired, err := l.Acquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrLockHeld
	}
	defer func() { _ = l.Release(ctx, key) }()

	return fn(ctx)
}

// Info returns information about a lock.
func (l *MemoryLock) Info(key string) (*LockInfo, bool) {
	l.store.mu.Lock()
	defer l.store.mu.Unlock()

	entry, exists := l.store.locks[key]
	if !exists {
		return nil, false
	}

	return &LockInfo{
		Key:        key,
		HolderID:   entry.holderID,
		AcquiredAt: entry.acquiredAt,
		ExpiresAt:  entry.expiresAt,
	}, true
}

// Cleanup removes expired locks.
func (l *MemoryLock) Cleanup() int {
	l.store.mu.Lock()
	defer l.store.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, entry := range l.store.locks {
		if entry.expiresAt.Before(now) {
			delete(l.store.locks, key)
			removed++
		}
	}
	return removed
}

// generateLockID creates a simple unique ID.
func generateLockID() string {
	return time.Now().Format("20060102150405.000000000")
}
