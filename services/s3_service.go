package services

import (
	"context"
	"fmt"
	"io"
	"oncloud/database"
	"oncloud/models"
	"oncloud/storage"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type S3Service struct {
	providerCollection *mongo.Collection
	fileCollection     *mongo.Collection
	client             storage.StorageInterface
	provider           *models.StorageProvider
}

func NewS3Service() *S3Service {
	return &S3Service{
		providerCollection: database.GetCollection("storage_providers"),
		fileCollection:     database.GetCollection("files"),
	}
}

// Initialize S3 client with provider configuration
func (s3s *S3Service) InitializeClient(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := s3s.providerCollection.FindOne(ctx, bson.M{
		"_id":       providerID,
		"type":      "s3",
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("S3 provider not found: %v", err)
	}

	client, err := storage.NewS3Client(&provider)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 client: %v", err)
	}

	s3s.client = client
	s3s.provider = &provider
	return nil
}

// Initialize with default S3 provider
func (s3s *S3Service) InitializeDefaultClient() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := s3s.providerCollection.FindOne(ctx, bson.M{
		"type":       "s3",
		"is_default": true,
		"is_active":  true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("default S3 provider not found: %v", err)
	}

	client, err := storage.NewS3Client(&provider)
	if err != nil {
		return fmt.Errorf("failed to initialize default S3 client: %v", err)
	}

	s3s.client = client
	s3s.provider = &provider
	return nil
}

// File Operations
func (s3s *S3Service) UploadFile(key string, data []byte, options *storage.UploadOptions) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	// Handle upload options for S3
	if options != nil {
		// S3Client would handle upload options in a real implementation
		// Including metadata, server-side encryption, storage class, etc.
	}

	err := s3s.client.Upload(key, data)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %v", err)
	}

	// Log upload activity
	s3s.logActivity("upload", key, int64(len(data)), nil)

	return nil
}

func (s3s *S3Service) UploadStream(key string, reader io.Reader, size int64) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.UploadStream(key, reader, size)
	if err != nil {
		return fmt.Errorf("failed to upload stream to S3: %v", err)
	}

	// Log upload activity
	s3s.logActivity("upload_stream", key, size, nil)

	return nil
}

func (s3s *S3Service) DownloadFile(key string) ([]byte, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	data, err := s3s.client.Download(key)
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %v", err)
	}

	// Log download activity
	s3s.logActivity("download", key, int64(len(data)), nil)

	return data, nil
}

func (s3s *S3Service) DownloadStream(key string) (io.ReadCloser, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	stream, err := s3s.client.DownloadStream(key)
	if err != nil {
		return nil, fmt.Errorf("failed to download stream from S3: %v", err)
	}

	// Log download activity
	go s3s.logActivity("download_stream", key, 0, nil)

	return stream, nil
}

func (s3s *S3Service) DeleteFile(key string) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.Delete(key)
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %v", err)
	}

	// Log delete activity
	s3s.logActivity("delete", key, 0, nil)

	return nil
}

func (s3s *S3Service) DeleteMultipleFiles(keys []string) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.DeleteMultiple(keys)
	if err != nil {
		return fmt.Errorf("failed to delete multiple files from S3: %v", err)
	}

	// Log bulk delete activity
	s3s.logActivity("bulk_delete", strings.Join(keys, ","), int64(len(keys)), map[string]interface{}{
		"file_count": len(keys),
	})

	return nil
}

func (s3s *S3Service) FileExists(key string) (bool, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return false, err
		}
	}

	exists, err := s3s.client.Exists(key)
	if err != nil {
		return false, fmt.Errorf("failed to check file existence in S3: %v", err)
	}

	return exists, nil
}

func (s3s *S3Service) GetFileSize(key string) (int64, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return 0, err
		}
	}

	size, err := s3s.client.GetSize(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get file size from S3: %v", err)
	}

	return size, nil
}

// URL Operations
func (s3s *S3Service) GetPublicURL(key string) (string, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return "", err
		}
	}

	url, err := s3s.client.GetURL(key)
	if err != nil {
		return "", fmt.Errorf("failed to get public URL from S3: %v", err)
	}

	return url, nil
}

func (s3s *S3Service) GetPresignedDownloadURL(key string, expiry time.Duration) (string, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return "", err
		}
	}

	url, err := s3s.client.GetPresignedURL(key, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to get presigned download URL from S3: %v", err)
	}

	return url, nil
}

func (s3s *S3Service) GetPresignedUploadURL(key string, expiry time.Duration, maxSize int64) (string, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return "", err
		}
	}

	url, err := s3s.client.GetPresignedUploadURL(key, expiry, maxSize)
	if err != nil {
		return "", fmt.Errorf("failed to get presigned upload URL from S3: %v", err)
	}

	return url, nil
}

// Multipart Upload Operations
func (s3s *S3Service) InitiateMultipartUpload(key string) (*storage.MultipartUpload, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	upload, err := s3s.client.InitiateMultipartUpload(key)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate multipart upload in S3: %v", err)
	}

	// Log multipart upload initiation
	s3s.logActivity("multipart_init", key, 0, map[string]interface{}{
		"upload_id": upload.UploadID,
	})

	return upload, nil
}

func (s3s *S3Service) UploadPart(uploadID, key string, partNumber int, data []byte) (*storage.UploadPart, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	part, err := s3s.client.UploadPart(uploadID, key, partNumber, data)
	if err != nil {
		return nil, fmt.Errorf("failed to upload part to S3: %v", err)
	}

	// Log part upload
	s3s.logActivity("multipart_upload_part", key, int64(len(data)), map[string]interface{}{
		"upload_id":   uploadID,
		"part_number": partNumber,
		"etag":        part.ETag,
	})

	return part, nil
}

func (s3s *S3Service) CompleteMultipartUpload(uploadID, key string, parts []storage.UploadPart) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.CompleteMultipartUpload(uploadID, key, parts)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload in S3: %v", err)
	}

	// Calculate total size
	var totalSize int64
	for _, part := range parts {
		totalSize += part.Size
	}

	// Log multipart upload completion
	s3s.logActivity("multipart_complete", key, totalSize, map[string]interface{}{
		"upload_id":  uploadID,
		"part_count": len(parts),
	})

	return nil
}

func (s3s *S3Service) AbortMultipartUpload(uploadID, key string) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.AbortMultipartUpload(uploadID, key)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload in S3: %v", err)
	}

	// Log multipart upload abortion
	s3s.logActivity("multipart_abort", key, 0, map[string]interface{}{
		"upload_id": uploadID,
	})

	return nil
}

// File Management Operations
func (s3s *S3Service) CopyFile(sourceKey, destKey string) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.CopyFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("failed to copy file in S3: %v", err)
	}

	// Log copy activity
	s3s.logActivity("copy", destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

func (s3s *S3Service) MoveFile(sourceKey, destKey string) error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.MoveFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("failed to move file in S3: %v", err)
	}

	// Log move activity
	s3s.logActivity("move", destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

// Provider Management
func (s3s *S3Service) GetProviderInfo() (*storage.ProviderInfo, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	return s3s.client.GetProviderInfo(), nil
}

func (s3s *S3Service) HealthCheck() error {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := s3s.client.HealthCheck()
	if err != nil {
		return fmt.Errorf("S3 health check failed: %v", err)
	}

	return nil
}

func (s3s *S3Service) GetStorageStats() (*storage.StorageStats, error) {
	if s3s.client == nil {
		if err := s3s.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	return s3s.client.GetStats()
}

// Provider Configuration
func (s3s *S3Service) CreateProvider(provider *models.StorageProvider) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Validate S3 provider configuration
	if err := s3s.validateS3Config(provider); err != nil {
		return nil, err
	}

	// Test connection
	testClient, err := storage.NewS3Client(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %v", err)
	}

	if err := testClient.HealthCheck(); err != nil {
		return nil, fmt.Errorf("S3 connection test failed: %v", err)
	}

	provider.ID = primitive.NewObjectID()
	provider.Type = "s3"
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()

	_, err = s3s.providerCollection.InsertOne(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to save S3 provider: %v", err)
	}

	return provider, nil
}

func (s3s *S3Service) UpdateProvider(providerID primitive.ObjectID, updates map[string]interface{}) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now()

	_, err := s3s.providerCollection.UpdateOne(ctx,
		bson.M{"_id": providerID, "type": "s3"},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update S3 provider: %v", err)
	}

	var provider models.StorageProvider
	err = s3s.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&provider)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func (s3s *S3Service) DeleteProvider(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if provider is in use
	fileCount, err := s3s.fileCollection.CountDocuments(ctx, bson.M{
		"storage_provider": "s3",
		"provider_id":      providerID,
	})
	if err != nil {
		return err
	}
	if fileCount > 0 {
		return fmt.Errorf("cannot delete S3 provider that is currently storing %d files", fileCount)
	}

	_, err = s3s.providerCollection.DeleteOne(ctx, bson.M{
		"_id":  providerID,
		"type": "s3",
	})
	if err != nil {
		return fmt.Errorf("failed to delete S3 provider: %v", err)
	}

	return nil
}

func (s3s *S3Service) TestConnection(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := s3s.providerCollection.FindOne(ctx, bson.M{
		"_id":  providerID,
		"type": "s3",
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("S3 provider not found: %v", err)
	}

	client, err := storage.NewS3Client(&provider)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	return client.HealthCheck()
}

// S3-specific Operations
func (s3s *S3Service) SetStorageClass(key, storageClass string) error {
	// Implementation for changing S3 storage class (Standard, IA, Glacier, etc.)
	s3s.logActivity("set_storage_class", key, 0, map[string]interface{}{
		"storage_class": storageClass,
	})

	return nil
}

func (s3s *S3Service) RestoreFromGlacier(key string, days int) error {
	// Implementation for restoring objects from Glacier
	s3s.logActivity("glacier_restore", key, 0, map[string]interface{}{
		"restore_days": days,
	})

	return nil
}

func (s3s *S3Service) EnableVersioning() error {
	// Implementation for enabling S3 bucket versioning
	s3s.logActivity("enable_versioning", "", 0, nil)

	return nil
}

func (s3s *S3Service) ListObjectVersions(key string) ([]map[string]interface{}, error) {
	// Implementation for listing object versions
	versions := []map[string]interface{}{
		{
			"version_id": "example-version-id",
			"key":        key,
			"size":       1024,
			"modified":   time.Now(),
		},
	}

	return versions, nil
}

func (s3s *S3Service) SetLifecyclePolicy(policy map[string]interface{}) error {
	// Implementation for setting S3 lifecycle policies
	s3s.logActivity("set_lifecycle_policy", "", 0, map[string]interface{}{
		"policy": policy,
	})

	return nil
}

func (s3s *S3Service) GetBucketMetrics() (map[string]interface{}, error) {
	// Implementation for getting CloudWatch metrics
	metrics := map[string]interface{}{
		"bucket_size":      1024 * 1024 * 100, // 100MB
		"object_count":     1000,
		"requests_per_day": 5000,
		"data_retrieved":   1024 * 1024 * 50, // 50MB
		"storage_classes": map[string]interface{}{
			"standard":   1024 * 1024 * 80, // 80MB
			"infrequent": 1024 * 1024 * 15, // 15MB
			"glacier":    1024 * 1024 * 5,  // 5MB
		},
	}

	return metrics, nil
}

// Helper functions
func (s3s *S3Service) validateS3Config(provider *models.StorageProvider) error {
	if provider.AccessKey == "" || provider.SecretKey == "" {
		return fmt.Errorf("AWS access key and secret key are required")
	}

	if provider.Region == "" {
		return fmt.Errorf("AWS region is required")
	}

	if provider.Bucket == "" {
		return fmt.Errorf("S3 bucket name is required")
	}

	return nil
}

func (s3s *S3Service) logActivity(action, key string, size int64, metadata map[string]interface{}) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		activity := bson.M{
			"provider":   "s3",
			"action":     action,
			"key":        key,
			"size":       size,
			"metadata":   metadata,
			"created_at": time.Now(),
		}

		if s3s.provider != nil {
			activity["provider_id"] = s3s.provider.ID
		}

		collection := database.GetCollection("storage_activities")
		collection.InsertOne(ctx, activity)
	}()
}
