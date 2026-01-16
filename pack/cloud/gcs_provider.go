package cloud

import (
	"context"
	"fmt"
	"io"
	"time"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSProvider implements the Provider interface for Google Cloud Storage.
type GCSProvider struct {
	client    *gcs.Client
	projectID string
}

// GCSConfig configures the GCS provider.
type GCSConfig struct {
	ProjectID          string // GCP project ID (required for bucket listing)
	CredentialsFile    string // Optional: path to service account JSON file
	CredentialsJSON    []byte // Optional: service account JSON content
}

// NewGCSProvider creates a new Google Cloud Storage provider.
func NewGCSProvider(ctx context.Context, cfg GCSConfig) (*GCSProvider, error) {
	var opts []option.ClientOption

	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	} else if len(cfg.CredentialsJSON) > 0 {
		opts = append(opts, option.WithCredentialsJSON(cfg.CredentialsJSON))
	}
	// If no credentials provided, uses Application Default Credentials

	client, err := gcs.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSProvider{
		client:    client,
		projectID: cfg.ProjectID,
	}, nil
}

// Name returns the provider name.
func (p *GCSProvider) Name() string {
	return "gcp-storage"
}

// Close closes the GCS client.
func (p *GCSProvider) Close() error {
	return p.client.Close()
}

// ListBuckets lists all GCS buckets in the project.
func (p *GCSProvider) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	if p.projectID == "" {
		return nil, fmt.Errorf("project ID is required to list buckets")
	}

	var buckets []BucketInfo
	it := p.client.Buckets(ctx, p.projectID)

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list buckets: %w", err)
		}

		bucket := BucketInfo{
			Name:         attrs.Name,
			Region:       attrs.Location,
			CreationDate: attrs.Created,
		}
		if attrs.Labels != nil {
			bucket.Tags = attrs.Labels
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

// ListObjects lists objects in a bucket with optional prefix.
func (p *GCSProvider) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]ObjectInfo, error) {
	query := &gcs.Query{
		Prefix: prefix,
	}

	var objects []ObjectInfo
	count := 0
	it := p.client.Bucket(bucket).Objects(ctx, query)

	for {
		if maxKeys > 0 && count >= maxKeys {
			break
		}

		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		object := ObjectInfo{
			Key:          attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
			ETag:         attrs.Etag,
			StorageClass: attrs.StorageClass,
		}
		objects = append(objects, object)
		count++
	}

	return objects, nil
}

// GetObject retrieves an object's content.
func (p *GCSProvider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectMetadata, error) {
	obj := p.client.Bucket(bucket).Object(key)

	// Get object attributes first
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object attributes: %w", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create object reader: %w", err)
	}

	metadata := &ObjectMetadata{
		ContentType:     attrs.ContentType,
		ContentLength:   attrs.Size,
		ContentEncoding: attrs.ContentEncoding,
		ETag:            attrs.Etag,
		LastModified:    attrs.Updated,
		Metadata:        attrs.Metadata,
	}

	return reader, metadata, nil
}

// PutObject uploads an object.
func (p *GCSProvider) PutObject(ctx context.Context, bucket, key string, data io.Reader, metadata *ObjectMetadata) error {
	obj := p.client.Bucket(bucket).Object(key)
	writer := obj.NewWriter(ctx)

	if metadata != nil {
		if metadata.ContentType != "" {
			writer.ContentType = metadata.ContentType
		}
		if metadata.ContentEncoding != "" {
			writer.ContentEncoding = metadata.ContentEncoding
		}
		if metadata.Metadata != nil {
			writer.Metadata = metadata.Metadata
		}
	}

	if _, err := io.Copy(writer, data); err != nil {
		writer.Close()
		return fmt.Errorf("failed to write object: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize object: %w", err)
	}

	return nil
}

// DeleteObject deletes an object.
func (p *GCSProvider) DeleteObject(ctx context.Context, bucket, key string) error {
	obj := p.client.Bucket(bucket).Object(key)

	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// GetObjectMetadata retrieves object metadata without content.
func (p *GCSProvider) GetObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error) {
	obj := p.client.Bucket(bucket).Object(key)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	return &ObjectMetadata{
		ContentType:     attrs.ContentType,
		ContentLength:   attrs.Size,
		ContentEncoding: attrs.ContentEncoding,
		ETag:            attrs.Etag,
		LastModified:    attrs.Updated,
		Metadata:        attrs.Metadata,
	}, nil
}

// BucketExists checks if a bucket exists.
func (p *GCSProvider) BucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := p.client.Bucket(bucket).Attrs(ctx)
	if err == gcs.ErrBucketNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check bucket: %w", err)
	}

	return true, nil
}

// GenerateSignedURL generates a signed URL for temporary access.
func (p *GCSProvider) GenerateSignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	opts := &gcs.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	}

	url, err := p.client.Bucket(bucket).SignedURL(key, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}
