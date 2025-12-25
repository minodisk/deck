package deck

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"google.golang.org/api/drive/v3"
)

// GoogleDriveUploader implements ImageUploader for Google Drive.
type GoogleDriveUploader struct {
	driveSrv *drive.Service
	folderID string
}

// NewGoogleDriveUploader creates a new GoogleDriveUploader.
func NewGoogleDriveUploader(driveSrv *drive.Service, folderID string) *GoogleDriveUploader {
	return &GoogleDriveUploader{
		driveSrv: driveSrv,
		folderID: folderID,
	}
}

// Upload uploads an image to Google Drive and returns the public URL and file ID.
func (u *GoogleDriveUploader) Upload(ctx context.Context, data []byte, mimeType, filename string) (string, string, error) {
	df := &drive.File{
		Name:     filename,
		MimeType: mimeType,
	}
	if u.folderID != "" {
		df.Parents = []string{u.folderID}
	}

	uploaded, err := u.driveSrv.Files.Create(df).Media(bytes.NewBuffer(data)).SupportsAllDrives(true).Do()
	if err != nil {
		return "", "", fmt.Errorf("failed to upload image to Google Drive: %w", err)
	}

	// Set permission to allow anyone to read
	permission := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}
	if _, err := u.driveSrv.Permissions.Create(uploaded.Id, permission).SupportsAllDrives(true).Context(ctx).Do(); err != nil {
		// Clean up uploaded file on permission error
		_ = u.Delete(ctx, uploaded.Id)
		return "", "", fmt.Errorf("failed to set permission for image: %w", err)
	}

	// Get webContentLink
	f, err := u.driveSrv.Files.Get(uploaded.Id).Fields("webContentLink").SupportsAllDrives(true).Do()
	if err != nil {
		_ = u.Delete(ctx, uploaded.Id)
		return "", "", fmt.Errorf("failed to get webContentLink for image: %w", err)
	}

	if f.WebContentLink == "" {
		_ = u.Delete(ctx, uploaded.Id)
		return "", "", fmt.Errorf("webContentLink is empty for image: %s", uploaded.Id)
	}

	return f.WebContentLink, uploaded.Id, nil
}

// Delete deletes an uploaded image from Google Drive.
func (u *GoogleDriveUploader) Delete(ctx context.Context, resourceID string) error {
	file, err := u.driveSrv.Files.Get(resourceID).SupportsAllDrives(true).Fields("capabilities").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("file not found or not accessible before deletion (file ID: %s): %w", resourceID, err)
	}

	if file.Capabilities == nil || file.Capabilities.CanDelete {
		return u.driveSrv.Files.Delete(resourceID).SupportsAllDrives(true).Context(ctx).Do()
	}
	if file.Capabilities.CanTrash {
		updateRequest := &drive.File{Trashed: true}
		_, err := u.driveSrv.Files.Update(resourceID, updateRequest).SupportsAllDrives(true).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to trash file: %w", err)
		}
		return nil
	}
	return fmt.Errorf("file cannot be deleted or trashed (file ID: %s)", resourceID)
}

// generateTempFilename generates a temporary filename for uploaded images.
func generateTempFilename() string {
	return fmt.Sprintf("________tmp-for-deck-%s", time.Now().Format(time.RFC3339))
}
