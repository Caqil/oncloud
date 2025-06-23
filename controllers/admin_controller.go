package controllers

import (
	"oncloud/models"
	"oncloud/services"
	"oncloud/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminController struct {
	adminService   *services.AdminService
	userService    *services.UserService
	fileService    *services.FileService
	planService    *services.PlanService
	storageService *services.StorageService
}

func NewAdminController() *AdminController {
	return &AdminController{
		adminService:   services.NewAdminService(),
		userService:    services.NewUserService(),
		fileService:    services.NewFileService(),
		planService:    services.NewPlanService(),
		storageService: services.NewStorageService(),
	}
}

// Admin authentication
func (ac *AdminController) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	admin, err := ac.adminService.Login(req.Email, req.Password)
	if err != nil {
		utils.UnauthorizedResponse(c, "Invalid credentials")
		return
	}

	if !admin.IsActive {
		utils.UnauthorizedResponse(c, "Admin account is deactivated")
		return
	}

	// Generate admin token
	token, err := utils.GenerateAdminToken(admin.ID, admin.Email, admin.Username, admin.Role, admin.Permissions)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate token")
		return
	}

	// Set session cookie for HTML panel
	c.SetCookie("admin_session", token, int(24*time.Hour.Seconds()), "/admin", "", false, true)

	utils.SuccessResponse(c, "Login successful", gin.H{
		"admin": admin,
		"token": token,
	})
}

func (ac *AdminController) Logout(c *gin.Context) {
	// Clear session cookie
	c.SetCookie("admin_session", "", -1, "/admin", "", false, true)
	utils.SuccessResponse(c, "Logout successful", nil)
}

// Plan management
func (ac *AdminController) GetPlans(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	includeInactive := c.Query("include_inactive") == "true"

	plans, total, err := ac.planService.GetPlansForAdmin(page, limit, includeInactive)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get plans")
		return
	}

	utils.PaginatedResponse(c, "Plans retrieved successfully", plans, page, limit, total)
}

func (ac *AdminController) GetPlan(c *gin.Context) {
	planID := c.Param("id")
	if !utils.IsValidObjectID(planID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	objID, _ := utils.StringToObjectID(planID)
	plan, err := ac.planService.GetPlanForAdmin(objID)
	if err != nil {
		utils.NotFoundResponse(c, "Plan not found")
		return
	}

	utils.SuccessResponse(c, "Plan retrieved successfully", plan)
}

func (ac *AdminController) CreatePlan(c *gin.Context) {
	var plan models.Plan
	if err := c.ShouldBindJSON(&plan); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&plan); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	createdPlan, err := ac.planService.CreatePlan(&plan)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create plan")
		return
	}

	utils.CreatedResponse(c, "Plan created successfully", createdPlan)
}

func (ac *AdminController) UpdatePlan(c *gin.Context) {
	planID := c.Param("id")
	if !utils.IsValidObjectID(planID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(planID)
	updatedPlan, err := ac.planService.UpdatePlan(objID, updates)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update plan")
		return
	}

	utils.SuccessResponse(c, "Plan updated successfully", updatedPlan)
}

func (ac *AdminController) DeletePlan(c *gin.Context) {
	planID := c.Param("id")
	if !utils.IsValidObjectID(planID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	objID, _ := utils.StringToObjectID(planID)
	err := ac.planService.DeletePlan(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete plan")
		return
	}

	utils.SuccessResponse(c, "Plan deleted successfully", nil)
}

func (ac *AdminController) ActivatePlan(c *gin.Context) {
	planID := c.Param("id")
	if !utils.IsValidObjectID(planID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	objID, _ := utils.StringToObjectID(planID)
	err := ac.planService.SetPlanStatus(objID, true)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to activate plan")
		return
	}

	utils.SuccessResponse(c, "Plan activated successfully", nil)
}

func (ac *AdminController) DeactivatePlan(c *gin.Context) {
	planID := c.Param("id")
	if !utils.IsValidObjectID(planID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	objID, _ := utils.StringToObjectID(planID)
	err := ac.planService.SetPlanStatus(objID, false)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to deactivate plan")
		return
	}

	utils.SuccessResponse(c, "Plan deactivated successfully", nil)
}

// Storage provider management
func (ac *AdminController) GetStorageProviders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	providers, total, err := ac.storageService.GetProvidersForAdmin(page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get storage providers")
		return
	}

	utils.PaginatedResponse(c, "Storage providers retrieved successfully", providers, page, limit, total)
}

func (ac *AdminController) GetStorageProvider(c *gin.Context) {
	providerID := c.Param("id")
	if !utils.IsValidObjectID(providerID) {
		utils.BadRequestResponse(c, "Invalid provider ID")
		return
	}

	objID, _ := utils.StringToObjectID(providerID)
	provider, err := ac.storageService.GetProviderForAdmin(objID)
	if err != nil {
		utils.NotFoundResponse(c, "Storage provider not found")
		return
	}

	utils.SuccessResponse(c, "Storage provider retrieved successfully", provider)
}

func (ac *AdminController) CreateStorageProvider(c *gin.Context) {
	var provider models.StorageProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&provider); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	createdProvider, err := ac.storageService.CreateProvider(&provider)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create storage provider")
		return
	}

	utils.CreatedResponse(c, "Storage provider created successfully", createdProvider)
}

func (ac *AdminController) UpdateStorageProvider(c *gin.Context) {
	providerID := c.Param("id")
	if !utils.IsValidObjectID(providerID) {
		utils.BadRequestResponse(c, "Invalid provider ID")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(providerID)
	updatedProvider, err := ac.storageService.UpdateProvider(objID, updates)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update storage provider")
		return
	}

	utils.SuccessResponse(c, "Storage provider updated successfully", updatedProvider)
}

func (ac *AdminController) DeleteStorageProvider(c *gin.Context) {
	providerID := c.Param("id")
	if !utils.IsValidObjectID(providerID) {
		utils.BadRequestResponse(c, "Invalid provider ID")
		return
	}

	objID, _ := utils.StringToObjectID(providerID)
	err := ac.storageService.DeleteProvider(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete storage provider")
		return
	}

	utils.SuccessResponse(c, "Storage provider deleted successfully", nil)
}

func (ac *AdminController) TestStorageProvider(c *gin.Context) {
	providerID := c.Param("id")
	if !utils.IsValidObjectID(providerID) {
		utils.BadRequestResponse(c, "Invalid provider ID")
		return
	}

	objID, _ := utils.StringToObjectID(providerID)
	testResult, err := ac.storageService.TestProvider(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to test storage provider")
		return
	}

	utils.SuccessResponse(c, "Storage provider test completed", testResult)
}

func (ac *AdminController) SyncStorageProvider(c *gin.Context) {
	providerID := c.Param("id")
	if !utils.IsValidObjectID(providerID) {
		utils.BadRequestResponse(c, "Invalid provider ID")
		return
	}

	objID, _ := utils.StringToObjectID(providerID)
	err := ac.storageService.SyncProvider(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to sync storage provider")
		return
	}

	utils.SuccessResponse(c, "Storage provider sync initiated", nil)
}

// System maintenance
func (ac *AdminController) GetSystemInfo(c *gin.Context) {
	systemInfo, err := ac.adminService.GetSystemInfo()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get system info")
		return
	}

	utils.SuccessResponse(c, "System info retrieved successfully", systemInfo)
}

func (ac *AdminController) GetSystemHealth(c *gin.Context) {
	healthStatus, err := ac.adminService.GetSystemHealth()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get system health")
		return
	}

	statusCode := http.StatusOK
	if healthy, ok := healthStatus["IsHealthy"].(bool); ok && !healthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"success":   healthStatus["IsHealthy"],
		"message":   "System health check completed",
		"data":      healthStatus,
		"timestamp": time.Now(),
	})
}

func (ac *AdminController) ClearCache(c *gin.Context) {
	var req struct {
		CacheType string `json:"cache_type"` // redis, memory, all
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := ac.adminService.ClearCache()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to clear cache")
		return
	}

	utils.SuccessResponse(c, "Cache cleared successfully", nil)
}

func (ac *AdminController) ClearLogs(c *gin.Context) {
	var req struct {
		LogType   string `json:"log_type"`   // access, error, all
		OlderThan int    `json:"older_than"` // days
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := ac.adminService.ClearLogs()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to clear logs")
		return
	}

	utils.SuccessResponse(c, "Logs cleared successfully", nil)
}

func (ac *AdminController) GetLogs(c *gin.Context) {
	logType := c.DefaultQuery("type", "all")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	logs, total, err := ac.adminService.GetLogs(page, limit, logType)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get logs")
		return
	}

	utils.PaginatedResponse(c, "Logs retrieved successfully", logs, page, limit, total)
}

func (ac *AdminController) CreateSystemBackup(c *gin.Context) {
	var req struct {
		BackupType string `json:"backup_type" validate:"required"` // database, files, full
		Name       string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	backup, err := ac.adminService.CreateSystemBackup(req.BackupType, req.Name)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create system backup")
		return
	}

	utils.CreatedResponse(c, "System backup created successfully", backup)
}

func (ac *AdminController) GetSystemBackups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	backups, err := ac.adminService.GetSystemBackups()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get system backups")
		return
	}

	utils.PaginatedResponse(c, "System backups retrieved successfully", backups, page, limit, total)
}

// HTML Admin Panel Controllers
type DashboardController struct {
	adminService     *services.AdminService
	analyticsService *services.AnalyticsService
}

func NewDashboardController() *DashboardController {
	return &DashboardController{
		adminService:     services.NewAdminService(),
		analyticsService: services.NewAnalyticsService(),
	}
}

// HTML pages for admin panel
func (dc *DashboardController) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/login.html", gin.H{
		"title": "Admin Login",
	})
}

func (dc *DashboardController) Dashboard(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	// Get dashboard data
	stats, err := dc.analyticsService.GetDashboardStats()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to load dashboard data",
		})
		return
	}

	c.HTML(http.StatusOK, "dashboard/index.html", gin.H{
		"title": "Dashboard",
		"admin": admin,
		"stats": stats,
	})
}

func (dc *DashboardController) UsersPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard/users.html", gin.H{
		"title": "User Management",
		"admin": admin,
	})
}

func (dc *DashboardController) UserDetailPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	userID := c.Param("id")
	c.HTML(http.StatusOK, "dashboard/user-detail.html", gin.H{
		"title":   "User Details",
		"admin":   admin,
		"user_id": userID,
	})
}

func (dc *DashboardController) EditUserPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	userID := c.Param("id")
	c.HTML(http.StatusOK, "dashboard/edit-user.html", gin.H{
		"title":   "Edit User",
		"admin":   admin,
		"user_id": userID,
	})
}

func (dc *DashboardController) FilesPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard/files.html", gin.H{
		"title": "File Management",
		"admin": admin,
	})
}

func (dc *DashboardController) FileDetailPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	fileID := c.Param("id")
	c.HTML(http.StatusOK, "dashboard/file-detail.html", gin.H{
		"title":   "File Details",
		"admin":   admin,
		"file_id": fileID,
	})
}

func (dc *DashboardController) PlansPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard/plans.html", gin.H{
		"title": "Plan Management",
		"admin": admin,
	})
}

func (dc *DashboardController) CreatePlanPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard/create-plan.html", gin.H{
		"title": "Create Plan",
		"admin": admin,
	})
}

func (dc *DashboardController) EditPlanPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	planID := c.Param("id")
	c.HTML(http.StatusOK, "dashboard/edit-plan.html", gin.H{
		"title":   "Edit Plan",
		"admin":   admin,
		"plan_id": planID,
	})
}

func (dc *DashboardController) SettingsPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "settings/general.html", gin.H{
		"title": "Settings",
		"admin": admin,
	})
}

func (dc *DashboardController) GeneralSettingsPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "settings/general.html", gin.H{
		"title": "General Settings",
		"admin": admin,
	})
}

func (dc *DashboardController) StorageSettingsPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "settings/storage.html", gin.H{
		"title": "Storage Settings",
		"admin": admin,
	})
}

func (dc *DashboardController) PricingSettingsPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "settings/pricing.html", gin.H{
		"title": "Pricing Settings",
		"admin": admin,
	})
}

func (dc *DashboardController) AnalyticsPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard/analytics.html", gin.H{
		"title": "Analytics",
		"admin": admin,
	})
}

func (dc *DashboardController) ReportsPage(c *gin.Context) {
	admin, exists := utils.GetAdminFromContext(c)
	if !exists {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard/reports.html", gin.H{
		"title": "Reports",
		"admin": admin,
	})
}
