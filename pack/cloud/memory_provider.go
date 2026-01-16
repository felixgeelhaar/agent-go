package cloud

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

// MemoryProvider is an in-memory implementation of the Provider interface.
// Useful for testing and development.
type MemoryProvider struct {
	mu      sync.RWMutex
	buckets map[string]*memoryBucket
}

type memoryBucket struct {
	info    BucketInfo
	objects map[string]*memoryObject
}

type memoryObject struct {
	content  []byte
	metadata ObjectMetadata
}

// NewMemoryProvider creates a new in-memory cloud provider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		buckets: make(map[string]*memoryBucket),
	}
}

// Name returns the provider name.
func (p *MemoryProvider) Name() string {
	return "memory"
}

// CreateBucket creates a new bucket.
func (p *MemoryProvider) CreateBucket(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.buckets[name]; exists {
		return errors.New("bucket already exists")
	}

	p.buckets[name] = &memoryBucket{
		info: BucketInfo{
			Name:         name,
			CreationDate: time.Now(),
		},
		objects: make(map[string]*memoryObject),
	}
	return nil
}

// ListBuckets lists all buckets.
func (p *MemoryProvider) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	buckets := make([]BucketInfo, 0, len(p.buckets))
	for _, b := range p.buckets {
		buckets = append(buckets, b.info)
	}
	return buckets, nil
}

// ListObjects lists objects in a bucket.
func (p *MemoryProvider) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]ObjectInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	b, exists := p.buckets[bucket]
	if !exists {
		return nil, errors.New("bucket not found")
	}

	objects := make([]ObjectInfo, 0)
	for key, obj := range b.objects {
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		objects = append(objects, ObjectInfo{
			Key:          key,
			Size:         int64(len(obj.content)),
			LastModified: obj.metadata.LastModified,
			ETag:         obj.metadata.ETag,
		})
		if len(objects) >= maxKeys {
			break
		}
	}
	return objects, nil
}

// GetObject retrieves an object's content.
func (p *MemoryProvider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectMetadata, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	b, exists := p.buckets[bucket]
	if !exists {
		return nil, nil, errors.New("bucket not found")
	}

	obj, exists := b.objects[key]
	if !exists {
		return nil, nil, errors.New("object not found")
	}

	metadata := obj.metadata
	metadata.ContentLength = int64(len(obj.content))

	return io.NopCloser(bytes.NewReader(obj.content)), &metadata, nil
}

// PutObject uploads an object.
func (p *MemoryProvider) PutObject(ctx context.Context, bucket, key string, data io.Reader, metadata *ObjectMetadata) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	b, exists := p.buckets[bucket]
	if !exists {
		return errors.New("bucket not found")
	}

	content, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	objMeta := ObjectMetadata{
		LastModified: time.Now(),
	}
	if metadata != nil {
		objMeta.ContentType = metadata.ContentType
		objMeta.ContentEncoding = metadata.ContentEncoding
		objMeta.Metadata = metadata.Metadata
	}

	b.objects[key] = &memoryObject{
		content:  content,
		metadata: objMeta,
	}
	return nil
}

// DeleteObject deletes an object.
func (p *MemoryProvider) DeleteObject(ctx context.Context, bucket, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	b, exists := p.buckets[bucket]
	if !exists {
		return errors.New("bucket not found")
	}

	if _, exists := b.objects[key]; !exists {
		return errors.New("object not found")
	}

	delete(b.objects, key)
	return nil
}

// GetObjectMetadata retrieves object metadata.
func (p *MemoryProvider) GetObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	b, exists := p.buckets[bucket]
	if !exists {
		return nil, errors.New("bucket not found")
	}

	obj, exists := b.objects[key]
	if !exists {
		return nil, errors.New("object not found")
	}

	metadata := obj.metadata
	metadata.ContentLength = int64(len(obj.content))
	return &metadata, nil
}

// BucketExists checks if a bucket exists.
func (p *MemoryProvider) BucketExists(ctx context.Context, bucket string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.buckets[bucket]
	return exists, nil
}
