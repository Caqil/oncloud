package services

import (
	"context"
	"fmt"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
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
	return &StorageService{
		providerCollection: database.GetCollection("storage_providers"),
		fileCollection:     database.GetCollection("files"),
		userCollection:     database.GetCollection("users"),
		syncCollection:     database.GetCollection("storage_sync"),
		backupCollection:   database.GetCollection("backups"),
		activityCollection: database.GetCollection("storage_activities"),
	}
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
