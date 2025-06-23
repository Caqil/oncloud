// services/file_service.go
package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"oncloud/database"
	"oncloud/models"
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

type FileService struct {
	fileCollection     *mongo.Collection
	userCollection     *mongo.Collection
	folderCollection   *mongo.Collection
	planCollection     *mongo.Collection
	shareCollection    *mongo.Collection
	versionCollection  *mongo.Collection
	providerCollection *mongo.Collection
	storageService     *StorageService
}

type FileFilters struct {
	FolderID  string
	Search    string
	FileType  string
	SortBy    string
	SortOrder string
}

type FileAdminFilters struct {
	Search    string
	FileType  string
	UserID    string
	Status    string
	SortBy    string
	SortOrder string
}

func NewFileService() *FileService {
	return &FileService{
		fileCollection:     database.GetCollection("files"),
		userCollection:     database.GetCollection("users"),
		folderCollection:   database.GetCollection("folders"),
		planCollection:     database.GetCollection("plans"),
		shareCollection:    database.GetCollection("file_shares"),
		versionCollection:  database.GetCollection("file_versions"),
		providerCollection: database.GetCollection("storage_providers"),
		storageService:     NewStorageService(),
	}
}

// GetUserFiles returns paginated user files with filters
func (fs *FileService) GetUserFiles(userID primitive.ObjectID, page, limit int, filters *FileFilters) ([]models.File, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build filter query
	filter := bson.M{
		"user_id":    userID,
		"is_deleted": false,
	}

	if filters.FolderID != "" {
		if filters.FolderID == "root" {
			filter["folder_id"] = bson.M{"$exists": false}
		} else if utils.IsValidObjectID(filters.FolderID) {
			folderObjID, _ := utils.StringToObjectID(filters.FolderID)
			filter["folder_id"] = folderObjID
		}
	}

	if filters.Search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": filters.Search, "$options": "i"}},
			{"original_name": bson.M{"$regex": filters.Search, "$options": "i"}},
			{"tags": bson.M{"$in": []string{filters.Search}}},
		}
	}

	if filters.FileType != "" {
		switch filters.FileType {
		case "images":
			filter["mime_type"] = bson.M{"$regex": "^image/"}
		case "videos":
			filter["mime_type"] = bson.M{"$regex": "^video/"}
		case "audio":
			filter["mime_type"] = bson.M{"$regex": "^audio/"}
		case "documents":
			filter["$or"] = []bson.M{
				{"mime_type": bson.M{"$regex": "pdf"}},
				{"mime_type": bson.M{"$regex": "document"}},
				{"mime_type": bson.M{"$regex": "text/"}},
			}
		default:
			filter["extension"] = "." + strings.ToLower(filters.FileType)
		}
	}

	// Set sort options
	sortField := "created_at"
	if filters.SortBy != "" {
		sortField = filters.SortBy
	}

	sortOrder := -1
	if filters.SortOrder == "asc" {
		sortOrder = 1
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Get files
	cursor, err := fs.fileCollection.Find(ctx, filter,
		options.Find().
			SetSort(bson.M{sortField: sortOrder}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var files []models.File
	if err = cursor.All(ctx, &files); err != nil {
		return nil, 0, err
	}

	// Get total count
	total, err := fs.fileCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return files, int(total), nil
}

// GetUserFile returns a specific file for user
func (fs *FileService) GetUserFile(userID, fileID primitive.ObjectID) (*models.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var file models.File
	err := fs.fileCollection.FindOne(ctx, bson.M{
		"_id":        fileID,
		"user_id":    userID,
		"is_deleted": false,
	}).Decode(&file)
	if err != nil {
		return nil, fmt.Errorf("file not found: %v", err)
	}

	return &file, nil
}

// UploadFile handles file upload
func (fs *FileService) UploadFile(userID primitive.ObjectID, fileHeader *multipart.FileHeader, req *models.FileUploadRequest) (*models.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get user's plan for validation
	plan, err := fs.GetUserPlan(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user plan: %v", err)
	}

	// Validate file
	if err := fs.validateFileUpload(fileHeader, plan); err != nil {
		return nil, err
	}

	// Process file
	uploadConfig := &utils.UploadConfig{
		MaxFileSize:       plan.MaxFileSize,
		AllowedTypes:      plan.AllowedTypes,
		StorageProvider:   "default",
		GenerateThumbnail: utils.IsImageFile(fileHeader.Filename),
	}

	fileInfo, err := utils.ProcessFileUpload(fileHeader, uploadConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to process file: %v", err)
	}

	// Read file content
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	fileContent := make([]byte, fileHeader.Size)
	_, err = file.Read(fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Get storage provider
	provider, err := fs.getDefaultStorageProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage provider: %v", err)
	}

	// Upload to storage
	err = fs.storageService.UploadFile(provider.Type, fileInfo.Path, fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %v", err)
	}

	// Handle folder
	var folderObjID *primitive.ObjectID
	if req.FolderID != "" && utils.IsValidObjectID(req.FolderID) {
		fid, _ := utils.StringToObjectID(req.FolderID)
		folderObjID = &fid

		// Verify folder belongs to user
		if err := fs.validateFolderOwnership(userID, fid); err != nil {
			return nil, err
		}
	}

	// Create file record
	fileModel := &models.File{
		ID:              primitive.NewObjectID(),
		UserID:          userID,
		FolderID:        folderObjID,
		Name:            fileInfo.Name,
		OriginalName:    fileInfo.OriginalName,
		DisplayName:     req.Name,
		Description:     req.Description,
		Path:            fileInfo.Path,
		Size:            fileInfo.Size,
		MimeType:        fileInfo.MimeType,
		Extension:       fileInfo.Extension,
		Hash:            fileInfo.Hash,
		StorageProvider: provider.Type,
		StorageKey:      fileInfo.Path,
		StorageBucket:   provider.Bucket,
		IsPublic:        req.IsPublic,
		Tags:            req.Tags,
		Metadata:        convertStringMapToInterface(req.Metadata),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Check for duplicates
	if duplicate, err := fs.findDuplicateFile(userID, fileInfo.Hash); err == nil && duplicate != nil {
		return nil, fmt.Errorf("file already exists: %s", duplicate.Name)
	}

	// Insert file record
	_, err = fs.fileCollection.InsertOne(ctx, fileModel)
	if err != nil {
		// Cleanup uploaded file on database error
		fs.storageService.DeleteFile(provider.Type, fileInfo.Path)
		return nil, fmt.Errorf("failed to save file record: %v", err)
	}

	// Update user storage usage
	err = fs.updateUserStorageUsage(userID, fileInfo.Size, true)
	if err != nil {
		// Log error but don't fail the upload
		fmt.Printf("Failed to update user storage usage: %v\n", err)
	}

	// Generate thumbnail if needed
	if uploadConfig.GenerateThumbnail {
		go fs.generateThumbnailAsync(fileModel)
	}

	return fileModel, nil
}

// CheckUploadLimits validates if user can upload file
func (fs *FileService) CheckUploadLimits(user *models.User, plan *models.Plan, fileSize int64) error {
	// Check storage limit
	if user.StorageUsed+fileSize > plan.StorageLimit {
		return fmt.Errorf("upload would exceed storage limit of %s", utils.FormatFileSize(plan.StorageLimit))
	}

	// Check file count limit
	if plan.FilesLimit > 0 && user.FilesCount >= plan.FilesLimit {
		return fmt.Errorf("file limit of %d reached", plan.FilesLimit)
	}

	// Check file size limit
	if fileSize > plan.MaxFileSize {
		return fmt.Errorf("file size exceeds limit of %s", utils.FormatFileSize(plan.MaxFileSize))
	}

	return nil
}

// GetUserPlan gets user's plan
func (fs *FileService) GetUserPlan(userID primitive.ObjectID) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get user
	var user models.User
	err := fs.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get plan
	var plan models.Plan
	err = fs.planCollection.FindOne(ctx, bson.M{"_id": user.PlanID}).Decode(&plan)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %v", err)
	}

	return &plan, nil
}

// UploadChunk handles chunked upload
func (fs *FileService) UploadChunk(userID primitive.ObjectID, uploadID string, chunkNumber, totalChunks int, chunk *multipart.FileHeader) (map[string]interface{}, error) {
	// Store chunk temporarily
	fmt.Sprintf("chunks/%s/%d", uploadID, chunkNumber)

	// Read chunk content
	file, err := chunk.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open chunk: %v", err)
	}
	defer file.Close()

	chunkContent := make([]byte, chunk.Size)
	_, err = file.Read(chunkContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk: %v", err)
	}

	// Store chunk (implement temporary storage)
	err = fs.storeChunk(uploadID, chunkNumber, chunkContent)
	if err != nil {
		return nil, fmt.Errorf("failed to store chunk: %v", err)
	}

	result := map[string]interface{}{
		"upload_id":    uploadID,
		"chunk_number": chunkNumber,
		"total_chunks": totalChunks,
		"chunk_size":   chunk.Size,
		"uploaded_at":  time.Now(),
	}

	return result, nil
}

// CompleteChunkUpload assembles chunks into final file
func (fs *FileService) CompleteChunkUpload(userID primitive.ObjectID, uploadID, fileName, folderID string) (*models.File, error) {
	// Assemble chunks into final file
	finalContent, err := fs.assembleChunks(uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble chunks: %v", err)
	}

	// Create a temporary file header for processing
	_ = &multipart.FileHeader{
		Filename: fileName,
		Size:     int64(len(finalContent)),
	}

	// Use a mock file for upload processing
	file := &models.File{
		Name:     fileName,
		FolderID: nil,
	}
	
	// Set folder ID if provided
	if folderID != "" && utils.IsValidObjectID(folderID) {
		fid, _ := utils.StringToObjectID(folderID)
		file.FolderID = &fid
	}
	
	// Implementation would create the file record and upload to storage

	// Cleanup chunks
	go fs.cleanupChunks(uploadID)

	return file, nil
}

// UpdateFile updates file metadata
func (fs *FileService) UpdateFile(userID, fileID primitive.ObjectID, req interface{}) (*models.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify file ownership
	_, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return nil, err
	}

	// Update fields based on request type
	updates := bson.M{"updated_at": time.Now()}

	// Implementation would handle different update request types

	_, err = fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID, "user_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update file: %v", err)
	}

	return fs.GetUserFile(userID, fileID)
}

// DeleteFile handles file deletion (soft or hard)
func (fs *FileService) DeleteFile(userID, fileID primitive.ObjectID, permanent bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get file
	file, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return err
	}

	if permanent {
		// Hard delete - remove from storage and database
		err = fs.storageService.DeleteFile(file.StorageProvider, file.StorageKey)
		if err != nil {
			return fmt.Errorf("failed to delete from storage: %v", err)
		}

		_, err = fs.fileCollection.DeleteOne(ctx, bson.M{"_id": fileID})
		if err != nil {
			return fmt.Errorf("failed to delete file record: %v", err)
		}

		// Update user storage usage
		fs.updateUserStorageUsage(userID, -file.Size, false)
	} else {
		// Soft delete - mark as deleted
		_, err = fs.fileCollection.UpdateOne(ctx,
			bson.M{"_id": fileID, "user_id": userID},
			bson.M{"$set": bson.M{
				"is_deleted": true,
				"deleted_at": time.Now(),
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return fmt.Errorf("failed to mark file as deleted: %v", err)
		}
	}

	return nil
}

// RestoreFile restores a soft-deleted file
func (fs *FileService) RestoreFile(userID, fileID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID, "user_id": userID, "is_deleted": true},
		bson.M{
			"$set": bson.M{
				"is_deleted": false,
				"updated_at": time.Now(),
			},
			"$unset": bson.M{"deleted_at": ""},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to restore file: %v", err)
	}

	return nil
}

// GetDownloadURL generates download URL for file
func (fs *FileService) GetDownloadURL(userID, fileID primitive.ObjectID) (string, error) {
	file, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return "", err
	}

	// Generate presigned URL
	url, err := fs.storageService.GetPresignedURL(file.StorageProvider, file.StorageKey, 1*time.Hour, file.StorageBucket)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %v", err)
	}

	return url, nil
}

// StreamFile streams file content
func (fs *FileService) StreamFile(userID, fileID primitive.ObjectID, w http.ResponseWriter, r *http.Request) error {
	file, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return err
	}

	// Get file content from storage
	content, err := fs.storageService.DownloadFile(file.StorageProvider, file.StorageKey)
	if err != nil {
		return fmt.Errorf("failed to get file content: %v", err)
	}

	// Set headers
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", file.OriginalName))

	// Write content
	_, err = w.Write(content)
	return err
}

// IncrementDownloadCount increments file download counter
func (fs *FileService) IncrementDownloadCount(fileID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID},
		bson.M{"$inc": bson.M{"downloads": 1}},
	)
	return err
}

// File sharing methods
func (fs *FileService) CreateShare(userID, fileID primitive.ObjectID, req *models.ShareRequest) (*models.FileShare, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify file ownership
	_, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return nil, err
	}

	// Generate share token
	shareToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate share token: %v", err)
	}

	// Hash password if provided
	var hashedPassword string
	if req.Password != "" {
		hashedPassword, err = utils.HashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
	}

	// Create share record
	share := &models.FileShare{
		ID:           primitive.NewObjectID(),
		FileID:       fileID,
		UserID:       userID,
		Token:        shareToken,
		Password:     hashedPassword,
		ExpiresAt:    req.ExpiresAt,
		MaxDownloads: req.MaxDownloads,
		IsActive:     true,
		CreatedAt:    time.Now(),
	}

	_, err = fs.shareCollection.InsertOne(ctx, share)
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %v", err)
	}

	// Mark file as shared
	fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID},
		bson.M{"$set": bson.M{
			"is_shared":   true,
			"share_token": shareToken,
			"updated_at":  time.Now(),
		}},
	)

	return share, nil
}

func (fs *FileService) GetShare(userID, fileID primitive.ObjectID) (*models.FileShare, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var share models.FileShare
	err := fs.shareCollection.FindOne(ctx, bson.M{
		"file_id":   fileID,
		"user_id":   userID,
		"is_active": true,
	}).Decode(&share)
	if err != nil {
		return nil, fmt.Errorf("share not found: %v", err)
	}

	return &share, nil
}

func (fs *FileService) UpdateShare(userID, fileID primitive.ObjectID, req *models.ShareRequest) (*models.FileShare, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates := bson.M{"updated_at": time.Now()}

	if req.ExpiresAt != nil {
		updates["expires_at"] = req.ExpiresAt
	}
	if req.MaxDownloads > 0 {
		updates["max_downloads"] = req.MaxDownloads
	}
	if req.Password != "" {
		hashedPassword, err := utils.HashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
		updates["password"] = hashedPassword
	}

	_, err := fs.shareCollection.UpdateOne(ctx,
		bson.M{"file_id": fileID, "user_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update share: %v", err)
	}

	return fs.GetShare(userID, fileID)
}

func (fs *FileService) DeleteShare(userID, fileID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete share record
	_, err := fs.shareCollection.DeleteOne(ctx, bson.M{
		"file_id": fileID,
		"user_id": userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete share: %v", err)
	}

	// Update file
	_, err = fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID},
		bson.M{
			"$set": bson.M{
				"is_shared":  false,
				"updated_at": time.Now(),
			},
			"$unset": bson.M{"share_token": ""},
		},
	)
	return err
}

func (fs *FileService) GetShareURL(userID, fileID primitive.ObjectID) (string, error) {
	share, err := fs.GetShare(userID, fileID)
	if err != nil {
		return "", err
	}

	// Generate share URL
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	shareURL := fmt.Sprintf("%s/shared/%s", baseURL, share.Token)

	return shareURL, nil
}

// File operations
func (fs *FileService) CopyFile(userID, fileID primitive.ObjectID, destFolderID, newName string) (*models.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get original file
	originalFile, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return nil, err
	}

	// Validate destination folder
	var destFolderObjID *primitive.ObjectID
	if destFolderID != "" && utils.IsValidObjectID(destFolderID) {
		fid, _ := utils.StringToObjectID(destFolderID)
		destFolderObjID = &fid
		if err := fs.validateFolderOwnership(userID, fid); err != nil {
			return nil, err
		}
	}

	// Generate new name if not provided
	if newName == "" {
		newName = "Copy of " + originalFile.Name
	}

	// Copy file content in storage
	newStorageKey := fmt.Sprintf("users/%s/%s", userID.Hex(), newName)
	err = fs.storageService.CopyFile(originalFile.StorageProvider, originalFile.StorageKey, newStorageKey, originalFile.StorageBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file in storage: %v", err)
	}

	// Create new file record
	newFile := &models.File{
		ID:              primitive.NewObjectID(),
		UserID:          userID,
		FolderID:        destFolderObjID,
		Name:            newName,
		OriginalName:    originalFile.OriginalName,
		DisplayName:     newName,
		Description:     originalFile.Description,
		Path:            newStorageKey,
		Size:            originalFile.Size,
		MimeType:        originalFile.MimeType,
		Extension:       originalFile.Extension,
		StorageProvider: originalFile.StorageProvider,
		StorageKey:      newStorageKey,
		StorageBucket:   originalFile.StorageBucket,
		Tags:            originalFile.Tags,
		Metadata:        originalFile.Metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err = fs.fileCollection.InsertOne(ctx, newFile)
	if err != nil {
		// Cleanup on error
		fs.storageService.DeleteFile(originalFile.StorageProvider, newStorageKey)
		return nil, fmt.Errorf("failed to create file record: %v", err)
	}

	// Update user storage usage
	fs.updateUserStorageUsage(userID, originalFile.Size, true)

	return newFile, nil
}

func (fs *FileService) MoveFile(userID, fileID primitive.ObjectID, destFolderID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate destination folder
	var destFolderObjID *primitive.ObjectID
	if destFolderID != "" && utils.IsValidObjectID(destFolderID) {
		fid, _ := utils.StringToObjectID(destFolderID)
		destFolderObjID = &fid
		if err := fs.validateFolderOwnership(userID, fid); err != nil {
			return err
		}
	}

	// Update file folder
	updates := bson.M{"updated_at": time.Now()}
	if destFolderObjID != nil {
		updates["folder_id"] = *destFolderObjID
	} else {
		updates["$unset"] = bson.M{"folder_id": ""}
	}

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID, "user_id": userID},
		bson.M{"$set": updates},
	)
	return err
}

func (fs *FileService) ToggleFavorite(userID, fileID primitive.ObjectID, isFavorite bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID, "user_id": userID},
		bson.M{"$set": bson.M{
			"is_favorite": isFavorite,
			"updated_at":  time.Now(),
		}},
	)
	return err
}

func (fs *FileService) UpdateTags(userID, fileID primitive.ObjectID, tags []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID, "user_id": userID},
		bson.M{"$set": bson.M{
			"tags":       tags,
			"updated_at": time.Now(),
		}},
	)
	return err
}

// File versions
func (fs *FileService) GetFileVersions(userID, fileID primitive.ObjectID) ([]models.FileVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify file ownership
	_, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return nil, err
	}

	cursor, err := fs.versionCollection.Find(ctx,
		bson.M{"file_id": fileID},
		options.Find().SetSort(bson.M{"version_number": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var versions []models.FileVersion
	if err = cursor.All(ctx, &versions); err != nil {
		return nil, err
	}

	return versions, nil
}

func (fs *FileService) CreateFileVersion(userID, fileID primitive.ObjectID, fileHeader *multipart.FileHeader) (*models.FileVersion, error) {
	// Implementation for creating file versions
	return nil, errors.New("not implemented")
}

func (fs *FileService) GetFileVersion(userID, fileID primitive.ObjectID, versionNumber int) (*models.FileVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify file ownership
	_, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return nil, err
	}

	var version models.FileVersion
	err = fs.versionCollection.FindOne(ctx, bson.M{
		"file_id":        fileID,
		"version_number": versionNumber,
	}).Decode(&version)
	if err != nil {
		return nil, fmt.Errorf("version not found: %v", err)
	}

	return &version, nil
}

func (fs *FileService) RestoreFileVersion(userID, fileID primitive.ObjectID, versionNumber int) error {
	// Implementation for restoring file versions
	return errors.New("not implemented")
}

func (fs *FileService) DeleteFileVersion(userID, fileID primitive.ObjectID, versionNumber int) error {
	// Implementation for deleting file versions
	return errors.New("not implemented")
}

// Bulk operations
func (fs *FileService) BulkDeleteFiles(userID primitive.ObjectID, fileIDs []primitive.ObjectID) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
	}

	for _, fileID := range fileIDs {
		err := fs.DeleteFile(userID, fileID, false)
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
		}
	}

	return results, nil
}

func (fs *FileService) BulkMoveFiles(userID primitive.ObjectID, fileIDs []primitive.ObjectID, destFolderID string) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
	}

	for _, fileID := range fileIDs {
		err := fs.MoveFile(userID, fileID, destFolderID)
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
		}
	}

	return results, nil
}

func (fs *FileService) BulkCopyFiles(userID primitive.ObjectID, fileIDs []primitive.ObjectID, destFolderID string) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
	}

	for _, fileID := range fileIDs {
		_, err := fs.CopyFile(userID, fileID, destFolderID, "")
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
		}
	}

	return results, nil
}

func (fs *FileService) CreateBulkDownload(userID primitive.ObjectID, fileIDs []primitive.ObjectID) (string, error) {
	// Create ZIP archive of files
	zipToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate download token: %v", err)
	}

	// Implementation would create ZIP file and return download URL
	downloadURL := fmt.Sprintf("/api/v1/files/download/bulk/%s", zipToken)

	return downloadURL, nil
}

func (fs *FileService) BulkShareFiles(userID primitive.ObjectID, fileIDs []primitive.ObjectID, shareData *models.ShareRequest) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
		"shares":  []string{},
	}

	for _, fileID := range fileIDs {
		share, err := fs.CreateShare(userID, fileID, shareData)
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
			results["shares"] = append(results["shares"].([]string), share.Token)
		}
	}

	return results, nil
}

// Public file access
func (fs *FileService) GetPublicDownloadURL(token string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var file models.File
	err := fs.fileCollection.FindOne(ctx, bson.M{
		"share_token": token,
		"is_public":   true,
		"is_deleted":  false,
	}).Decode(&file)
	if err != nil {
		return "", fmt.Errorf("file not found or not public: %v", err)
	}

	// Generate download URL
	url, err := fs.storageService.GetPresignedURL(file.StorageProvider, file.StorageKey, 1*time.Hour, file.StorageBucket)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %v", err)
	}

	return url, nil
}

func (fs *FileService) GetSharedDownloadURL(token string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find share by token
	var share models.FileShare
	err := fs.shareCollection.FindOne(ctx, bson.M{
		"token":     token,
		"is_active": true,
	}).Decode(&share)
	if err != nil {
		return "", fmt.Errorf("share not found: %v", err)
	}

	// Check expiration
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now()) {
		return "", errors.New("share has expired")
	}

	// Check download limit
	if share.MaxDownloads > 0 && share.Downloads >= share.MaxDownloads {
		return "", errors.New("download limit reached")
	}

	// Get file
	var file models.File
	err = fs.fileCollection.FindOne(ctx, bson.M{
		"_id":        share.FileID,
		"is_deleted": false,
	}).Decode(&file)
	if err != nil {
		return "", fmt.Errorf("file not found: %v", err)
	}

	// Generate download URL
	url, err := fs.storageService.GetPresignedURL(file.StorageProvider, file.StorageKey, 1*time.Hour, file.StorageBucket)
	if err != nil {
		return "", fmt.Errorf("failed to generate download URL: %v", err)
	}

	// Increment download count
	fs.shareCollection.UpdateOne(ctx,
		bson.M{"_id": share.ID},
		bson.M{"$inc": bson.M{"downloads": 1}},
	)

	return url, nil
}

func (fs *FileService) VerifySharePassword(token, password string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var share models.FileShare
	err := fs.shareCollection.FindOne(ctx, bson.M{
		"token":     token,
		"is_active": true,
	}).Decode(&share)
	if err != nil {
		return nil, fmt.Errorf("share not found: %v", err)
	}

	// Check password if required
	if share.Password != "" {
		if !utils.CheckPasswordHash(password, share.Password) {
			return nil, errors.New("invalid password")
		}
	}

	return map[string]interface{}{
		"access_granted": true,
		"share_id":       share.ID,
		"file_id":        share.FileID,
	}, nil
}

// File preview and thumbnails
func (fs *FileService) GeneratePreview(userID, fileID primitive.ObjectID) (string, error) {
	_, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return "", err
	}

	// Generate preview URL based on file type
	previewURL := fmt.Sprintf("/api/v1/files/%s/preview", fileID.Hex())
	return previewURL, nil
}

func (fs *FileService) GetThumbnail(userID, fileID primitive.ObjectID) (string, error) {
	file, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return "", err
	}

	if file.ThumbnailURL != "" {
		return file.ThumbnailURL, nil
	}

	return "", errors.New("thumbnail not available")
}

func (fs *FileService) GenerateThumbnail(userID, fileID primitive.ObjectID) (string, error) {
	file, err := fs.GetUserFile(userID, fileID)
	if err != nil {
		return "", err
	}

	// Generate thumbnail for images
	if !utils.IsImageFile(file.Name) {
		return "", errors.New("thumbnails only available for images")
	}

	// Implementation would generate thumbnail and upload to storage
	thumbnailURL := fmt.Sprintf("/thumbnails/%s_thumb.jpg", fileID.Hex())

	// Update file record with thumbnail URL
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID},
		bson.M{"$set": bson.M{"thumbnail_url": thumbnailURL}},
	)

	return thumbnailURL, nil
}

// Admin methods
func (fs *FileService) GetFilesForAdmin(page, limit int, filters *FileAdminFilters) ([]models.File, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build filter
	filter := bson.M{}

	if filters.Search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": filters.Search, "$options": "i"}},
			{"original_name": bson.M{"$regex": filters.Search, "$options": "i"}},
		}
	}

	if filters.UserID != "" && utils.IsValidObjectID(filters.UserID) {
		userObjID, _ := utils.StringToObjectID(filters.UserID)
		filter["user_id"] = userObjID
	}

	if filters.Status != "" {
		switch filters.Status {
		case "active":
			filter["is_deleted"] = false
		case "deleted":
			filter["is_deleted"] = true
		case "reported":
			filter["is_reported"] = true
		}
	}

	if filters.FileType != "" {
		filter["mime_type"] = bson.M{"$regex": "^" + filters.FileType + "/"}
	}

	// Sort and pagination
	sortField := "created_at"
	if filters.SortBy != "" {
		sortField = filters.SortBy
	}

	sortOrder := -1
	if filters.SortOrder == "asc" {
		sortOrder = 1
	}

	skip := (page - 1) * limit

	cursor, err := fs.fileCollection.Find(ctx, filter,
		options.Find().
			SetSort(bson.M{sortField: sortOrder}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var files []models.File
	if err = cursor.All(ctx, &files); err != nil {
		return nil, 0, err
	}

	total, err := fs.fileCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return files, int(total), nil
}

func (fs *FileService) GetFileForAdmin(fileID primitive.ObjectID) (*models.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var file models.File
	err := fs.fileCollection.FindOne(ctx, bson.M{"_id": fileID}).Decode(&file)
	if err != nil {
		return nil, fmt.Errorf("file not found: %v", err)
	}

	return &file, nil
}

func (fs *FileService) DeleteFileByAdmin(fileID primitive.ObjectID, reason string, permanent bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if permanent {
		// Get file for cleanup
		var file models.File
		err := fs.fileCollection.FindOne(ctx, bson.M{"_id": fileID}).Decode(&file)
		if err != nil {
			return fmt.Errorf("file not found: %v", err)
		}

		// Delete from storage
		fs.storageService.DeleteFile(file.StorageProvider, file.StorageKey)

		// Delete from database
		_, err = fs.fileCollection.DeleteOne(ctx, bson.M{"_id": fileID})
		return err
	} else {
		// Soft delete
		_, err := fs.fileCollection.UpdateOne(ctx,
			bson.M{"_id": fileID},
			bson.M{"$set": bson.M{
				"is_deleted":       true,
				"deleted_at":       time.Now(),
				"deletion_reason":  reason,
				"deleted_by_admin": true,
			}},
		)
		return err
	}
}

func (fs *FileService) RestoreFileByAdmin(fileID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID},
		bson.M{
			"$set": bson.M{"is_deleted": false},
			"$unset": bson.M{
				"deleted_at":       "",
				"deletion_reason":  "",
				"deleted_by_admin": "",
			},
		},
	)
	return err
}

func (fs *FileService) ModerateFile(fileID primitive.ObjectID, action, reason, notes string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates := bson.M{
		"moderation_action": action,
		"moderation_reason": reason,
		"moderation_notes":  notes,
		"moderated_at":      time.Now(),
	}

	switch action {
	case "approve":
		updates["is_approved"] = true
	case "reject":
		updates["is_approved"] = false
	case "flag":
		updates["is_flagged"] = true
	case "quarantine":
		updates["is_quarantined"] = true
	}

	_, err := fs.fileCollection.UpdateOne(ctx,
		bson.M{"_id": fileID},
		bson.M{"$set": updates},
	)
	return err
}

func (fs *FileService) GetReportedFiles(status string, page, limit int) ([]map[string]interface{}, int, error) {
	// Implementation for getting reported files
	return []map[string]interface{}{}, 0, nil
}

func (fs *FileService) ScanFile(fileID primitive.ObjectID, scanType string, force bool) (map[string]interface{}, error) {
	// Implementation for file scanning (virus, malware, content)
	return map[string]interface{}{
		"scan_id":    primitive.NewObjectID(),
		"file_id":    fileID,
		"scan_type":  scanType,
		"status":     "initiated",
		"started_at": time.Now(),
	}, nil
}

// Helper methods
func (fs *FileService) validateFileUpload(header *multipart.FileHeader, plan *models.Plan) error {
	// Check file size
	if header.Size > plan.MaxFileSize {
		return fmt.Errorf("file size exceeds limit of %s", utils.FormatFileSize(plan.MaxFileSize))
	}

	// Check file type if restricted
	if len(plan.AllowedTypes) > 0 {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !utils.SliceContains(plan.AllowedTypes, ext) {
			return fmt.Errorf("file type %s not allowed", ext)
		}
	}

	return nil
}

func (fs *FileService) validateFolderOwnership(userID, folderID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var folder models.Folder
	err := fs.folderCollection.FindOne(ctx, bson.M{
		"_id":     folderID,
		"user_id": userID,
	}).Decode(&folder)
	if err != nil {
		return fmt.Errorf("folder not found or access denied: %v", err)
	}

	return nil
}

func (fs *FileService) findDuplicateFile(userID primitive.ObjectID, hash string) (*models.File, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var file models.File
	err := fs.fileCollection.FindOne(ctx, bson.M{
		"user_id":    userID,
		"hash":       hash,
		"is_deleted": false,
	}).Decode(&file)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (fs *FileService) updateUserStorageUsage(userID primitive.ObjectID, sizeChange int64, increment bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{}
	if increment {
		update["$inc"] = bson.M{
			"storage_used": sizeChange,
			"files_count":  1,
		}
	} else {
		update["$inc"] = bson.M{
			"storage_used": -sizeChange,
			"files_count":  -1,
		}
	}

	_, err := fs.userCollection.UpdateOne(ctx,
		bson.M{"_id": userID},
		update,
	)
	return err
}

func (fs *FileService) getDefaultStorageProvider() (*models.StorageProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var provider models.StorageProvider
	err := fs.providerCollection.FindOne(ctx, bson.M{
		"is_default": true,
		"is_active":  true,
	}).Decode(&provider)
	if err != nil {
		return nil, fmt.Errorf("no default storage provider found: %v", err)
	}

	return &provider, nil
}

func (fs *FileService) storeChunk(uploadID string, chunkNumber int, content []byte) error {
	// Implementation for storing file chunks temporarily
	return nil
}

func (fs *FileService) assembleChunks(uploadID string) ([]byte, error) {
	// Implementation for assembling chunks into final file
	return []byte{}, nil
}

func (fs *FileService) cleanupChunks(uploadID string) {
	// Implementation for cleaning up temporary chunks
}

func (fs *FileService) generateThumbnailAsync(file *models.File) {
	// Implementation for async thumbnail generation
}

func convertStringMapToInterface(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}
