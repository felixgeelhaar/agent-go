// Package secrets provides secret management operation tools.
package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Provider defines the interface for secret management operations.
// Implementations exist for HashiCorp Vault and AWS Secrets Manager.
type Provider interface {
	// Name returns the provider name (e.g., "vault", "aws-secrets-manager").
	Name() string

	// GetSecret retrieves a secret by path/name.
	GetSecret(ctx context.Context, path string) (*Secret, error)

	// PutSecret creates or updates a secret.
	PutSecret(ctx context.Context, path string, data map[string]interface{}, opts SecretOptions) error

	// DeleteSecret removes a secret.
	DeleteSecret(ctx context.Context, path string) error

	// ListSecrets lists secret paths.
	ListSecrets(ctx context.Context, prefix string) ([]string, error)

	// RotateSecret triggers secret rotation.
	RotateSecret(ctx context.Context, path string) (*Secret, error)

	// GetSecretMetadata retrieves secret metadata without the value.
	GetSecretMetadata(ctx context.Context, path string) (*SecretMetadata, error)

	// Close releases provider resources.
	Close() error
}

// Secret represents a retrieved secret.
type Secret struct {
	Path      string                 `json:"path"`
	Data      map[string]interface{} `json:"data"`
	Metadata  SecretMetadata         `json:"metadata"`
	Version   int                    `json:"version,omitempty"`
}

// SecretMetadata contains metadata about a secret.
type SecretMetadata struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int       `json:"version"`
	VersionInfo []VersionInfo `json:"version_info,omitempty"`
}

// VersionInfo contains information about a secret version.
type VersionInfo struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Deleted   bool      `json:"deleted,omitempty"`
}

// SecretOptions configures secret storage.
type SecretOptions struct {
	// TTL is the time-to-live for the secret.
	TTL time.Duration `json:"ttl,omitempty"`

	// Metadata for the secret.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Config configures the secrets pack.
type Config struct {
	// Provider is the secrets provider (required).
	Provider Provider

	// ReadOnly disables write/delete operations.
	ReadOnly bool

	// AllowDelete enables delete operations (requires !ReadOnly).
	AllowDelete bool

	// AllowRotate enables rotation operations.
	AllowRotate bool

	// Timeout for operations.
	Timeout time.Duration

	// RequireApproval requires approval for all operations.
	// Note: All secrets operations are marked as requiring approval by default.
	RequireApproval bool
}

// Option configures the secrets pack.
type Option func(*Config)

// WithWriteAccess enables write operations.
func WithWriteAccess() Option {
	return func(c *Config) {
		c.ReadOnly = false
	}
}

// WithDeleteAccess enables delete operations.
func WithDeleteAccess() Option {
	return func(c *Config) {
		c.AllowDelete = true
	}
}

// WithRotationAccess enables rotation operations.
func WithRotationAccess() Option {
	return func(c *Config) {
		c.AllowRotate = true
	}
}

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// New creates the secrets pack.
func New(provider Provider, opts ...Option) (*pack.Pack, error) {
	if provider == nil {
		return nil, errors.New("secrets provider is required")
	}

	cfg := Config{
		Provider:        provider,
		ReadOnly:        true, // Read-only by default for safety
		AllowDelete:    false,
		AllowRotate:    false,
		Timeout:         30 * time.Second,
		RequireApproval: true, // Secrets always require approval
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	builder := pack.NewBuilder("secrets").
		WithDescription(fmt.Sprintf("Secret management operations (%s)", provider.Name())).
		WithVersion("1.0.0").
		AddTools(
			listSecretsTool(&cfg),
			getSecretTool(&cfg),
			getSecretMetadataTool(&cfg),
		).
		// All secrets operations require Act state (they're sensitive)
		AllowInState(agent.StateAct, "secret_list", "secret_get", "secret_metadata")

	// Add write tool if enabled
	if !cfg.ReadOnly {
		builder = builder.AddTools(putSecretTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, "secret_put")
	}

	// Add delete tool if enabled
	if cfg.AllowDelete {
		builder = builder.AddTools(deleteSecretTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, "secret_delete")
	}

	// Add rotate tool if enabled
	if cfg.AllowRotate {
		builder = builder.AddTools(rotateSecretTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, "secret_rotate")
	}

	return builder.Build(), nil
}

// listSecretsInput is the input for the secret_list tool.
type listSecretsInput struct {
	Prefix string `json:"prefix,omitempty"`
}

// listSecretsOutput is the output for the secret_list tool.
type listSecretsOutput struct {
	Provider string   `json:"provider"`
	Prefix   string   `json:"prefix,omitempty"`
	Paths    []string `json:"paths"`
	Count    int      `json:"count"`
}

func listSecretsTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("secret_list").
		WithDescription("List secret paths").
		ReadOnly().
		RequiresApproval(). // Listing secrets still requires approval
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in listSecretsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			paths, err := cfg.Provider.ListSecrets(ctx, in.Prefix)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list secrets: %w", err)
			}

			out := listSecretsOutput{
				Provider: cfg.Provider.Name(),
				Prefix:   in.Prefix,
				Paths:    paths,
				Count:    len(paths),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// getSecretInput is the input for the secret_get tool.
type getSecretInput struct {
	Path string `json:"path"`
}

// getSecretOutput is the output for the secret_get tool.
type getSecretOutput struct {
	Path     string                 `json:"path"`
	Data     map[string]interface{} `json:"data"`
	Version  int                    `json:"version"`
	Metadata SecretMetadata         `json:"metadata"`
}

func getSecretTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("secret_get").
		WithDescription("Retrieve a secret value").
		ReadOnly().
		RequiresApproval(). // Getting secrets requires approval
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in getSecretInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Path == "" {
				return tool.Result{}, errors.New("path is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			secret, err := cfg.Provider.GetSecret(ctx, in.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get secret: %w", err)
			}

			out := getSecretOutput{
				Path:     secret.Path,
				Data:     secret.Data,
				Version:  secret.Version,
				Metadata: secret.Metadata,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// getSecretMetadataInput is the input for the secret_metadata tool.
type getSecretMetadataInput struct {
	Path string `json:"path"`
}

// getSecretMetadataOutput is the output for the secret_metadata tool.
type getSecretMetadataOutput struct {
	Path     string         `json:"path"`
	Metadata SecretMetadata `json:"metadata"`
}

func getSecretMetadataTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("secret_metadata").
		WithDescription("Get secret metadata without retrieving the value").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in getSecretMetadataInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Path == "" {
				return tool.Result{}, errors.New("path is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			metadata, err := cfg.Provider.GetSecretMetadata(ctx, in.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get secret metadata: %w", err)
			}

			out := getSecretMetadataOutput{
				Path:     in.Path,
				Metadata: *metadata,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// putSecretInput is the input for the secret_put tool.
type putSecretInput struct {
	Path     string                 `json:"path"`
	Data     map[string]interface{} `json:"data"`
	TTL      string                 `json:"ttl,omitempty"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

// putSecretOutput is the output for the secret_put tool.
type putSecretOutput struct {
	Path    string `json:"path"`
	Version int    `json:"version"`
	Created bool   `json:"created"`
}

func putSecretTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("secret_put").
		WithDescription("Create or update a secret").
		Destructive().
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in putSecretInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Path == "" {
				return tool.Result{}, errors.New("path is required")
			}
			if in.Data == nil {
				return tool.Result{}, errors.New("data is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			var ttl time.Duration
			if in.TTL != "" {
				var err error
				ttl, err = time.ParseDuration(in.TTL)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid ttl: %w", err)
				}
			}

			opts := SecretOptions{
				TTL:      ttl,
				Metadata: in.Metadata,
			}

			err := cfg.Provider.PutSecret(ctx, in.Path, in.Data, opts)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to put secret: %w", err)
			}

			out := putSecretOutput{
				Path:    in.Path,
				Version: 1, // Will be updated by provider if versioned
				Created: true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// deleteSecretInput is the input for the secret_delete tool.
type deleteSecretInput struct {
	Path string `json:"path"`
}

// deleteSecretOutput is the output for the secret_delete tool.
type deleteSecretOutput struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

func deleteSecretTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("secret_delete").
		WithDescription("Delete a secret").
		Destructive().
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in deleteSecretInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Path == "" {
				return tool.Result{}, errors.New("path is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			err := cfg.Provider.DeleteSecret(ctx, in.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete secret: %w", err)
			}

			out := deleteSecretOutput{
				Path:    in.Path,
				Deleted: true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// rotateSecretInput is the input for the secret_rotate tool.
type rotateSecretInput struct {
	Path string `json:"path"`
}

// rotateSecretOutput is the output for the secret_rotate tool.
type rotateSecretOutput struct {
	Path       string `json:"path"`
	NewVersion int    `json:"new_version"`
	Rotated    bool   `json:"rotated"`
}

func rotateSecretTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("secret_rotate").
		WithDescription("Rotate a secret to a new value").
		Destructive().
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in rotateSecretInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Path == "" {
				return tool.Result{}, errors.New("path is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			secret, err := cfg.Provider.RotateSecret(ctx, in.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to rotate secret: %w", err)
			}

			out := rotateSecretOutput{
				Path:       in.Path,
				NewVersion: secret.Version,
				Rotated:    true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
