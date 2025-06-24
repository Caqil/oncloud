
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"oncloud/models"
)

// LocalClient implements local file system storage
type LocalClient struct {
	basePath string
	provider *models.StorageProvider
}

// NewLocalClient creates a new local storage client
func NewLocalClient(provider *models.StorageProvider) (StorageInterface, error) {
	basePath, exists := provider.Settings["base_path"].(string)
	if !exists || basePath == "" {
		basePath = "./uploads"
	}

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %v", err)
	}

	return &LocalClient{
		basePath: basePath,
		provider: provider,
	}, nil
}

// Upload saves data to local file system
func (lc *LocalClient) Upload(key string, data []byte) error {
	fullPath := filepath.Join(lc.basePath, key)
	
	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Write file
	return os.WriteFile(fullPath, data, 0644)
}

// UploadStream saves data from a stream to local file system
func (lc *LocalClient) UploadStream(key string, reader io.Reader, size int64) error {
	fullPath := filepath.Join(lc.basePath, key)
	
	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Copy from reader to file
	_, err = io.Copy(file, reader)
	return err
}

// Download reads data from local file system
func (lc *LocalClient) Download(key string) ([]byte, error) {
	fullPath := filepath.Join(lc.basePath, key)
	return os.ReadFile(fullPath)
}

// DownloadStream returns a reader for the file
func (lc *LocalClient) DownloadStream(key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(lc.basePath, key)
	return os.Open(fullPath)
}

// Delete removes a file from local file system
func (lc *LocalClient) Delete(key string) error {
	fullPath := filepath.Join(lc.basePath, key)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, consider it deleted
	}
	return err
}

// Exists checks if a file exists
func (lc *LocalClient) Exists(key string) (bool, error) {
	fullPath := filepath.Join(lc.basePath, key)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetSize returns the size of a file
func (lc *LocalClient) GetSize(key string) (int64, error) {
	fullPath := filepath.Join(lc.basePath, key)
	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// GetURL returns the local file URL
func (lc *LocalClient) GetURL(key string) (string, error) {
	// For local storage, this could be a file:// URL or HTTP URL if served
	return fmt.Sprintf("file://%s", filepath.Join(lc.basePath, key)), nil
}

// GetPresignedURL generates a presigned URL (for local storage, this is simplified)
func (lc *LocalClient) GetPresignedURL(key string, expiry time.Duration) (string, error) {
	// For local storage, return a simple URL with expiry token
	// In a real implementation, you'd store this token and validate it
	return fmt.Sprintf("/uploads/%s?expires=%d", key, time.Now().Add(expiry).Unix()), nil
}

// GetPresignedUploadURL generates a presigned upload URL
func (lc *LocalClient) GetPresignedUploadURL(key string, expiry time.Duration, maxSize int64) (string, error) {
	return fmt.Sprintf("/uploads/%s?action=upload&expires=%d", key, time.Now().Add(expiry).Unix()), nil
}

// Multipart upload operations (simplified for local storage)
func (lc *LocalClient) InitiateMultipartUpload(key string) (*MultipartUpload, error) {
	return &MultipartUpload{
		UploadID: fmt.Sprintf("local_%d", time.Now().UnixNano()),
		Key:      key,
		Provider: "local",
	}, nil
}

func (lc *LocalClient) UploadPart(uploadID, key string, partNumber int, data []byte) (*UploadPart, error) {
	// For local storage, we can just append to a temp file
	tempPath := filepath.Join(lc.basePath, ".tmp", uploadID)
	if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return nil, err
	}

	return &UploadPart{
		PartNumber: partNumber,
		ETag:       fmt.Sprintf("%d_%d", partNumber, len(data)),
		Size:       int64(len(data)),
	}, nil
}

func (lc *LocalClient) CompleteMultipartUpload(uploadID, key string, parts []UploadPart) error {
	tempPath := filepath.Join(lc.basePath, ".tmp", uploadID)
	finalPath := filepath.Join(lc.basePath, key)

	// Ensure directory exists
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Move temp file to final location
	return os.Rename(tempPath, finalPath)
}

func (lc *LocalClient) AbortMultipartUpload(uploadID, key string) error {
	tempPath := filepath.Join(lc.basePath, ".tmp", uploadID)
	return os.Remove(tempPath)
}

// Batch operations
func (lc *LocalClient) DeleteMultiple(keys []string) error {
	for _, key := range keys {
		if err := lc.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (lc *LocalClient) CopyFile(sourceKey, destKey string) error {
	data, err := lc.Download(sourceKey)
	if err != nil {
		return err
	}
	return lc.Upload(destKey, data)
}

func (lc *LocalClient) MoveFile(sourceKey, destKey string) error {
	sourcePath := filepath.Join(lc.basePath, sourceKey)
	destPath := filepath.Join(lc.basePath, destKey)

	// Ensure destination directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.Rename(sourcePath, destPath)
}

// Provider info
func (lc *LocalClient) GetProviderInfo() *ProviderInfo {
	return &ProviderInfo{
		Name:        lc.provider.Name,
		Type:        "local",
		Region:      "local",
		Endpoint:    lc.basePath,
		MaxFileSize: lc.provider.MaxFileSize,
		Features:    []string{"upload", "download", "delete", "multipart"},
		Metadata: map[string]string{
			"base_path": lc.basePath,
		},
	}
}

// HealthCheck verifies local storage is accessible
func (lc *LocalClient) HealthCheck() error {
	// Test write access
	testFile := filepath.Join(lc.basePath, ".health_check")
	
	// Try to write a test file
	if err := os.WriteFile(testFile, []byte("health_check"), 0644); err != nil {
		return fmt.Errorf("local storage write test failed: %v", err)
	}

	// Try to read it back
	if _, err := os.ReadFile(testFile); err != nil {
		return fmt.Errorf("local storage read test failed: %v", err)
	}

	// Clean up
	os.Remove(testFile)

	return nil
}

// GetStats returns storage statistics
func (lc *LocalClient) GetStats() (*StorageStats, error) {
	stats := &StorageStats{
		TotalFiles: 0,
		TotalSize:  0,
	}

	// Walk through all files to get stats
	err := filepath.Walk(lc.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			stats.TotalFiles++
			stats.TotalSize += info.Size()
		}
		return nil
	})

	return stats, err
}