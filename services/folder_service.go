package services

import (
	"context"
	"errors"
	"fmt"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FolderService struct {
	folderCollection *mongo.Collection
	fileCollection   *mongo.Collection
	userCollection   *mongo.Collection
	shareCollection  *mongo.Collection
}

func NewFolderService() *FolderService {
	return &FolderService{
		folderCollection: database.GetCollection("folders"),
		fileCollection:   database.GetCollection("files"),
		userCollection:   database.GetCollection("users"),
		shareCollection:  database.GetCollection("folder_shares"),
	}
}

// GetUserFolders returns paginated user folders
func (fs *FolderService) GetUserFolders(userID primitive.ObjectID, parentID, search string, page, limit int) ([]models.Folder, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build filter query
	filter := bson.M{
		"user_id":    userID,
		"is_deleted": false,
	}

	// Handle parent folder filter
	if parentID == "" || parentID == "root" {
		filter["parent_id"] = bson.M{"$exists": false}
	} else if utils.IsValidObjectID(parentID) {
		parentObjID, _ := utils.StringToObjectID(parentID)
		filter["parent_id"] = parentObjID
	}

	// Handle search
	if search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"description": bson.M{"$regex": search, "$options": "i"}},
			{"tags": bson.M{"$in": []string{search}}},
		}
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Get folders
	cursor, err := fs.folderCollection.Find(ctx, filter,
		options.Find().
			SetSort(bson.M{"name": 1}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, 0, err
	}

	// Get total count
	total, err := fs.folderCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return folders, int(total), nil
}

// GetUserFolder returns a specific folder for user
func (fs *FolderService) GetUserFolder(userID, folderID primitive.ObjectID) (*models.Folder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var folder models.Folder
	err := fs.folderCollection.FindOne(ctx, bson.M{
		"_id":        folderID,
		"user_id":    userID,
		"is_deleted": false,
	}).Decode(&folder)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %v", err)
	}

	return &folder, nil
}

// CreateFolder creates a new folder
func (fs *FolderService) CreateFolder(userID primitive.ObjectID, req *models.FolderCreateRequest) (*models.Folder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Validate parent folder if specified
	var parentObjID *primitive.ObjectID
	if req.ParentID != "" && utils.IsValidObjectID(req.ParentID) {
		pid, _ := utils.StringToObjectID(req.ParentID)
		parentObjID = &pid

		// Verify parent folder exists and belongs to user
		if err := fs.validateFolderOwnership(userID, pid); err != nil {
			return nil, fmt.Errorf("invalid parent folder: %v", err)
		}
	}

	// Check for duplicate folder name in same parent
	if err := fs.checkDuplicateFolderName(userID, req.Name, parentObjID); err != nil {
		return nil, err
	}

	// Generate folder path
	path, err := fs.generateFolderPath(userID, req.Name, parentObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate folder path: %v", err)
	}

	// Create folder
	folder := &models.Folder{
		ID:          primitive.NewObjectID(),
		UserID:      userID,
		ParentID:    parentObjID,
		Name:        req.Name,
		Description: req.Description,
		Path:        path,
		Color:       req.Color,
		Icon:        req.Icon,
		IsPublic:    req.IsPublic,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Insert folder
	_, err = fs.folderCollection.InsertOne(ctx, folder)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %v", err)
	}

	// Update user folder count
	fs.updateUserFolderCount(userID, 1)

	return folder, nil
}

// UpdateFolder updates folder information
func (fs *FolderService) UpdateFolder(userID, folderID primitive.ObjectID, req interface{}) (*models.Folder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify folder ownership
	_, err := fs.GetUserFolder(userID, folderID)
	if err != nil {
		return nil, err
	}

	// Build updates based on request
	updates := bson.M{"updated_at": time.Now()}

	// This would be implemented based on the actual request structure
	// For now, we'll use a generic approach
	if reqMap, ok := req.(*map[string]interface{}); ok {
		for key, value := range *reqMap {
			if key != "_id" && key != "user_id" && key != "created_at" {
				updates[key] = value
			}
		}
	}

	// Update folder
	_, err = fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID, "user_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update folder: %v", err)
	}

	return fs.GetUserFolder(userID, folderID)
}

// DeleteFolder handles folder deletion (soft or hard)
func (fs *FolderService) DeleteFolder(userID, folderID primitive.ObjectID, permanent bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get folder
	_, err := fs.GetUserFolder(userID, folderID)
	if err != nil {
		return err
	}

	if permanent {
		// Hard delete - recursively delete all contents
		if err := fs.deleteAllFolderContents(ctx, userID, folderID); err != nil {
			return fmt.Errorf("failed to delete folder contents: %v", err)
		}

		// Delete folder
		_, err = fs.folderCollection.DeleteOne(ctx, bson.M{"_id": folderID})
		if err != nil {
			return fmt.Errorf("failed to delete folder: %v", err)
		}
	} else {
		// Soft delete - mark as deleted
		_, err = fs.folderCollection.UpdateOne(ctx,
			bson.M{"_id": folderID, "user_id": userID},
			bson.M{"$set": bson.M{
				"is_deleted": true,
				"deleted_at": time.Now(),
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return fmt.Errorf("failed to mark folder as deleted: %v", err)
		}

		// Soft delete all files in folder
		fs.fileCollection.UpdateMany(ctx,
			bson.M{"folder_id": folderID, "user_id": userID},
			bson.M{"$set": bson.M{
				"is_deleted": true,
				"deleted_at": time.Now(),
			}},
		)

		// Soft delete all subfolders
		fs.softDeleteSubfolders(ctx, userID, folderID)
	}

	// Update user folder count
	fs.updateUserFolderCount(userID, -1)

	return nil
}

// RestoreFolder restores a soft-deleted folder
func (fs *FolderService) RestoreFolder(userID, folderID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Restore folder
	_, err := fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID, "user_id": userID, "is_deleted": true},
		bson.M{
			"$set": bson.M{
				"is_deleted": false,
				"updated_at": time.Now(),
			},
			"$unset": bson.M{"deleted_at": ""},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to restore folder: %v", err)
	}

	// Restore files in folder
	fs.fileCollection.UpdateMany(ctx,
		bson.M{"folder_id": folderID, "user_id": userID, "is_deleted": true},
		bson.M{
			"$set":   bson.M{"is_deleted": false},
			"$unset": bson.M{"deleted_at": ""},
		},
	)

	return nil
}

// GetFolderContents returns folder contents (files and subfolders)
func (fs *FolderService) GetFolderContents(userID, folderID primitive.ObjectID, page, limit int, sortBy, sortOrder string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify folder ownership
	_, err := fs.GetUserFolder(userID, folderID)
	if err != nil {
		return nil, err
	}

	// Get subfolders
	subfolders, err := fs.getFolderSubfolders(ctx, userID, folderID, sortBy, sortOrder)
	if err != nil {
		return nil, err
	}

	// Get files in folder with pagination
	files, filesTotal, err := fs.getFolderFiles(ctx, userID, folderID, page, limit, sortBy, sortOrder)
	if err != nil {
		return nil, err
	}

	// Get folder statistics
	stats, err := fs.calculateFolderStats(ctx, userID, folderID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"subfolders":  subfolders,
		"files":       files,
		"files_total": filesTotal,
		"stats":       stats,
		"page":        page,
		"limit":       limit,
	}, nil
}

// GetFolderTree returns hierarchical folder tree
func (fs *FolderService) GetFolderTree(userID, rootFolderID primitive.ObjectID) (*models.FolderTree, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// If rootFolderID is nil, start from root
	var rootFolder *models.Folder
	if !rootFolderID.IsZero() {
		var err error
		rootFolder, err = fs.GetUserFolder(userID, rootFolderID)
		if err != nil {
			return nil, err
		}
	}

	// Build tree recursively
	tree, err := fs.buildFolderTree(ctx, userID, rootFolder, 3) // Max depth of 3
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// GetBreadcrumb returns folder breadcrumb path
func (fs *FolderService) GetBreadcrumb(userID, folderID primitive.ObjectID) ([]models.Folder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var breadcrumb []models.Folder
	currentFolderID := folderID

	// Traverse up the folder hierarchy
	for !currentFolderID.IsZero() {
		var folder models.Folder
		err := fs.folderCollection.FindOne(ctx, bson.M{
			"_id":        currentFolderID,
			"user_id":    userID,
			"is_deleted": false,
		}).Decode(&folder)
		if err != nil {
			break
		}

		// Prepend to breadcrumb (so we get root -> ... -> current)
		breadcrumb = append([]models.Folder{folder}, breadcrumb...)

		// Move to parent
		if folder.ParentID != nil {
			currentFolderID = *folder.ParentID
		} else {
			break
		}
	}

	return breadcrumb, nil
}

// GetRootFolderContents returns root folder contents
func (fs *FolderService) GetRootFolderContents(userID primitive.ObjectID, page, limit int) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get root folders (folders without parent)
	rootFolders, err := fs.getRootFolders(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get root files (files without folder)
	rootFiles, filesTotal, err := fs.getRootFiles(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"folders":     rootFolders,
		"files":       rootFiles,
		"files_total": filesTotal,
		"page":        page,
		"limit":       limit,
	}, nil
}

// GetRecentFolders returns recently accessed folders
func (fs *FolderService) GetRecentFolders(userID primitive.ObjectID, limit int) ([]models.Folder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := fs.folderCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"is_deleted": false,
		},
		options.Find().
			SetSort(bson.M{"updated_at": -1}).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

// GetFavoriteFolders returns user's favorite folders
func (fs *FolderService) GetFavoriteFolders(userID primitive.ObjectID, page, limit int) ([]models.Folder, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := fs.folderCollection.Find(ctx,
		bson.M{
			"user_id":     userID,
			"is_favorite": true,
			"is_deleted":  false,
		},
		options.Find().
			SetSort(bson.M{"name": 1}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, 0, err
	}

	// Get total count
	total, err := fs.folderCollection.CountDocuments(ctx, bson.M{
		"user_id":     userID,
		"is_favorite": true,
		"is_deleted":  false,
	})
	if err != nil {
		return nil, 0, err
	}

	return folders, int(total), nil
}

// GetDeletedFolders returns deleted folders (trash)
func (fs *FolderService) GetDeletedFolders(userID primitive.ObjectID, page, limit int) ([]models.Folder, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := fs.folderCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"is_deleted": true,
		},
		options.Find().
			SetSort(bson.M{"deleted_at": -1}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, 0, err
	}

	// Get total count
	total, err := fs.folderCollection.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"is_deleted": true,
	})
	if err != nil {
		return nil, 0, err
	}

	return folders, int(total), nil
}

// Folder operations
func (fs *FolderService) CopyFolder(userID, folderID primitive.ObjectID, destParentID, newName string) (*models.Folder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get original folder
	originalFolder, err := fs.GetUserFolder(userID, folderID)
	if err != nil {
		return nil, err
	}

	// Validate destination parent
	var destParentObjID *primitive.ObjectID
	if destParentID != "" && utils.IsValidObjectID(destParentID) {
		pid, _ := utils.StringToObjectID(destParentID)
		destParentObjID = &pid
		if err := fs.validateFolderOwnership(userID, pid); err != nil {
			return nil, err
		}
	}

	// Generate new name if not provided
	if newName == "" {
		newName = "Copy of " + originalFolder.Name
	}

	// Check for duplicate name
	if err := fs.checkDuplicateFolderName(userID, newName, destParentObjID); err != nil {
		return nil, err
	}

	// Create new folder
	newFolder := &models.Folder{
		ID:          primitive.NewObjectID(),
		UserID:      userID,
		ParentID:    destParentObjID,
		Name:        newName,
		Description: originalFolder.Description,
		Color:       originalFolder.Color,
		Icon:        originalFolder.Icon,
		Tags:        originalFolder.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Generate new path
	newFolder.Path, err = fs.generateFolderPath(userID, newName, destParentObjID)
	if err != nil {
		return nil, err
	}

	// Insert new folder
	_, err = fs.folderCollection.InsertOne(ctx, newFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder copy: %v", err)
	}

	// Copy all contents recursively
	go fs.copyFolderContentsAsync(userID, folderID, newFolder.ID)

	// Update user folder count
	fs.updateUserFolderCount(userID, 1)

	return newFolder, nil
}

func (fs *FolderService) MoveFolder(userID, folderID primitive.ObjectID, destParentID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get folder
	folder, err := fs.GetUserFolder(userID, folderID)
	if err != nil {
		return err
	}

	// Validate destination parent
	var destParentObjID *primitive.ObjectID
	if destParentID != "" && utils.IsValidObjectID(destParentID) {
		pid, _ := utils.StringToObjectID(destParentID)
		destParentObjID = &pid

		// Check for circular reference
		if err := fs.checkCircularReference(userID, folderID, pid); err != nil {
			return err
		}

		if err := fs.validateFolderOwnership(userID, pid); err != nil {
			return err
		}
	}

	// Check for duplicate name in destination
	if err := fs.checkDuplicateFolderName(userID, folder.Name, destParentObjID); err != nil {
		return err
	}

	// Generate new path
	newPath, err := fs.generateFolderPath(userID, folder.Name, destParentObjID)
	if err != nil {
		return err
	}

	// Update folder
	updates := bson.M{
		"path":       newPath,
		"updated_at": time.Now(),
	}

	if destParentObjID != nil {
		updates["parent_id"] = *destParentObjID
	} else {
		updates["$unset"] = bson.M{"parent_id": ""}
	}

	_, err = fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID, "user_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return fmt.Errorf("failed to move folder: %v", err)
	}

	// Update paths of all subfolders
	go fs.updateSubfolderPathsAsync(userID, folderID, newPath)

	return nil
}

func (fs *FolderService) ToggleFavorite(userID, folderID primitive.ObjectID, isFavorite bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID, "user_id": userID},
		bson.M{"$set": bson.M{
			"is_favorite": isFavorite,
			"updated_at":  time.Now(),
		}},
	)
	return err
}

func (fs *FolderService) UpdateTags(userID, folderID primitive.ObjectID, tags []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID, "user_id": userID},
		bson.M{"$set": bson.M{
			"tags":       tags,
			"updated_at": time.Now(),
		}},
	)
	return err
}

// Folder sharing
func (fs *FolderService) CreateShare(userID, folderID primitive.ObjectID, req *models.ShareRequest) (*models.FileShare, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify folder ownership
	_, err := fs.GetUserFolder(userID, folderID)
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

	// Create share record (reusing FileShare structure for folders)
	share := &models.FileShare{
		ID:           primitive.NewObjectID(),
		FileID:       folderID, // Using file_id field for folder_id
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

	// Mark folder as shared
	fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID},
		bson.M{"$set": bson.M{
			"is_shared":   true,
			"share_token": shareToken,
			"updated_at":  time.Now(),
		}},
	)

	return share, nil
}

func (fs *FolderService) GetShare(userID, folderID primitive.ObjectID) (*models.FileShare, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var share models.FileShare
	err := fs.shareCollection.FindOne(ctx, bson.M{
		"file_id":   folderID, // Using file_id field for folder_id
		"user_id":   userID,
		"is_active": true,
	}).Decode(&share)
	if err != nil {
		return nil, fmt.Errorf("share not found: %v", err)
	}

	return &share, nil
}

func (fs *FolderService) UpdateShare(userID, folderID primitive.ObjectID, req *models.ShareRequest) (*models.FileShare, error) {
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
		bson.M{"file_id": folderID, "user_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update share: %v", err)
	}

	return fs.GetShare(userID, folderID)
}

func (fs *FolderService) DeleteShare(userID, folderID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete share record
	_, err := fs.shareCollection.DeleteOne(ctx, bson.M{
		"file_id": folderID,
		"user_id": userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete share: %v", err)
	}

	// Update folder
	_, err = fs.folderCollection.UpdateOne(ctx,
		bson.M{"_id": folderID},
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

func (fs *FolderService) GetShareURL(userID, folderID primitive.ObjectID) (string, error) {
	share, err := fs.GetShare(userID, folderID)
	if err != nil {
		return "", err
	}

	// Generate share URL
	baseURL := os.Getenv("BASE_URL")
	shareURL := fmt.Sprintf("%s/shared/folder/%s", baseURL, share.Token)

	return shareURL, nil
}

// Folder statistics
func (fs *FolderService) GetFolderStats(userID, folderID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return fs.calculateFolderStats(ctx, userID, folderID)
}

func (fs *FolderService) GetFolderSize(userID, folderID primitive.ObjectID) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calculate total size recursively
	totalSize, err := fs.calculateFolderSizeRecursive(ctx, userID, folderID)
	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

// Bulk operations
func (fs *FolderService) BulkDeleteFolders(userID primitive.ObjectID, folderIDs []primitive.ObjectID) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
	}

	for _, folderID := range folderIDs {
		err := fs.DeleteFolder(userID, folderID, false)
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
		}
	}

	return results, nil
}

func (fs *FolderService) BulkMoveFolders(userID primitive.ObjectID, folderIDs []primitive.ObjectID, destParentID string) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
	}

	for _, folderID := range folderIDs {
		err := fs.MoveFolder(userID, folderID, destParentID)
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
		}
	}

	return results, nil
}

func (fs *FolderService) BulkCopyFolders(userID primitive.ObjectID, folderIDs []primitive.ObjectID, destParentID string) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
	}

	for _, folderID := range folderIDs {
		_, err := fs.CopyFolder(userID, folderID, destParentID, "")
		if err != nil {
			results["failed"] = results["failed"].(int) + 1
			results["errors"] = append(results["errors"].([]string), err.Error())
		} else {
			results["success"] = results["success"].(int) + 1
		}
	}

	return results, nil
}

func (fs *FolderService) BulkShareFolders(userID primitive.ObjectID, folderIDs []primitive.ObjectID, shareData *models.ShareRequest) (map[string]interface{}, error) {
	results := map[string]interface{}{
		"success": 0,
		"failed":  0,
		"errors":  []string{},
		"shares":  []string{},
	}

	for _, folderID := range folderIDs {
		share, err := fs.CreateShare(userID, folderID, shareData)
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

// Public folder access
func (fs *FolderService) GetPublicFolderContents(token string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find folder by token
	var folder models.Folder
	err := fs.folderCollection.FindOne(ctx, bson.M{
		"share_token": token,
		"is_public":   true,
		"is_deleted":  false,
	}).Decode(&folder)
	if err != nil {
		return nil, fmt.Errorf("folder not found or not public: %v", err)
	}

	// Get folder contents (limited view for public access)
	subfolders, err := fs.getFolderSubfolders(ctx, folder.UserID, folder.ID, "name", "asc")
	if err != nil {
		return nil, err
	}

	files, _, err := fs.getFolderFiles(ctx, folder.UserID, folder.ID, 1, 50, "name", "asc")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"folder":     folder,
		"subfolders": subfolders,
		"files":      files,
	}, nil
}

func (fs *FolderService) GetSharedFolderContents(token string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find share by token
	var share models.FileShare
	err := fs.shareCollection.FindOne(ctx, bson.M{
		"token":     token,
		"is_active": true,
	}).Decode(&share)
	if err != nil {
		return nil, fmt.Errorf("share not found: %v", err)
	}

	// Check expiration
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("share has expired")
	}

	// Get folder
	var folder models.Folder
	err = fs.folderCollection.FindOne(ctx, bson.M{
		"_id":        share.FileID, // Using file_id field for folder_id
		"is_deleted": false,
	}).Decode(&folder)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %v", err)
	}

	// Get folder contents
	subfolders, err := fs.getFolderSubfolders(ctx, folder.UserID, folder.ID, "name", "asc")
	if err != nil {
		return nil, err
	}

	files, _, err := fs.getFolderFiles(ctx, folder.UserID, folder.ID, 1, 50, "name", "asc")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"folder":     folder,
		"subfolders": subfolders,
		"files":      files,
		"share":      share,
	}, nil
}

// Helper methods
func (fs *FolderService) validateFolderOwnership(userID, folderID primitive.ObjectID) error {
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

func (fs *FolderService) checkDuplicateFolderName(userID primitive.ObjectID, name string, parentID *primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"user_id":    userID,
		"name":       name,
		"is_deleted": false,
	}

	if parentID != nil {
		filter["parent_id"] = *parentID
	} else {
		filter["parent_id"] = bson.M{"$exists": false}
	}

	count, err := fs.folderCollection.CountDocuments(ctx, filter)
	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("folder with this name already exists in the same location")
	}

	return nil
}

func (fs *FolderService) generateFolderPath(userID primitive.ObjectID, folderName string, parentID *primitive.ObjectID) (string, error) {
	if parentID == nil {
		return "/" + folderName, nil
	}

	// Get parent path
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var parent models.Folder
	err := fs.folderCollection.FindOne(ctx, bson.M{"_id": *parentID}).Decode(&parent)
	if err != nil {
		return "", err
	}

	return parent.Path + "/" + folderName, nil
}

func (fs *FolderService) checkCircularReference(userID, folderID, newParentID primitive.ObjectID) error {
	// Check if newParentID is a descendant of folderID
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	currentParentID := newParentID
	for !currentParentID.IsZero() {
		if currentParentID == folderID {
			return errors.New("cannot move folder into its own subfolder")
		}

		var parent models.Folder
		err := fs.folderCollection.FindOne(ctx, bson.M{"_id": currentParentID}).Decode(&parent)
		if err != nil {
			break
		}

		if parent.ParentID != nil {
			currentParentID = *parent.ParentID
		} else {
			break
		}
	}

	return nil
}

func (fs *FolderService) getFolderSubfolders(ctx context.Context, userID, folderID primitive.ObjectID, sortBy, sortOrder string) ([]models.Folder, error) {
	sort := bson.M{sortBy: 1}
	if sortOrder == "desc" {
		sort = bson.M{sortBy: -1}
	}

	cursor, err := fs.folderCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"parent_id":  folderID,
			"is_deleted": false,
		},
		options.Find().SetSort(sort),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subfolders []models.Folder
	if err = cursor.All(ctx, &subfolders); err != nil {
		return nil, err
	}

	return subfolders, nil
}

func (fs *FolderService) getFolderFiles(ctx context.Context, userID, folderID primitive.ObjectID, page, limit int, sortBy, sortOrder string) ([]models.File, int, error) {
	sort := bson.M{sortBy: 1}
	if sortOrder == "desc" {
		sort = bson.M{sortBy: -1}
	}

	skip := (page - 1) * limit

	cursor, err := fs.fileCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"folder_id":  folderID,
			"is_deleted": false,
		},
		options.Find().
			SetSort(sort).
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
	total, err := fs.fileCollection.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"folder_id":  folderID,
		"is_deleted": false,
	})
	if err != nil {
		return nil, 0, err
	}

	return files, int(total), nil
}

func (fs *FolderService) getRootFolders(ctx context.Context, userID primitive.ObjectID) ([]models.Folder, error) {
	cursor, err := fs.folderCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"parent_id":  bson.M{"$exists": false},
			"is_deleted": false,
		},
		options.Find().SetSort(bson.M{"name": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

func (fs *FolderService) getRootFiles(ctx context.Context, userID primitive.ObjectID, page, limit int) ([]models.File, int, error) {
	skip := (page - 1) * limit

	cursor, err := fs.fileCollection.Find(ctx,
		bson.M{
			"user_id":    userID,
			"folder_id":  bson.M{"$exists": false},
			"is_deleted": false,
		},
		options.Find().
			SetSort(bson.M{"name": 1}).
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
	total, err := fs.fileCollection.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"folder_id":  bson.M{"$exists": false},
		"is_deleted": false,
	})
	if err != nil {
		return nil, 0, err
	}

	return files, int(total), nil
}

func (fs *FolderService) calculateFolderStats(ctx context.Context, userID, folderID primitive.ObjectID) (map[string]interface{}, error) {
	// Count subfolders
	subfoldersCount, err := fs.folderCollection.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"parent_id":  folderID,
		"is_deleted": false,
	})
	if err != nil {
		return nil, err
	}

	// Count files
	filesCount, err := fs.fileCollection.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"folder_id":  folderID,
		"is_deleted": false,
	})
	if err != nil {
		return nil, err
	}

	// Calculate total size
	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id":    userID,
			"folder_id":  folderID,
			"is_deleted": false,
		}},
		{"$group": bson.M{
			"_id":        nil,
			"total_size": bson.M{"$sum": "$size"},
		}},
	}

	cursor, err := fs.fileCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil {
		return nil, err
	}

	totalSize := int64(0)
	if len(result) > 0 {
		if size, ok := result[0]["total_size"].(int64); ok {
			totalSize = size
		}
	}

	return map[string]interface{}{
		"subfolders_count": subfoldersCount,
		"files_count":      filesCount,
		"total_size":       totalSize,
		"formatted_size":   utils.FormatFileSize(totalSize),
	}, nil
}

func (fs *FolderService) calculateFolderSizeRecursive(ctx context.Context, userID, folderID primitive.ObjectID) (int64, error) {
	// Get direct files size
	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id":    userID,
			"folder_id":  folderID,
			"is_deleted": false,
		}},
		{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$size"},
		}},
	}

	cursor, err := fs.fileCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil {
		return 0, err
	}

	directSize := int64(0)
	if len(result) > 0 {
		if size, ok := result[0]["total"].(int64); ok {
			directSize = size
		}
	}

	// Get subfolders
	subfolders, err := fs.getFolderSubfolders(ctx, userID, folderID, "name", "asc")
	if err != nil {
		return directSize, err
	}

	// Calculate subfolder sizes recursively
	for _, subfolder := range subfolders {
		subfolderSize, err := fs.calculateFolderSizeRecursive(ctx, userID, subfolder.ID)
		if err != nil {
			continue // Skip on error
		}
		directSize += subfolderSize
	}

	return directSize, nil
}

func (fs *FolderService) buildFolderTree(ctx context.Context, userID primitive.ObjectID, rootFolder *models.Folder, maxDepth int) (*models.FolderTree, error) {
	if maxDepth <= 0 {
		return nil, nil
	}

	var tree *models.FolderTree
	var folderID primitive.ObjectID

	if rootFolder != nil {
		tree = &models.FolderTree{Folder: rootFolder}
		folderID = rootFolder.ID
	} else {
		// Root level
		tree = &models.FolderTree{}
	}

	// Get subfolders
	var subfolders []models.Folder
	var err error

	if rootFolder == nil {
		// Get root folders
		subfolders, err = fs.getRootFolders(ctx, userID)
	} else {
		subfolders, err = fs.getFolderSubfolders(ctx, userID, folderID, "name", "asc")
	}

	if err != nil {
		return tree, err
	}

	// Build child trees
	for _, subfolder := range subfolders {
		childTree, err := fs.buildFolderTree(ctx, userID, &subfolder, maxDepth-1)
		if err != nil {
			continue // Skip on error
		}
		tree.Children = append(tree.Children, childTree)
	}

	// Get files if this is a leaf or near-leaf node
	if maxDepth <= 2 {
		var files []models.File
		if rootFolder == nil {
			files, _, err = fs.getRootFiles(ctx, userID, 1, 20)
		} else {
			files, _, err = fs.getFolderFiles(ctx, userID, folderID, 1, 20, "name", "asc")
		}
		if err == nil {
			// Convert []models.File to []*models.File
			filePointers := make([]*models.File, len(files))
			for i := range files {
				filePointers[i] = &files[i]
			}
			tree.Files = filePointers
		}
	}

	return tree, nil
}

func (fs *FolderService) updateUserFolderCount(userID primitive.ObjectID, change int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fs.userCollection.UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$inc": bson.M{"folders_count": change}},
	)
}

func (fs *FolderService) deleteAllFolderContents(ctx context.Context, userID, folderID primitive.ObjectID) error {
	// Delete all files in folder
	_, err := fs.fileCollection.DeleteMany(ctx, bson.M{
		"user_id":   userID,
		"folder_id": folderID,
	})
	if err != nil {
		return err
	}

	// Get all subfolders
	subfolders, err := fs.getFolderSubfolders(ctx, userID, folderID, "name", "asc")
	if err != nil {
		return err
	}

	// Recursively delete subfolder contents
	for _, subfolder := range subfolders {
		if err := fs.deleteAllFolderContents(ctx, userID, subfolder.ID); err != nil {
			continue // Continue with other folders
		}
	}

	// Delete all subfolders
	_, err = fs.folderCollection.DeleteMany(ctx, bson.M{
		"user_id":   userID,
		"parent_id": folderID,
	})

	return err
}

func (fs *FolderService) softDeleteSubfolders(ctx context.Context, userID, folderID primitive.ObjectID) {
	// Mark all subfolders as deleted
	fs.folderCollection.UpdateMany(ctx,
		bson.M{
			"user_id":   userID,
			"parent_id": folderID,
		},
		bson.M{"$set": bson.M{
			"is_deleted": true,
			"deleted_at": time.Now(),
		}},
	)

	// Get subfolders for recursive deletion
	subfolders, err := fs.getFolderSubfolders(ctx, userID, folderID, "name", "asc")
	if err != nil {
		return
	}

	// Recursively soft delete
	for _, subfolder := range subfolders {
		fs.softDeleteSubfolders(ctx, userID, subfolder.ID)
	}
}

func (fs *FolderService) copyFolderContentsAsync(userID, sourceFolderID, destFolderID primitive.ObjectID) {
	// Implementation for async folder copying
	// This would copy all files and subfolders recursively
}

func (fs *FolderService) updateSubfolderPathsAsync(userID, folderID primitive.ObjectID, newBasePath string) {
	// Implementation for async path updates
	// This would update all subfolder paths recursively
}
