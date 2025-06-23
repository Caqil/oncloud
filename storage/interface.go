package storage

import (
	"io"
	"time"
)

// StorageInterface defines the common interface for all storage providers
type StorageInterface interface {
	// Basic file operations
	Upload(key string, data []byte) error
	UploadStream(key string, reader io.Reader, size int64) error
	Download(key string) ([]byte, error)
	DownloadStream(key string) (io.ReadCloser, error)
	Delete(key string) error
	Exists(key string) (bool, error)
	GetSize(key string) (int64, error)

	// URL operations
	GetURL(key string) (string, error)
	GetPresignedURL(key string, expiry time.Duration) (string, error)
	GetPresignedUploadURL(key string, expiry time.Duration, maxSize int64) (string, error)

	// Multipart upload operations
	InitiateMultipartUpload(key string) (*MultipartUpload, error)
	UploadPart(uploadID, key string, partNumber int, data []byte) (*UploadPart, error)
	CompleteMultipartUpload(uploadID, key string, parts []UploadPart) error
	AbortMultipartUpload(uploadID, key string) error

	// Batch operations
	DeleteMultiple(keys []string) error
	CopyFile(sourceKey, destKey string) error
	MoveFile(sourceKey, destKey string) error

	// Provider info
	GetProviderInfo() *ProviderInfo
	HealthCheck() error
	GetStats() (*StorageStats, error)
}

// MultipartUpload represents a multipart upload session
type MultipartUpload struct {
	UploadID string `json:"upload_id"`
	Key      string `json:"key"`
	Provider string `json:"provider"`
}

// UploadPart represents a part of multipart upload
type UploadPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// ProviderInfo contains information about the storage provider
type ProviderInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Region      string            `json:"region"`
	Endpoint    string            `json:"endpoint"`
	CDNUrl      string            `json:"cdn_url"`
	MaxFileSize int64             `json:"max_file_size"`
	Features    []string          `json:"features"`
	Metadata    map[string]string `json:"metadata"`
}

// StorageStats contains storage usage statistics
type StorageStats struct {
	TotalFiles     int64 `json:"total_files"`
	TotalSize      int64 `json:"total_size"`
	UsedSpace      int64 `json:"used_space"`
	AvailableSpace int64 `json:"available_space,omitempty"`
}

// UploadOptions contains options for file upload
type UploadOptions struct {
	ContentType          string            `json:"content_type"`
	CacheControl         string            `json:"cache_control"`
	ContentEncoding      string            `json:"content_encoding"`
	Metadata             map[string]string `json:"metadata"`
	Tags                 map[string]string `json:"tags"`
	ServerSideEncryption bool              `json:"server_side_encryption"`
}

// StorageError represents storage-specific errors
type StorageError struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Key      string `json:"key,omitempty"`
}

func (e *StorageError) Error() string {
	return e.Message
}

// NewStorageError creates a new storage error
func NewStorageError(provider, code, message, key string) *StorageError {
	return &StorageError{
		Provider: provider,
		Code:     code,
		Message:  message,
		Key:      key,
	}
}
