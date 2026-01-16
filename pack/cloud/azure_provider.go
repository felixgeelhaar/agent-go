package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

// AzureProvider implements the Provider interface for Azure Blob Storage.
type AzureProvider struct {
	client      *azblob.Client
	accountName string
}

// AzureConfig configures the Azure Blob Storage provider.
type AzureConfig struct {
	AccountName      string // Azure Storage account name
	AccountKey       string // Optional: storage account key
	ConnectionString string // Optional: full connection string
	// If neither AccountKey nor ConnectionString is provided, uses DefaultAzureCredential
}

// NewAzureProvider creates a new Azure Blob Storage provider.
func NewAzureProvider(ctx context.Context, cfg AzureConfig) (*AzureProvider, error) {
	if cfg.AccountName == "" && cfg.ConnectionString == "" {
		return nil, fmt.Errorf("account name or connection string is required")
	}

	var client *azblob.Client
	var err error

	if cfg.ConnectionString != "" {
		// Use connection string
		client, err = azblob.NewClientFromConnectionString(cfg.ConnectionString, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client from connection string: %w", err)
		}
	} else if cfg.AccountKey != "" {
		// Use shared key credential
		serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
		cred, err := azblob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create shared key credential: %w", err)
		}
		client, err = azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client with shared key: %w", err)
		}
	} else {
		// Use default Azure credential (managed identity, environment, etc.)
		serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default credential: %w", err)
		}
		client, err = azblob.NewClient(serviceURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client with default credential: %w", err)
		}
	}

	return &AzureProvider{
		client:      client,
		accountName: cfg.AccountName,
	}, nil
}

// Name returns the provider name.
func (p *AzureProvider) Name() string {
	return "azure-blob"
}

// ListBuckets lists all containers (Azure's equivalent of buckets).
func (p *AzureProvider) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	var buckets []BucketInfo

	pager := p.client.NewListContainersPager(&service.ListContainersOptions{})
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers: %w", err)
		}

		for _, c := range resp.ContainerItems {
			bucket := BucketInfo{
				Name: *c.Name,
			}
			if c.Properties != nil && c.Properties.LastModified != nil {
				bucket.CreationDate = *c.Properties.LastModified
			}
			if c.Metadata != nil {
				bucket.Tags = make(map[string]string)
				for k, v := range c.Metadata {
					if v != nil {
						bucket.Tags[k] = *v
					}
				}
			}
			buckets = append(buckets, bucket)
		}
	}

	return buckets, nil
}

// ListObjects lists blobs in a container with optional prefix.
func (p *AzureProvider) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]ObjectInfo, error) {
	containerClient := p.client.ServiceClient().NewContainerClient(bucket)

	var objects []ObjectInfo
	count := 0

	listOpts := &container.ListBlobsFlatOptions{}
	if prefix != "" {
		listOpts.Prefix = &prefix
	}
	if maxKeys > 0 {
		maxResults := int32(maxKeys)
		listOpts.MaxResults = &maxResults
	}

	pager := containerClient.NewListBlobsFlatPager(listOpts)
	for pager.More() {
		if maxKeys > 0 && count >= maxKeys {
			break
		}

		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %w", err)
		}

		for _, b := range resp.Segment.BlobItems {
			if maxKeys > 0 && count >= maxKeys {
				break
			}

			object := ObjectInfo{
				Key: *b.Name,
			}
			if b.Properties != nil {
				if b.Properties.ContentLength != nil {
					object.Size = *b.Properties.ContentLength
				}
				if b.Properties.LastModified != nil {
					object.LastModified = *b.Properties.LastModified
				}
				if b.Properties.ETag != nil {
					object.ETag = string(*b.Properties.ETag)
				}
				if b.Properties.AccessTier != nil {
					object.StorageClass = string(*b.Properties.AccessTier)
				}
			}
			objects = append(objects, object)
			count++
		}
	}

	return objects, nil
}

// GetObject retrieves a blob's content.
func (p *AzureProvider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectMetadata, error) {
	blobClient := p.client.ServiceClient().NewContainerClient(bucket).NewBlobClient(key)

	// Get properties first
	props, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	// Download content
	resp, err := blobClient.DownloadStream(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download blob: %w", err)
	}

	metadata := &ObjectMetadata{
		ContentLength: *props.ContentLength,
	}
	if props.ContentType != nil {
		metadata.ContentType = *props.ContentType
	}
	if props.ContentEncoding != nil {
		metadata.ContentEncoding = *props.ContentEncoding
	}
	if props.ETag != nil {
		metadata.ETag = string(*props.ETag)
	}
	if props.LastModified != nil {
		metadata.LastModified = *props.LastModified
	}
	if props.Metadata != nil {
		metadata.Metadata = make(map[string]string)
		for k, v := range props.Metadata {
			if v != nil {
				metadata.Metadata[k] = *v
			}
		}
	}

	return resp.Body, metadata, nil
}

// PutObject uploads a blob.
func (p *AzureProvider) PutObject(ctx context.Context, bucket, key string, data io.Reader, metadata *ObjectMetadata) error {
	blobClient := p.client.ServiceClient().NewContainerClient(bucket).NewBlockBlobClient(key)

	uploadOpts := &blockblob.UploadStreamOptions{}
	if metadata != nil {
		httpHeaders := &blob.HTTPHeaders{}
		if metadata.ContentType != "" {
			httpHeaders.BlobContentType = &metadata.ContentType
		}
		if metadata.ContentEncoding != "" {
			httpHeaders.BlobContentEncoding = &metadata.ContentEncoding
		}
		uploadOpts.HTTPHeaders = httpHeaders

		if metadata.Metadata != nil {
			uploadOpts.Metadata = make(map[string]*string)
			for k, v := range metadata.Metadata {
				val := v
				uploadOpts.Metadata[k] = &val
			}
		}
	}

	_, err := blobClient.UploadStream(ctx, data, uploadOpts)
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}

	return nil
}

// DeleteObject deletes a blob.
func (p *AzureProvider) DeleteObject(ctx context.Context, bucket, key string) error {
	blobClient := p.client.ServiceClient().NewContainerClient(bucket).NewBlobClient(key)

	_, err := blobClient.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// GetObjectMetadata retrieves blob metadata without content.
func (p *AzureProvider) GetObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error) {
	blobClient := p.client.ServiceClient().NewContainerClient(bucket).NewBlobClient(key)

	props, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	metadata := &ObjectMetadata{
		ContentLength: *props.ContentLength,
	}
	if props.ContentType != nil {
		metadata.ContentType = *props.ContentType
	}
	if props.ContentEncoding != nil {
		metadata.ContentEncoding = *props.ContentEncoding
	}
	if props.ETag != nil {
		metadata.ETag = string(*props.ETag)
	}
	if props.LastModified != nil {
		metadata.LastModified = *props.LastModified
	}
	if props.Metadata != nil {
		metadata.Metadata = make(map[string]string)
		for k, v := range props.Metadata {
			if v != nil {
				metadata.Metadata[k] = *v
			}
		}
	}

	return metadata, nil
}

// BucketExists checks if a container exists.
func (p *AzureProvider) BucketExists(ctx context.Context, bucket string) (bool, error) {
	containerClient := p.client.ServiceClient().NewContainerClient(bucket)

	_, err := containerClient.GetProperties(ctx, nil)
	if err != nil {
		// Check if it's a not found error
		var respErr *azcore.ResponseError
		if ok := errors.As(err, &respErr); ok && respErr.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check container: %w", err)
	}

	return true, nil
}

// GenerateSASURL generates a SAS URL for temporary access.
func (p *AzureProvider) GenerateSASURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	// Note: This requires the client to be created with SharedKeyCredential
	// and is not available with DefaultAzureCredential
	return "", fmt.Errorf("SAS URL generation requires shared key credential")
}
