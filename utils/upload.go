package utils

import (
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

type FileInfo struct {
	Name         string            `json:"name"`
	OriginalName string            `json:"original_name"`
	Size         int64             `json:"size"`
	Extension    string            `json:"extension"`
	MimeType     string            `json:"mime_type"`
	Hash         string            `json:"hash"`
	Path         string            `json:"path"`
	Metadata     map[string]string `json:"metadata"`
}

type UploadConfig struct {
	MaxFileSize       int64    `json:"max_file_size"`
	AllowedTypes      []string `json:"allowed_types"`
	StorageProvider   string   `json:"storage_provider"`
	GenerateThumbnail bool     `json:"generate_thumbnail"`
	CompressImages    bool     `json:"compress_images"`
}

// ProcessFileUpload processes uploaded file and returns file information
func ProcessFileUpload(file *multipart.FileHeader, config *UploadConfig) (*FileInfo, error) {
	// Validate file size
	if file.Size > config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", file.Size, config.MaxFileSize)
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		return nil, fmt.Errorf("file must have an extension")
	}

	// Validate file type
	if !isAllowedFileType(ext, config.AllowedTypes) {
		return nil, fmt.Errorf("file type %s is not allowed", ext)
	}

	// Get MIME type
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Open file to calculate hash
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()

	// Calculate file hash
	hash, err := calculateFileHash(src)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %v", err)
	}

	// Generate unique filename
	uniqueName := generateUniqueFileName(file.Filename, ext)

	// Generate storage path
	storagePath := generateStoragePath(uniqueName)

	// Extract metadata
	metadata := extractFileMetadata(file, mimeType)

	return &FileInfo{
		Name:         uniqueName,
		OriginalName: file.Filename,
		Size:         file.Size,
		Extension:    ext,
		MimeType:     mimeType,
		Hash:         hash,
		Path:         storagePath,
		Metadata:     metadata,
	}, nil
}

// isAllowedFileType checks if file extension is allowed
func isAllowedFileType(ext string, allowedTypes []string) bool {
	if len(allowedTypes) == 0 {
		return true // No restrictions
	}

	for _, allowed := range allowedTypes {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}
	return false
}

// calculateFileHash calculates MD5 hash of file content
func calculateFileHash(file io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// generateUniqueFileName generates a unique filename with timestamp and random string
func generateUniqueFileName(originalName, ext string) string {
	timestamp := time.Now().Unix()
	randomStr := generateRandomString(8)
	name := strings.TrimSuffix(originalName, ext)

	// Clean filename
	name = cleanFileName(name)

	return fmt.Sprintf("%s_%d_%s%s", name, timestamp, randomStr, ext)
}

// generateStoragePath generates storage path based on date
func generateStoragePath(filename string) string {
	now := time.Now()
	return fmt.Sprintf("%d/%02d/%02d/%s", now.Year(), now.Month(), now.Day(), filename)
}

// cleanFileName removes invalid characters from filename
func cleanFileName(name string) string {
	// Replace invalid characters with underscore
	invalid := []string{" ", "<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	result := name

	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Remove multiple consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	return strings.Trim(result, "_")
}

// extractFileMetadata extracts metadata from file
func extractFileMetadata(file *multipart.FileHeader, mimeType string) map[string]string {
	metadata := make(map[string]string)

	metadata["original_name"] = file.Filename
	metadata["mime_type"] = mimeType
	metadata["upload_time"] = time.Now().Format(time.RFC3339)

	// Add file category
	metadata["category"] = getFileCategory(mimeType)

	return metadata
}

// getFileCategory determines file category based on MIME type
func getFileCategory(mimeType string) string {
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	} else if strings.HasPrefix(mimeType, "video/") {
		return "video"
	} else if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	} else if strings.Contains(mimeType, "pdf") {
		return "document"
	} else if strings.Contains(mimeType, "text/") {
		return "text"
	} else if strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "archive") {
		return "archive"
	}
	return "other"
}

// ValidateUploadLimits validates upload against user limits
func ValidateUploadLimits(fileSize int64, userStorageUsed, userStorageLimit int64) error {
	if userStorageUsed+fileSize > userStorageLimit {
		return fmt.Errorf("upload would exceed storage limit")
	}
	return nil
}

// GenerateUploadPath generates a unique upload path for chunked uploads
func GenerateUploadPath(userID, filename string) string {
	timestamp := time.Now().Unix()
	randomStr := generateRandomString(8)
	return fmt.Sprintf("uploads/%s/%d_%s_%s", userID, timestamp, randomStr, filename)
}
