package secrets

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryProvider is an in-memory implementation of Provider for testing.
type MemoryProvider struct {
	mu      sync.RWMutex
	secrets map[string]*memorySecret
}

type memorySecret struct {
	path     string
	data     map[string]interface{}
	version  int
	created  time.Time
	updated  time.Time
	metadata map[string]string
	versions []VersionInfo
}

// NewMemoryProvider creates a new in-memory secrets provider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		secrets: make(map[string]*memorySecret),
	}
}

// Name returns the provider name.
func (p *MemoryProvider) Name() string {
	return "memory"
}

// GetSecret retrieves a secret by path.
func (p *MemoryProvider) GetSecret(ctx context.Context, path string) (*Secret, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	s, ok := p.secrets[path]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", path)
	}

	// Copy data to avoid external mutations
	dataCopy := make(map[string]interface{})
	for k, v := range s.data {
		dataCopy[k] = v
	}

	return &Secret{
		Path:    path,
		Data:    dataCopy,
		Version: s.version,
		Metadata: SecretMetadata{
			CreatedAt:   s.created,
			UpdatedAt:   s.updated,
			Version:     s.version,
			VersionInfo: s.versions,
		},
	}, nil
}

// PutSecret creates or updates a secret.
func (p *MemoryProvider) PutSecret(ctx context.Context, path string, data map[string]interface{}, opts SecretOptions) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	// Copy data to avoid external mutations
	dataCopy := make(map[string]interface{})
	for k, v := range data {
		dataCopy[k] = v
	}

	s, ok := p.secrets[path]
	if !ok {
		s = &memorySecret{
			path:     path,
			created:  now,
			version:  0,
			versions: make([]VersionInfo, 0),
			metadata: opts.Metadata,
		}
		p.secrets[path] = s
	}

	s.data = dataCopy
	s.updated = now
	s.version++
	s.versions = append(s.versions, VersionInfo{
		Version:   s.version,
		CreatedAt: now,
	})

	if opts.Metadata != nil {
		s.metadata = opts.Metadata
	}

	return nil
}

// DeleteSecret removes a secret.
func (p *MemoryProvider) DeleteSecret(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.secrets[path]; !ok {
		return fmt.Errorf("secret %s not found", path)
	}

	delete(p.secrets, path)
	return nil
}

// ListSecrets lists secret paths.
func (p *MemoryProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	paths := make([]string, 0)
	for path := range p.secrets {
		if prefix == "" || strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

// RotateSecret triggers secret rotation.
func (p *MemoryProvider) RotateSecret(ctx context.Context, path string) (*Secret, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.secrets[path]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", path)
	}

	now := time.Now()

	// Generate new secret data (simulate rotation)
	newData := make(map[string]interface{})
	for k := range s.data {
		newData[k] = uuid.New().String()
	}

	s.data = newData
	s.updated = now
	s.version++
	s.versions = append(s.versions, VersionInfo{
		Version:   s.version,
		CreatedAt: now,
	})

	// Copy data for return
	dataCopy := make(map[string]interface{})
	for k, v := range s.data {
		dataCopy[k] = v
	}

	return &Secret{
		Path:    path,
		Data:    dataCopy,
		Version: s.version,
		Metadata: SecretMetadata{
			CreatedAt:   s.created,
			UpdatedAt:   s.updated,
			Version:     s.version,
			VersionInfo: s.versions,
		},
	}, nil
}

// GetSecretMetadata retrieves secret metadata without the value.
func (p *MemoryProvider) GetSecretMetadata(ctx context.Context, path string) (*SecretMetadata, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	s, ok := p.secrets[path]
	if !ok {
		return nil, fmt.Errorf("secret %s not found", path)
	}

	return &SecretMetadata{
		CreatedAt:   s.created,
		UpdatedAt:   s.updated,
		Version:     s.version,
		VersionInfo: s.versions,
	}, nil
}

// Close releases provider resources.
func (p *MemoryProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.secrets = make(map[string]*memorySecret)
	return nil
}

// SecretCount returns the number of secrets for testing.
func (p *MemoryProvider) SecretCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.secrets)
}
