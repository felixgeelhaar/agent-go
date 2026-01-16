// Package cloud provides cloud storage operation tools.
package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Provider defines the interface for cloud storage operations.
// Implementations exist for AWS S3, GCP Cloud Storage, and Azure Blob Storage.
type Provider interface {
	// Name returns the provider name (e.g., "aws", "gcp", "azure").
	Name() string

	// ListBuckets lists all buckets/containers.
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	// ListObjects lists objects in a bucket with optional prefix.
	ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]ObjectInfo, error)

	// GetObject retrieves an object's content.
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectMetadata, error)

	// PutObject uploads an object.
	PutObject(ctx context.Context, bucket, key string, data io.Reader, metadata *ObjectMetadata) error

	// DeleteObject deletes an object.
	DeleteObject(ctx context.Context, bucket, key string) error

	// GetObjectMetadata retrieves object metadata without content.
	GetObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error)

	// BucketExists checks if a bucket exists.
	BucketExists(ctx context.Context, bucket string) (bool, error)
}

// BucketInfo contains information about a storage bucket.
type BucketInfo struct {
	Name         string            `json:"name"`
	Region       string            `json:"region,omitempty"`
	CreationDate time.Time         `json:"creation_date,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// ObjectInfo contains information about a stored object.
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
	StorageClass string    `json:"storage_class,omitempty"`
}

// ObjectMetadata contains metadata for an object.
type ObjectMetadata struct {
	ContentType     string            `json:"content_type,omitempty"`
	ContentLength   int64             `json:"content_length,omitempty"`
	ContentEncoding string            `json:"content_encoding,omitempty"`
	ETag            string            `json:"etag,omitempty"`
	LastModified    time.Time         `json:"last_modified,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Config configures the cloud pack.
type Config struct {
	// Provider is the cloud storage provider (required).
	Provider Provider

	// ReadOnly disables all write operations.
	ReadOnly bool

	// AllowDelete enables delete operations (requires !ReadOnly).
	AllowDelete bool

	// MaxObjectSize limits the size of objects that can be read.
	MaxObjectSize int64

	// Timeout for operations.
	Timeout time.Duration
}

// Option configures the cloud pack.
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

// WithMaxObjectSize sets the maximum object size for reads.
func WithMaxObjectSize(size int64) Option {
	return func(c *Config) {
		c.MaxObjectSize = size
	}
}

// WithTimeout sets the operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// New creates the cloud pack.
func New(provider Provider, opts ...Option) (*pack.Pack, error) {
	if provider == nil {
		return nil, errors.New("cloud provider is required")
	}

	cfg := Config{
		Provider:      provider,
		ReadOnly:      true, // Read-only by default for safety
		AllowDelete:   false,
		MaxObjectSize: 10 * 1024 * 1024, // 10MB default
		Timeout:       60 * time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	builder := pack.NewBuilder("cloud").
		WithDescription(fmt.Sprintf("Cloud storage operations (%s)", provider.Name())).
		WithVersion("1.0.0").
		AddTools(
			listBucketsTool(&cfg),
			listObjectsTool(&cfg),
			getObjectTool(&cfg),
			getObjectMetadataTool(&cfg),
		).
		AllowInState(agent.StateExplore, "cloud_list_buckets", "cloud_list_objects", "cloud_get_object", "cloud_get_object_metadata").
		AllowInState(agent.StateValidate, "cloud_list_buckets", "cloud_list_objects", "cloud_get_object", "cloud_get_object_metadata")

	readTools := []string{"cloud_list_buckets", "cloud_list_objects", "cloud_get_object", "cloud_get_object_metadata"}

	// Add write tools if enabled
	if !cfg.ReadOnly {
		builder = builder.AddTools(putObjectTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, append(readTools, "cloud_put_object")...)

		// Add delete tool if enabled
		if cfg.AllowDelete {
			builder = builder.AddTools(deleteObjectTool(&cfg))
			builder = builder.AllowInState(agent.StateAct, append(readTools, "cloud_put_object", "cloud_delete_object")...)
		}
	} else {
		builder = builder.AllowInState(agent.StateAct, readTools...)
	}

	return builder.Build(), nil
}

// listBucketsOutput is the output for the cloud_list_buckets tool.
type listBucketsOutput struct {
	Provider string       `json:"provider"`
	Buckets  []BucketInfo `json:"buckets"`
	Count    int          `json:"count"`
}

func listBucketsTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("cloud_list_buckets").
		WithDescription("List all storage buckets/containers").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			buckets, err := cfg.Provider.ListBuckets(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list buckets: %w", err)
			}

			out := listBucketsOutput{
				Provider: cfg.Provider.Name(),
				Buckets:  buckets,
				Count:    len(buckets),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// listObjectsInput is the input for the cloud_list_objects tool.
type listObjectsInput struct {
	Bucket  string `json:"bucket"`
	Prefix  string `json:"prefix,omitempty"`
	MaxKeys int    `json:"max_keys,omitempty"`
}

// listObjectsOutput is the output for the cloud_list_objects tool.
type listObjectsOutput struct {
	Bucket  string       `json:"bucket"`
	Prefix  string       `json:"prefix,omitempty"`
	Objects []ObjectInfo `json:"objects"`
	Count   int          `json:"count"`
}

func listObjectsTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("cloud_list_objects").
		WithDescription("List objects in a bucket").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in listObjectsInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Bucket == "" {
				return tool.Result{}, errors.New("bucket is required")
			}

			maxKeys := in.MaxKeys
			if maxKeys == 0 {
				maxKeys = 1000
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			objects, err := cfg.Provider.ListObjects(ctx, in.Bucket, in.Prefix, maxKeys)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list objects: %w", err)
			}

			out := listObjectsOutput{
				Bucket:  in.Bucket,
				Prefix:  in.Prefix,
				Objects: objects,
				Count:   len(objects),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// getObjectInput is the input for the cloud_get_object tool.
type getObjectInput struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// getObjectOutput is the output for the cloud_get_object tool.
type getObjectOutput struct {
	Bucket      string          `json:"bucket"`
	Key         string          `json:"key"`
	Content     string          `json:"content"`
	ContentType string          `json:"content_type,omitempty"`
	Size        int64           `json:"size"`
	Truncated   bool            `json:"truncated,omitempty"`
	Metadata    *ObjectMetadata `json:"metadata,omitempty"`
}

func getObjectTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("cloud_get_object").
		WithDescription("Get an object's content from a bucket").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in getObjectInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Bucket == "" {
				return tool.Result{}, errors.New("bucket is required")
			}
			if in.Key == "" {
				return tool.Result{}, errors.New("key is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			reader, metadata, err := cfg.Provider.GetObject(ctx, in.Bucket, in.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get object: %w", err)
			}
			defer reader.Close()

			// Read content with size limit
			var content []byte
			truncated := false
			if metadata != nil && metadata.ContentLength > cfg.MaxObjectSize {
				content = make([]byte, cfg.MaxObjectSize)
				_, err = io.ReadFull(reader, content)
				truncated = true
			} else {
				content, err = io.ReadAll(io.LimitReader(reader, cfg.MaxObjectSize+1))
				if int64(len(content)) > cfg.MaxObjectSize {
					content = content[:cfg.MaxObjectSize]
					truncated = true
				}
			}
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return tool.Result{}, fmt.Errorf("failed to read object content: %w", err)
			}

			out := getObjectOutput{
				Bucket:    in.Bucket,
				Key:       in.Key,
				Content:   string(content),
				Size:      int64(len(content)),
				Truncated: truncated,
				Metadata:  metadata,
			}

			if metadata != nil {
				out.ContentType = metadata.ContentType
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// getObjectMetadataInput is the input for the cloud_get_object_metadata tool.
type getObjectMetadataInput struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// getObjectMetadataOutput is the output for the cloud_get_object_metadata tool.
type getObjectMetadataOutput struct {
	Bucket   string          `json:"bucket"`
	Key      string          `json:"key"`
	Metadata *ObjectMetadata `json:"metadata"`
}

func getObjectMetadataTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("cloud_get_object_metadata").
		WithDescription("Get metadata for an object without downloading content").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in getObjectMetadataInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Bucket == "" {
				return tool.Result{}, errors.New("bucket is required")
			}
			if in.Key == "" {
				return tool.Result{}, errors.New("key is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			metadata, err := cfg.Provider.GetObjectMetadata(ctx, in.Bucket, in.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get object metadata: %w", err)
			}

			out := getObjectMetadataOutput{
				Bucket:   in.Bucket,
				Key:      in.Key,
				Metadata: metadata,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// putObjectInput is the input for the cloud_put_object tool.
type putObjectInput struct {
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	Content     string            `json:"content"`
	ContentType string            `json:"content_type,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// putObjectOutput is the output for the cloud_put_object tool.
type putObjectOutput struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	Size   int64  `json:"size"`
}

func putObjectTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("cloud_put_object").
		WithDescription("Upload an object to a bucket").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in putObjectInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Bucket == "" {
				return tool.Result{}, errors.New("bucket is required")
			}
			if in.Key == "" {
				return tool.Result{}, errors.New("key is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			content := strings.NewReader(in.Content)
			metadata := &ObjectMetadata{
				ContentType: in.ContentType,
				Metadata:    in.Metadata,
			}

			if metadata.ContentType == "" {
				metadata.ContentType = "application/octet-stream"
			}

			err := cfg.Provider.PutObject(ctx, in.Bucket, in.Key, content, metadata)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to put object: %w", err)
			}

			out := putObjectOutput{
				Bucket: in.Bucket,
				Key:    in.Key,
				Size:   int64(len(in.Content)),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// deleteObjectInput is the input for the cloud_delete_object tool.
type deleteObjectInput struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// deleteObjectOutput is the output for the cloud_delete_object tool.
type deleteObjectOutput struct {
	Bucket  string `json:"bucket"`
	Key     string `json:"key"`
	Deleted bool   `json:"deleted"`
}

func deleteObjectTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("cloud_delete_object").
		WithDescription("Delete an object from a bucket").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in deleteObjectInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Bucket == "" {
				return tool.Result{}, errors.New("bucket is required")
			}
			if in.Key == "" {
				return tool.Result{}, errors.New("key is required")
			}

			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			err := cfg.Provider.DeleteObject(ctx, in.Bucket, in.Key)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete object: %w", err)
			}

			out := deleteObjectOutput{
				Bucket:  in.Bucket,
				Key:     in.Key,
				Deleted: true,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}
