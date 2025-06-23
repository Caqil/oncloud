package controllers

import (
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AnalyticsController struct {
	analyticsService *services.AnalyticsService
	adminService     *services.AdminService
}

func NewAnalyticsController() *AnalyticsController {
	return &AnalyticsController{
		analyticsService: services.NewAnalyticsService(),
	}
}

// GetDashboard returns dashboard analytics data
func (ac *AnalyticsController) GetDashboard(c *gin.Context) {
	c.DefaultQuery("period", "30") // days

	dashboard, err := ac.adminService.GetDashboardStats()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get dashboard analytics")
		return
	}

	utils.SuccessResponse(c, "Dashboard analytics retrieved successfully", dashboard)
}

// GetUserAnalytics returns user-related analytics
func (ac *AnalyticsController) GetUserAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "30")     // days
	groupBy := c.DefaultQuery("group_by", "day") // day, week, month

	analytics, err := ac.analyticsService.GetUserAnalytics(period, groupBy)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user analytics")
		return
	}

	utils.SuccessResponse(c, "User analytics retrieved successfully", analytics)
}

// GetFileAnalytics returns file-related analytics
func (ac *AnalyticsController) GetFileAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "30") // days
	groupBy := c.DefaultQuery("group_by", "day")

	analytics, err := ac.analyticsService.GetFileAnalytics(period, groupBy)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get file analytics")
		return
	}

	utils.SuccessResponse(c, "File analytics retrieved successfully", analytics)
}

// GetStorageAnalytics returns storage-related analytics
func (ac *AnalyticsController) GetStorageAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "30")     // days
	groupBy := c.DefaultQuery("group_by", "day") // day, week, month
	providerID := c.Query("provider_id")

	analytics, err := ac.analyticsService.GetStorageAnalytics(period, groupBy, providerID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get storage analytics")
		return
	}

	utils.SuccessResponse(c, "Storage analytics retrieved successfully", analytics)
}

// GetRevenueAnalytics returns revenue-related analytics
func (ac *AnalyticsController) GetRevenueAnalytics(c *gin.Context) {
	period := c.DefaultQuery("period", "30")     // days
	groupBy := c.DefaultQuery("group_by", "day") // day, week, month
	currency := c.DefaultQuery("currency", "USD")

	analytics, err := ac.analyticsService.GetRevenueAnalytics(period, groupBy, currency)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get revenue analytics")
		return
	}

	utils.SuccessResponse(c, "Revenue analytics retrieved successfully", analytics)
}

// GetRealTimeStats returns real-time statistics
func (ac *AnalyticsController) GetRealTimeStats(c *gin.Context) {
	stats, err := ac.analyticsService.GetRealTimeStats()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get real-time stats")
		return
	}

	utils.SuccessResponse(c, "Real-time stats retrieved successfully", stats)
}

// GetTopFiles returns most downloaded/viewed files
func (ac *AnalyticsController) GetTopFiles(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sort_by", "downloads") // days

	topFiles, err := ac.analyticsService.GetTopFiles(sortBy, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get top files")
		return
	}

	utils.SuccessResponse(c, "Top files retrieved successfully", topFiles)
}

// GetTopUsers returns most active users
func (ac *AnalyticsController) GetTopUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sort_by", "storage_used") // storage_used, files_count, downloads
	period := c.DefaultQuery("period", "30")            // days

	topUsers, err := ac.analyticsService.GetTopUsers(limit, sortBy, period)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get top users")
		return
	}

	utils.SuccessResponse(c, "Top users retrieved successfully", topUsers)
}

// GetSystemMetrics returns system performance metrics
func (ac *AnalyticsController) GetSystemMetrics(c *gin.Context) {
	period := c.DefaultQuery("period", "24") // hours

	metrics, err := ac.analyticsService.GetSystemMetrics(period)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get system metrics")
		return
	}

	utils.SuccessResponse(c, "System metrics retrieved successfully", metrics)
}

// ExportAnalytics exports analytics data
func (ac *AnalyticsController) ExportAnalytics(c *gin.Context) {
	var req struct {
		Type    string `json:"type" validate:"required"` // users, files, storage, revenue
		Period  string `json:"period"`                   // 7, 30, 90 days
		Format  string `json:"format"`                   // csv, excel, pdf
		Email   string `json:"email"`                    // send to email
		GroupBy string `json:"group_by"`                 // day, week, month
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	exportResult, err := ac.analyticsService.ExportAnalytics(req.Type, req.Period, req.Format, req.Email, req.GroupBy)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to export analytics")
		return
	}

	utils.SuccessResponse(c, "Analytics export initiated successfully", exportResult)
}

// func (pc *PlanController) PayPalWebhook(c *gin.Context) {
// 	// PayPal webhook signature verification
// 	payload, err := c.GetRawData()
// 	if err != nil {
// 		utils.BadRequestResponse(c, "Failed to read request body")
// 		return
// 	}

// 	err = pc.planService.HandlePayPalWebhook(payload)
// 	if err != nil {
// 		utils.BadRequestResponse(c, "Failed to process webhook")
// 		return
// 	}

// 	c.Status(200)
// }

// func (pc *PlanController) RazorpayWebhook(c *gin.Context) {
// 	signature := c.GetHeader("X-Razorpay-Signature")
// 	if signature == "" {
// 		utils.BadRequestResponse(c, "Missing Razorpay signature")
// 		return
// 	}

// 	payload, err := c.GetRawData()
// 	if err != nil {
// 		utils.BadRequestResponse(c, "Failed to read request body")
// 		return
// 	}

// 	err = pc.planService.HandleRazorpayWebhook(signature, payload)
// 	if err != nil {
// 		utils.BadRequestResponse(c, "Failed to process webhook")
// 		return
// 	}

// 	c.Status(200)
// }
