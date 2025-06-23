package controllers

import (
	"oncloud/models"
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService *services.UserService
	fileService *services.FileService
}

func NewUserController() *UserController {
	return &UserController{
		userService: services.NewUserService(),
		fileService: services.NewFileService(),
	}
}

// GetProfile returns user profile
func (uc *UserController) GetProfile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	// Get user plan
	plan, err := uc.userService.GetUserPlan(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user plan")
		return
	}

	profile := &models.UserProfile{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Avatar:    user.Avatar,
		Plan:      plan,
	}

	utils.SuccessResponse(c, "Profile retrieved successfully", profile)
}

// UpdateProfile updates user profile
func (uc *UserController) UpdateProfile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FirstName string `json:"first_name" validate:"required"`
		LastName  string `json:"last_name" validate:"required"`
		Phone     string `json:"phone"`
		Country   string `json:"country"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	updatedUser, err := uc.userService.UpdateProfile(user.ID, req.FirstName, req.LastName, req.Phone, req.Country)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update profile")
		return
	}

	utils.SuccessResponse(c, "Profile updated successfully", updatedUser)
}

// UploadAvatar handles avatar upload
func (uc *UserController) UploadAvatar(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		utils.BadRequestResponse(c, "No avatar file provided")
		return
	}
	defer file.Close()

	// Validate file type and size
	if !utils.IsImageFile(header.Filename) {
		utils.BadRequestResponse(c, "Only image files are allowed")
		return
	}

	if header.Size > 5*1024*1024 { // 5MB limit
		utils.BadRequestResponse(c, "Avatar size must be less than 5MB")
		return
	}

	avatarURL, err := uc.userService.UploadAvatar(user.ID, file, header)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to upload avatar")
		return
	}

	utils.SuccessResponse(c, "Avatar uploaded successfully", gin.H{
		"avatar_url": avatarURL,
	})
}

// DeleteAvatar deletes user avatar
func (uc *UserController) DeleteAvatar(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	err := uc.userService.DeleteAvatar(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete avatar")
		return
	}

	utils.SuccessResponse(c, "Avatar deleted successfully", nil)
}

// GetUserStats returns user statistics
func (uc *UserController) GetUserStats(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	stats, err := uc.userService.GetUserStats(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user stats")
		return
	}

	utils.SuccessResponse(c, "User stats retrieved successfully", stats)
}

// GetDashboard returns dashboard data
func (uc *UserController) GetDashboard(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	dashboard, err := uc.userService.GetDashboardData(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get dashboard data")
		return
	}

	utils.SuccessResponse(c, "Dashboard data retrieved successfully", dashboard)
}

// GetActivity returns user activity log
func (uc *UserController) GetActivity(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	activities, total, err := uc.userService.GetUserActivity(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user activity")
		return
	}

	utils.PaginatedResponse(c, "User activity retrieved successfully", activities, page, limit, total)
}

// GetNotifications returns user notifications
func (uc *UserController) GetNotifications(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	notifications, total, err := uc.userService.GetNotifications(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get notifications")
		return
	}

	utils.PaginatedResponse(c, "Notifications retrieved successfully", notifications, page, limit, total)
}

// MarkNotificationRead marks notification as read
func (uc *UserController) MarkNotificationRead(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	notificationID := c.Param("id")
	if !utils.IsValidObjectID(notificationID) {
		utils.BadRequestResponse(c, "Invalid notification ID")
		return
	}

	objID, _ := utils.StringToObjectID(notificationID)
	err := uc.userService.MarkNotificationRead(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to mark notification as read")
		return
	}

	utils.SuccessResponse(c, "Notification marked as read", nil)
}

// GetSettings returns user settings
func (uc *UserController) GetSettings(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	settings, err := uc.userService.GetUserSettings(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user settings")
		return
	}

	utils.SuccessResponse(c, "User settings retrieved successfully", settings)
}

// UpdateSettings updates user settings
func (uc *UserController) UpdateSettings(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		utils.BadRequestResponse(c, "Invalid settings data")
		return
	}

	err := uc.userService.UpdateUserSettings(user.ID, settings)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update user settings")
		return
	}

	utils.SuccessResponse(c, "User settings updated successfully", nil)
}

// GetActiveSessions returns user's active sessions
func (uc *UserController) GetActiveSessions(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	sessions, err := uc.userService.GetActiveSessions(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get active sessions")
		return
	}

	utils.SuccessResponse(c, "Active sessions retrieved successfully", sessions)
}

// RevokeSession revokes a user session
func (uc *UserController) RevokeSession(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	sessionID := c.Param("id")
	err := uc.userService.RevokeSession(user.ID, sessionID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to revoke session")
		return
	}

	utils.SuccessResponse(c, "Session revoked successfully", nil)
}

// API Keys management
func (uc *UserController) GetAPIKeys(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	apiKeys, err := uc.userService.GetAPIKeys(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get API keys")
		return
	}

	utils.SuccessResponse(c, "API keys retrieved successfully", apiKeys)
}

func (uc *UserController) CreateAPIKey(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Name        string   `json:"name" validate:"required"`
		Permissions []string `json:"permissions"`
		ExpiresAt   *int64   `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	apiKey, err := uc.userService.CreateAPIKey(user.ID, req.Name, req.Permissions, req.ExpiresAt)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create API key")
		return
	}

	utils.CreatedResponse(c, "API key created successfully", apiKey)
}

func (uc *UserController) UpdateAPIKey(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	keyID := c.Param("id")
	if !utils.IsValidObjectID(keyID) {
		utils.BadRequestResponse(c, "Invalid API key ID")
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		IsActive    *bool    `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(keyID)
	err := uc.userService.UpdateAPIKey(user.ID, objID, req.Name, req.Permissions, req.IsActive)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update API key")
		return
	}

	utils.SuccessResponse(c, "API key updated successfully", nil)
}

func (uc *UserController) DeleteAPIKey(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	keyID := c.Param("id")
	if !utils.IsValidObjectID(keyID) {
		utils.BadRequestResponse(c, "Invalid API key ID")
		return
	}

	objID, _ := utils.StringToObjectID(keyID)
	err := uc.userService.DeleteAPIKey(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete API key")
		return
	}

	utils.SuccessResponse(c, "API key deleted successfully", nil)
}

// 2FA methods
func (uc *UserController) Get2FAStatus(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	status, err := uc.userService.Get2FAStatus(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get 2FA status")
		return
	}

	utils.SuccessResponse(c, "2FA status retrieved successfully", status)
}

func (uc *UserController) Enable2FA(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	qrCode, secret, err := uc.userService.Enable2FA(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to enable 2FA")
		return
	}

	utils.SuccessResponse(c, "2FA enabled successfully", gin.H{
		"qr_code": qrCode,
		"secret":  secret,
	})
}

func (uc *UserController) Verify2FA(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Code string `json:"code" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	backupCodes, err := uc.userService.Verify2FA(user.ID, req.Code)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid 2FA code")
		return
	}

	utils.SuccessResponse(c, "2FA verified successfully", gin.H{
		"backup_codes": backupCodes,
	})
}

func (uc *UserController) Disable2FA(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Code string `json:"code" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := uc.userService.Disable2FA(user.ID, req.Code)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid 2FA code")
		return
	}

	utils.SuccessResponse(c, "2FA disabled successfully", nil)
}

func (uc *UserController) GetBackupCodes(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	codes, err := uc.userService.GetBackupCodes(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get backup codes")
		return
	}

	utils.SuccessResponse(c, "Backup codes retrieved successfully", codes)
}

func (uc *UserController) RegenerateBackupCodes(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	codes, err := uc.userService.RegenerateBackupCodes(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to regenerate backup codes")
		return
	}

	utils.SuccessResponse(c, "Backup codes regenerated successfully", codes)
}
