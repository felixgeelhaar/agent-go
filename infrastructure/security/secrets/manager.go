// Package secrets provides secret management for agent operations.
package secrets

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
)

// Manager defines the interface for secret management.
type Manager interface {
	// Get retrieves a secret by key.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a secret.
	Set(ctx context.Context, key, value string) error

	// Delete removes a secret.
	Delete(ctx context.Context, key string) error

	// List returns all secret keys matching the prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Exists checks if a secret exists.
	Exists(ctx context.Context, key string) (bool, error)
}

// ErrSecretNotFound is returned when a secret is not found.
var ErrSecretNotFound = errors.New("secret not found")

// ErrSecretReadOnly is returned when trying to write to a read-only manager.
var ErrSecretReadOnly = errors.New("secret manager is read-only")

// EnvManager implements Manager using environment variables.
type EnvManager struct {
	prefix   string
	readOnly bool
}

// EnvOption configures the environment manager.
type EnvOption func(*EnvManager)

// WithPrefix sets the environment variable prefix.
func WithPrefix(prefix string) EnvOption {
	return func(m *EnvManager) {
		m.prefix = prefix
	}
}

// WithEnvReadOnly makes the manager read-only.
func WithEnvReadOnly() EnvOption {
	return func(m *EnvManager) {
		m.readOnly = true
	}
}

// NewEnvManager creates a new environment-based secret manager.
func NewEnvManager(opts ...EnvOption) *EnvManager {
	m := &EnvManager{}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *EnvManager) envKey(key string) string {
	if m.prefix != "" {
		return m.prefix + key
	}
	return key
}

// Get retrieves a secret from environment variables.
func (m *EnvManager) Get(ctx context.Context, key string) (string, error) {
	value := os.Getenv(m.envKey(key))
	if value == "" {
		return "", ErrSecretNotFound
	}
	return value, nil
}

// Set stores a secret in environment variables.
func (m *EnvManager) Set(ctx context.Context, key, value string) error {
	if m.readOnly {
		return ErrSecretReadOnly
	}
	return os.Setenv(m.envKey(key), value)
}

// Delete removes a secret from environment variables.
func (m *EnvManager) Delete(ctx context.Context, key string) error {
	if m.readOnly {
		return ErrSecretReadOnly
	}
	return os.Unsetenv(m.envKey(key))
}

// List returns all secret keys with the given prefix.
func (m *EnvManager) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := m.envKey(prefix)
	var keys []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], fullPrefix) {
			key := parts[0]
			if m.prefix != "" {
				key = strings.TrimPrefix(key, m.prefix)
			}
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// Exists checks if a secret exists.
func (m *EnvManager) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := os.LookupEnv(m.envKey(key))
	return exists, nil
}

// MemoryManager implements Manager using in-memory storage.
// Useful for testing and development.
type MemoryManager struct {
	mu       sync.RWMutex
	secrets  map[string]string
	readOnly bool
}

// MemoryOption configures the memory manager.
type MemoryOption func(*MemoryManager)

// WithMemoryReadOnly makes the manager read-only.
func WithMemoryReadOnly() MemoryOption {
	return func(m *MemoryManager) {
		m.readOnly = true
	}
}

// WithInitialSecrets sets initial secrets.
func WithInitialSecrets(secrets map[string]string) MemoryOption {
	return func(m *MemoryManager) {
		for k, v := range secrets {
			m.secrets[k] = v
		}
	}
}

// NewMemoryManager creates a new in-memory secret manager.
func NewMemoryManager(opts ...MemoryOption) *MemoryManager {
	m := &MemoryManager{
		secrets: make(map[string]string),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Get retrieves a secret from memory.
func (m *MemoryManager) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.secrets[key]
	if !exists {
		return "", ErrSecretNotFound
	}
	return value, nil
}

// Set stores a secret in memory.
func (m *MemoryManager) Set(ctx context.Context, key, value string) error {
	if m.readOnly {
		return ErrSecretReadOnly
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.secrets[key] = value
	return nil
}

// Delete removes a secret from memory.
func (m *MemoryManager) Delete(ctx context.Context, key string) error {
	if m.readOnly {
		return ErrSecretReadOnly
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.secrets[key]; !exists {
		return ErrSecretNotFound
	}
	delete(m.secrets, key)
	return nil
}

// List returns all secret keys with the given prefix.
func (m *MemoryManager) List(ctx context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for key := range m.secrets {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// Exists checks if a secret exists.
func (m *MemoryManager) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.secrets[key]
	return exists, nil
}

// ChainedManager chains multiple managers together.
// Reads go through managers in order until one succeeds.
// Writes go to the primary (first) manager only.
type ChainedManager struct {
	managers []Manager
}

// NewChainedManager creates a new chained manager.
// The first manager is primary for writes.
func NewChainedManager(managers ...Manager) *ChainedManager {
	return &ChainedManager{managers: managers}
}

// Get retrieves a secret from the first manager that has it.
func (m *ChainedManager) Get(ctx context.Context, key string) (string, error) {
	for _, manager := range m.managers {
		value, err := manager.Get(ctx, key)
		if err == nil {
			return value, nil
		}
		if !errors.Is(err, ErrSecretNotFound) {
			return "", err
		}
	}
	return "", ErrSecretNotFound
}

// Set stores a secret in the primary manager.
func (m *ChainedManager) Set(ctx context.Context, key, value string) error {
	if len(m.managers) == 0 {
		return errors.New("no managers configured")
	}
	return m.managers[0].Set(ctx, key, value)
}

// Delete removes a secret from the primary manager.
func (m *ChainedManager) Delete(ctx context.Context, key string) error {
	if len(m.managers) == 0 {
		return errors.New("no managers configured")
	}
	return m.managers[0].Delete(ctx, key)
}

// List returns all secret keys from all managers.
func (m *ChainedManager) List(ctx context.Context, prefix string) ([]string, error) {
	seen := make(map[string]bool)
	var keys []string

	for _, manager := range m.managers {
		managerKeys, err := manager.List(ctx, prefix)
		if err != nil {
			continue
		}
		for _, key := range managerKeys {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	return keys, nil
}

// Exists checks if a secret exists in any manager.
func (m *ChainedManager) Exists(ctx context.Context, key string) (bool, error) {
	for _, manager := range m.managers {
		exists, err := manager.Exists(ctx, key)
		if err != nil {
			continue
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// RedactedManager wraps a manager and redacts secret values in logs.
type RedactedManager struct {
	inner        Manager
	redactedKeys map[string]bool
}

// NewRedactedManager creates a manager that redacts specified keys.
func NewRedactedManager(inner Manager, redactedKeys ...string) *RedactedManager {
	keys := make(map[string]bool)
	for _, k := range redactedKeys {
		keys[k] = true
	}
	return &RedactedManager{
		inner:        inner,
		redactedKeys: keys,
	}
}

// Get retrieves a secret, returning redacted value for sensitive keys.
func (m *RedactedManager) Get(ctx context.Context, key string) (string, error) {
	return m.inner.Get(ctx, key)
}

// GetRedacted returns the secret value or "[REDACTED]" for sensitive keys.
func (m *RedactedManager) GetRedacted(ctx context.Context, key string) (string, error) {
	value, err := m.inner.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if m.redactedKeys[key] {
		return "[REDACTED]", nil
	}
	return value, nil
}

// Set stores a secret.
func (m *RedactedManager) Set(ctx context.Context, key, value string) error {
	return m.inner.Set(ctx, key, value)
}

// Delete removes a secret.
func (m *RedactedManager) Delete(ctx context.Context, key string) error {
	return m.inner.Delete(ctx, key)
}

// List returns all secret keys.
func (m *RedactedManager) List(ctx context.Context, prefix string) ([]string, error) {
	return m.inner.List(ctx, prefix)
}

// Exists checks if a secret exists.
func (m *RedactedManager) Exists(ctx context.Context, key string) (bool, error) {
	return m.inner.Exists(ctx, key)
}

// IsRedacted checks if a key is redacted.
func (m *RedactedManager) IsRedacted(key string) bool {
	return m.redactedKeys[key]
}
