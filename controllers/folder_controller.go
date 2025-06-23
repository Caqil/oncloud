package controllers

import (
	"oncloud/models"
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FolderController struct {
	folderService *services.FolderService
	fileService   *services.FileService
}

func NewFolderController() *FolderController {
	return &FolderController{
		folderService: services.NewFolderService(),
		fileService:   services.NewFileService(),
	}
}

// GetFolders returns list of user folders
func (fc *FolderController) GetFolders(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	parentID := c.Query("parent_id")
	search := c.Query("search")

	folders, total, err := fc.folderService.GetUserFolders(user.ID, parentID, search, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get folders")
		return
	}

	utils.PaginatedResponse(c, "Folders retrieved successfully", folders, page, limit, total)
}

// GetFolder returns a specific folder
func (fc *FolderController) GetFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	folder, err := fc.folderService.GetUserFolder(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "Folder not found")
		return
	}

	utils.SuccessResponse(c, "Folder retrieved successfully", folder)
}

// CreateFolder creates a new folder
func (fc *FolderController) CreateFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req models.FolderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	folder, err := fc.folderService.CreateFolder(user.ID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create folder")
		return
	}

	utils.CreatedResponse(c, "Folder created successfully", folder)
}

// UpdateFolder updates folder metadata
func (fc *FolderController) UpdateFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Color       string   `json:"color"`
		Icon        string   `json:"icon"`
		Tags        []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	folder, err := fc.folderService.UpdateFolder(user.ID, objID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update folder")
		return
	}

	utils.SuccessResponse(c, "Folder updated successfully", folder)
}

// DeleteFolder deletes a folder (soft delete)
func (fc *FolderController) DeleteFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.DeleteFolder(user.ID, objID, false) // Soft delete
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete folder")
		return
	}

	utils.SuccessResponse(c, "Folder deleted successfully", nil)
}

// RestoreFolder restores a deleted folder
func (fc *FolderController) RestoreFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.RestoreFolder(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to restore folder")
		return
	}

	utils.SuccessResponse(c, "Folder restored successfully", nil)
}

// PermanentDelete permanently deletes a folder
func (fc *FolderController) PermanentDelete(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.DeleteFolder(user.ID, objID, true) // Permanent delete
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to permanently delete folder")
		return
	}

	utils.SuccessResponse(c, "Folder permanently deleted", nil)
}

// GetFolderContents returns folder contents (files and subfolders)
func (fc *FolderController) GetFolderContents(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	sortBy := c.DefaultQuery("sort", "name")
	sortOrder := c.DefaultQuery("order", "asc")

	objID, _ := utils.StringToObjectID(folderID)
	contents, err := fc.folderService.GetFolderContents(user.ID, objID, page, limit, sortBy, sortOrder)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get folder contents")
		return
	}

	utils.SuccessResponse(c, "Folder contents retrieved successfully", contents)
}

// GetFolderTree returns hierarchical folder tree
func (fc *FolderController) GetFolderTree(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	var objID primitive.ObjectID
	var err error

	if folderID != "" && folderID != "root" {
		if !utils.IsValidObjectID(folderID) {
			utils.BadRequestResponse(c, "Invalid folder ID")
			return
		}
		objID, _ = utils.StringToObjectID(folderID)
	}

	tree, err := fc.folderService.GetFolderTree(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get folder tree")
		return
	}

	utils.SuccessResponse(c, "Folder tree retrieved successfully", tree)
}

// GetBreadcrumb returns folder breadcrumb path
func (fc *FolderController) GetBreadcrumb(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	breadcrumb, err := fc.folderService.GetBreadcrumb(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get breadcrumb")
		return
	}

	utils.SuccessResponse(c, "Breadcrumb retrieved successfully", breadcrumb)
}

// GetRootFolder returns user's root folder
func (fc *FolderController) GetRootFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	contents, err := fc.folderService.GetRootFolderContents(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get root folder contents")
		return
	}

	utils.SuccessResponse(c, "Root folder contents retrieved successfully", contents)
}

// GetRecentFolders returns recently accessed folders
func (fc *FolderController) GetRecentFolders(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	folders, err := fc.folderService.GetRecentFolders(user.ID, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get recent folders")
		return
	}

	utils.SuccessResponse(c, "Recent folders retrieved successfully", folders)
}

// GetFavoriteFolders returns user's favorite folders
func (fc *FolderController) GetFavoriteFolders(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	folders, total, err := fc.folderService.GetFavoriteFolders(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get favorite folders")
		return
	}

	utils.PaginatedResponse(c, "Favorite folders retrieved successfully", folders, page, limit, total)
}

// GetDeletedFolders returns deleted folders (trash)
func (fc *FolderController) GetDeletedFolders(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	folders, total, err := fc.folderService.GetDeletedFolders(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get deleted folders")
		return
	}

	utils.PaginatedResponse(c, "Deleted folders retrieved successfully", folders, page, limit, total)
}

// Folder operations
func (fc *FolderController) CopyFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	var req struct {
		DestParentID string `json:"dest_parent_id"`
		NewName      string `json:"new_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	newFolder, err := fc.folderService.CopyFolder(user.ID, objID, req.DestParentID, req.NewName)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to copy folder")
		return
	}

	utils.CreatedResponse(c, "Folder copied successfully", newFolder)
}

func (fc *FolderController) MoveFolder(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	var req struct {
		DestParentID string `json:"dest_parent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.MoveFolder(user.ID, objID, req.DestParentID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to move folder")
		return
	}

	utils.SuccessResponse(c, "Folder moved successfully", nil)
}

func (fc *FolderController) AddToFavorites(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.ToggleFavorite(user.ID, objID, true)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to add to favorites")
		return
	}

	utils.SuccessResponse(c, "Folder added to favorites", nil)
}

func (fc *FolderController) RemoveFromFavorites(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.ToggleFavorite(user.ID, objID, false)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to remove from favorites")
		return
	}

	utils.SuccessResponse(c, "Folder removed from favorites", nil)
}

func (fc *FolderController) UpdateTags(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	var req struct {
		Tags []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.UpdateTags(user.ID, objID, req.Tags)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update tags")
		return
	}

	utils.SuccessResponse(c, "Tags updated successfully", nil)
}

// Folder sharing
func (fc *FolderController) CreateShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	var req models.ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	share, err := fc.folderService.CreateShare(user.ID, objID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create share")
		return
	}

	utils.CreatedResponse(c, "Folder share created successfully", share)
}

func (fc *FolderController) GetShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	share, err := fc.folderService.GetShare(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "Share not found")
		return
	}

	utils.SuccessResponse(c, "Folder share retrieved successfully", share)
}

func (fc *FolderController) UpdateShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	var req models.ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	share, err := fc.folderService.UpdateShare(user.ID, objID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update share")
		return
	}

	utils.SuccessResponse(c, "Folder share updated successfully", share)
}

func (fc *FolderController) DeleteShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	err := fc.folderService.DeleteShare(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete share")
		return
	}

	utils.SuccessResponse(c, "Folder share deleted successfully", nil)
}

func (fc *FolderController) GetShareURL(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	shareURL, err := fc.folderService.GetShareURL(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get share URL")
		return
	}

	utils.SuccessResponse(c, "Share URL retrieved successfully", gin.H{
		"share_url": shareURL,
	})
}

// Folder statistics
func (fc *FolderController) GetFolderStats(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	stats, err := fc.folderService.GetFolderStats(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get folder stats")
		return
	}

	utils.SuccessResponse(c, "Folder stats retrieved successfully", stats)
}

func (fc *FolderController) GetFolderSize(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	folderID := c.Param("id")
	if !utils.IsValidObjectID(folderID) {
		utils.BadRequestResponse(c, "Invalid folder ID")
		return
	}

	objID, _ := utils.StringToObjectID(folderID)
	size, err := fc.folderService.GetFolderSize(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get folder size")
		return
	}

	utils.SuccessResponse(c, "Folder size retrieved successfully", gin.H{
		"size":           size,
		"formatted_size": utils.FormatFileSize(size),
	})
}

// Bulk operations
func (fc *FolderController) BulkDelete(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FolderIDs []string `json:"folder_ids" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all folder IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FolderIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid folder ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.folderService.BulkDeleteFolders(user.ID, objIDs)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete folders")
		return
	}

	utils.SuccessResponse(c, "Bulk delete completed", results)
}

func (fc *FolderController) BulkMove(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FolderIDs    []string `json:"folder_ids" validate:"required"`
		DestParentID string   `json:"dest_parent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all folder IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FolderIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid folder ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.folderService.BulkMoveFolders(user.ID, objIDs, req.DestParentID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to move folders")
		return
	}

	utils.SuccessResponse(c, "Bulk move completed", results)
}

func (fc *FolderController) BulkCopy(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FolderIDs    []string `json:"folder_ids" validate:"required"`
		DestParentID string   `json:"dest_parent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all folder IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FolderIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid folder ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.folderService.BulkCopyFolders(user.ID, objIDs, req.DestParentID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to copy folders")
		return
	}

	utils.SuccessResponse(c, "Bulk copy completed", results)
}

func (fc *FolderController) BulkShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FolderIDs []string            `json:"folder_ids" validate:"required"`
		ShareData models.ShareRequest `json:"share_data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all folder IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FolderIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid folder ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.folderService.BulkShareFolders(user.ID, objIDs, &req.ShareData)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to share folders")
		return
	}

	utils.SuccessResponse(c, "Bulk share completed", results)
}

// Public folder access
func (fc *FolderController) PublicFolderAccess(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Access token is required")
		return
	}

	folder, err := fc.folderService.GetPublicFolderContents(token)
	if err != nil {
		utils.NotFoundResponse(c, "Folder not found or access denied")
		return
	}

	utils.SuccessResponse(c, "Public folder accessed successfully", folder)
}

func (fc *FolderController) SharedFolderAccess(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Share token is required")
		return
	}

	folder, err := fc.folderService.GetSharedFolderContents(token)
	if err != nil {
		utils.NotFoundResponse(c, "Folder not found or access denied")
		return
	}

	utils.SuccessResponse(c, "Shared folder accessed successfully", folder)
}
