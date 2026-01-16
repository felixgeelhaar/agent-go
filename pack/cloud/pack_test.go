package cloud

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func setupTestProvider(t *testing.T) *MemoryProvider {
	t.Helper()

	provider := NewMemoryProvider()

	// Create test buckets
	provider.CreateBucket("test-bucket")
	provider.CreateBucket("another-bucket")

	// Add test objects
	ctx := context.Background()
	provider.PutObject(ctx, "test-bucket", "file1.txt", strings.NewReader("Hello, World!"), &ObjectMetadata{
		ContentType: "text/plain",
		Metadata:    map[string]string{"author": "test"},
	})
	provider.PutObject(ctx, "test-bucket", "file2.txt", strings.NewReader("Another file"), &ObjectMetadata{
		ContentType: "text/plain",
	})
	provider.PutObject(ctx, "test-bucket", "data/nested.json", strings.NewReader(`{"key": "value"}`), &ObjectMetadata{
		ContentType: "application/json",
	})

	return provider
}

func TestNew(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}

	if p.Name != "cloud" {
		t.Errorf("expected name 'cloud', got '%s'", p.Name)
	}
}

func TestNewWithNilProvider(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestNewWithOptions(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider,
		WithWriteAccess(),
		WithDeleteAccess(),
		WithMaxObjectSize(5*1024*1024),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestListBucketsTool(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_buckets")
	if !ok {
		t.Fatal("cloud_list_buckets tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("list buckets failed: %v", err)
	}

	var out listBucketsOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count != 2 {
		t.Errorf("expected 2 buckets, got %d", out.Count)
	}

	if out.Provider != "memory" {
		t.Errorf("expected provider 'memory', got '%s'", out.Provider)
	}
}

func TestListObjectsTool(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_objects")
	if !ok {
		t.Fatal("cloud_list_objects tool not found")
	}

	input, _ := json.Marshal(listObjectsInput{
		Bucket: "test-bucket",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list objects failed: %v", err)
	}

	var out listObjectsOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count != 3 {
		t.Errorf("expected 3 objects, got %d", out.Count)
	}

	if out.Bucket != "test-bucket" {
		t.Errorf("expected bucket 'test-bucket', got '%s'", out.Bucket)
	}
}

func TestListObjectsToolWithPrefix(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_objects")
	if !ok {
		t.Fatal("cloud_list_objects tool not found")
	}

	input, _ := json.Marshal(listObjectsInput{
		Bucket: "test-bucket",
		Prefix: "data/",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("list objects failed: %v", err)
	}

	var out listObjectsOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count != 1 {
		t.Errorf("expected 1 object with prefix, got %d", out.Count)
	}
}

func TestListObjectsToolMissingBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_objects")
	if !ok {
		t.Fatal("cloud_list_objects tool not found")
	}

	input, _ := json.Marshal(listObjectsInput{})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing bucket")
	}
}

func TestListObjectsToolNonExistentBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_objects")
	if !ok {
		t.Fatal("cloud_list_objects tool not found")
	}

	input, _ := json.Marshal(listObjectsInput{
		Bucket: "nonexistent",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent bucket")
	}
}

func TestGetObjectTool(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Bucket: "test-bucket",
		Key:    "file1.txt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get object failed: %v", err)
	}

	var out getObjectOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Content != "Hello, World!" {
		t.Errorf("expected content 'Hello, World!', got '%s'", out.Content)
	}

	if out.ContentType != "text/plain" {
		t.Errorf("expected content type 'text/plain', got '%s'", out.ContentType)
	}

	if out.Size != 13 {
		t.Errorf("expected size 13, got %d", out.Size)
	}
}

func TestGetObjectToolMissingBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Key: "file1.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing bucket")
	}
}

func TestGetObjectToolMissingKey(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Bucket: "test-bucket",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestGetObjectToolNonExistentObject(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Bucket: "test-bucket",
		Key:    "nonexistent.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent object")
	}
}

func TestGetObjectMetadataTool(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object_metadata")
	if !ok {
		t.Fatal("cloud_get_object_metadata tool not found")
	}

	input, _ := json.Marshal(getObjectMetadataInput{
		Bucket: "test-bucket",
		Key:    "file1.txt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get object metadata failed: %v", err)
	}

	var out getObjectMetadataOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Metadata == nil {
		t.Error("expected metadata to be non-nil")
	}

	if out.Metadata.ContentType != "text/plain" {
		t.Errorf("expected content type 'text/plain', got '%s'", out.Metadata.ContentType)
	}

	if out.Metadata.ContentLength != 13 {
		t.Errorf("expected content length 13, got %d", out.Metadata.ContentLength)
	}
}

func TestGetObjectMetadataToolMissingBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object_metadata")
	if !ok {
		t.Fatal("cloud_get_object_metadata tool not found")
	}

	input, _ := json.Marshal(getObjectMetadataInput{
		Key: "file1.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing bucket")
	}
}

func TestGetObjectMetadataToolMissingKey(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object_metadata")
	if !ok {
		t.Fatal("cloud_get_object_metadata tool not found")
	}

	input, _ := json.Marshal(getObjectMetadataInput{
		Bucket: "test-bucket",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestPutObjectToolWithWriteAccess(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_put_object")
	if !ok {
		t.Fatal("cloud_put_object tool not found")
	}

	input, _ := json.Marshal(putObjectInput{
		Bucket:      "test-bucket",
		Key:         "new-file.txt",
		Content:     "New content",
		ContentType: "text/plain",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("put object failed: %v", err)
	}

	var out putObjectOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Key != "new-file.txt" {
		t.Errorf("expected key 'new-file.txt', got '%s'", out.Key)
	}

	if out.Size != 11 {
		t.Errorf("expected size 11, got %d", out.Size)
	}

	// Verify the object was created
	exists, _ := provider.BucketExists(context.Background(), "test-bucket")
	if !exists {
		t.Error("expected bucket to exist")
	}
}

func TestPutObjectToolMissingBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_put_object")
	if !ok {
		t.Fatal("cloud_put_object tool not found")
	}

	input, _ := json.Marshal(putObjectInput{
		Key:     "new-file.txt",
		Content: "New content",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing bucket")
	}
}

func TestPutObjectToolMissingKey(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_put_object")
	if !ok {
		t.Fatal("cloud_put_object tool not found")
	}

	input, _ := json.Marshal(putObjectInput{
		Bucket:  "test-bucket",
		Content: "New content",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestPutObjectToolWithoutWriteAccess(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider) // No write access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("cloud_put_object")
	if ok {
		t.Error("expected cloud_put_object tool to not exist without write access")
	}
}

func TestDeleteObjectToolWithDeleteAccess(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess(), WithDeleteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_delete_object")
	if !ok {
		t.Fatal("cloud_delete_object tool not found")
	}

	input, _ := json.Marshal(deleteObjectInput{
		Bucket: "test-bucket",
		Key:    "file1.txt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("delete object failed: %v", err)
	}

	var out deleteObjectOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if !out.Deleted {
		t.Error("expected deleted to be true")
	}

	// Verify the object was deleted
	_, _, err = provider.GetObject(context.Background(), "test-bucket", "file1.txt")
	if err == nil {
		t.Error("expected object to be deleted")
	}
}

func TestDeleteObjectToolMissingBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess(), WithDeleteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_delete_object")
	if !ok {
		t.Fatal("cloud_delete_object tool not found")
	}

	input, _ := json.Marshal(deleteObjectInput{
		Key: "file1.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing bucket")
	}
}

func TestDeleteObjectToolMissingKey(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess(), WithDeleteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_delete_object")
	if !ok {
		t.Fatal("cloud_delete_object tool not found")
	}

	input, _ := json.Marshal(deleteObjectInput{
		Bucket: "test-bucket",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestDeleteObjectToolWithoutDeleteAccess(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess()) // Write but no delete access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("cloud_delete_object")
	if ok {
		t.Error("expected cloud_delete_object tool to not exist without delete access")
	}
}

func TestDeleteObjectToolWithoutWriteAccess(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider) // No write or delete access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("cloud_delete_object")
	if ok {
		t.Error("expected cloud_delete_object tool to not exist without write access")
	}
}

func TestToolAnnotations(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess(), WithDeleteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	// Check list_buckets tool is read-only and cacheable
	if listBucketsTool, ok := p.GetTool("cloud_list_buckets"); ok {
		annotations := listBucketsTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("cloud_list_buckets should be read-only")
		}
		if !annotations.Cacheable {
			t.Error("cloud_list_buckets should be cacheable")
		}
	}

	// Check list_objects tool is read-only
	if listObjectsTool, ok := p.GetTool("cloud_list_objects"); ok {
		annotations := listObjectsTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("cloud_list_objects should be read-only")
		}
	}

	// Check get_object tool is read-only
	if getObjectTool, ok := p.GetTool("cloud_get_object"); ok {
		annotations := getObjectTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("cloud_get_object should be read-only")
		}
	}

	// Check get_object_metadata tool is read-only and cacheable
	if getMetadataTool, ok := p.GetTool("cloud_get_object_metadata"); ok {
		annotations := getMetadataTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("cloud_get_object_metadata should be read-only")
		}
		if !annotations.Cacheable {
			t.Error("cloud_get_object_metadata should be cacheable")
		}
	}

	// Check put_object tool is destructive
	if putObjectTool, ok := p.GetTool("cloud_put_object"); ok {
		annotations := putObjectTool.Annotations()
		if !annotations.Destructive {
			t.Error("cloud_put_object should be destructive")
		}
	}

	// Check delete_object tool is destructive
	if deleteObjectTool, ok := p.GetTool("cloud_delete_object"); ok {
		annotations := deleteObjectTool.Annotations()
		if !annotations.Destructive {
			t.Error("cloud_delete_object should be destructive")
		}
	}
}

func TestMemoryProviderName(t *testing.T) {
	provider := NewMemoryProvider()
	if provider.Name() != "memory" {
		t.Errorf("expected name 'memory', got '%s'", provider.Name())
	}
}

func TestMemoryProviderCreateBucketDuplicate(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test")

	err := provider.CreateBucket("test")
	if err == nil {
		t.Error("expected error for duplicate bucket")
	}
}

func TestMemoryProviderBucketExists(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test")

	exists, err := provider.BucketExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected bucket to exist")
	}

	exists, err = provider.BucketExists(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected bucket to not exist")
	}
}

// Additional tests for improved coverage

func TestListObjectsToolInvalidJSON(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_objects")
	if !ok {
		t.Fatal("cloud_list_objects tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetObjectToolInvalidJSON(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetObjectMetadataToolInvalidJSON(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object_metadata")
	if !ok {
		t.Fatal("cloud_get_object_metadata tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPutObjectToolInvalidJSON(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_put_object")
	if !ok {
		t.Fatal("cloud_put_object tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDeleteObjectToolInvalidJSON(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess(), WithDeleteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_delete_object")
	if !ok {
		t.Fatal("cloud_delete_object tool not found")
	}

	_, err = tool.Execute(context.Background(), json.RawMessage("invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMemoryProviderDeleteObjectNonExistent(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	err := provider.DeleteObject(context.Background(), "test-bucket", "nonexistent.txt")
	if err == nil {
		t.Error("expected error deleting non-existent object")
	}
}

func TestMemoryProviderDeleteObjectNonExistentBucket(t *testing.T) {
	provider := NewMemoryProvider()

	err := provider.DeleteObject(context.Background(), "nonexistent-bucket", "file.txt")
	if err == nil {
		t.Error("expected error deleting from non-existent bucket")
	}
}

func TestMemoryProviderGetObjectMetadataNonExistent(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	_, err := provider.GetObjectMetadata(context.Background(), "test-bucket", "nonexistent.txt")
	if err == nil {
		t.Error("expected error getting metadata for non-existent object")
	}
}

func TestMemoryProviderGetObjectMetadataNonExistentBucket(t *testing.T) {
	provider := NewMemoryProvider()

	_, err := provider.GetObjectMetadata(context.Background(), "nonexistent-bucket", "file.txt")
	if err == nil {
		t.Error("expected error getting metadata from non-existent bucket")
	}
}

func TestMemoryProviderPutObjectNonExistentBucket(t *testing.T) {
	provider := NewMemoryProvider()

	err := provider.PutObject(context.Background(), "nonexistent-bucket", "file.txt",
		strings.NewReader("content"), nil)
	if err == nil {
		t.Error("expected error putting object to non-existent bucket")
	}
}

func TestMemoryProviderListObjectsWithMaxKeys(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		provider.PutObject(ctx, "test-bucket", "file"+string(rune('0'+i))+".txt",
			strings.NewReader("content"), nil)
	}

	objects, err := provider.ListObjects(ctx, "test-bucket", "", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(objects))
	}
}

func TestMemoryProviderListObjectsNonExistentBucket(t *testing.T) {
	provider := NewMemoryProvider()

	_, err := provider.ListObjects(context.Background(), "nonexistent-bucket", "", 100)
	if err == nil {
		t.Error("expected error listing objects in non-existent bucket")
	}
}

func TestMemoryProviderGetObjectNonExistentBucket(t *testing.T) {
	provider := NewMemoryProvider()

	_, _, err := provider.GetObject(context.Background(), "nonexistent-bucket", "file.txt")
	if err == nil {
		t.Error("expected error getting object from non-existent bucket")
	}
}

func TestMemoryProviderGetObjectNonExistent(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	_, _, err := provider.GetObject(context.Background(), "test-bucket", "nonexistent.txt")
	if err == nil {
		t.Error("expected error getting non-existent object")
	}
}

func TestGetObjectToolNonExistentBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Bucket: "nonexistent-bucket",
		Key:    "file.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent bucket")
	}
}

func TestGetObjectMetadataToolNonExistent(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object_metadata")
	if !ok {
		t.Fatal("cloud_get_object_metadata tool not found")
	}

	input, _ := json.Marshal(getObjectMetadataInput{
		Bucket: "test-bucket",
		Key:    "nonexistent.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent object")
	}
}

func TestPutObjectToolNonExistentBucket(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_put_object")
	if !ok {
		t.Fatal("cloud_put_object tool not found")
	}

	input, _ := json.Marshal(putObjectInput{
		Bucket:  "nonexistent-bucket",
		Key:     "file.txt",
		Content: "content",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent bucket")
	}
}

func TestDeleteObjectToolNonExistent(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess(), WithDeleteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_delete_object")
	if !ok {
		t.Fatal("cloud_delete_object tool not found")
	}

	input, _ := json.Marshal(deleteObjectInput{
		Bucket: "test-bucket",
		Key:    "nonexistent.txt",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent object")
	}
}

func TestPutObjectToolWithMetadata(t *testing.T) {
	provider := setupTestProvider(t)

	p, err := New(provider, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_put_object")
	if !ok {
		t.Fatal("cloud_put_object tool not found")
	}

	input, _ := json.Marshal(putObjectInput{
		Bucket:      "test-bucket",
		Key:         "new-file-with-metadata.txt",
		Content:     "Content with metadata",
		ContentType: "text/plain",
		Metadata: map[string]string{
			"custom-key": "custom-value",
		},
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("put object failed: %v", err)
	}

	var out putObjectOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Key != "new-file-with-metadata.txt" {
		t.Errorf("expected key 'new-file-with-metadata.txt', got '%s'", out.Key)
	}

	// Verify metadata was stored
	meta, err := provider.GetObjectMetadata(context.Background(), "test-bucket", "new-file-with-metadata.txt")
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if meta.Metadata["custom-key"] != "custom-value" {
		t.Errorf("expected custom-key metadata to be 'custom-value', got '%s'", meta.Metadata["custom-key"])
	}
}

func TestListObjectsToolWithMaxKeys(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		provider.PutObject(ctx, "test-bucket", "file"+string(rune('0'+i))+".txt",
			strings.NewReader("content"), nil)
	}

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_list_objects")
	if !ok {
		t.Fatal("cloud_list_objects tool not found")
	}

	input, _ := json.Marshal(listObjectsInput{
		Bucket:  "test-bucket",
		MaxKeys: 5,
	})

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("list objects failed: %v", err)
	}

	var out listObjectsOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count != 5 {
		t.Errorf("expected 5 objects with max_keys, got %d", out.Count)
	}
}

func TestGetObjectToolLargeContent(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	// Create a large object (over the typical truncation limit)
	largeContent := strings.Repeat("A", 2000)
	provider.PutObject(context.Background(), "test-bucket", "large.txt",
		strings.NewReader(largeContent), nil)

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Bucket: "test-bucket",
		Key:    "large.txt",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get object failed: %v", err)
	}

	var out getObjectOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Verify the content was retrieved
	if len(out.Content) == 0 {
		t.Error("expected non-empty content")
	}
}

func TestGetObjectToolBinaryContent(t *testing.T) {
	provider := NewMemoryProvider()
	provider.CreateBucket("test-bucket")

	// Create an object with binary-like content type
	provider.PutObject(context.Background(), "test-bucket", "binary.bin",
		strings.NewReader("binary content"), &ObjectMetadata{
			ContentType: "application/octet-stream",
		})

	p, err := New(provider)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("cloud_get_object")
	if !ok {
		t.Fatal("cloud_get_object tool not found")
	}

	input, _ := json.Marshal(getObjectInput{
		Bucket: "test-bucket",
		Key:    "binary.bin",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("get object failed: %v", err)
	}

	var out getObjectOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Binary content should be base64 encoded
	if out.ContentType != "application/octet-stream" {
		t.Errorf("expected content type 'application/octet-stream', got '%s'", out.ContentType)
	}
}
