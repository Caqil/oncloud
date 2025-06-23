package controllers

import (
	"oncloud/services"
	"oncloud/utils"

	"github.com/gin-gonic/gin"
)

type SettingsController struct {
	settingsService *services.SettingsService
}

func NewSettingsController() *SettingsController {
	return &SettingsController{
		settingsService: services.NewSettingsService(),
	}
}

// GetSettings returns all system settings
func (sc *SettingsController) GetSettings(c *gin.Context) {
	group := c.Query("group")
	includePrivate := c.Query("include_private") == "true"

	settings, err := sc.settingsService.GetSettings(group, includePrivate)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get settings")
		return
	}

	utils.SuccessResponse(c, "Settings retrieved successfully", settings)
}

// UpdateSettings updates multiple settings at once
func (sc *SettingsController) UpdateSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := sc.settingsService.UpdateSettings(req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update settings")
		return
	}

	utils.SuccessResponse(c, "Settings updated successfully", nil)
}

// GetSettingGroups returns available setting groups
func (sc *SettingsController) GetSettingGroups(c *gin.Context) {
	groups, err := sc.settingsService.GetSettingGroups()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get setting groups")
		return
	}

	utils.SuccessResponse(c, "Setting groups retrieved successfully", groups)
}

// GetSettingsByGroup returns settings for a specific group
func (sc *SettingsController) GetSettingsByGroup(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		utils.BadRequestResponse(c, "Group parameter is required")
		return
	}

	settings, err := sc.settingsService.GetSettingsByGroup(group)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get settings by group")
		return
	}

	utils.SuccessResponse(c, "Settings retrieved successfully", settings)
}

// UpdateSetting updates a single setting
func (sc *SettingsController) UpdateSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		utils.BadRequestResponse(c, "Setting key is required")
		return
	}

	var req struct {
		Value interface{} `json:"value" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err := sc.settingsService.UpdateSetting(key, req.Value)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update setting")
		return
	}

	utils.SuccessResponse(c, "Setting updated successfully", nil)
}

// BackupSettings creates a backup of current settings
func (sc *SettingsController) BackupSettings(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	backup, err := sc.settingsService.BackupSettings(req.Name, req.Description)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to backup settings")
		return
	}

	utils.CreatedResponse(c, "Settings backup created successfully", backup)
}

// RestoreSettings restores settings from a backup
func (sc *SettingsController) RestoreSettings(c *gin.Context) {
	var req struct {
		BackupID string `json:"backup_id" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	if !utils.IsValidObjectID(req.BackupID) {
		utils.BadRequestResponse(c, "Invalid backup ID")
		return
	}

	backupObjID, _ := utils.StringToObjectID(req.BackupID)
	err := sc.settingsService.RestoreSettings(backupObjID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to restore settings")
		return
	}

	utils.SuccessResponse(c, "Settings restored successfully", nil)
}
