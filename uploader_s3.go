package deck

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Environment variables for S3 configuration.
const (
	// EnvS3AccessKeyID - AWS access key ID for S3 (optional, falls back to AWS SDK default chain).
	EnvS3AccessKeyID = "DECK_S3_ACCESS_KEY_ID"

	// EnvS3SecretAccessKey - AWS secret access key for S3 (optional, falls back to AWS SDK default chain).
	EnvS3SecretAccessKey = "DECK_S3_SECRET_ACCESS_KEY"

	// EnvS3Bucket - The S3 bucket name to upload images to.
	EnvS3Bucket = "DECK_S3_BUCKET"

	// EnvS3Prefix - Optional prefix (folder path) for uploaded images.
	EnvS3Prefix = "DECK_S3_PREFIX"

	// EnvS3Region - AWS region for the S3 bucket.
	EnvS3Region = "DECK_S3_REGION"

	// EnvS3Endpoint - Custom S3 endpoint (for S3-compatible services like MinIO, Cloudflare R2).
	EnvS3Endpoint = "DECK_S3_ENDPOINT"
)

// S3Uploader implements ImageUploader for AWS S3.
type S3Uploader struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	prefix        string
}

// NewS3Uploader creates a new S3Uploader from environment variables.
func NewS3Uploader(ctx context.Context) (*S3Uploader, error) {
	bucketName := os.Getenv(EnvS3Bucket)
	if bucketName == "" {
		return nil, fmt.Errorf("%s is required for S3 storage", EnvS3Bucket)
	}

	var cfg aws.Config
	var err error

	// Check for deck-specific credentials first
	accessKeyID := os.Getenv(EnvS3AccessKeyID)
	secretAccessKey := os.Getenv(EnvS3SecretAccessKey)

	opts := []func(*config.LoadOptions) error{}

	// Set region if specified
	if region := os.Getenv(EnvS3Region); region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	// Use deck-specific credentials if provided
	if accessKeyID != "" && secretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		))
	}

	cfg, err = config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with optional custom endpoint
	var clientOpts []func(*s3.Options)
	if endpoint := os.Getenv(EnvS3Endpoint); endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // Required for most S3-compatible services
		})
	}

	client := s3.NewFromConfig(cfg, clientOpts...)
	presignClient := s3.NewPresignClient(client)

	return &S3Uploader{
		client:        client,
		presignClient: presignClient,
		bucketName:    bucketName,
		prefix:        os.Getenv(EnvS3Prefix),
	}, nil
}

// Upload uploads an image to S3 and returns a presigned URL.
func (u *S3Uploader) Upload(ctx context.Context, data []byte, mimeType, filename string) (string, string, error) {
	key := u.prefix + filename

	// Upload the image (private by default)
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &u.bucketName,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: &mimeType,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to upload image to S3: %w", err)
	}

	// Generate presigned URL (valid for 1 hour)
	presignedReq, err := u.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &u.bucketName,
		Key:    &key,
	}, s3.WithPresignExpires(1*time.Hour))
	if err != nil {
		// Clean up on error
		_ = u.Delete(ctx, key)
		return "", "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedReq.URL, key, nil
}

// Delete deletes an uploaded image from S3.
func (u *S3Uploader) Delete(ctx context.Context, resourceID string) error {
	_, err := u.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &u.bucketName,
		Key:    &resourceID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}
	return nil
}
