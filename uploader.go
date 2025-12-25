package deck

import (
	"context"
	"os"
)

// Environment variables for image storage selection.
const (
	// EnvImageStorage specifies the image storage backend.
	// Valid values: "gdrive" (default), "gcs", "s3"
	EnvImageStorage = "DECK_IMAGE_STORAGE"
)

// ImageUploader is the interface for uploading images to various storage backends.
type ImageUploader interface {
	// Upload uploads an image and returns the public URL and resource ID.
	// The public URL is used by Google Slides API to fetch the image.
	// The resource ID is used for cleanup (deletion) after the image is fetched.
	Upload(ctx context.Context, data []byte, mimeType, filename string) (publicURL, resourceID string, err error)

	// Delete deletes an uploaded image by its resource ID.
	Delete(ctx context.Context, resourceID string) error
}

// uploadedImageInfo holds information about uploaded images for cleanup.
type uploadedImageInfo struct {
	resourceID string
	image      *Image
}

// GetImageStorageType returns the configured image storage type.
func GetImageStorageType() string {
	storage := os.Getenv(EnvImageStorage)
	if storage == "" {
		return "gdrive"
	}
	return storage
}
