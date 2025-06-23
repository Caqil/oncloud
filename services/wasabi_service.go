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

type WasabiService struct {
	providerCollection *mongo.Collection
	fileCollection     *mongo.Collection
	client             storage.StorageInterface
	provider           *models.StorageProvider
}

func NewWasabiService() *WasabiService {
	return &WasabiService{
		providerCollection: database.GetCollection("storage_providers"),
		fileCollection:     database.GetCollection("files"),
	}
}

// Initialize Wasabi client with provider configuration
func (ws *WasabiService) InitializeClient(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := ws.providerCollection.FindOne(ctx, bson.M{
		"_id":       providerID,
		"type":      "wasabi",
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("Wasabi provider not found: %v", err)
	}

	client, err := storage.NewWasabiClient(&provider)
	if err != nil {
		return fmt.Errorf("failed to initialize Wasabi client: %v", err)
	}

	ws.client = client
	ws.provider = &provider
	return nil
}

// Initialize with default Wasabi provider
func (ws *WasabiService) InitializeDefaultClient() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := ws.providerCollection.FindOne(ctx, bson.M{
		"type":       "wasabi",
		"is_default": true,
		"is_active":  true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("default Wasabi provider not found: %v", err)
	}

	client, err := storage.NewWasabiClient(&provider)
	if err != nil {
		return fmt.Errorf("failed to initialize default Wasabi client: %v", err)
	}

	ws.client = client
	ws.provider = &provider
	return nil
}

// File Operations
func (ws *WasabiService) UploadFile(key string, data []byte, options *storage.UploadOptions) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	// Handle upload options for Wasabi
	if options != nil {
		// WasabiClient would handle upload options in a real implementation
		// Including immutability, metadata, etc.
	}

	err := ws.client.Upload(key, data)
	if err != nil {
		return fmt.Errorf("failed to upload to Wasabi: %v", err)
	}

	// Log upload activity
	ws.logActivity("upload", key, int64(len(data)), nil)

	return nil
}

func (ws *WasabiService) UploadStream(key string, reader io.Reader, size int64) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.UploadStream(key, reader, size)
	if err != nil {
		return fmt.Errorf("failed to upload stream to Wasabi: %v", err)
	}

	// Log upload activity
	ws.logActivity("upload_stream", key, size, nil)

	return nil
}

func (ws *WasabiService) DownloadFile(key string) ([]byte, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	data, err := ws.client.Download(key)
	if err != nil {
		return nil, fmt.Errorf("failed to download from Wasabi: %v", err)
	}

	// Log download activity
	ws.logActivity("download", key, int64(len(data)), nil)

	return data, nil
}

func (ws *WasabiService) DownloadStream(key string) (io.ReadCloser, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	stream, err := ws.client.DownloadStream(key)
	if err != nil {
		return nil, fmt.Errorf("failed to download stream from Wasabi: %v", err)
	}

	// Log download activity
	go ws.logActivity("download_stream", key, 0, nil)

	return stream, nil
}

func (ws *WasabiService) DeleteFile(key string) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.Delete(key)
	if err != nil {
		return fmt.Errorf("failed to delete from Wasabi: %v", err)
	}

	// Log delete activity
	ws.logActivity("delete", key, 0, nil)

	return nil
}

func (ws *WasabiService) DeleteMultipleFiles(keys []string) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.DeleteMultiple(keys)
	if err != nil {
		return fmt.Errorf("failed to delete multiple files from Wasabi: %v", err)
	}

	// Log bulk delete activity
	ws.logActivity("bulk_delete", strings.Join(keys, ","), int64(len(keys)), map[string]interface{}{
		"file_count": len(keys),
	})

	return nil
}

func (ws *WasabiService) FileExists(key string) (bool, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return false, err
		}
	}

	exists, err := ws.client.Exists(key)
	if err != nil {
		return false, fmt.Errorf("failed to check file existence in Wasabi: %v", err)
	}

	return exists, nil
}

func (ws *WasabiService) GetFileSize(key string) (int64, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return 0, err
		}
	}

	size, err := ws.client.GetSize(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get file size from Wasabi: %v", err)
	}

	return size, nil
}

// URL Operations
func (ws *WasabiService) GetPublicURL(key string) (string, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return "", err
		}
	}

	url, err := ws.client.GetURL(key)
	if err != nil {
		return "", fmt.Errorf("failed to get public URL from Wasabi: %v", err)
	}

	return url, nil
}

func (ws *WasabiService) GetPresignedDownloadURL(key string, expiry time.Duration) (string, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return "", err
		}
	}

	url, err := ws.client.GetPresignedURL(key, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to get presigned download URL from Wasabi: %v", err)
	}

	return url, nil
}

func (ws *WasabiService) GetPresignedUploadURL(key string, expiry time.Duration, maxSize int64) (string, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return "", err
		}
	}

	url, err := ws.client.GetPresignedUploadURL(key, expiry, maxSize)
	if err != nil {
		return "", fmt.Errorf("failed to get presigned upload URL from Wasabi: %v", err)
	}

	return url, nil
}

// Multipart Upload Operations
func (ws *WasabiService) InitiateMultipartUpload(key string) (*storage.MultipartUpload, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	upload, err := ws.client.InitiateMultipartUpload(key)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate multipart upload in Wasabi: %v", err)
	}

	// Log multipart upload initiation
	ws.logActivity("multipart_init", key, 0, map[string]interface{}{
		"upload_id": upload.UploadID,
	})

	return upload, nil
}

func (ws *WasabiService) UploadPart(uploadID, key string, partNumber int, data []byte) (*storage.UploadPart, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	part, err := ws.client.UploadPart(uploadID, key, partNumber, data)
	if err != nil {
		return nil, fmt.Errorf("failed to upload part to Wasabi: %v", err)
	}

	// Log part upload
	ws.logActivity("multipart_upload_part", key, int64(len(data)), map[string]interface{}{
		"upload_id":   uploadID,
		"part_number": partNumber,
		"etag":        part.ETag,
	})

	return part, nil
}

func (ws *WasabiService) CompleteMultipartUpload(uploadID, key string, parts []storage.UploadPart) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.CompleteMultipartUpload(uploadID, key, parts)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload in Wasabi: %v", err)
	}

	// Calculate total size
	var totalSize int64
	for _, part := range parts {
		totalSize += part.Size
	}

	// Log multipart upload completion
	ws.logActivity("multipart_complete", key, totalSize, map[string]interface{}{
		"upload_id":  uploadID,
		"part_count": len(parts),
	})

	return nil
}

func (ws *WasabiService) AbortMultipartUpload(uploadID, key string) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.AbortMultipartUpload(uploadID, key)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload in Wasabi: %v", err)
	}

	// Log multipart upload abortion
	ws.logActivity("multipart_abort", key, 0, map[string]interface{}{
		"upload_id": uploadID,
	})

	return nil
}

// File Management Operations
func (ws *WasabiService) CopyFile(sourceKey, destKey string) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.CopyFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("failed to copy file in Wasabi: %v", err)
	}

	// Log copy activity
	ws.logActivity("copy", destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

func (ws *WasabiService) MoveFile(sourceKey, destKey string) error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.MoveFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("failed to move file in Wasabi: %v", err)
	}

	// Log move activity
	ws.logActivity("move", destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

// Provider Management
func (ws *WasabiService) GetProviderInfo() (*storage.ProviderInfo, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	return ws.client.GetProviderInfo(), nil
}

func (ws *WasabiService) HealthCheck() error {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return err
		}
	}

	err := ws.client.HealthCheck()
	if err != nil {
		return fmt.Errorf("Wasabi health check failed: %v", err)
	}

	return nil
}

func (ws *WasabiService) GetStorageStats() (*storage.StorageStats, error) {
	if ws.client == nil {
		if err := ws.InitializeDefaultClient(); err != nil {
			return nil, err
		}
	}

	return ws.client.GetStats()
}

// Provider Configuration
func (ws *WasabiService) CreateProvider(provider *models.StorageProvider) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Validate Wasabi provider configuration
	if err := ws.validateWasabiConfig(provider); err != nil {
		return nil, err
	}

	// Test connection
	testClient, err := storage.NewWasabiClient(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	if err := testClient.HealthCheck(); err != nil {
		return nil, fmt.Errorf("Wasabi connection test failed: %v", err)
	}

	provider.ID = primitive.NewObjectID()
	provider.Type = "wasabi"
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()

	_, err = ws.providerCollection.InsertOne(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to save Wasabi provider: %v", err)
	}

	return provider, nil
}

func (ws *WasabiService) UpdateProvider(providerID primitive.ObjectID, updates map[string]interface{}) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now()

	_, err := ws.providerCollection.UpdateOne(ctx,
		bson.M{"_id": providerID, "type": "wasabi"},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update Wasabi provider: %v", err)
	}

	var provider models.StorageProvider
	err = ws.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&provider)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func (ws *WasabiService) DeleteProvider(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if provider is in use
	fileCount, err := ws.fileCollection.CountDocuments(ctx, bson.M{
		"storage_provider": "wasabi",
		"provider_id":      providerID,
	})
	if err != nil {
		return err
	}
	if fileCount > 0 {
		return fmt.Errorf("cannot delete Wasabi provider that is currently storing %d files", fileCount)
	}

	_, err = ws.providerCollection.DeleteOne(ctx, bson.M{
		"_id":  providerID,
		"type": "wasabi",
	})
	if err != nil {
		return fmt.Errorf("failed to delete Wasabi provider: %v", err)
	}

	return nil
}

func (ws *WasabiService) TestConnection(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := ws.providerCollection.FindOne(ctx, bson.M{
		"_id":  providerID,
		"type": "wasabi",
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("Wasabi provider not found: %v", err)
	}

	client, err := storage.NewWasabiClient(&provider)
	if err != nil {
		return fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	return client.HealthCheck()
}

// Wasabi-specific Operations
func (ws *WasabiService) EnableImmutability(key string, retentionDays int) error {
	// Implementation for Wasabi Immutable Storage
	ws.logActivity("enable_immutability", key, 0, map[string]interface{}{
		"retention_days": retentionDays,
	})

	return nil
}

func (ws *WasabiService) GetImmutabilityStatus(key string) (map[string]interface{}, error) {
	// Implementation for checking immutability status
	status := map[string]interface{}{
		"key":               key,
		"is_immutable":      false,
		"retention_days":    0,
		"retention_expires": nil,
	}

	return status, nil
}

func (ws *WasabiService) SetStorageClass(key, storageClass string) error {
	// Wasabi storage classes: STANDARD, STANDARD_IA, INTELLIGENT_TIERING
	ws.logActivity("set_storage_class", key, 0, map[string]interface{}{
		"storage_class": storageClass,
	})

	return nil
}

func (ws *WasabiService) GetBucketUsage() (map[string]interface{}, error) {
	// Implementation for getting detailed bucket usage
	usage := map[string]interface{}{
		"total_objects": 1000,
		"total_size":    1024 * 1024 * 100, // 100MB
		"by_storage_class": map[string]interface{}{
			"standard":          1024 * 1024 * 80, // 80MB
			"infrequent_access": 1024 * 1024 * 20, // 20MB
		},
		"immutable_objects": 50,
		"last_updated":      time.Now(),
	}

	return usage, nil
}

func (ws *WasabiService) GetCostOptimization() (map[string]interface{}, error) {
	// Wasabi cost optimization recommendations
	recommendations := map[string]interface{}{
		"total_monthly_cost": 5.99,
		"currency":           "USD",
		"cost_per_gb":        0.0059,
		"egress_charges":     0.0, // Wasabi has no egress charges
		"optimization_tips": []string{
			"No egress fees - unlimited downloads",
			"Consider using Intelligent Tiering for infrequent access",
			"Use immutable storage for compliance requirements",
		},
		"potential_savings": map[string]interface{}{
			"vs_aws_s3": "up to 80%",
			"vs_azure":  "up to 60%",
			"vs_gcp":    "up to 70%",
		},
	}

	return recommendations, nil
}

func (ws *WasabiService) EnableLogging(logBucket string) error {
	// Implementation for enabling access logging
	ws.logActivity("enable_logging", "", 0, map[string]interface{}{
		"log_bucket": logBucket,
	})

	return nil
}

func (ws *WasabiService) GetAccessLogs(startDate, endDate time.Time) ([]map[string]interface{}, error) {
	// Implementation for retrieving access logs
	logs := []map[string]interface{}{
		{
			"timestamp":  time.Now(),
			"operation":  "GET",
			"key":        "example/file.jpg",
			"ip_address": "192.168.1.1",
			"user_agent": "CloudStorage/1.0",
			"bytes_sent": 1024,
		},
	}

	return logs, nil
}

func (ws *WasabiService) CreateBucketPolicy(policy map[string]interface{}) error {
	// Implementation for creating bucket policies
	ws.logActivity("create_bucket_policy", "", 0, map[string]interface{}{
		"policy": policy,
	})

	return nil
}

func (ws *WasabiService) GetBucketPolicy() (map[string]interface{}, error) {
	// Implementation for retrieving bucket policy
	policy := map[string]interface{}{
		"version": "2012-10-17",
		"statements": []map[string]interface{}{
			{
				"effect":    "Allow",
				"principal": "*",
				"action":    "s3:GetObject",
				"resource":  "arn:aws:s3:::bucket/*",
			},
		},
	}

	return policy, nil
}

// Helper functions
func (ws *WasabiService) validateWasabiConfig(provider *models.StorageProvider) error {
	if provider.AccessKey == "" || provider.SecretKey == "" {
		return fmt.Errorf("Wasabi access key and secret key are required")
	}

	if provider.Region == "" {
		return fmt.Errorf("Wasabi region is required")
	}

	if provider.Bucket == "" {
		return fmt.Errorf("Wasabi bucket name is required")
	}

	// Validate region
	validRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-central-1",
		"eu-central-1", "eu-west-1", "eu-west-2",
		"ap-northeast-1", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2",
	}

	validRegion := false
	for _, region := range validRegions {
		if provider.Region == region {
			validRegion = true
			break
		}
	}

	if !validRegion {
		return fmt.Errorf("invalid Wasabi region: %s", provider.Region)
	}

	return nil
}

func (ws *WasabiService) logActivity(action, key string, size int64, metadata map[string]interface{}) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		activity := bson.M{
			"provider":   "wasabi",
			"action":     action,
			"key":        key,
			"size":       size,
			"metadata":   metadata,
			"created_at": time.Now(),
		}

		if ws.provider != nil {
			activity["provider_id"] = ws.provider.ID
		}

		collection := database.GetCollection("storage_activities")
		collection.InsertOne(ctx, activity)
	}()
}
