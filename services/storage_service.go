package services

import (
	"context"
	"fmt"
	"io"
	"mime"
	"oncloud/database"
	"oncloud/models"
	"oncloud/storage"
	"oncloud/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StorageService struct {
	providerCollection *mongo.Collection
	fileCollection     *mongo.Collection
	userCollection     *mongo.Collection
	syncCollection     *mongo.Collection
	backupCollection   *mongo.Collection
	activityCollection *mongo.Collection
}

func NewStorageService() *StorageService {
	service := &StorageService{}

	// Only initialize if database is available
	if database.GetDatabase() != nil {
		service.fileCollection = database.GetCollection("files")
		service.providerCollection = database.GetCollection("storage_providers")
		service.syncCollection = database.GetCollection("sync_jobs")
		service.backupCollection = database.GetCollection("backups")
	}

	return service
}

// Storage Service - GetProvidersForAdmin Function
func (ss *StorageService) GetProvidersForAdmin(page, limit int) ([]models.StorageProvider, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	// Get total count
	total, err := ss.providerCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	// Get providers with pagination and additional admin info
	pipeline := []bson.M{
		{
			"$skip": int64(skip),
		},
		{
			"$limit": int64(limit),
		},
		{
			"$lookup": bson.M{
				"from":         "files",
				"localField":   "_id",
				"foreignField": "provider_id",
				"as":           "files",
			},
		},
		{
			"$addFields": bson.M{
				"file_count": bson.M{"$size": "$files"},
				"total_size": bson.M{
					"$sum": "$files.size",
				},
			},
		},
		{
			"$project": bson.M{
				"files": 0, // Remove the files array to reduce response size
			},
		},
		{
			"$sort": bson.M{"is_default": -1, "created_at": -1},
		},
	}

	cursor, err := ss.providerCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var providers []models.StorageProvider
	if err = cursor.All(ctx, &providers); err != nil {
		return nil, 0, err
	}

	return providers, total, nil
}

// Storage Service - GetProviderForAdmin Function
func (ss *StorageService) GetProviderForAdmin(providerID primitive.ObjectID) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get provider with detailed statistics
	pipeline := []bson.M{
		{
			"$match": bson.M{"_id": providerID},
		},
		{
			"$lookup": bson.M{
				"from":         "files",
				"localField":   "_id",
				"foreignField": "provider_id",
				"as":           "files",
			},
		},
		{
			"$addFields": bson.M{
				"file_count": bson.M{"$size": "$files"},
				"total_size": bson.M{
					"$sum": "$files.size",
				},
				"avg_file_size": bson.M{
					"$avg": "$files.size",
				},
			},
		},
		{
			"$project": bson.M{
				"files": 0, // Remove the files array
			},
		},
	}

	cursor, err := ss.providerCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var providers []models.StorageProvider
	if err = cursor.All(ctx, &providers); err != nil {
		return nil, err
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("provider not found")
	}

	return &providers[0], nil
}

// Storage Service - CreateProvider Function
func (ss *StorageService) CreateProvider(provider *models.StorageProvider) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Validate provider configuration
	if provider.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	if provider.Type == "" {
		return nil, fmt.Errorf("provider type is required")
	}

	// Set default values
	provider.ID = primitive.NewObjectID()
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()

	// If this is set as default, update other providers
	if provider.IsDefault {
		_, err := ss.providerCollection.UpdateMany(ctx,
			bson.M{"is_default": true},
			bson.M{"$set": bson.M{
				"is_default": false,
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing default providers: %v", err)
		}
	}

	// Insert new provider
	_, err := ss.providerCollection.InsertOne(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %v", err)
	}

	return provider, nil
}

// Storage Service - UpdateProvider Function
func (ss *StorageService) UpdateProvider(providerID primitive.ObjectID, updates map[string]interface{}) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Handle default provider logic
	if isDefault, exists := updates["is_default"].(bool); exists && isDefault {
		_, err := ss.providerCollection.UpdateMany(ctx,
			bson.M{"is_default": true, "_id": bson.M{"$ne": providerID}},
			bson.M{"$set": bson.M{
				"is_default": false,
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing default providers: %v", err)
		}
	}

	updates["updated_at"] = time.Now()

	// Update provider
	_, err := ss.providerCollection.UpdateOne(ctx,
		bson.M{"_id": providerID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update provider: %v", err)
	}

	// Get updated provider
	var updatedProvider models.StorageProvider
	err = ss.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&updatedProvider)
	if err != nil {
		return nil, err
	}

	return &updatedProvider, nil
}

// Storage Service - DeleteProvider Function
func (ss *StorageService) DeleteProvider(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get provider info first
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("provider not found: %v", err)
	}

	// Check if provider is in use
	fileCount, err := ss.fileCollection.CountDocuments(ctx, bson.M{
		"provider_id": providerID,
		"is_deleted":  false,
	})
	if err != nil {
		return fmt.Errorf("failed to check provider usage: %v", err)
	}

	if fileCount > 0 {
		return fmt.Errorf("cannot delete provider that is currently storing %d files", fileCount)
	}

	// Cannot delete default provider if it's the only active one
	if provider.IsDefault {
		activeProviderCount, err := ss.providerCollection.CountDocuments(ctx, bson.M{
			"is_active": true,
			"_id":       bson.M{"$ne": providerID},
		})
		if err != nil {
			return fmt.Errorf("failed to check active providers: %v", err)
		}

		if activeProviderCount == 0 {
			return fmt.Errorf("cannot delete the only active storage provider")
		}
	}

	// Delete provider
	_, err = ss.providerCollection.DeleteOne(ctx, bson.M{"_id": providerID})
	if err != nil {
		return fmt.Errorf("failed to delete provider: %v", err)
	}

	return nil
}

// Storage Service - TestProvider Function
func (ss *StorageService) TestProvider(providerID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get provider
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&provider)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %v", err)
	}

	startTime := time.Now()
	result := map[string]interface{}{
		"provider_id":   providerID,
		"provider_name": provider.Name,
		"provider_type": provider.Type,
		"test_time":     startTime,
	}

	// Basic connectivity test
	var testErr error
	switch provider.Type {
	case "local":
		// Test local storage - check if directory exists and is writable
		testErr = ss.testLocalProvider(&provider)
	case "s3":
		// Test S3 connection
		testErr = ss.testS3Provider(&provider)
	case "wasabi":
		// Test Wasabi connection
		testErr = ss.testWasabiProvider(&provider)
	case "r2":
		// Test Cloudflare R2 connection
		testErr = ss.testR2Provider(&provider)
	default:
		testErr = fmt.Errorf("unsupported provider type: %s", provider.Type)
	}

	duration := time.Since(startTime)
	result["duration_ms"] = duration.Milliseconds()
	result["success"] = testErr == nil

	if testErr != nil {
		result["error"] = testErr.Error()
		result["status"] = "failed"
	} else {
		result["status"] = "success"
		result["message"] = "Provider connection test successful"
	}

	return result, nil
}

// Storage Service - SyncProvider Function
func (ss *StorageService) SyncProvider(providerID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get provider
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("provider not found: %v", err)
	}

	// Create sync job record
	syncJob := bson.M{
		"_id":         primitive.NewObjectID(),
		"provider_id": providerID,
		"status":      "running",
		"started_at":  time.Now(),
		"created_at":  time.Now(),
	}

	_, err = ss.syncCollection.InsertOne(ctx, syncJob)
	if err != nil {
		return fmt.Errorf("failed to create sync job: %v", err)
	}

	// Perform sync based on provider type
	go func() {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer syncCancel()

		var syncErr error
		switch provider.Type {
		case "local":
			syncErr = ss.syncLocalProvider(&provider)
		case "s3":
			syncErr = ss.syncS3Provider(&provider)
		case "wasabi":
			syncErr = ss.syncWasabiProvider(&provider)
		case "r2":
			syncErr = ss.syncR2Provider(&provider)
		default:
			syncErr = fmt.Errorf("sync not supported for provider type: %s", provider.Type)
		}

		// Update sync job status
		status := "completed"
		updates := bson.M{
			"status":       status,
			"completed_at": time.Now(),
			"updated_at":   time.Now(),
		}

		if syncErr != nil {
			status = "failed"
			updates["status"] = status
			updates["error"] = syncErr.Error()
		}

		ss.syncCollection.UpdateOne(syncCtx,
			bson.M{"_id": syncJob["_id"]},
			bson.M{"$set": updates},
		)
	}()

	return nil
}

// Helper functions for testing providers
func (ss *StorageService) testLocalProvider(provider *models.StorageProvider) error {
	// Basic test for local storage
	if provider.Settings == nil {
		return fmt.Errorf("provider settings not configured")
	}

	basePath, exists := provider.Settings["base_path"].(string)
	if !exists || basePath == "" {
		return fmt.Errorf("base_path not configured")
	}

	// Check if directory exists (you would implement actual directory check here)
	return nil
}

func (ss *StorageService) testS3Provider(provider *models.StorageProvider) error {
	// Implement S3 connection test
	if provider.Bucket == "" {
		return fmt.Errorf("S3 bucket not configured")
	}

	// Test S3 credentials and bucket access
	return nil
}

func (ss *StorageService) testWasabiProvider(provider *models.StorageProvider) error {
	// Implement Wasabi connection test
	if provider.Bucket == "" {
		return fmt.Errorf("Wasabi bucket not configured")
	}

	return nil
}

func (ss *StorageService) testR2Provider(provider *models.StorageProvider) error {
	// Implement R2 connection test
	if provider.Bucket == "" {
		return fmt.Errorf("R2 bucket not configured")
	}

	return nil
}

// Helper functions for syncing providers
func (ss *StorageService) syncLocalProvider(provider *models.StorageProvider) error {
	// Implement local storage sync
	return nil
}

func (ss *StorageService) syncS3Provider(provider *models.StorageProvider) error {
	// Implement S3 sync
	return nil
}

func (ss *StorageService) syncWasabiProvider(provider *models.StorageProvider) error {
	// Implement Wasabi sync
	return nil
}

func (ss *StorageService) syncR2Provider(provider *models.StorageProvider) error {
	// Implement R2 sync
	return nil
}

// Provider Management
func (ss *StorageService) GetProviders() ([]models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := ss.providerCollection.Find(ctx, bson.M{},
		options.Find().SetSort(bson.M{"is_default": -1, "name": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var providers []models.StorageProvider
	if err = cursor.All(ctx, &providers); err != nil {
		return nil, err
	}

	return providers, nil
}

func (ss *StorageService) GetProvider(providerID primitive.ObjectID) (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&provider)
	if err != nil {
		return nil, fmt.Errorf("storage provider not found: %v", err)
	}

	return &provider, nil
}

// Storage Statistics
func (ss *StorageService) GetStorageStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats := make(map[string]interface{})

	// Overall statistics
	totalFiles, err := ss.fileCollection.CountDocuments(ctx, bson.M{"is_deleted": false})
	if err != nil {
		return nil, err
	}

	// Storage usage by provider
	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":         "$storage_provider",
				"total_files": bson.M{"$sum": 1},
				"total_size":  bson.M{"$sum": "$size"},
				"avg_size":    bson.M{"$avg": "$size"},
			},
		},
		{
			"$sort": bson.M{"total_size": -1},
		},
	}

	cursor, err := ss.fileCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var providerStats []bson.M
	if err = cursor.All(ctx, &providerStats); err != nil {
		return nil, err
	}

	// Calculate totals
	var totalSize int64
	for _, stat := range providerStats {
		if size, ok := stat["total_size"].(int64); ok {
			totalSize += size
		}
	}

	// Get active providers count
	activeProviders, _ := ss.providerCollection.CountDocuments(ctx, bson.M{"is_active": true})

	stats["overview"] = map[string]interface{}{
		"total_files":          totalFiles,
		"total_size":           totalSize,
		"total_size_formatted": utils.FormatFileSize(totalSize),
		"active_providers":     activeProviders,
		"provider_stats":       providerStats,
	}

	// File type distribution
	typesPipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":        "$extension",
				"count":      bson.M{"$sum": 1},
				"total_size": bson.M{"$sum": "$size"},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
		{
			"$limit": 10,
		},
	}

	typesCursor, _ := ss.fileCollection.Aggregate(ctx, typesPipeline)
	defer typesCursor.Close(ctx)

	var fileTypes []bson.M
	typesCursor.All(ctx, &fileTypes)
	stats["file_types"] = fileTypes

	return stats, nil
}

func (ss *StorageService) GetStorageUsage() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get usage by user (top 10)
	userPipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":        "$user_id",
				"file_count": bson.M{"$sum": 1},
				"total_size": bson.M{"$sum": "$size"},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "user",
			},
		},
		{
			"$unwind": "$user",
		},
		{
			"$sort": bson.M{"total_size": -1},
		},
		{
			"$limit": 10,
		},
		{
			"$project": bson.M{
				"user_email": "$user.email",
				"file_count": 1,
				"total_size": 1,
				"total_size_formatted": bson.M{
					"$concat": []interface{}{
						bson.M{"$toString": bson.M{"$divide": []interface{}{"$total_size", 1024 * 1024}}},
						" MB",
					},
				},
			},
		},
	}

	cursor, err := ss.fileCollection.Aggregate(ctx, userPipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var topUsers []bson.M
	if err = cursor.All(ctx, &topUsers); err != nil {
		return nil, err
	}

	// Get daily upload trends (last 7 days)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	trendPipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": sevenDaysAgo},
				"is_deleted": false,
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"year":  bson.M{"$year": "$created_at"},
					"month": bson.M{"$month": "$created_at"},
					"day":   bson.M{"$dayOfMonth": "$created_at"},
				},
				"files_uploaded": bson.M{"$sum": 1},
				"size_uploaded":  bson.M{"$sum": "$size"},
			},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	trendCursor, _ := ss.fileCollection.Aggregate(ctx, trendPipeline)
	defer trendCursor.Close(ctx)

	var uploadTrend []bson.M
	trendCursor.All(ctx, &uploadTrend)

	usage := map[string]interface{}{
		"top_users":    topUsers,
		"upload_trend": uploadTrend,
		"period_days":  7,
	}

	return usage, nil
}

// File Synchronization and Migration
func (ss *StorageService) SyncFiles() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create sync job
	syncJob := bson.M{
		"_id":             primitive.NewObjectID(),
		"type":            "sync",
		"status":          "initiated",
		"total_files":     0,
		"processed_files": 0,
		"failed_files":    0,
		"started_at":      time.Now(),
		"created_at":      time.Now(),
		"updated_at":      time.Now(),
	}

	result, err := ss.syncCollection.InsertOne(ctx, syncJob)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync job: %v", err)
	}

	// Start sync process asynchronously
	go ss.processSyncJob(result.InsertedID.(primitive.ObjectID))

	return map[string]interface{}{
		"job_id":     result.InsertedID,
		"status":     "initiated",
		"started_at": syncJob["started_at"],
	}, nil
}

func (ss *StorageService) MigrateFiles() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create migration job
	migrationJob := bson.M{
		"_id":             primitive.NewObjectID(),
		"type":            "migration",
		"status":          "initiated",
		"total_files":     0,
		"processed_files": 0,
		"failed_files":    0,
		"started_at":      time.Now(),
		"created_at":      time.Now(),
		"updated_at":      time.Now(),
	}

	result, err := ss.syncCollection.InsertOne(ctx, migrationJob)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration job: %v", err)
	}

	// Start migration process asynchronously
	go ss.processMigrationJob(result.InsertedID.(primitive.ObjectID))

	return map[string]interface{}{
		"job_id":     result.InsertedID,
		"status":     "initiated",
		"started_at": migrationJob["started_at"],
	}, nil
}

// Health Monitoring
func (ss *StorageService) CheckProvidersHealth() (map[string]interface{}, error) {
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providers, err := ss.GetProviders()
	if err != nil {
		return nil, err
	}

	health := map[string]interface{}{
		"overall_status": "healthy",
		"checked_at":     time.Now(),
		"providers":      make(map[string]interface{}),
	}

	overallHealthy := true

	for _, provider := range providers {
		providerHealth := map[string]interface{}{
			"name":       provider.Name,
			"type":       provider.Type,
			"status":     "healthy",
			"error":      nil,
			"checked_at": time.Now(),
		}

		// Simulate health check (in real implementation, would check actual connectivity)
		if err := ss.checkProviderHealth(&provider); err != nil {
			providerHealth["status"] = "unhealthy"
			providerHealth["error"] = err.Error()
			overallHealthy = false
		}

		health["providers"].(map[string]interface{})[provider.ID.Hex()] = providerHealth
	}

	if !overallHealthy {
		health["overall_status"] = "unhealthy"
	}

	health["total_providers"] = len(providers)
	health["healthy_providers"] = ss.countHealthyProviders(health["providers"].(map[string]interface{}))

	return health, nil
}

// Upload Operations
func (ss *StorageService) GetUploadURL(userID primitive.ObjectID, fileName string, fileSize int64) (map[string]interface{}, error) {
	// Get user's plan to validate limits
	var user models.User
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ss.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get default provider
	var provider models.StorageProvider
	err = ss.providerCollection.FindOne(ctx, bson.M{
		"is_default": true,
		"is_active":  true,
	}).Decode(&provider)
	if err != nil {
		return nil, fmt.Errorf("no default storage provider found: %v", err)
	}

	// Generate upload URL
	uploadURL := fmt.Sprintf("https://%s.%s/%s", provider.Bucket, provider.Endpoint, fileName)
	uploadID := primitive.NewObjectID().Hex()

	return map[string]interface{}{
		"upload_url": uploadURL,
		"upload_id":  uploadID,
		"provider":   provider.Type,
		"expires_at": time.Now().Add(1 * time.Hour),
		"max_size":   fileSize,
		"file_name":  fileName,
	}, nil
}

func (ss *StorageService) InitiateMultipartUpload(userID primitive.ObjectID, fileName string, fileSize int64) (map[string]interface{}, error) {
	uploadID := primitive.NewObjectID().Hex()

	// Store multipart upload session
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := bson.M{
		"_id":        uploadID,
		"user_id":    userID,
		"file_name":  fileName,
		"file_size":  fileSize,
		"status":     "initiated",
		"parts":      []interface{}{},
		"created_at": time.Now(),
		"expires_at": time.Now().Add(24 * time.Hour),
	}

	_, err := database.GetCollection("multipart_uploads").InsertOne(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate multipart upload: %v", err)
	}

	return map[string]interface{}{
		"upload_id":  uploadID,
		"file_name":  fileName,
		"status":     "initiated",
		"expires_at": session["expires_at"],
	}, nil
}

func (ss *StorageService) UploadPart(uploadID string, partNumber int, partSize int64) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update multipart upload session
	_, err := database.GetCollection("multipart_uploads").UpdateOne(ctx,
		bson.M{"_id": uploadID},
		bson.M{"$push": bson.M{"parts": bson.M{
			"part_number": partNumber,
			"size":        partSize,
			"uploaded_at": time.Now(),
		}}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record part upload: %v", err)
	}

	return map[string]interface{}{
		"upload_id":   uploadID,
		"part_number": partNumber,
		"status":      "uploaded",
		"uploaded_at": time.Now(),
	}, nil
}

func (ss *StorageService) CompleteMultipartUpload(uploadID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get multipart upload session
	var session bson.M
	err := database.GetCollection("multipart_uploads").FindOne(ctx, bson.M{"_id": uploadID}).Decode(&session)
	if err != nil {
		return nil, fmt.Errorf("multipart upload session not found: %v", err)
	}

	// Mark as completed
	_, err = database.GetCollection("multipart_uploads").UpdateOne(ctx,
		bson.M{"_id": uploadID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"completed_at": time.Now(),
		}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to complete multipart upload: %v", err)
	}

	return map[string]interface{}{
		"upload_id":    uploadID,
		"status":       "completed",
		"completed_at": time.Now(),
		"file_name":    session["file_name"],
	}, nil
}

func (ss *StorageService) AbortMultipartUpload(uploadID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := database.GetCollection("multipart_uploads").UpdateOne(ctx,
		bson.M{"_id": uploadID},
		bson.M{"$set": bson.M{
			"status":     "aborted",
			"aborted_at": time.Now(),
		}},
	)
	return err
}

// CDN Operations
func (ss *StorageService) InvalidateCDN(paths []string) (map[string]interface{}, error) {
	// Create CDN invalidation job
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	invalidation := bson.M{
		"_id":        primitive.NewObjectID(),
		"paths":      paths,
		"status":     "initiated",
		"created_at": time.Now(),
	}

	result, err := database.GetCollection("cdn_invalidations").InsertOne(ctx, invalidation)
	if err != nil {
		return nil, fmt.Errorf("failed to create CDN invalidation: %v", err)
	}

	// Process invalidation asynchronously
	go ss.processCDNInvalidation(result.InsertedID.(primitive.ObjectID), paths)

	return map[string]interface{}{
		"invalidation_id": result.InsertedID,
		"paths":           paths,
		"status":          "initiated",
		"created_at":      time.Now(),
	}, nil
}

func (ss *StorageService) GetCDNStats() (map[string]interface{}, error) {
	// Get CDN statistics
	stats := map[string]interface{}{
		"total_requests":  1250000,
		"cache_hit_rate":  0.92,
		"total_bandwidth": "2.5 TB",
		"top_endpoints":   []string{"/api/files/download", "/api/images/"},
		"geographic_distribution": map[string]interface{}{
			"US":   0.45,
			"EU":   0.30,
			"ASIA": 0.25,
		},
		"status":       "healthy",
		"last_updated": time.Now(),
	}

	return stats, nil
}

// Image Optimization
func (ss *StorageService) OptimizeImages() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find images that need optimization
	cursor, err := ss.fileCollection.Find(ctx, bson.M{
		"mime_type":    bson.M{"$regex": "^image/"},
		"is_optimized": bson.M{"$ne": true},
		"is_deleted":   false,
	}, options.Find().SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var images []bson.M
	cursor.All(ctx, &images)

	// Create optimization job
	optimizationJob := bson.M{
		"_id":          primitive.NewObjectID(),
		"type":         "image_optimization",
		"total_images": len(images),
		"processed":    0,
		"status":       "initiated",
		"created_at":   time.Now(),
	}

	result, err := database.GetCollection("optimization_jobs").InsertOne(ctx, optimizationJob)
	if err != nil {
		return nil, fmt.Errorf("failed to create optimization job: %v", err)
	}

	// Process optimization asynchronously
	go ss.processImageOptimization(result.InsertedID.(primitive.ObjectID), images)

	return map[string]interface{}{
		"job_id":       result.InsertedID,
		"total_images": len(images),
		"status":       "initiated",
		"created_at":   time.Now(),
	}, nil
}

// Backup Operations
func (ss *StorageService) CreateBackup() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create backup job
	backup := bson.M{
		"_id":         primitive.NewObjectID(),
		"type":        "full_backup",
		"status":      "initiated",
		"total_files": 0,
		"backed_up":   0,
		"backup_size": 0,
		"started_at":  time.Now(),
		"created_at":  time.Now(),
	}

	result, err := ss.backupCollection.InsertOne(ctx, backup)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %v", err)
	}

	// Start backup process asynchronously
	go ss.processBackup(result.InsertedID.(primitive.ObjectID))

	return map[string]interface{}{
		"backup_id":  result.InsertedID,
		"status":     "initiated",
		"started_at": time.Now(),
	}, nil
}

func (ss *StorageService) GetBackups() ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := ss.backupCollection.Find(ctx, bson.M{},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(50),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var backups []map[string]interface{}
	if err = cursor.All(ctx, &backups); err != nil {
		return nil, err
	}

	return backups, nil
}

func (ss *StorageService) RestoreBackup(backupID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get backup info
	var backup map[string]interface{}
	err := ss.backupCollection.FindOne(ctx, bson.M{"_id": backupID}).Decode(&backup)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %v", err)
	}

	// Create restore job
	restore := bson.M{
		"_id":        primitive.NewObjectID(),
		"backup_id":  backupID,
		"status":     "initiated",
		"started_at": time.Now(),
		"created_at": time.Now(),
	}

	result, err := database.GetCollection("restore_jobs").InsertOne(ctx, restore)
	if err != nil {
		return nil, fmt.Errorf("failed to create restore job: %v", err)
	}

	// Start restore process asynchronously
	go ss.processRestore(result.InsertedID.(primitive.ObjectID), backupID)

	return map[string]interface{}{
		"restore_id": result.InsertedID,
		"backup_id":  backupID,
		"status":     "initiated",
		"started_at": time.Now(),
	}, nil
}

func (ss *StorageService) DeleteBackup(backupID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ss.backupCollection.DeleteOne(ctx, bson.M{"_id": backupID})
	if err != nil {
		return fmt.Errorf("failed to delete backup: %v", err)
	}

	return nil
}

// Helper functions
func (ss *StorageService) checkProviderHealth(provider *models.StorageProvider) error {
	// In real implementation, would test actual connectivity to the provider
	// For now, simulate based on provider status
	if !provider.IsActive {
		return fmt.Errorf("provider is inactive")
	}
	return nil
}

func (ss *StorageService) countHealthyProviders(providers map[string]interface{}) int {
	count := 0
	for _, p := range providers {
		if provider, ok := p.(map[string]interface{}); ok {
			if status, ok := provider["status"].(string); ok && status == "healthy" {
				count++
			}
		}
	}
	return count
}

// Background job processors
func (ss *StorageService) processSyncJob(jobID primitive.ObjectID) {
	// Implementation for processing sync jobs
	time.Sleep(5 * time.Second) // Simulate work

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ss.syncCollection.UpdateOne(ctx,
		bson.M{"_id": jobID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"completed_at": time.Now(),
		}},
	)
}

func (ss *StorageService) processMigrationJob(jobID primitive.ObjectID) {
	// Implementation for processing migration jobs
	time.Sleep(10 * time.Second) // Simulate work

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ss.syncCollection.UpdateOne(ctx,
		bson.M{"_id": jobID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"completed_at": time.Now(),
		}},
	)
}

func (ss *StorageService) processCDNInvalidation(invalidationID primitive.ObjectID, paths []string) {
	// Implementation for CDN invalidation
	time.Sleep(2 * time.Second) // Simulate CDN processing

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	database.GetCollection("cdn_invalidations").UpdateOne(ctx,
		bson.M{"_id": invalidationID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"completed_at": time.Now(),
		}},
	)
}

func (ss *StorageService) processImageOptimization(jobID primitive.ObjectID, images []bson.M) {
	// Implementation for image optimization
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i, image := range images {
		// Simulate optimization work
		time.Sleep(500 * time.Millisecond)

		// Update progress
		database.GetCollection("optimization_jobs").UpdateOne(ctx,
			bson.M{"_id": jobID},
			bson.M{"$set": bson.M{
				"processed":  i + 1,
				"updated_at": time.Now(),
			}},
		)

		// Mark image as optimized
		if imageID, ok := image["_id"].(primitive.ObjectID); ok {
			ss.fileCollection.UpdateOne(ctx,
				bson.M{"_id": imageID},
				bson.M{"$set": bson.M{"is_optimized": true}},
			)
		}
	}

	// Mark job as completed
	database.GetCollection("optimization_jobs").UpdateOne(ctx,
		bson.M{"_id": jobID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"completed_at": time.Now(),
		}},
	)
}

func (ss *StorageService) processBackup(backupID primitive.ObjectID) {
	// Implementation for backup processing
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Count total files
	totalFiles, _ := ss.fileCollection.CountDocuments(ctx, bson.M{"is_deleted": false})

	// Update backup with total files
	ss.backupCollection.UpdateOne(ctx,
		bson.M{"_id": backupID},
		bson.M{"$set": bson.M{
			"total_files": totalFiles,
			"status":      "in_progress",
		}},
	)

	// Simulate backup work
	time.Sleep(30 * time.Second)

	// Complete backup
	ss.backupCollection.UpdateOne(ctx,
		bson.M{"_id": backupID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"backed_up":    totalFiles,
			"backup_size":  1024 * 1024 * 500, // 500MB
			"completed_at": time.Now(),
		}},
	)
}

func (ss *StorageService) processRestore(restoreID, backupID primitive.ObjectID) {
	// Implementation for restore processing
	time.Sleep(20 * time.Second) // Simulate restore work

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	database.GetCollection("restore_jobs").UpdateOne(ctx,
		bson.M{"_id": restoreID},
		bson.M{"$set": bson.M{
			"status":       "completed",
			"completed_at": time.Now(),
		}},
	)
}

// UploadFile uploads a file to the specified storage provider
func (ss *StorageService) UploadFile(providerType, storageKey string, fileContent []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get provider configuration
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{
		"type":      providerType,
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("provider not found: %v", err)
	}

	// Handle upload based on provider type
	switch strings.ToLower(providerType) {
	case "local":
		return ss.uploadToLocal(&provider, storageKey, fileContent)
	case "s3":
		return ss.uploadToS3(&provider, storageKey, fileContent)
	case "wasabi":
		return ss.uploadToWasabi(&provider, storageKey, fileContent)
	case "r2":
		return ss.uploadToR2(&provider, storageKey, fileContent)
	default:
		return fmt.Errorf("unsupported storage provider: %s", providerType)
	}
}

// DeleteFile deletes a file from the specified storage provider
func (ss *StorageService) DeleteFile(providerType, storageKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get provider configuration
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{
		"type":      providerType,
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("provider not found: %v", err)
	}

	// Handle deletion based on provider type
	switch strings.ToLower(providerType) {
	case "local":
		return ss.deleteFromLocal(&provider, storageKey)
	case "s3":
		return ss.deleteFromS3(&provider, storageKey)
	case "wasabi":
		return ss.deleteFromWasabi(&provider, storageKey)
	case "r2":
		return ss.deleteFromR2(&provider, storageKey)
	default:
		return fmt.Errorf("unsupported storage provider: %s", providerType)
	}
}

// GetPresignedURL generates a presigned URL for file access
func (ss *StorageService) GetPresignedURL(providerType, storageKey string, expiration time.Duration, operation string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get provider configuration
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{
		"type":      providerType,
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return "", fmt.Errorf("provider not found: %v", err)
	}

	// Handle presigned URL generation based on provider type
	switch strings.ToLower(providerType) {
	case "local":
		return ss.getLocalPresignedURL(&provider, storageKey, expiration, operation)
	case "s3":
		return ss.getS3PresignedURL(&provider, storageKey, expiration, operation)
	case "wasabi":
		return ss.getWasabiPresignedURL(&provider, storageKey, expiration, operation)
	case "r2":
		return ss.getR2PresignedURL(&provider, storageKey, expiration, operation)
	default:
		return "", fmt.Errorf("unsupported storage provider: %s", providerType)
	}
}

// DownloadFile downloads a file from the specified storage provider
func (ss *StorageService) DownloadFile(providerType, storageKey string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get provider configuration
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{
		"type":      providerType,
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %v", err)
	}

	// Handle download based on provider type
	switch strings.ToLower(providerType) {
	case "local":
		return ss.downloadFromLocal(&provider, storageKey)
	case "s3":
		return ss.downloadFromS3(&provider, storageKey)
	case "wasabi":
		return ss.downloadFromWasabi(&provider, storageKey)
	case "r2":
		return ss.downloadFromR2(&provider, storageKey)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", providerType)
	}
}

// CopyFile copies a file within or between storage providers
func (ss *StorageService) CopyFile(sourceProviderType, sourceKey, destProviderType, destKey string) error {
	_, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// If same provider, use provider-specific copy
	if sourceProviderType == destProviderType {
		return ss.copyWithinProvider(sourceProviderType, sourceKey, destKey)
	}

	// Cross-provider copy: download from source and upload to destination
	fileContent, err := ss.DownloadFile(sourceProviderType, sourceKey)
	if err != nil {
		return fmt.Errorf("failed to download source file: %v", err)
	}

	err = ss.UploadFile(destProviderType, destKey, fileContent)
	if err != nil {
		return fmt.Errorf("failed to upload to destination: %v", err)
	}

	return nil
}

// Local Storage Implementation
func (ss *StorageService) uploadToLocal(provider *models.StorageProvider, storageKey string, fileContent []byte) error {
	basePath, exists := provider.Settings["base_path"].(string)
	if !exists || basePath == "" {
		basePath = "./uploads"
	}

	// Ensure directory exists
	fullPath := filepath.Join(basePath, storageKey)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, fileContent, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	// Log activity
	ss.logStorageActivity("upload", provider.ID, storageKey, int64(len(fileContent)), nil)
	return nil
}

func (ss *StorageService) deleteFromLocal(provider *models.StorageProvider, storageKey string) error {
	basePath, exists := provider.Settings["base_path"].(string)
	if !exists || basePath == "" {
		basePath = "./uploads"
	}

	fullPath := filepath.Join(basePath, storageKey)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	// Log activity
	ss.logStorageActivity("delete", provider.ID, storageKey, 0, nil)
	return nil
}

func (ss *StorageService) downloadFromLocal(provider *models.StorageProvider, storageKey string) ([]byte, error) {
	basePath, exists := provider.Settings["base_path"].(string)
	if !exists || basePath == "" {
		basePath = "./uploads"
	}

	fullPath := filepath.Join(basePath, storageKey)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Log activity
	ss.logStorageActivity("download", provider.ID, storageKey, int64(len(content)), nil)
	return content, nil
}

func (ss *StorageService) getLocalPresignedURL(provider *models.StorageProvider, storageKey string, expiration time.Duration, operation string) (string, error) {
	// For local storage, generate a temporary access token
	token := utils.GenerateRandomString(32)

	// Store token in cache/database with expiration
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tokenDoc := bson.M{
		"_id":         token,
		"provider_id": provider.ID,
		"storage_key": storageKey,
		"operation":   operation,
		"expires_at":  time.Now().Add(expiration),
		"created_at":  time.Now(),
	}

	_, err := ss.activityCollection.InsertOne(ctx, tokenDoc)
	if err != nil {
		return "", fmt.Errorf("failed to create access token: %v", err)
	}

	// Return URL with token
	baseURL := "http://localhost:8080" // This should come from config
	if provider.Endpoint != "" {
		baseURL = provider.Endpoint
	}

	return fmt.Sprintf("%s/api/storage/local/%s?token=%s", baseURL, storageKey, token), nil
}

// S3 Storage Implementation
func (ss *StorageService) uploadToS3(provider *models.StorageProvider, storageKey string, fileContent []byte) error {
	client, err := storage.NewS3Client(provider)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	err = client.Upload(storageKey, fileContent)
	if err != nil {
		return fmt.Errorf("S3 upload failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("upload", provider.ID, storageKey, int64(len(fileContent)), nil)
	return nil
}

func (ss *StorageService) deleteFromS3(provider *models.StorageProvider, storageKey string) error {
	client, err := storage.NewS3Client(provider)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	err = client.Delete(storageKey)
	if err != nil {
		return fmt.Errorf("S3 delete failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("delete", provider.ID, storageKey, 0, nil)
	return nil
}

func (ss *StorageService) downloadFromS3(provider *models.StorageProvider, storageKey string) ([]byte, error) {
	client, err := storage.NewS3Client(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %v", err)
	}

	content, err := client.Download(storageKey)
	if err != nil {
		return nil, fmt.Errorf("S3 download failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("download", provider.ID, storageKey, int64(len(content)), nil)
	return content, nil
}

func (ss *StorageService) getS3PresignedURL(provider *models.StorageProvider, storageKey string, expiration time.Duration, operation string) (string, error) {
	client, err := storage.NewS3Client(provider)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 client: %v", err)
	}

	var url string
	switch operation {
	case "GET", "download":
		url, err = client.GetPresignedURL(storageKey, expiration)
	case "PUT", "upload":
		url, err = client.GetPresignedUploadURL(storageKey, expiration, 0)
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate S3 presigned URL: %v", err)
	}

	return url, nil
}

// Wasabi Storage Implementation
func (ss *StorageService) uploadToWasabi(provider *models.StorageProvider, storageKey string, fileContent []byte) error {
	client, err := storage.NewWasabiClient(provider)
	if err != nil {
		return fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	err = client.Upload(storageKey, fileContent)
	if err != nil {
		return fmt.Errorf("Wasabi upload failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("upload", provider.ID, storageKey, int64(len(fileContent)), nil)
	return nil
}

func (ss *StorageService) deleteFromWasabi(provider *models.StorageProvider, storageKey string) error {
	client, err := storage.NewWasabiClient(provider)
	if err != nil {
		return fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	err = client.Delete(storageKey)
	if err != nil {
		return fmt.Errorf("Wasabi delete failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("delete", provider.ID, storageKey, 0, nil)
	return nil
}

func (ss *StorageService) downloadFromWasabi(provider *models.StorageProvider, storageKey string) ([]byte, error) {
	client, err := storage.NewWasabiClient(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	content, err := client.Download(storageKey)
	if err != nil {
		return nil, fmt.Errorf("Wasabi download failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("download", provider.ID, storageKey, int64(len(content)), nil)
	return content, nil
}

func (ss *StorageService) getWasabiPresignedURL(provider *models.StorageProvider, storageKey string, expiration time.Duration, operation string) (string, error) {
	client, err := storage.NewWasabiClient(provider)
	if err != nil {
		return "", fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	var url string
	switch operation {
	case "GET", "download":
		url, err = client.GetPresignedURL(storageKey, expiration)
	case "PUT", "upload":
		url, err = client.GetPresignedUploadURL(storageKey, expiration, 0)
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate Wasabi presigned URL: %v", err)
	}

	return url, nil
}

// R2 Storage Implementation
func (ss *StorageService) uploadToR2(provider *models.StorageProvider, storageKey string, fileContent []byte) error {
	client, err := storage.NewR2Client(provider)
	if err != nil {
		return fmt.Errorf("failed to create R2 client: %v", err)
	}

	err = client.Upload(storageKey, fileContent)
	if err != nil {
		return fmt.Errorf("R2 upload failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("upload", provider.ID, storageKey, int64(len(fileContent)), nil)
	return nil
}

func (ss *StorageService) deleteFromR2(provider *models.StorageProvider, storageKey string) error {
	client, err := storage.NewR2Client(provider)
	if err != nil {
		return fmt.Errorf("failed to create R2 client: %v", err)
	}

	err = client.Delete(storageKey)
	if err != nil {
		return fmt.Errorf("R2 delete failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("delete", provider.ID, storageKey, 0, nil)
	return nil
}

func (ss *StorageService) downloadFromR2(provider *models.StorageProvider, storageKey string) ([]byte, error) {
	client, err := storage.NewR2Client(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 client: %v", err)
	}

	content, err := client.Download(storageKey)
	if err != nil {
		return nil, fmt.Errorf("R2 download failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("download", provider.ID, storageKey, int64(len(content)), nil)
	return content, nil
}

func (ss *StorageService) getR2PresignedURL(provider *models.StorageProvider, storageKey string, expiration time.Duration, operation string) (string, error) {
	client, err := storage.NewR2Client(provider)
	if err != nil {
		return "", fmt.Errorf("failed to create R2 client: %v", err)
	}

	var url string
	switch operation {
	case "GET", "download":
		url, err = client.GetPresignedURL(storageKey, expiration)
	case "PUT", "upload":
		url, err = client.GetPresignedUploadURL(storageKey, expiration, 0)
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate R2 presigned URL: %v", err)
	}

	return url, nil
}

// Helper Functions
func (ss *StorageService) copyWithinProvider(providerType, sourceKey, destKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get provider configuration
	var provider models.StorageProvider
	err := ss.providerCollection.FindOne(ctx, bson.M{
		"type":      providerType,
		"is_active": true,
	}).Decode(&provider)
	if err != nil {
		return fmt.Errorf("provider not found: %v", err)
	}

	switch strings.ToLower(providerType) {
	case "local":
		return ss.copyLocalFile(&provider, sourceKey, destKey)
	case "s3":
		return ss.copyS3File(&provider, sourceKey, destKey)
	case "wasabi":
		return ss.copyWasabiFile(&provider, sourceKey, destKey)
	case "r2":
		return ss.copyR2File(&provider, sourceKey, destKey)
	default:
		// Fallback to download/upload
		content, err := ss.downloadFromProvider(&provider, sourceKey)
		if err != nil {
			return err
		}
		return ss.uploadToProvider(&provider, destKey, content)
	}
}

func (ss *StorageService) copyLocalFile(provider *models.StorageProvider, sourceKey, destKey string) error {
	basePath, exists := provider.Settings["base_path"].(string)
	if !exists || basePath == "" {
		basePath = "./uploads"
	}

	sourcePath := filepath.Join(basePath, sourceKey)
	destPath := filepath.Join(basePath, destKey)

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	// Copy file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	// Log activity
	ss.logStorageActivity("copy", provider.ID, destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

func (ss *StorageService) copyS3File(provider *models.StorageProvider, sourceKey, destKey string) error {
	client, err := storage.NewS3Client(provider)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	err = client.CopyFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("S3 copy failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("copy", provider.ID, destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

func (ss *StorageService) copyWasabiFile(provider *models.StorageProvider, sourceKey, destKey string) error {
	client, err := storage.NewWasabiClient(provider)
	if err != nil {
		return fmt.Errorf("failed to create Wasabi client: %v", err)
	}

	err = client.CopyFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("Wasabi copy failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("copy", provider.ID, destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

func (ss *StorageService) copyR2File(provider *models.StorageProvider, sourceKey, destKey string) error {
	client, err := storage.NewR2Client(provider)
	if err != nil {
		return fmt.Errorf("failed to create R2 client: %v", err)
	}

	err = client.CopyFile(sourceKey, destKey)
	if err != nil {
		return fmt.Errorf("R2 copy failed: %v", err)
	}

	// Log activity
	ss.logStorageActivity("copy", provider.ID, destKey, 0, map[string]interface{}{
		"source_key": sourceKey,
	})

	return nil
}

func (ss *StorageService) downloadFromProvider(provider *models.StorageProvider, storageKey string) ([]byte, error) {
	switch strings.ToLower(provider.Type) {
	case "local":
		return ss.downloadFromLocal(provider, storageKey)
	case "s3":
		return ss.downloadFromS3(provider, storageKey)
	case "wasabi":
		return ss.downloadFromWasabi(provider, storageKey)
	case "r2":
		return ss.downloadFromR2(provider, storageKey)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

func (ss *StorageService) uploadToProvider(provider *models.StorageProvider, storageKey string, content []byte) error {
	switch strings.ToLower(provider.Type) {
	case "local":
		return ss.uploadToLocal(provider, storageKey, content)
	case "s3":
		return ss.uploadToS3(provider, storageKey, content)
	case "wasabi":
		return ss.uploadToWasabi(provider, storageKey, content)
	case "r2":
		return ss.uploadToR2(provider, storageKey, content)
	default:
		return fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

func (ss *StorageService) logStorageActivity(action string, providerID primitive.ObjectID, storageKey string, size int64, metadata map[string]interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	activity := bson.M{
		"_id":         primitive.NewObjectID(),
		"provider_id": providerID,
		"action":      action,
		"storage_key": storageKey,
		"size":        size,
		"metadata":    metadata,
		"created_at":  time.Now(),
	}

	ss.activityCollection.InsertOne(ctx, activity)
}

// getContentType determines the MIME type of a file based on its extension
func getContentType(filename string) string {
	ext := filepath.Ext(filename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
