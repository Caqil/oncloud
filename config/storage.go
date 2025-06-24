package main

import (
	"fmt"
	"io"
	"log"
	"oncloud/models"
	"oncloud/services"
	"oncloud/storage"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StorageManager handles storage operations and provider management
type StorageManager struct {
	config          *Config
	storageService  *services.StorageService
	providers       map[string]storage.StorageInterface
	defaultProvider string
}

// NewStorageManager creates a new storage manager
func NewStorageManager(config *Config) *StorageManager {
	return &StorageManager{
		config:          config,
		storageService:  services.NewStorageService(),
		providers:       make(map[string]storage.StorageInterface),
		defaultProvider: config.DefaultStorageProvider,
	}
}

// Initialize initializes the storage subsystem
func (sm *StorageManager) Initialize() error {
	log.Println("Initializing storage subsystem...")

	// Create local upload directory if it doesn't exist
	if err := os.MkdirAll(sm.config.UploadPath, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	// Initialize default local storage provider
	if err := sm.initializeLocalStorage(); err != nil {
		return fmt.Errorf("failed to initialize local storage: %v", err)
	}

	// Initialize other storage providers from database
	if err := sm.initializeProvidersFromDB(); err != nil {
		log.Printf("Warning: Failed to initialize providers from database: %v", err)
	}

	log.Printf("Storage subsystem initialized with default provider: %s", sm.defaultProvider)
	return nil
}

// initializeLocalStorage sets up the local storage provider
func (sm *StorageManager) initializeLocalStorage() error {
	localProvider := &models.StorageProvider{
		Name:   "Local Storage",
		Type:   "local",
		Region: "local",
		Bucket: "uploads",
		Settings: map[string]interface{}{
			"base_path": sm.config.UploadPath,
		},
		IsActive:  true,
		IsDefault: true,
		Priority:  1,
	}

	client, err := storage.NewStorageClient(localProvider)
	if err != nil {
		return fmt.Errorf("failed to create local storage client: %v", err)
	}

	sm.providers["local"] = client
	return nil
}

// initializeProvidersFromDB loads and initializes storage providers from database
func (sm *StorageManager) initializeProvidersFromDB() error {
	providers, err := sm.storageService.GetProviders()
	if err != nil {
		return fmt.Errorf("failed to get active providers: %v", err)
	}

	for _, provider := range providers {
		if err := sm.initializeProvider(&provider); err != nil {
			log.Printf("Warning: Failed to initialize provider %s (%s): %v",
				provider.Name, provider.Type, err)
			continue
		}
		log.Printf("Initialized storage provider: %s (%s)", provider.Name, provider.Type)
	}

	return nil
}

// initializeProvider initializes a specific storage provider
func (sm *StorageManager) initializeProvider(provider *models.StorageProvider) error {
	var client storage.StorageInterface
	var err error

	switch strings.ToLower(provider.Type) {
	case "local":
		client, err = storage.NewStorageClient(provider)
	case "s3":
		client, err = storage.NewS3Client(provider)
	case "r2":
		client, err = storage.NewR2Client(provider)
	case "wasabi":
		client, err = storage.NewWasabiClient(provider)
	default:
		return fmt.Errorf("unsupported storage provider type: %s", provider.Type)
	}

	if err != nil {
		return err
	}

	// Test the connection
	if err := client.HealthCheck(); err != nil {
		return fmt.Errorf("provider health check failed: %v", err)
	}

	// Store the client
	sm.providers[provider.Type] = client

	// Set as default if specified
	if provider.IsDefault {
		sm.defaultProvider = provider.Type
	}

	return nil
}

// GetProvider returns a storage provider client
func (sm *StorageManager) GetProvider(providerType string) (storage.StorageInterface, error) {
	if providerType == "" {
		providerType = sm.defaultProvider
	}

	client, exists := sm.providers[providerType]
	if !exists {
		return nil, fmt.Errorf("storage provider not found: %s", providerType)
	}

	return client, nil
}

// GetDefaultProvider returns the default storage provider
func (sm *StorageManager) GetDefaultProvider() storage.StorageInterface {
	client, exists := sm.providers[sm.defaultProvider]
	if !exists {
		// Fallback to local if default is not available
		return sm.providers["local"]
	}
	return client
}

// UploadFile uploads a file using the specified or default provider
func (sm *StorageManager) UploadFile(providerType, key string, data []byte) error {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return err
	}

	return provider.Upload(key, data)
}

// UploadFileStream uploads a file from a stream
func (sm *StorageManager) UploadFileStream(providerType, key string, reader io.Reader, size int64) error {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return err
	}

	return provider.UploadStream(key, reader, size)
}

// DownloadFile downloads a file from the specified provider
func (sm *StorageManager) DownloadFile(providerType, key string) ([]byte, error) {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return nil, err
	}

	return provider.Download(key)
}

// DeleteFile deletes a file from the specified provider
func (sm *StorageManager) DeleteFile(providerType, key string) error {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return err
	}

	return provider.Delete(key)
}

// GetPresignedURL generates a presigned URL for file access
func (sm *StorageManager) GetPresignedURL(providerType, key string, expiration time.Duration, operation string) (string, error) {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return "", err
	}

	return provider.GetPresignedURL(key, expiration)
}

// MoveFile moves a file between storage locations
func (sm *StorageManager) MoveFile(providerType, sourceKey, destKey string) error {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return err
	}

	// Download from source
	data, err := provider.Download(sourceKey)
	if err != nil {
		return fmt.Errorf("failed to download source file: %v", err)
	}

	// Upload to destination
	if err := provider.Upload(destKey, data); err != nil {
		return fmt.Errorf("failed to upload to destination: %v", err)
	}

	// Delete source
	if err := provider.Delete(sourceKey); err != nil {
		log.Printf("Warning: Failed to delete source file after move: %v", err)
	}

	return nil
}

// CopyFile copies a file within the same storage provider
func (sm *StorageManager) CopyFile(providerType, sourceKey, destKey string) error {
	provider, err := sm.GetProvider(providerType)
	if err != nil {
		return err
	}

	// Download from source
	data, err := provider.Download(sourceKey)
	if err != nil {
		return fmt.Errorf("failed to download source file: %v", err)
	}

	// Upload to destination
	if err := provider.Upload(destKey, data); err != nil {
		return fmt.Errorf("failed to upload copy: %v", err)
	}

	return nil
}

// GetStorageStats returns storage statistics for all providers
func (sm *StorageManager) GetStorageStats() map[string]interface{} {
	stats := make(map[string]interface{})

	for providerType, provider := range sm.providers {
		if providerStats, err := provider.GetStats(); err == nil {
			stats[providerType] = providerStats
		} else {
			stats[providerType] = map[string]interface{}{
				"error": err.Error(),
			}
		}
	}

	return stats
}

// HealthCheck performs health checks on all storage providers
func (sm *StorageManager) HealthCheck() map[string]bool {
	results := make(map[string]bool)

	for providerType, provider := range sm.providers {
		err := provider.HealthCheck()
		results[providerType] = err == nil

		if err != nil {
			log.Printf("Storage provider %s health check failed: %v", providerType, err)
		}
	}

	return results
}

// ValidateFileType checks if a file type is allowed
func (sm *StorageManager) ValidateFileType(filename string) bool {
	if len(sm.config.AllowedFileTypes) == 0 {
		return true // Allow all types if none specified
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" && ext[0] == '.' {
		ext = ext[1:] // Remove the dot
	}

	for _, allowedType := range sm.config.AllowedFileTypes {
		if strings.ToLower(allowedType) == ext {
			return true
		}
	}

	return false
}

// ValidateFileSize checks if a file size is within limits
func (sm *StorageManager) ValidateFileSize(size int64) bool {
	return size <= sm.config.MaxUploadSize
}

// GenerateStorageKey generates a unique storage key for a file
func (sm *StorageManager) GenerateStorageKey(userID, filename string) string {
	timestamp := time.Now().Unix()
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// Sanitize filename
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	return fmt.Sprintf("%s/%d_%s%s", userID, timestamp, name, ext)
}

// CleanupOrphanedFiles removes files that exist in storage but not in database
func (sm *StorageManager) CleanupOrphanedFiles() error {
	log.Println("Starting orphaned files cleanup...")

	// This is a simplified implementation
	// In a real scenario, you'd want to compare storage contents with database records
	for providerType, provider := range sm.providers {
		log.Printf("Checking for orphaned files in provider: %s", providerType)

		// Get provider info to check if cleanup is supported
		if info := provider.GetProviderInfo(); info != nil {
			log.Printf("Provider %s info: %+v", providerType, info)
		}
	}

	log.Println("Orphaned files cleanup completed")
	return nil
}

// AddProvider dynamically adds a new storage provider
func (sm *StorageManager) AddProvider(provider *models.StorageProvider) error {
	if err := sm.initializeProvider(provider); err != nil {
		return fmt.Errorf("failed to initialize new provider: %v", err)
	}

	log.Printf("Successfully added storage provider: %s (%s)", provider.Name, provider.Type)
	return nil
}

// RemoveProvider removes a storage provider
func (sm *StorageManager) RemoveProvider(providerType string) error {
	if providerType == "local" {
		return fmt.Errorf("cannot remove local storage provider")
	}

	if providerType == sm.defaultProvider {
		return fmt.Errorf("cannot remove default storage provider")
	}

	delete(sm.providers, providerType)
	log.Printf("Removed storage provider: %s", providerType)
	return nil
}

// SyncProviders synchronizes providers with database configuration
func (sm *StorageManager) SyncProviders() error {
	log.Println("Synchronizing storage providers with database...")

	// Clear existing providers (except local)
	for providerType := range sm.providers {
		if providerType != "local" {
			delete(sm.providers, providerType)
		}
	}

	// Reload from database
	return sm.initializeProvidersFromDB()
}

// GetProviderTypes returns all available provider types
func (sm *StorageManager) GetProviderTypes() []string {
	types := make([]string, 0, len(sm.providers))
	for providerType := range sm.providers {
		types = append(types, providerType)
	}
	return types
}
