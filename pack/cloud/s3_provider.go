package cloud

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Provider implements the Provider interface for AWS S3.
type S3Provider struct {
	client *s3.Client
	region string
}

// S3Config configures the S3 provider.
type S3Config struct {
	Region          string // AWS region (default: us-east-1)
	AccessKeyID     string // Optional: AWS access key (uses default credential chain if empty)
	SecretAccessKey string // Optional: AWS secret key
	SessionToken    string // Optional: AWS session token
	Endpoint        string // Optional: custom endpoint for S3-compatible storage
}

// NewS3Provider creates a new AWS S3 provider.
func NewS3Provider(ctx context.Context, cfg S3Config) (*S3Provider, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	var awsCfg aws.Config
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use explicit credentials
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				cfg.SessionToken,
			)),
		)
	} else {
		// Use default credential chain
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build S3 client options
	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for most S3-compatible storage
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Provider{
		client: client,
		region: region,
	}, nil
}

// Name returns the provider name.
func (p *S3Provider) Name() string {
	return "aws-s3"
}

// ListBuckets lists all S3 buckets.
func (p *S3Provider) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	output, err := p.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	buckets := make([]BucketInfo, 0, len(output.Buckets))
	for _, b := range output.Buckets {
		bucket := BucketInfo{
			Name: aws.ToString(b.Name),
		}
		if b.CreationDate != nil {
			bucket.CreationDate = *b.CreationDate
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

// ListObjects lists objects in a bucket with optional prefix.
func (p *S3Provider) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]ObjectInfo, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(int32(maxKeys)),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	output, err := p.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	objects := make([]ObjectInfo, 0, len(output.Contents))
	for _, obj := range output.Contents {
		object := ObjectInfo{
			Key:  aws.ToString(obj.Key),
			Size: aws.ToInt64(obj.Size),
		}
		if obj.LastModified != nil {
			object.LastModified = *obj.LastModified
		}
		if obj.ETag != nil {
			object.ETag = strings.Trim(*obj.ETag, "\"")
		}
		if obj.StorageClass != "" {
			object.StorageClass = string(obj.StorageClass)
		}
		objects = append(objects, object)
	}

	return objects, nil
}

// GetObject retrieves an object's content.
func (p *S3Provider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectMetadata, error) {
	output, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object: %w", err)
	}

	metadata := &ObjectMetadata{
		ContentType:   aws.ToString(output.ContentType),
		ContentLength: aws.ToInt64(output.ContentLength),
	}
	if output.ETag != nil {
		metadata.ETag = strings.Trim(*output.ETag, "\"")
	}
	if output.LastModified != nil {
		metadata.LastModified = *output.LastModified
	}
	if output.ContentEncoding != nil {
		metadata.ContentEncoding = *output.ContentEncoding
	}
	if output.Metadata != nil {
		metadata.Metadata = output.Metadata
	}

	return output.Body, metadata, nil
}

// PutObject uploads an object.
func (p *S3Provider) PutObject(ctx context.Context, bucket, key string, data io.Reader, metadata *ObjectMetadata) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   data,
	}

	if metadata != nil {
		if metadata.ContentType != "" {
			input.ContentType = aws.String(metadata.ContentType)
		}
		if metadata.ContentEncoding != "" {
			input.ContentEncoding = aws.String(metadata.ContentEncoding)
		}
		if metadata.Metadata != nil {
			input.Metadata = metadata.Metadata
		}
	}

	_, err := p.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

// DeleteObject deletes an object.
func (p *S3Provider) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// GetObjectMetadata retrieves object metadata without content.
func (p *S3Provider) GetObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error) {
	output, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	metadata := &ObjectMetadata{
		ContentType:   aws.ToString(output.ContentType),
		ContentLength: aws.ToInt64(output.ContentLength),
	}
	if output.ETag != nil {
		metadata.ETag = strings.Trim(*output.ETag, "\"")
	}
	if output.LastModified != nil {
		metadata.LastModified = *output.LastModified
	}
	if output.ContentEncoding != nil {
		metadata.ContentEncoding = *output.ContentEncoding
	}
	if output.Metadata != nil {
		metadata.Metadata = output.Metadata
	}

	return metadata, nil
}

// BucketExists checks if a bucket exists.
func (p *S3Provider) BucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := p.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// Check if it's a not found error
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bucket: %w", err)
	}

	return true, nil
}
