package controllers

import (
	"oncloud/services"
	"oncloud/utils"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StorageController struct {
	storageService *services.StorageService
}

func NewStorageController() *StorageController {
	return &StorageController{
		storageService: services.NewStorageService(),
	}
}

// GetProviders returns list of available storage providers
func (sc *StorageController) GetProviders(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	providers, err := sc.storageService.GetAvailableProviders(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get storage providers")
		return
	}

	utils.SuccessResponse(c, "Storage providers retrieved successfully", providers)
}

// GetProvider returns a specific storage provider
func (sc *StorageController) GetProvider(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	providerID := c.Param("id")
	if !utils.IsValidObjectID(providerID) {
		utils.BadRequestResponse(c, "Invalid provider ID")
		return
	}

	objID, _ := utils.StringToObjectID(providerID)
	provider, err := sc.storageService.GetProvider(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "Storage provider not found")
		return
	}

	utils.SuccessResponse(c, "Storage provider retrieved successfully", provider)
}

// GetStorageStats returns storage statistics
func (sc *StorageController) GetStorageStats(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	stats, err := sc.storageService.GetUserStorageStats(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get storage stats")
		return
	}

	utils.SuccessResponse(c, "Storage stats retrieved successfully", stats)
}

// GetStorageUsage returns detailed storage usage
func (sc *StorageController) GetStorageUsage(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	period := c.DefaultQuery("period", "30")     // days
	groupBy := c.DefaultQuery("group_by", "day") // day, week, month

	usage, err := sc.storageService.GetStorageUsage(user.ID, period, groupBy)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get storage usage")
		return
	}

	utils.SuccessResponse(c, "Storage usage retrieved successfully", usage)
}

// SyncFiles synchronizes files across storage providers
func (sc *StorageController) SyncFiles(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		SourceProviderID string   `json:"source_provider_id" validate:"required"`
		TargetProviderID string   `json:"target_provider_id" validate:"required"`
		FileIDs          []string `json:"file_ids"`
		SyncAll          bool     `json:"sync_all"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if !utils.IsValidObjectID(req.SourceProviderID) || !utils.IsValidObjectID(req.TargetProviderID) {
		utils.BadRequestResponse(c, "Invalid provider IDs")
		return
	}

	sourceID, _ := utils.StringToObjectID(req.SourceProviderID)
	targetID, _ := utils.StringToObjectID(req.TargetProviderID)

	var fileObjIDs []primitive.ObjectID
	if !req.SyncAll {
		for _, id := range req.FileIDs {
			if !utils.IsValidObjectID(id) {
				utils.BadRequestResponse(c, "Invalid file ID: "+id)
				return
			}
			objID, _ := utils.StringToObjectID(id)
			fileObjIDs = append(fileObjIDs, objID)
		}
	}

	syncResult, err := sc.storageService.SyncFiles(user.ID, sourceID, targetID, fileObjIDs, req.SyncAll)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to sync files")
		return
	}

	utils.SuccessResponse(c, "File sync initiated successfully", syncResult)
}

// MigrateFiles migrates files between storage providers
func (sc *StorageController) MigrateFiles(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		SourceProviderID string   `json:"source_provider_id" validate:"required"`
		TargetProviderID string   `json:"target_provider_id" validate:"required"`
		FileIDs          []string `json:"file_ids"`
		MigrateAll       bool     `json:"migrate_all"`
		DeleteSource     bool     `json:"delete_source"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if !utils.IsValidObjectID(req.SourceProviderID) || !utils.IsValidObjectID(req.TargetProviderID) {
		utils.BadRequestResponse(c, "Invalid provider IDs")
		return
	}

	sourceID, _ := utils.StringToObjectID(req.SourceProviderID)
	targetID, _ := utils.StringToObjectID(req.TargetProviderID)

	var fileObjIDs []primitive.ObjectID
	if !req.MigrateAll {
		for _, id := range req.FileIDs {
			if !utils.IsValidObjectID(id) {
				utils.BadRequestResponse(c, "Invalid file ID: "+id)
				return
			}
			objID, _ := utils.StringToObjectID(id)
			fileObjIDs = append(fileObjIDs, objID)
		}
	}

	migrationResult, err := sc.storageService.MigrateFiles(user.ID, sourceID, targetID, fileObjIDs, req.MigrateAll, req.DeleteSource)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to migrate files")
		return
	}

	utils.SuccessResponse(c, "File migration initiated successfully", migrationResult)
}

// CheckProvidersHealth checks health of all storage providers
func (sc *StorageController) CheckProvidersHealth(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	healthStatus, err := sc.storageService.CheckProvidersHealth(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to check providers health")
		return
	}

	utils.SuccessResponse(c, "Providers health checked successfully", healthStatus)
}

// Upload operations
func (sc *StorageController) GetUploadURL(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileName      string `json:"file_name" validate:"required"`
		FileSize      int64  `json:"file_size" validate:"required"`
		ContentType   string `json:"content_type"`
		FolderID      string `json:"folder_id"`
		ProviderID    string `json:"provider_id"`
		ExpiryMinutes int    `json:"expiry_minutes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if req.ExpiryMinutes == 0 {
		req.ExpiryMinutes = 60 // Default 1 hour
	}

	if req.ProviderID != "" {
		var providerObjID primitive.ObjectID
		if !utils.IsValidObjectID(req.ProviderID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(req.ProviderID)
	}

	uploadURL, err := sc.storageService.GetPresignedUploadURL(
		user.ID,
		req.FileName,
		req.FileSize,
		req.ContentType,
		req.FolderID,
		providerObjID,
		time.Duration(req.ExpiryMinutes)*time.Minute,
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate upload URL")
		return
	}

	utils.SuccessResponse(c, "Upload URL generated successfully", gin.H{
		"upload_url": uploadURL.URL,
		"fields":     uploadURL.Fields,
		"expires_at": uploadURL.ExpiresAt,
	})
}

// Multipart upload operations
func (sc *StorageController) InitiateMultipartUpload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileName    string `json:"file_name" validate:"required"`
		FileSize    int64  `json:"file_size" validate:"required"`
		ContentType string `json:"content_type"`
		FolderID    string `json:"folder_id"`
		ProviderID  string `json:"provider_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	var providerObjID primitive.ObjectID
	if req.ProviderID != "" {
		if !utils.IsValidObjectID(req.ProviderID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(req.ProviderID)
	}

	upload, err := sc.storageService.InitiateMultipartUpload(user.ID, req.FileName, req.FileSize)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to initiate multipart upload")
		return
	}

	utils.CreatedResponse(c, "Multipart upload initiated successfully", upload)
}

func (sc *StorageController) UploadPart(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	uploadID := c.Param("upload_id")
	partNumberStr := c.Param("part_number")

	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid part number")
		return
	}

	// Get part data from request body
	partData, err := c.GetRawData()
	if err != nil {
		utils.BadRequestResponse(c, "Failed to read part data")
		return
	}

	part, err := sc.storageService.UploadPart(uploadID, partNumber, int64(len(partData)))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to upload part")
		return
	}

	utils.SuccessResponse(c, "Part uploaded successfully", part)
}

func (sc *StorageController) CompleteMultipartUpload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	uploadID := c.Param("upload_id")

	var req struct {
		Parts []struct {
			PartNumber int    `json:"part_number"`
			ETag       string `json:"etag"`
		} `json:"parts" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	file, err := sc.storageService.CompleteMultipartUpload(user.ID, uploadID, req.Parts)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to complete multipart upload")
		return
	}

	utils.SuccessResponse(c, "Multipart upload completed successfully", file)
}

func (sc *StorageController) AbortMultipartUpload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	uploadID := c.Param("upload_id")

	err := sc.storageService.AbortMultipartUpload(uploadID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to abort multipart upload")
		return
	}

	utils.SuccessResponse(c, "Multipart upload aborted successfully", nil)
}

// CDN and optimization
func (sc *StorageController) InvalidateCDN(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Paths      []string `json:"paths" validate:"required"`
		ProviderID string   `json:"provider_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	var providerObjID primitive.ObjectID
	if req.ProviderID != "" {
		if !utils.IsValidObjectID(req.ProviderID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(req.ProviderID)
	}

	invalidationID, err := sc.storageService.InvalidateCDN(user.ID, req.Paths, providerObjID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to invalidate CDN")
		return
	}

	utils.SuccessResponse(c, "CDN invalidation initiated successfully", gin.H{
		"invalidation_id": invalidationID,
	})
}

func (sc *StorageController) GetCDNStats(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	providerID := c.Query("provider_id")
	period := c.DefaultQuery("period", "30") // days

	var providerObjID primitive.ObjectID
	if providerID != "" {
		if !utils.IsValidObjectID(providerID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(providerID)
	}

	stats, err := sc.storageService.GetCDNStats(user.ID, providerObjID, period)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get CDN stats")
		return
	}

	utils.SuccessResponse(c, "CDN stats retrieved successfully", stats)
}

func (sc *StorageController) OptimizeImages(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileIDs    []string `json:"file_ids" validate:"required"`
		Quality    int      `json:"quality"` // 1-100
		Format     string   `json:"format"`  // jpeg, png, webp
		MaxWidth   int      `json:"max_width"`
		MaxHeight  int      `json:"max_height"`
		ProviderID string   `json:"provider_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate file IDs
	var fileObjIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		fileObjIDs = append(fileObjIDs, objID)
	}

	var providerObjID primitive.ObjectID
	if req.ProviderID != "" {
		if !utils.IsValidObjectID(req.ProviderID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(req.ProviderID)
	}

	optimizationResult, err := sc.storageService.OptimizeImages(user.ID, fileObjIDs, req.Quality, req.Format, req.MaxWidth, req.MaxHeight, providerObjID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to optimize images")
		return
	}

	utils.SuccessResponse(c, "Image optimization initiated successfully", optimizationResult)
}

// Backup and restore
func (sc *StorageController) CreateBackup(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Name       string   `json:"name" validate:"required"`
		FileIDs    []string `json:"file_ids"`
		FolderIDs  []string `json:"folder_ids"`
		BackupAll  bool     `json:"backup_all"`
		ProviderID string   `json:"provider_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	var fileObjIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		fileObjIDs = append(fileObjIDs, objID)
	}

	var folderObjIDs []primitive.ObjectID
	for _, id := range req.FolderIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid folder ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		folderObjIDs = append(folderObjIDs, objID)
	}

	var providerObjID primitive.ObjectID
	if req.ProviderID != "" {
		if !utils.IsValidObjectID(req.ProviderID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(req.ProviderID)
	}

	backup, err := sc.storageService.CreateBackup(user.ID, req.Name, fileObjIDs, folderObjIDs, req.BackupAll, providerObjID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create backup")
		return
	}

	utils.CreatedResponse(c, "Backup created successfully", backup)
}

func (sc *StorageController) GetBackups(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	backups, total, err := sc.storageService.GetUserBackups(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get backups")
		return
	}

	utils.PaginatedResponse(c, "Backups retrieved successfully", backups, page, limit, total)
}

func (sc *StorageController) RestoreBackup(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	backupID := c.Param("backup_id")
	if !utils.IsValidObjectID(backupID) {
		utils.BadRequestResponse(c, "Invalid backup ID")
		return
	}

	var req struct {
		RestorePath string `json:"restore_path"`
		ProviderID  string `json:"provider_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(backupID)

	var providerObjID primitive.ObjectID
	if req.ProviderID != "" {
		if !utils.IsValidObjectID(req.ProviderID) {
			utils.BadRequestResponse(c, "Invalid provider ID")
			return
		}
		providerObjID, _ = utils.StringToObjectID(req.ProviderID)
	}

	restoreResult, err := sc.storageService.RestoreBackup(user.ID, objID, req.RestorePath, providerObjID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to restore backup")
		return
	}

	utils.SuccessResponse(c, "Backup restore initiated successfully", restoreResult)
}

func (sc *StorageController) DeleteBackup(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	backupID := c.Param("backup_id")
	if !utils.IsValidObjectID(backupID) {
		utils.BadRequestResponse(c, "Invalid backup ID")
		return
	}

	objID, _ := utils.StringToObjectID(backupID)
	err := sc.storageService.DeleteBackup(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete backup")
		return
	}

	utils.SuccessResponse(c, "Backup deleted successfully", nil)
}
