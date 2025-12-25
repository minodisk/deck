package deck

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// Environment variables for GCS configuration.
const (
	// EnvGCSServiceAccountKey - JSON content of a service account key for GCS.
	EnvGCSServiceAccountKey = "DECK_GCS_SERVICE_ACCOUNT_KEY"

	// EnvGCSBucket - The GCS bucket name to upload images to.
	EnvGCSBucket = "DECK_GCS_BUCKET"

	// EnvGCSPrefix - Optional prefix (folder path) for uploaded images.
	EnvGCSPrefix = "DECK_GCS_PREFIX"
)

// GCSUploader implements ImageUploader for Google Cloud Storage.
type GCSUploader struct {
	client     *storage.Client
	bucketName string
	prefix     string
}

// NewGCSUploader creates a new GCSUploader from environment variables.
func NewGCSUploader(ctx context.Context) (*GCSUploader, error) {
	bucketName := os.Getenv(EnvGCSBucket)
	if bucketName == "" {
		return nil, fmt.Errorf("%s is required for GCS storage", EnvGCSBucket)
	}

	var client *storage.Client
	var err error

	if credsJSON := os.Getenv(EnvGCSServiceAccountKey); credsJSON != "" {
		// Use service account key from environment variable
		client, err = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(credsJSON)))
	} else {
		// Fall back to Application Default Credentials
		client, err = storage.NewClient(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSUploader{
		client:     client,
		bucketName: bucketName,
		prefix:     os.Getenv(EnvGCSPrefix),
	}, nil
}

// Upload uploads an image to GCS and returns a signed URL.
func (u *GCSUploader) Upload(ctx context.Context, data []byte, mimeType, filename string) (string, string, error) {
	objectName := u.prefix + filename

	// Upload the image
	obj := u.client.Bucket(u.bucketName).Object(objectName)
	w := obj.NewWriter(ctx)
	w.ContentType = mimeType

	if _, err := w.Write(data); err != nil {
		return "", "", fmt.Errorf("failed to write image to GCS: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close GCS writer: %w", err)
	}

	// Generate signed URL (valid for 1 hour)
	signedURL, err := u.client.Bucket(u.bucketName).SignedURL(objectName, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		// Clean up on error
		_ = u.Delete(ctx, objectName)
		return "", "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return signedURL, objectName, nil
}

// Delete deletes an uploaded image from GCS.
func (u *GCSUploader) Delete(ctx context.Context, resourceID string) error {
	if err := u.client.Bucket(u.bucketName).Object(resourceID).Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object from GCS: %w", err)
	}
	return nil
}
