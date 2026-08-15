// Package secrets provides secret management tools for agent-go.
//
// Tools operate through a SecretStore backend. MemoryStore and FileStore are
// included for local/dev use; Vault/cloud providers can implement SecretStore.
//
// Security notes:
//   - secrets_list returns keys only (never values)
//   - FileStore writes mode 0600; treat it as local-dev, not production KMS
//   - Callers should avoid logging tool outputs that include secret values
package secrets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ErrNotFound indicates the secret key does not exist.
var ErrNotFound = errors.New("secret not found")

// Secret is a versioned secret record. Value may be empty in list views.
type Secret struct {
	Key       string    `json:"key"`
	Value     string    `json:"value,omitempty"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SecretStore is the backend for secret tools.
type SecretStore interface {
	Get(ctx context.Context, key string) (Secret, error)
	GetVersion(ctx context.Context, key string, version int) (Secret, error)
	Set(ctx context.Context, key, value string) (Secret, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]Secret, error)
	Rotate(ctx context.Context, key, newValue string) (Secret, error)
}

// Pack returns secret management tools backed by store.
func Pack(store SecretStore) *pack.Pack {
	if store == nil {
		panic("secrets.Pack: store is required")
	}
	p := &secretsPack{store: store}
	return pack.NewBuilder("secrets").
		WithDescription("Secret management tools for secure credential storage").
		WithVersion("0.1.0").
		AddTools(
			p.secretsGet(),
			p.secretsSet(),
			p.secretsDelete(),
			p.secretsList(),
			p.secretsRotate(),
			p.secretsVersion(),
		).
		AllowInState(agent.StateExplore, "secrets_list").
		AllowInState(agent.StateAct, "secrets_get", "secrets_set", "secrets_delete", "secrets_list", "secrets_rotate", "secrets_version").
		Build()
}

type secretsPack struct {
	store SecretStore
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *secretsPack) secretsGet() tool.Tool {
	return tool.NewBuilder("secrets_get").
		WithDescription("Retrieve a secret value by key").
		ReadOnly().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"key": json.RawMessage(`{"type":"string"}`),
		}, []string{"key"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Key string `json:"key"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Key == "" {
				return tool.Result{}, fmt.Errorf("%w: key is required", tool.ErrInvalidInput)
			}
			sec, err := p.store.Get(ctx, in.Key)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(sec)
		}).
		MustBuild()
}

func (p *secretsPack) secretsSet() tool.Tool {
	return tool.NewBuilder("secrets_set").
		WithDescription("Store or update a secret").
		Idempotent().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"key":   json.RawMessage(`{"type":"string"}`),
			"value": json.RawMessage(`{"type":"string"}`),
		}, []string{"key", "value"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Key == "" {
				return tool.Result{}, fmt.Errorf("%w: key is required", tool.ErrInvalidInput)
			}
			sec, err := p.store.Set(ctx, in.Key, in.Value)
			if err != nil {
				return tool.Result{}, err
			}
			// Do not echo value back.
			return resultOK(map[string]any{
				"key":        sec.Key,
				"version":    sec.Version,
				"updated_at": sec.UpdatedAt,
				"set":        true,
			})
		}).
		MustBuild()
}

func (p *secretsPack) secretsDelete() tool.Tool {
	return tool.NewBuilder("secrets_delete").
		WithDescription("Delete a secret").
		Destructive().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"key": json.RawMessage(`{"type":"string"}`),
		}, []string{"key"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Key string `json:"key"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if err := p.store.Delete(ctx, in.Key); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"key": in.Key, "deleted": true})
		}).
		MustBuild()
}

func (p *secretsPack) secretsList() tool.Tool {
	return tool.NewBuilder("secrets_list").
		WithDescription("List available secret keys (not values)").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"prefix": json.RawMessage(`{"type":"string"}`),
		}, nil)).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Prefix string `json:"prefix"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			secs, err := p.store.List(ctx, in.Prefix)
			if err != nil {
				return tool.Result{}, err
			}
			keys := make([]map[string]any, 0, len(secs))
			for _, s := range secs {
				keys = append(keys, map[string]any{
					"key":        s.Key,
					"version":    s.Version,
					"updated_at": s.UpdatedAt,
				})
			}
			return resultOK(map[string]any{"secrets": keys, "count": len(keys)})
		}).
		MustBuild()
}

func (p *secretsPack) secretsRotate() tool.Tool {
	return tool.NewBuilder("secrets_rotate").
		WithDescription("Rotate a secret to a new value (auto-generates if value omitted)").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"key":   json.RawMessage(`{"type":"string"}`),
			"value": json.RawMessage(`{"type":"string"}`),
		}, []string{"key"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Key == "" {
				return tool.Result{}, fmt.Errorf("%w: key is required", tool.ErrInvalidInput)
			}
			val := in.Value
			if val == "" {
				buf := make([]byte, 32)
				if _, err := rand.Read(buf); err != nil {
					return tool.Result{}, err
				}
				val = hex.EncodeToString(buf)
			}
			sec, err := p.store.Rotate(ctx, in.Key, val)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{
				"key":        sec.Key,
				"version":    sec.Version,
				"updated_at": sec.UpdatedAt,
				"rotated":    true,
			})
		}).
		MustBuild()
}

func (p *secretsPack) secretsVersion() tool.Tool {
	return tool.NewBuilder("secrets_version").
		WithDescription("Get a specific version of a secret").
		ReadOnly().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"key":     json.RawMessage(`{"type":"string"}`),
			"version": json.RawMessage(`{"type":"integer"}`),
		}, []string{"key", "version"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Key     string `json:"key"`
				Version int    `json:"version"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Key == "" || in.Version < 1 {
				return tool.Result{}, fmt.Errorf("%w: key and version>=1 are required", tool.ErrInvalidInput)
			}
			sec, err := p.store.GetVersion(ctx, in.Key, in.Version)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(sec)
		}).
		MustBuild()
}

// ---------------------------------------------------------------------------
// MemoryStore
// ---------------------------------------------------------------------------

type memEntry struct {
	versions []Secret // index 0 = version 1
}

// MemoryStore is an in-memory SecretStore for tests and ephemeral use.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]*memEntry
}

// NewMemoryStore creates an empty in-memory secret store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]*memEntry)}
}

func (s *MemoryStore) Get(_ context.Context, key string) (Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok || len(e.versions) == 0 {
		return Secret{}, ErrNotFound
	}
	return e.versions[len(e.versions)-1], nil
}

func (s *MemoryStore) GetVersion(_ context.Context, key string, version int) (Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok || version < 1 || version > len(e.versions) {
		return Secret{}, ErrNotFound
	}
	return e.versions[version-1], nil
}

func (s *MemoryStore) Set(_ context.Context, key, value string) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok {
		e = &memEntry{}
		s.data[key] = e
	}
	sec := Secret{Key: key, Value: value, Version: len(e.versions) + 1, UpdatedAt: time.Now().UTC()}
	e.versions = append(e.versions, sec)
	return sec, nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		return ErrNotFound
	}
	delete(s.data, key)
	return nil
}

func (s *MemoryStore) List(_ context.Context, prefix string) ([]Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Secret, 0, len(s.data))
	for k, e := range s.data {
		if prefix != "" && !hasPrefix(k, prefix) {
			continue
		}
		latest := e.versions[len(e.versions)-1]
		out = append(out, Secret{Key: latest.Key, Version: latest.Version, UpdatedAt: latest.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *MemoryStore) Rotate(ctx context.Context, key, newValue string) (Secret, error) {
	if _, err := s.Get(ctx, key); err != nil {
		return Secret{}, err
	}
	return s.Set(ctx, key, newValue)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ---------------------------------------------------------------------------
// FileStore — JSON file, mode 0600 (local/dev)
// ---------------------------------------------------------------------------

type fileData struct {
	Secrets map[string][]Secret `json:"secrets"`
}

// FileStore persists secrets to a JSON file (0600). For local/dev only.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates a file-backed secret store at path.
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	s := &FileStore{path: path}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := s.save(fileData{Secrets: map[string][]Secret{}}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *FileStore) load() (fileData, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{}, err
	}
	var d fileData
	if err := json.Unmarshal(b, &d); err != nil {
		return fileData{}, err
	}
	if d.Secrets == nil {
		d.Secrets = map[string][]Secret{}
	}
	return d, nil
}

func (s *FileStore) save(d fileData) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FileStore) Get(_ context.Context, key string) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Secret{}, err
	}
	vers := d.Secrets[key]
	if len(vers) == 0 {
		return Secret{}, ErrNotFound
	}
	return vers[len(vers)-1], nil
}

func (s *FileStore) GetVersion(_ context.Context, key string, version int) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Secret{}, err
	}
	vers := d.Secrets[key]
	if version < 1 || version > len(vers) {
		return Secret{}, ErrNotFound
	}
	return vers[version-1], nil
}

func (s *FileStore) Set(_ context.Context, key, value string) (Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return Secret{}, err
	}
	sec := Secret{Key: key, Value: value, Version: len(d.Secrets[key]) + 1, UpdatedAt: time.Now().UTC()}
	d.Secrets[key] = append(d.Secrets[key], sec)
	if err := s.save(d); err != nil {
		return Secret{}, err
	}
	return sec, nil
}

func (s *FileStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := d.Secrets[key]; !ok {
		return ErrNotFound
	}
	delete(d.Secrets, key)
	return s.save(d)
}

func (s *FileStore) List(_ context.Context, prefix string) ([]Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Secret, 0, len(d.Secrets))
	for k, vers := range d.Secrets {
		if prefix != "" && !hasPrefix(k, prefix) {
			continue
		}
		if len(vers) == 0 {
			continue
		}
		latest := vers[len(vers)-1]
		out = append(out, Secret{Key: latest.Key, Version: latest.Version, UpdatedAt: latest.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *FileStore) Rotate(ctx context.Context, key, newValue string) (Secret, error) {
	if _, err := s.Get(ctx, key); err != nil {
		return Secret{}, err
	}
	return s.Set(ctx, key, newValue)
}
