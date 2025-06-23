package controllers

import (
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

type FileAdminController struct {
	fileService  *services.FileService
	adminService *services.AdminService
}

func NewFileAdminController() *FileAdminController {
	return &FileAdminController{
		fileService:  services.NewFileService(),
		adminService: services.NewAdminService(),
	}
}

// GetFiles returns list of files for admin
func (fac *FileAdminController) GetFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")
	fileType := c.Query("type")
	userID := c.Query("user_id")
	status := c.Query("status") // active, deleted, reported
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	filters := &services.FileAdminFilters{
		Search:    search,
		FileType:  fileType,
		UserID:    userID,
		Status:    status,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	files, total, err := fac.fileService.GetFilesForAdmin(page, limit, filters)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get files")
		return
	}

	utils.PaginatedResponse(c, "Files retrieved successfully", files, page, limit, total)
}

// GetFile returns a specific file for admin
func (fac *FileAdminController) GetFile(c *gin.Context) {
	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	file, err := fac.fileService.GetFileForAdmin(objID)
	if err != nil {
		utils.NotFoundResponse(c, "File not found")
		return
	}

	utils.SuccessResponse(c, "File retrieved successfully", file)
}

// DeleteFile deletes a file (admin only)
func (fac *FileAdminController) DeleteFile(c *gin.Context) {
	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		Reason    string `json:"reason"`
		Permanent bool   `json:"permanent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fac.fileService.DeleteFileByAdmin(objID, req.Reason, req.Permanent)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete file")
		return
	}

	utils.SuccessResponse(c, "File deleted successfully", nil)
}

// RestoreFile restores a deleted file
func (fac *FileAdminController) RestoreFile(c *gin.Context) {
	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fac.fileService.RestoreFileByAdmin(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to restore file")
		return
	}

	utils.SuccessResponse(c, "File restored successfully", nil)
}

// ModerateFile handles file moderation actions
func (fac *FileAdminController) ModerateFile(c *gin.Context) {
	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		Action string `json:"action" validate:"required"` // approve, reject, flag, quarantine
		Reason string `json:"reason"`
		Notes  string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	err := fac.fileService.ModerateFile(objID, req.Action, req.Reason, req.Notes)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to moderate file")
		return
	}

	utils.SuccessResponse(c, "File moderated successfully", nil)
}

// GetReportedFiles returns list of reported files
func (fac *FileAdminController) GetReportedFiles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.DefaultQuery("status", "pending") // pending, reviewed, resolved

	reportedFiles, total, err := fac.fileService.GetReportedFiles(status, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get reported files")
		return
	}

	utils.PaginatedResponse(c, "Reported files retrieved successfully", reportedFiles, page, limit, total)
}

// ScanFile initiates virus/malware scan for a file
func (fac *FileAdminController) ScanFile(c *gin.Context) {
	fileID := c.Param("id")
	if !utils.IsValidObjectID(fileID) {
		utils.BadRequestResponse(c, "Invalid file ID")
		return
	}

	var req struct {
		ScanType string `json:"scan_type"` // virus, malware, content
		Force    bool   `json:"force"`     // force rescan even if already scanned
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(fileID)
	scanResult, err := fac.fileService.ScanFile(objID, req.ScanType, req.Force)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to scan file")
		return
	}

	utils.SuccessResponse(c, "File scan initiated successfully", scanResult)
}
