package controllers

import (
	"net/http"
	"oncloud/models"
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FileController struct {
	fileService    *services.FileService
	storageService *services.StorageService
}

func NewFileController() *FileController {
	return &FileController{
		fileService:    services.NewFileService(),
		storageService: services.NewStorageService(),
	}
}

// GetFiles returns list of user files
func (fc *FileController) GetFiles(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	folderID := c.Query("folder_id")
	search := c.Query("search")
	fileType := c.Query("type")
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	filters := &services.FileFilters{
		FolderID:  folderID,
		Search:    search,
		FileType:  fileType,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	files, total, err := fc.fileService.GetUserFiles(user.ID, page, limit, filters)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get files")
		return
	}

	utils.PaginatedResponse(c, "Files retrieved successfully", files, page, limit, total)
}

// GetFile returns a specific file
func (fc *FileController) GetFile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	file, err := fc.fileService.GetUserFile(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "File not found")
		return
	}

	utils.SuccessResponse(c, "File retrieved successfully", file)
}

// Upload handles file upload
func (fc *FileController) Upload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	// Parse form data
	var req models.FileUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid form data")
		return
	}

	// Get uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.BadRequestResponse(c, "No file provided")
		return
	}

	// Check user limits
	plan, err := fc.fileService.GetUserPlan(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user plan")
		return
	}

	if err := fc.fileService.CheckUploadLimits(user, plan, fileHeader.Size); err != nil {
		utils.ForbiddenResponse(c, err.Error())
		return
	}

	// Upload file
	file, err := fc.fileService.UploadFile(user.ID, fileHeader, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to upload file")
		return
	}

	utils.FileUploadResponse(c, "File uploaded successfully", file, "")
}

// ChunkUpload handles chunked file upload
func (fc *FileController) ChunkUpload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		UploadID    string `form:"upload_id" validate:"required"`
		ChunkNumber int    `form:"chunk_number" validate:"required"`
		TotalChunks int    `form:"total_chunks" validate:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid form data")
		return
	}

	chunk, err := c.FormFile("chunk")
	if err != nil {
		utils.BadRequestResponse(c, "No chunk provided")
		return
	}

	result, err := fc.fileService.UploadChunk(user.ID, req.UploadID, req.ChunkNumber, req.TotalChunks, chunk)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to upload chunk")
		return
	}

	utils.SuccessResponse(c, "Chunk uploaded successfully", result)
}

// CompleteChunkUpload completes a chunked upload
func (fc *FileController) CompleteChunkUpload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		UploadID string `json:"upload_id" validate:"required"`
		FileName string `json:"file_name" validate:"required"`
		FolderID string `json:"folder_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	file, err := fc.fileService.CompleteChunkUpload(user.ID, req.UploadID, req.FileName, req.FolderID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to complete upload")
		return
	}

	utils.SuccessResponse(c, "Upload completed successfully", file)
}

// UpdateFile updates file metadata
func (fc *FileController) UpdateFile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Metadata    map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	file, err := fc.fileService.UpdateFile(user.ID, objID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update file")
		return
	}

	utils.SuccessResponse(c, "File updated successfully", file)
}

// DeleteFile deletes a file (soft delete)
func (fc *FileController) DeleteFile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.DeleteFile(user.ID, objID, false) // Soft delete
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete file")
		return
	}

	utils.SuccessResponse(c, "File deleted successfully", nil)
}

// RestoreFile restores a deleted file
func (fc *FileController) RestoreFile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.RestoreFile(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to restore file")
		return
	}

	utils.SuccessResponse(c, "File restored successfully", nil)
}

// PermanentDelete permanently deletes a file
func (fc *FileController) PermanentDelete(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.DeleteFile(user.ID, objID, true) // Permanent delete
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to permanently delete file")
		return
	}

	utils.SuccessResponse(c, "File permanently deleted", nil)
}

// Download handles file download
func (fc *FileController) Download(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	downloadURL, err := fc.fileService.GetDownloadURL(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate download URL")
		return
	}

	// Increment download counter
	fc.fileService.IncrementDownloadCount(objID)

	c.Redirect(http.StatusFound, downloadURL)
}

// Stream handles file streaming
func (fc *FileController) Stream(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.StreamFile(user.ID, objID, c.Writer, c.Request)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to stream file")
		return
	}
}

// Preview generates file preview
func (fc *FileController) Preview(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	previewURL, err := fc.fileService.GeneratePreview(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate preview")
		return
	}

	utils.SuccessResponse(c, "Preview generated successfully", gin.H{
		"preview_url": previewURL,
	})
}

// GetThumbnail returns file thumbnail
func (fc *FileController) GetThumbnail(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	thumbnailURL, err := fc.fileService.GetThumbnail(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "Thumbnail not found")
		return
	}

	c.Redirect(http.StatusFound, thumbnailURL)
}

// GenerateThumbnail generates thumbnail for file
func (fc *FileController) GenerateThumbnail(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	thumbnailURL, err := fc.fileService.GenerateThumbnail(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate thumbnail")
		return
	}

	utils.SuccessResponse(c, "Thumbnail generated successfully", gin.H{
		"thumbnail_url": thumbnailURL,
	})
}

// File sharing methods
func (fc *FileController) CreateShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req models.ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	share, err := fc.fileService.CreateShare(user.ID, objID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create share")
		return
	}

	utils.CreatedResponse(c, "Share created successfully", share)
}

func (fc *FileController) GetShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	share, err := fc.fileService.GetShare(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "Share not found")
		return
	}

	utils.SuccessResponse(c, "Share retrieved successfully", share)
}

func (fc *FileController) UpdateShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req models.ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	share, err := fc.fileService.UpdateShare(user.ID, objID, &req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update share")
		return
	}

	utils.SuccessResponse(c, "Share updated successfully", share)
}

func (fc *FileController) DeleteShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.DeleteShare(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete share")
		return
	}

	utils.SuccessResponse(c, "Share deleted successfully", nil)
}

func (fc *FileController) GetShareURL(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	shareURL, err := fc.fileService.GetShareURL(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get share URL")
		return
	}

	utils.SuccessResponse(c, "Share URL retrieved successfully", gin.H{
		"share_url": shareURL,
	})
}

// File operations
func (fc *FileController) CopyFile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		DestFolderID string `json:"dest_folder_id"`
		NewName      string `json:"new_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	newFile, err := fc.fileService.CopyFile(user.ID, objID, req.DestFolderID, req.NewName)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to copy file")
		return
	}

	utils.CreatedResponse(c, "File copied successfully", newFile)
}

func (fc *FileController) MoveFile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		DestFolderID string `json:"dest_folder_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.MoveFile(user.ID, objID, req.DestFolderID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to move file")
		return
	}

	utils.SuccessResponse(c, "File moved successfully", nil)
}

func (fc *FileController) AddToFavorites(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.ToggleFavorite(user.ID, objID, true)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to add to favorites")
		return
	}

	utils.SuccessResponse(c, "File added to favorites", nil)
}

func (fc *FileController) RemoveFromFavorites(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.ToggleFavorite(user.ID, objID, false)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to remove from favorites")
		return
	}

	utils.SuccessResponse(c, "File removed from favorites", nil)
}

func (fc *FileController) UpdateTags(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		Tags []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fc.fileService.UpdateTags(user.ID, objID, req.Tags)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update tags")
		return
	}

	utils.SuccessResponse(c, "Tags updated successfully", nil)
}

// File versions
func (fc *FileController) GetVersions(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	versions, err := fc.fileService.GetFileVersions(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get file versions")
		return
	}

	utils.SuccessResponse(c, "File versions retrieved successfully", versions)
}

func (fc *FileController) CreateVersion(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.BadRequestResponse(c, "No file provided")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	version, err := fc.fileService.CreateFileVersion(user.ID, objID, fileHeader)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create file version")
		return
	}

	utils.CreatedResponse(c, "File version created successfully", version)
}

func (fc *FileController) GetVersion(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	versionStr := c.Param("version")

	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid version number")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	fileVersion, err := fc.fileService.GetFileVersion(user.ID, objID, version)
	if err != nil {
		utils.NotFoundResponse(c, "File version not found")
		return
	}

	utils.SuccessResponse(c, "File version retrieved successfully", fileVersion)
}

func (fc *FileController) RestoreVersion(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	versionStr := c.Param("version")

	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid version number")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err = fc.fileService.RestoreFileVersion(user.ID, objID, version)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to restore file version")
		return
	}

	utils.SuccessResponse(c, "File version restored successfully", nil)
}

func (fc *FileController) DeleteVersion(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	fileID := c.Param("id")
	versionStr := c.Param("version")

	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid version number")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err = fc.fileService.DeleteFileVersion(user.ID, objID, version)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete file version")
		return
	}

	utils.SuccessResponse(c, "File version deleted successfully", nil)
}

// Bulk operations
func (fc *FileController) BulkDelete(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileIDs []string `json:"file_ids" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all file IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.fileService.BulkDeleteFiles(user.ID, objIDs)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete files")
		return
	}

	utils.SuccessResponse(c, "Bulk delete completed", results)
}

func (fc *FileController) BulkMove(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileIDs      []string `json:"file_ids" validate:"required"`
		DestFolderID string   `json:"dest_folder_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all file IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.fileService.BulkMoveFiles(user.ID, objIDs, req.DestFolderID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to move files")
		return
	}

	utils.SuccessResponse(c, "Bulk move completed", results)
}

func (fc *FileController) BulkCopy(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileIDs      []string `json:"file_ids" validate:"required"`
		DestFolderID string   `json:"dest_folder_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all file IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.fileService.BulkCopyFiles(user.ID, objIDs, req.DestFolderID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to copy files")
		return
	}

	utils.SuccessResponse(c, "Bulk copy completed", results)
}

func (fc *FileController) BulkDownload(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileIDs []string `json:"file_ids" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all file IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	zipURL, err := fc.fileService.CreateBulkDownload(user.ID, objIDs)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create bulk download")
		return
	}

	utils.SuccessResponse(c, "Bulk download created successfully", gin.H{
		"download_url": zipURL,
	})
}

func (fc *FileController) BulkShare(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FileIDs   []string            `json:"file_ids" validate:"required"`
		ShareData models.ShareRequest `json:"share_data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate all file IDs
	var objIDs []primitive.ObjectID
	for _, id := range req.FileIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid file ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	results, err := fc.fileService.BulkShareFiles(user.ID, objIDs, &req.ShareData)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to share files")
		return
	}

	utils.SuccessResponse(c, "Bulk share completed", results)
}

// Public file access (no authentication required)
func (fc *FileController) PublicDownload(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Access token is required")
		return
	}

	downloadURL, err := fc.fileService.GetPublicDownloadURL(token)
	if err != nil {
		utils.NotFoundResponse(c, "File not found or access denied")
		return
	}

	c.Redirect(http.StatusFound, downloadURL)
}

func (fc *FileController) SharedDownload(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Share token is required")
		return
	}

	downloadURL, err := fc.fileService.GetSharedDownloadURL(token)
	if err != nil {
		utils.NotFoundResponse(c, "File not found or access denied")
		return
	}

	c.Redirect(http.StatusFound, downloadURL)
}

func (fc *FileController) VerifySharePassword(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Share token is required")
		return
	}

	var req struct {
		Password string `json:"password" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	access, err := fc.fileService.VerifySharePassword(token, req.Password)
	if err != nil {
		utils.UnauthorizedResponse(c, "Invalid password")
		return
	}

	utils.SuccessResponse(c, "Password verified successfully", access)
}
