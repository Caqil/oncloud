package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"oncloud/database"
	"oncloud/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AnalyticsService struct {
	*BaseService
}

func NewAnalyticsService() *AnalyticsService {
	return &AnalyticsService{
		BaseService: NewBaseService(),
	}
}

// Dashboard Analytics
func (as *AnalyticsService) GetDashboard() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dashboard := make(map[string]interface{})

	// Get current date info
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfWeek := startOfDay.AddDate(0, 0, -int(now.Weekday()))
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Overall statistics
	totalUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{})
	totalFiles, _ := as.collections.Files().CountDocuments(ctx, bson.M{"is_deleted": false})
	totalRevenue := as.getTotalRevenue(ctx)
	activeUsers := as.getActiveUsers(ctx, 30) // Last 30 days

	dashboard["overview"] = map[string]interface{}{
		"total_users":   totalUsers,
		"total_files":   totalFiles,
		"total_revenue": totalRevenue,
		"active_users":  activeUsers,
	}

	// Growth metrics
	dailyGrowth := as.getGrowthMetrics(ctx, startOfDay, "day")
	weeklyGrowth := as.getGrowthMetrics(ctx, startOfWeek, "week")
	monthlyGrowth := as.getGrowthMetrics(ctx, startOfMonth, "month")

	dashboard["growth"] = map[string]interface{}{
		"daily":   dailyGrowth,
		"weekly":  weeklyGrowth,
		"monthly": monthlyGrowth,
	}

	// Recent activity
	recentActivity := as.getRecentActivity(ctx, 10)
	dashboard["recent_activity"] = recentActivity

	// Storage usage
	storageStats := as.getStorageStats(ctx)
	dashboard["storage"] = storageStats

	// Top files
	topFiles := as.getTopFiles(ctx, 5)
	dashboard["top_files"] = topFiles

	// Revenue trend (last 30 days)
	revenueTrend := as.getRevenueTrend(ctx, 30)
	dashboard["revenue_trend"] = revenueTrend

	return dashboard, nil
}

// User Analytics
func (as *AnalyticsService) GetUserAnalytics(period, groupBy string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	analytics := make(map[string]interface{})

	// User registrations over time
	registrationTrend := as.getUserRegistrationTrend(ctx, startDate, groupBy)
	analytics["registration_trend"] = registrationTrend

	// User activity metrics
	activityMetrics := as.getUserActivityMetrics(ctx, startDate)
	analytics["activity_metrics"] = activityMetrics

	// User segmentation
	userSegmentation := as.getUserSegmentation(ctx)
	analytics["segmentation"] = userSegmentation

	// Plan distribution
	planDistribution := as.getPlanDistribution(ctx)
	analytics["plan_distribution"] = planDistribution

	// Geographic distribution
	geographicData := as.getGeographicDistribution(ctx)
	analytics["geographic"] = geographicData

	// User retention
	retentionData := as.getUserRetention(ctx, startDate)
	analytics["retention"] = retentionData

	return analytics, nil
}

// File Analytics
func (as *AnalyticsService) GetFileAnalytics(period, groupBy string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	analytics := make(map[string]interface{})

	// File uploads over time
	uploadTrend := as.getFileUploadTrend(ctx, startDate, groupBy)
	analytics["upload_trend"] = uploadTrend

	// File type distribution
	fileTypes := as.getFileTypeDistribution(ctx, startDate)
	analytics["file_types"] = fileTypes

	// File size distribution
	fileSizes := as.getFileSizeDistribution(ctx, startDate)
	analytics["file_sizes"] = fileSizes

	// Download metrics
	downloadMetrics := as.getDownloadMetrics(ctx, startDate)
	analytics["downloads"] = downloadMetrics

	// Storage usage by user
	storageByUser := as.getStorageByUser(ctx, 10) // Top 10 users
	analytics["storage_by_user"] = storageByUser

	// Most popular files
	popularFiles := as.getPopularFiles(ctx, startDate, 10)
	analytics["popular_files"] = popularFiles

	// File lifecycle metrics
	lifecycleMetrics := as.getFileLifecycleMetrics(ctx, startDate)
	analytics["lifecycle"] = lifecycleMetrics

	return analytics, nil
}

// Storage Analytics
func (as *AnalyticsService) GetStorageAnalytics(period, groupBy, providerID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	analytics := make(map[string]interface{})

	// Storage usage trend
	usageTrend := as.getStorageUsageTrend(ctx, startDate, groupBy)
	analytics["usage_trend"] = usageTrend

	// Provider distribution
	providerStats := as.getProviderDistribution(ctx, providerID)
	analytics["providers"] = providerStats

	// Bandwidth usage
	bandwidthUsage := as.getBandwidthUsage(ctx, startDate, groupBy)
	analytics["bandwidth"] = bandwidthUsage

	// Storage efficiency
	efficiency := as.getStorageEfficiency(ctx)
	analytics["efficiency"] = efficiency

	// Cost analysis
	costAnalysis := as.getStorageCostAnalysis(ctx, startDate)
	analytics["costs"] = costAnalysis

	// Performance metrics
	performanceMetrics := as.getStoragePerformance(ctx, startDate)
	analytics["performance"] = performanceMetrics

	return analytics, nil
}

// Revenue Analytics
func (as *AnalyticsService) GetRevenueAnalytics(period, groupBy, currency string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	analytics := make(map[string]interface{})

	if currency == "" {
		currency = "USD"
	}

	// Revenue trend
	revenueTrend := as.getDetailedRevenueTrend(ctx, startDate, groupBy, currency)
	analytics["revenue_trend"] = revenueTrend

	// Revenue by plan
	revenueByPlan := as.getRevenueByPlan(ctx, startDate, currency)
	analytics["revenue_by_plan"] = revenueByPlan

	// MRR (Monthly Recurring Revenue)
	mrr := as.getMRR(ctx, currency)
	analytics["mrr"] = mrr

	// ARR (Annual Recurring Revenue)
	arr := as.getARR(ctx, currency)
	analytics["arr"] = arr

	// Customer lifetime value
	clv := as.getCustomerLifetimeValue(ctx, currency)
	analytics["customer_lifetime_value"] = clv

	// Churn analysis
	churnAnalysis := as.getChurnAnalysis(ctx, startDate)
	analytics["churn"] = churnAnalysis

	// Payment method distribution
	paymentMethods := as.getPaymentMethodDistribution(ctx, startDate)
	analytics["payment_methods"] = paymentMethods

	// Revenue forecasting
	forecast := as.getRevenueForecast(ctx, 90, currency) // 90 days forecast
	analytics["forecast"] = forecast

	return analytics, nil
}

// Real-time Statistics
func (as *AnalyticsService) GetRealTimeStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats := make(map[string]interface{})

	// Current online users (last 5 minutes)
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	onlineUsers, _ := as.collections.Sessions().CountDocuments(ctx, bson.M{
		"last_activity": bson.M{"$gte": fiveMinutesAgo},
		"is_active":     true,
	})

	// Today's activity
	today := time.Now().Truncate(24 * time.Hour)
	todayUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": today},
	})

	todayUploads, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": today},
	})

	// Recent uploads (last hour)
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	recentUploads, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": oneHourAgo},
	})

	// Current system load
	systemLoad := as.getSystemLoad(ctx)

	stats["online_users"] = onlineUsers
	stats["today_active_users"] = todayUsers
	stats["today_uploads"] = todayUploads
	stats["recent_uploads"] = recentUploads
	stats["system_load"] = systemLoad
	stats["timestamp"] = time.Now()

	return stats, nil
}

// Top Files Analytics
func (as *AnalyticsService) GetTopFiles(period string, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 7
	}

	startDate := time.Now().AddDate(0, 0, -days)

	// Get most downloaded files
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"action":     "download",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id":            "$file_id",
				"download_count": bson.M{"$sum": 1},
				"total_bytes":    bson.M{"$sum": "$bytes"},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "files",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "file",
			},
		},
		{
			"$unwind": "$file",
		},
		{
			"$sort": bson.M{"download_count": -1},
		},
		{
			"$limit": limit,
		},
	}

	cursor, err := as.collections.Activities().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var topFiles []map[string]interface{}
	if err = cursor.All(ctx, &topFiles); err != nil {
		return nil, err
	}

	return topFiles, nil
}

// Event Tracking
func (as *AnalyticsService) TrackEvent(eventType, action string, userID *primitive.ObjectID, metadata map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event := bson.M{
		"_id":       primitive.NewObjectID(),
		"type":      eventType,
		"action":    action,
		"user_id":   userID,
		"metadata":  metadata,
		"timestamp": time.Now(),
	}

	_, err := as.collections.Analytics().InsertOne(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to track event: %v", err)
	}

	return nil
}

func (as *AnalyticsService) TrackUserActivity(userID primitive.ObjectID, action, resource string, metadata map[string]interface{}) error {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["resource"] = resource

	return as.TrackEvent("user_activity", action, &userID, metadata)
}

func (as *AnalyticsService) TrackFileActivity(userID, fileID primitive.ObjectID, action string, bytes int64) error {
	metadata := map[string]interface{}{
		"file_id":  fileID,
		"bytes":    bytes,
		"resource": "file",
	}

	return as.TrackEvent("file_activity", action, &userID, metadata)
}

// Helper functions
func (as *AnalyticsService) getTotalRevenue(ctx context.Context) float64 {
	pipeline := []bson.M{
		{
			"$match": bson.M{"status": "completed"},
		},
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": "$amount"},
			},
		},
	}

	cursor, err := as.collections.Payments().Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil || len(result) == 0 {
		return 0
	}

	if total, ok := result[0]["total"].(float64); ok {
		return total
	}
	return 0
}

func (as *AnalyticsService) getActiveUsers(ctx context.Context, days int) int64 {
	startDate := time.Now().AddDate(0, 0, -days)
	count, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": startDate},
	})
	return count
}

func (as *AnalyticsService) getGrowthMetrics(ctx context.Context, startDate time.Time, period string) map[string]interface{} {
	endDate := time.Now()

	newUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	})

	newFiles, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	})

	// Calculate previous period for comparison
	var previousStart time.Time
	switch period {
	case "day":
		previousStart = startDate.AddDate(0, 0, -1)
	case "week":
		previousStart = startDate.AddDate(0, 0, -7)
	case "month":
		previousStart = startDate.AddDate(0, -1, 0)
	}

	prevUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"created_at": bson.M{
			"$gte": previousStart,
			"$lt":  startDate,
		},
	})

	prevFiles, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{
			"$gte": previousStart,
			"$lt":  startDate,
		},
	})

	// Calculate growth rates
	userGrowth := calculateGrowthRate(int64(prevUsers), newUsers)
	fileGrowth := calculateGrowthRate(int64(prevFiles), newFiles)

	return map[string]interface{}{
		"new_users":   newUsers,
		"new_files":   newFiles,
		"user_growth": userGrowth,
		"file_growth": fileGrowth,
		"period":      period,
	}
}

// Analytics Service - ExportAnalytics Function
func (as *AnalyticsService) ExportAnalytics(dataType, period, format, email, groupBy string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Validate parameters
	if dataType == "" {
		return nil, fmt.Errorf("data type is required")
	}
	if format == "" {
		format = "csv"
	}
	if groupBy == "" {
		groupBy = "day"
	}

	// Generate export job ID
	exportID := primitive.NewObjectID()

	result := map[string]interface{}{
		"export_id":  exportID.Hex(),
		"data_type":  dataType,
		"format":     format,
		"period":     period,
		"group_by":   groupBy,
		"status":     "initiated",
		"created_at": time.Now(),
	}

	// Create export job record
	exportJob := bson.M{
		"_id":        exportID,
		"data_type":  dataType,
		"period":     period,
		"format":     format,
		"email":      email,
		"group_by":   groupBy,
		"status":     "processing",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}

	_, err := as.collections.Exports().InsertOne(ctx, exportJob)
	if err != nil {
		return nil, fmt.Errorf("failed to create export job: %v", err)
	}

	// Process export asynchronously
	go func() {
		exportCtx, exportCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer exportCancel()

		var exportData interface{}
		var exportErr error

		// Get data based on type
		switch dataType {
		case "users":
			exportData, exportErr = as.exportUserData(exportCtx, period, groupBy)
		case "files":
			exportData, exportErr = as.exportFileData(exportCtx, period, groupBy)
		case "storage":
			exportData, exportErr = as.exportStorageData(exportCtx, period, groupBy)
		case "revenue":
			exportData, exportErr = as.exportRevenueData(exportCtx, period, groupBy)
		default:
			exportErr = fmt.Errorf("unsupported data type: %s", dataType)
		}

		if exportErr != nil {
			// Update job status to failed
			as.collections.Exports().UpdateOne(exportCtx,
				bson.M{"_id": exportID},
				bson.M{"$set": bson.M{
					"status":     "failed",
					"error":      exportErr.Error(),
					"updated_at": time.Now(),
				}},
			)
			return
		}

		// Generate file based on format
		fileName, fileErr := as.generateExportFile(exportData, format, dataType, period)
		if fileErr != nil {
			as.collections.Exports().UpdateOne(exportCtx,
				bson.M{"_id": exportID},
				bson.M{"$set": bson.M{
					"status":     "failed",
					"error":      fileErr.Error(),
					"updated_at": time.Now(),
				}},
			)
			return
		}

		// Update job status to completed
		updates := bson.M{
			"status":       "completed",
			"file_name":    fileName,
			"completed_at": time.Now(),
			"updated_at":   time.Now(),
		}

		// Send email if requested
		if email != "" {
			emailErr := as.sendExportEmail(email, fileName, dataType, format)
			if emailErr != nil {
				updates["email_error"] = emailErr.Error()
			} else {
				updates["email_sent"] = true
			}
		}

		as.collections.Exports().UpdateOne(exportCtx,
			bson.M{"_id": exportID},
			bson.M{"$set": updates},
		)
	}()

	return result, nil
}

// Analytics Service - GetTopUsers Function
func (as *AnalyticsService) GetTopUsers(limit int, sortBy string, period string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)

	var pipeline []bson.M

	switch sortBy {
	case "storage_used":
		pipeline = []bson.M{
			{
				"$lookup": bson.M{
					"from":         "files",
					"localField":   "_id",
					"foreignField": "user_id",
					"as":           "files",
				},
			},
			{
				"$addFields": bson.M{
					"total_storage": bson.M{
						"$sum": bson.M{
							"$map": bson.M{
								"input": "$files",
								"as":    "file",
								"in": bson.M{
									"$cond": bson.M{
										"if":   bson.M{"$eq": []interface{}{"$$file.is_deleted", false}},
										"then": "$$file.size",
										"else": 0,
									},
								},
							},
						},
					},
					"file_count": bson.M{
						"$size": bson.M{
							"$filter": bson.M{
								"input": "$files",
								"cond":  bson.M{"$eq": []interface{}{"$$this.is_deleted", false}},
							},
						},
					},
				},
			},
			{
				"$sort": bson.M{"total_storage": -1},
			},
		}
	case "files_count":
		pipeline = []bson.M{
			{
				"$lookup": bson.M{
					"from":         "files",
					"localField":   "_id",
					"foreignField": "user_id",
					"as":           "files",
				},
			},
			{
				"$addFields": bson.M{
					"file_count": bson.M{
						"$size": bson.M{
							"$filter": bson.M{
								"input": "$files",
								"cond":  bson.M{"$eq": []interface{}{"$$this.is_deleted", false}},
							},
						},
					},
					"total_storage": bson.M{
						"$sum": bson.M{
							"$map": bson.M{
								"input": "$files",
								"as":    "file",
								"in": bson.M{
									"$cond": bson.M{
										"if":   bson.M{"$eq": []interface{}{"$$file.is_deleted", false}},
										"then": "$$file.size",
										"else": 0,
									},
								},
							},
						},
					},
				},
			},
			{
				"$sort": bson.M{"file_count": -1},
			},
		}
	case "downloads":
		pipeline = []bson.M{
			{
				"$lookup": bson.M{
					"from": "activities",
					"let":  bson.M{"userId": "$_id"},
					"pipeline": []bson.M{
						{
							"$match": bson.M{
								"$expr":      bson.M{"$eq": []interface{}{"$user_id", "$$userId"}},
								"action":     "download",
								"created_at": bson.M{"$gte": startDate},
							},
						},
						{
							"$group": bson.M{
								"_id":   nil,
								"count": bson.M{"$sum": 1},
							},
						},
					},
					"as": "download_stats",
				},
			},
			{
				"$addFields": bson.M{
					"download_count": bson.M{
						"$ifNull": []interface{}{
							bson.M{"$arrayElemAt": []interface{}{"$download_stats.count", 0}},
							0,
						},
					},
				},
			},
			{
				"$lookup": bson.M{
					"from":         "files",
					"localField":   "_id",
					"foreignField": "user_id",
					"as":           "files",
				},
			},
			{
				"$addFields": bson.M{
					"file_count": bson.M{
						"$size": bson.M{
							"$filter": bson.M{
								"input": "$files",
								"cond":  bson.M{"$eq": []interface{}{"$$this.is_deleted", false}},
							},
						},
					},
					"total_storage": bson.M{
						"$sum": bson.M{
							"$map": bson.M{
								"input": "$files",
								"as":    "file",
								"in": bson.M{
									"$cond": bson.M{
										"if":   bson.M{"$eq": []interface{}{"$$file.is_deleted", false}},
										"then": "$$file.size",
										"else": 0,
									},
								},
							},
						},
					},
				},
			},
			{
				"$sort": bson.M{"download_count": -1},
			},
		}
	default:
		// Default to storage_used
		return as.GetTopUsers(limit, "storage_used", period)
	}

	// Add common pipeline stages
	pipeline = append(pipeline, []bson.M{
		{
			"$lookup": bson.M{
				"from":         "plans",
				"localField":   "plan_id",
				"foreignField": "_id",
				"as":           "plan",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$plan",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$project": bson.M{
				"username":       1,
				"email":          1,
				"first_name":     1,
				"last_name":      1,
				"created_at":     1,
				"last_login_at":  1,
				"is_verified":    1,
				"file_count":     bson.M{"$ifNull": []interface{}{"$file_count", 0}},
				"total_storage":  bson.M{"$ifNull": []interface{}{"$total_storage", 0}},
				"download_count": bson.M{"$ifNull": []interface{}{"$download_count", 0}},
				"plan_name":      "$plan.name",
				"files":          0, // Exclude files array to reduce response size
			},
		},
		{
			"$limit": int64(limit),
		},
	}...)

	cursor, err := as.collections.Users().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top users: %v", err)
	}
	defer cursor.Close(ctx)

	var topUsers []map[string]interface{}
	if err = cursor.All(ctx, &topUsers); err != nil {
		return nil, err
	}

	// Format storage sizes for better readability
	for i := range topUsers {
		if totalStorage, ok := topUsers[i]["total_storage"].(int64); ok {
			topUsers[i]["total_storage_formatted"] = utils.FormatFileSize(totalStorage)
		}
	}

	return topUsers, nil
}

// Analytics Service - GetSystemMetrics Function
func (as *AnalyticsService) GetSystemMetrics(period string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hours, _ := strconv.Atoi(period)
	if hours == 0 {
		hours = 24
	}

	startTime := time.Now().Add(-time.Duration(hours) * time.Hour)
	metrics := make(map[string]interface{})

	// Database performance metrics
	dbMetrics, err := as.getDatabaseMetrics(ctx)
	if err == nil {
		metrics["database"] = dbMetrics
	}

	// API performance metrics
	apiMetrics, err := as.getAPIMetrics(ctx, startTime)
	if err == nil {
		metrics["api"] = apiMetrics
	}

	// Storage performance metrics
	storageMetrics, err := as.getStorageMetrics(ctx, startTime)
	if err == nil {
		metrics["storage"] = storageMetrics
	}

	// System resource usage
	resourceMetrics := as.getResourceMetrics(ctx, startTime)
	metrics["resources"] = resourceMetrics

	// Error rates and response times
	errorMetrics, err := as.getErrorMetrics(ctx, startTime)
	if err == nil {
		metrics["errors"] = errorMetrics
	}

	// Active connections and sessions
	connectionMetrics, err := as.getConnectionMetrics(ctx)
	if err == nil {
		metrics["connections"] = connectionMetrics
	}

	// Cache performance (if using cache)
	cacheMetrics := as.getCacheMetrics(ctx, startTime)
	metrics["cache"] = cacheMetrics

	metrics["period_hours"] = hours
	metrics["generated_at"] = time.Now()

	return metrics, nil
}

func (as *AnalyticsService) getRecentActivity(ctx context.Context, limit int) []map[string]interface{} {
	cursor, err := as.collections.Activities().Find(ctx, bson.M{},
		options.Find().
			SetSort(bson.M{"created_at": -1}).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var activities []map[string]interface{}
	cursor.All(ctx, &activities)
	return activities
}

func (as *AnalyticsService) getStorageStats(ctx context.Context) map[string]interface{} {
	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":         "$storage_provider",
				"total_files": bson.M{"$sum": 1},
				"total_size":  bson.M{"$sum": "$size"},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var stats []bson.M
	cursor.All(ctx, &stats)

	return map[string]interface{}{
		"by_provider":     stats,
		"total_providers": len(stats),
	}
}

func (as *AnalyticsService) getTopFiles(ctx context.Context, limit int) []map[string]interface{} {
	// Get files with highest download count
	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$sort": bson.M{"download_count": -1},
		},
		{
			"$limit": limit,
		},
		{
			"$project": bson.M{
				"name":           1,
				"size":           1,
				"download_count": 1,
				"created_at":     1,
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var files []map[string]interface{}
	cursor.All(ctx, &files)
	return files
}

func (as *AnalyticsService) getRevenueTrend(ctx context.Context, days int) []map[string]interface{} {
	startDate := time.Now().AddDate(0, 0, -days)

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":     "completed",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"year":  bson.M{"$year": "$created_at"},
					"month": bson.M{"$month": "$created_at"},
					"day":   bson.M{"$dayOfMonth": "$created_at"},
				},
				"revenue": bson.M{"$sum": "$amount"},
				"count":   bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := as.collections.Payments().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var trend []map[string]interface{}
	cursor.All(ctx, &trend)
	return trend
}

func (as *AnalyticsService) getUserRegistrationTrend(ctx context.Context, startDate time.Time, groupBy string) []map[string]interface{} {
	var groupStage bson.M
	switch groupBy {
	case "hour":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
				"hour":  bson.M{"$hour": "$created_at"},
			},
		}
	case "week":
		groupStage = bson.M{
			"_id": bson.M{
				"year": bson.M{"$year": "$created_at"},
				"week": bson.M{"$week": "$created_at"},
			},
		}
	case "month":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
			},
		}
	default: // day
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
			},
		}
	}

	groupStage["count"] = bson.M{"$sum": 1}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": groupStage,
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := as.collections.Users().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var trend []map[string]interface{}
	cursor.All(ctx, &trend)
	return trend
}

func (as *AnalyticsService) getUserActivityMetrics(ctx context.Context, startDate time.Time) map[string]interface{} {
	activeUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": startDate},
	})

	totalSessions, _ := as.collections.Sessions().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startDate},
	})

	// Average session duration
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
				"ended_at":   bson.M{"$exists": true},
			},
		},
		{
			"$group": bson.M{
				"_id":            nil,
				"avg_duration":   bson.M{"$avg": bson.M{"$subtract": []interface{}{"$ended_at", "$created_at"}}},
				"total_sessions": bson.M{"$sum": 1},
			},
		},
	}

	cursor, _ := as.collections.Sessions().Aggregate(ctx, pipeline)
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	avgDuration := float64(0)
	if len(result) > 0 {
		if duration, ok := result[0]["avg_duration"].(float64); ok {
			avgDuration = duration / 1000 / 60 // Convert to minutes
		}
	}

	return map[string]interface{}{
		"active_users":         activeUsers,
		"total_sessions":       totalSessions,
		"avg_session_duration": avgDuration,
	}
}

func (as *AnalyticsService) getUserSegmentation(ctx context.Context) map[string]interface{} {
	// User segmentation by plan, activity level, etc.
	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "plans",
				"localField":   "plan_id",
				"foreignField": "_id",
				"as":           "plan",
			},
		},
		{
			"$unwind": "$plan",
		},
		{
			"$group": bson.M{
				"_id":   "$plan.name",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := as.collections.Users().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var planSegmentation []bson.M
	cursor.All(ctx, &planSegmentation)

	return map[string]interface{}{
		"by_plan": planSegmentation,
	}
}

func (as *AnalyticsService) getPlanDistribution(ctx context.Context) []map[string]interface{} {
	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "plans",
				"localField":   "plan_id",
				"foreignField": "_id",
				"as":           "plan",
			},
		},
		{
			"$unwind": "$plan",
		},
		{
			"$group": bson.M{
				"_id":        "$plan.name",
				"user_count": bson.M{"$sum": 1},
				"plan_price": bson.M{"$first": "$plan.price"},
			},
		},
		{
			"$sort": bson.M{"user_count": -1},
		},
	}

	cursor, err := as.collections.Users().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var distribution []map[string]interface{}
	cursor.All(ctx, &distribution)
	return distribution
}

func (as *AnalyticsService) getGeographicDistribution(ctx context.Context) []map[string]interface{} {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   "$country",
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
		{
			"$limit": 20,
		},
	}

	cursor, err := as.collections.Users().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var distribution []map[string]interface{}
	cursor.All(ctx, &distribution)
	return distribution
}

func (as *AnalyticsService) getUserRetention(ctx context.Context, startDate time.Time) map[string]interface{} {
	// Simplified retention calculation
	totalUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startDate},
	})

	activeUsers, _ := as.collections.Users().CountDocuments(ctx, bson.M{
		"created_at":    bson.M{"$gte": startDate},
		"last_login_at": bson.M{"$gte": time.Now().AddDate(0, 0, -7)},
	})

	retentionRate := float64(0)
	if totalUsers > 0 {
		retentionRate = (float64(activeUsers) / float64(totalUsers)) * 100
	}

	return map[string]interface{}{
		"total_users":    totalUsers,
		"active_users":   activeUsers,
		"retention_rate": retentionRate,
	}
}

func (as *AnalyticsService) getFileUploadTrend(ctx context.Context, startDate time.Time, groupBy string) []map[string]interface{} {
	// Similar to user registration trend but for files
	var groupStage bson.M
	switch groupBy {
	case "hour":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
				"hour":  bson.M{"$hour": "$created_at"},
			},
		}
	case "week":
		groupStage = bson.M{
			"_id": bson.M{
				"year": bson.M{"$year": "$created_at"},
				"week": bson.M{"$week": "$created_at"},
			},
		}
	case "month":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
			},
		}
	default: // day
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
			},
		}
	}

	groupStage["count"] = bson.M{"$sum": 1}
	groupStage["total_size"] = bson.M{"$sum": "$size"}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
				"is_deleted": false,
			},
		},
		{
			"$group": groupStage,
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var trend []map[string]interface{}
	cursor.All(ctx, &trend)
	return trend
}

func (as *AnalyticsService) getFileTypeDistribution(ctx context.Context, startDate time.Time) []map[string]interface{} {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
				"is_deleted": false,
			},
		},
		{
			"$group": bson.M{
				"_id":        "$extension",
				"count":      bson.M{"$sum": 1},
				"total_size": bson.M{"$sum": "$size"},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var distribution []map[string]interface{}
	cursor.All(ctx, &distribution)
	return distribution
}

func (as *AnalyticsService) getFileSizeDistribution(ctx context.Context, startDate time.Time) map[string]interface{} {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
				"is_deleted": false,
			},
		},
		{
			"$bucket": bson.M{
				"groupBy": "$size",
				"boundaries": []int64{
					0,
					1024 * 1024,             // 1MB
					10 * 1024 * 1024,        // 10MB
					100 * 1024 * 1024,       // 100MB
					1024 * 1024 * 1024,      // 1GB
					10 * 1024 * 1024 * 1024, // 10GB
				},
				"default": "10GB+",
				"output": bson.M{
					"count":      bson.M{"$sum": 1},
					"total_size": bson.M{"$sum": "$size"},
				},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var distribution []bson.M
	cursor.All(ctx, &distribution)

	return map[string]interface{}{
		"distribution": distribution,
	}
}

func (as *AnalyticsService) getDownloadMetrics(ctx context.Context, startDate time.Time) map[string]interface{} {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"action":     "download",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id":             nil,
				"total_downloads": bson.M{"$sum": 1},
				"total_bytes":     bson.M{"$sum": "$bytes"},
				"unique_files":    bson.M{"$addToSet": "$file_id"},
			},
		},
		{
			"$project": bson.M{
				"total_downloads": 1,
				"total_bytes":     1,
				"unique_files":    bson.M{"$size": "$unique_files"},
			},
		},
	}

	cursor, err := as.collections.Activities().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	if len(result) > 0 {
		return map[string]interface{}{
			"total_downloads": result[0]["total_downloads"],
			"total_bytes":     result[0]["total_bytes"],
			"unique_files":    result[0]["unique_files"],
		}
	}

	return map[string]interface{}{
		"total_downloads": 0,
		"total_bytes":     0,
		"unique_files":    0,
	}
}

func (as *AnalyticsService) getStorageByUser(ctx context.Context, limit int) []map[string]interface{} {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":        "$user_id",
				"file_count": bson.M{"$sum": 1},
				"total_size": bson.M{"$sum": "$size"},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "user",
			},
		},
		{
			"$unwind": "$user",
		},
		{
			"$sort": bson.M{"total_size": -1},
		},
		{
			"$limit": limit,
		},
		{
			"$project": bson.M{
				"user_email": "$user.email",
				"file_count": 1,
				"total_size": 1,
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var users []map[string]interface{}
	cursor.All(ctx, &users)
	return users
}

func (as *AnalyticsService) getPopularFiles(ctx context.Context, startDate time.Time, limit int) []map[string]interface{} {
	// Get most downloaded files in the period
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"action":     "download",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id":            "$file_id",
				"download_count": bson.M{"$sum": 1},
				"total_bytes":    bson.M{"$sum": "$bytes"},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "files",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "file",
			},
		},
		{
			"$unwind": "$file",
		},
		{
			"$sort": bson.M{"download_count": -1},
		},
		{
			"$limit": limit,
		},
		{
			"$project": bson.M{
				"file_name":      "$file.name",
				"file_size":      "$file.size",
				"download_count": 1,
				"total_bytes":    1,
			},
		},
	}

	cursor, err := as.collections.Activities().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var files []map[string]interface{}
	cursor.All(ctx, &files)
	return files
}

func (as *AnalyticsService) getFileLifecycleMetrics(ctx context.Context, startDate time.Time) map[string]interface{} {
	// Average time from upload to first download
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
				"is_deleted": false,
			},
		},
		{
			"$lookup": bson.M{
				"from": "activities",
				"let":  bson.M{"file_id": "$_id"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": []bson.M{
									{"$eq": []interface{}{"$file_id", "$$file_id"}},
									{"$eq": []interface{}{"$action", "download"}},
								},
							},
						},
					},
					{
						"$sort": bson.M{"created_at": 1},
					},
					{
						"$limit": 1,
					},
				},
				"as": "first_download",
			},
		},
		{
			"$match": bson.M{
				"first_download": bson.M{"$ne": []interface{}{}},
			},
		},
		{
			"$addFields": bson.M{
				"time_to_first_download": bson.M{
					"$subtract": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$first_download.created_at", 0}},
						"$created_at",
					},
				},
			},
		},
		{
			"$group": bson.M{
				"_id":                        nil,
				"avg_time_to_first_download": bson.M{"$avg": "$time_to_first_download"},
				"total_files":                bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	if len(result) > 0 {
		avgTime := float64(0)
		if time, ok := result[0]["avg_time_to_first_download"].(float64); ok {
			avgTime = time / 1000 / 60 / 60 // Convert to hours
		}

		return map[string]interface{}{
			"avg_time_to_first_download_hours": avgTime,
			"files_with_downloads":             result[0]["total_files"],
		}
	}

	return map[string]interface{}{
		"avg_time_to_first_download_hours": 0,
		"files_with_downloads":             0,
	}
}

func (as *AnalyticsService) getStorageUsageTrend(ctx context.Context, startDate time.Time, groupBy string) []map[string]interface{} {
	// Implementation similar to file upload trend
	return as.getFileUploadTrend(ctx, startDate, groupBy)
}

func (as *AnalyticsService) getProviderDistribution(ctx context.Context, providerID string) []map[string]interface{} {
	filter := bson.M{"is_deleted": false}
	if providerID != "" {
		objID, _ := primitive.ObjectIDFromHex(providerID)
		filter["provider_id"] = objID
	}

	pipeline := []bson.M{
		{
			"$match": filter,
		},
		{
			"$group": bson.M{
				"_id":        "$storage_provider",
				"file_count": bson.M{"$sum": 1},
				"total_size": bson.M{"$sum": "$size"},
			},
		},
		{
			"$sort": bson.M{"total_size": -1},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var distribution []map[string]interface{}
	cursor.All(ctx, &distribution)
	return distribution
}

func (as *AnalyticsService) getBandwidthUsage(ctx context.Context, startDate time.Time, groupBy string) []map[string]interface{} {
	// Calculate bandwidth usage from download activities
	var groupStage bson.M
	switch groupBy {
	case "hour":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
				"hour":  bson.M{"$hour": "$created_at"},
			},
		}
	case "week":
		groupStage = bson.M{
			"_id": bson.M{
				"year": bson.M{"$year": "$created_at"},
				"week": bson.M{"$week": "$created_at"},
			},
		}
	case "month":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
			},
		}
	default: // day
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
			},
		}
	}

	groupStage["total_bytes"] = bson.M{"$sum": "$bytes"}
	groupStage["download_count"] = bson.M{"$sum": 1}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"action":     "download",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": groupStage,
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := as.collections.Activities().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var usage []map[string]interface{}
	cursor.All(ctx, &usage)
	return usage
}

func (as *AnalyticsService) getStorageEfficiency(ctx context.Context) map[string]interface{} {
	// Calculate storage efficiency metrics
	totalFiles, _ := as.collections.Files().CountDocuments(ctx, bson.M{"is_deleted": false})
	duplicateFiles := as.getDuplicateFileCount(ctx)

	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":        nil,
				"total_size": bson.M{"$sum": "$size"},
				"avg_size":   bson.M{"$avg": "$size"},
				"max_size":   bson.M{"$max": "$size"},
				"min_size":   bson.M{"$min": "$size"},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	efficiency := map[string]interface{}{
		"total_files":     totalFiles,
		"duplicate_files": duplicateFiles,
		"efficiency_rate": float64(totalFiles-duplicateFiles) / float64(totalFiles) * 100,
	}

	if len(result) > 0 {
		efficiency["total_size"] = result[0]["total_size"]
		efficiency["avg_file_size"] = result[0]["avg_size"]
		efficiency["max_file_size"] = result[0]["max_size"]
		efficiency["min_file_size"] = result[0]["min_size"]
	}

	return efficiency
}

func (as *AnalyticsService) getDuplicateFileCount(ctx context.Context) int64 {
	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":   "$hash",
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$match": bson.M{
				"count": bson.M{"$gt": 1},
			},
		},
		{
			"$group": bson.M{
				"_id":             nil,
				"duplicate_count": bson.M{"$sum": bson.M{"$subtract": []interface{}{"$count", 1}}},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	if len(result) > 0 {
		if count, ok := result[0]["duplicate_count"].(int64); ok {
			return count
		}
	}

	return 0
}

func (as *AnalyticsService) getStorageCostAnalysis(ctx context.Context, startDate time.Time) map[string]interface{} {
	// Calculate estimated storage costs by provider
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startDate},
				"is_deleted": false,
			},
		},
		{
			"$group": bson.M{
				"_id":        "$storage_provider",
				"total_size": bson.M{"$sum": "$size"},
				"file_count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var providerStats []bson.M
	cursor.All(ctx, &providerStats)

	// Estimate costs based on provider pricing (these would be configurable)
	providerPricing := map[string]float64{
		"s3":     0.023,  // per GB per month
		"r2":     0.015,  // per GB per month
		"wasabi": 0.0059, // per GB per month
	}

	totalCost := float64(0)
	costByProvider := make(map[string]interface{})

	for _, stat := range providerStats {
		provider := stat["_id"].(string)
		sizeGB := float64(stat["total_size"].(int64)) / (1024 * 1024 * 1024)

		if pricing, exists := providerPricing[provider]; exists {
			cost := sizeGB * pricing
			totalCost += cost

			costByProvider[provider] = map[string]interface{}{
				"size_gb":    sizeGB,
				"cost_usd":   cost,
				"file_count": stat["file_count"],
			}
		}
	}

	return map[string]interface{}{
		"total_cost_usd": totalCost,
		"by_provider":    costByProvider,
		"period_days":    int(time.Since(startDate).Hours() / 24),
	}
}

func (as *AnalyticsService) getStoragePerformance(ctx context.Context, startDate time.Time) map[string]interface{} {
	// Calculate upload/download performance metrics
	uploadPipeline := []bson.M{
		{
			"$match": bson.M{
				"action":     "upload",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id":           nil,
				"avg_duration":  bson.M{"$avg": "$duration"},
				"total_uploads": bson.M{"$sum": 1},
				"total_bytes":   bson.M{"$sum": "$bytes"},
			},
		},
	}

	downloadPipeline := []bson.M{
		{
			"$match": bson.M{
				"action":     "download",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id":             nil,
				"avg_duration":    bson.M{"$avg": "$duration"},
				"total_downloads": bson.M{"$sum": 1},
				"total_bytes":     bson.M{"$sum": "$bytes"},
			},
		},
	}

	uploadCursor, _ := as.collections.Activities().Aggregate(ctx, uploadPipeline)
	downloadCursor, _ := as.collections.Activities().Aggregate(ctx, downloadPipeline)

	var uploadResult, downloadResult []bson.M
	uploadCursor.All(ctx, &uploadResult)
	downloadCursor.All(ctx, &downloadResult)

	performance := map[string]interface{}{
		"upload_performance": map[string]interface{}{
			"avg_duration": 0,
			"total_count":  0,
			"total_bytes":  0,
		},
		"download_performance": map[string]interface{}{
			"avg_duration": 0,
			"total_count":  0,
			"total_bytes":  0,
		},
	}

	if len(uploadResult) > 0 {
		performance["upload_performance"] = uploadResult[0]
	}

	if len(downloadResult) > 0 {
		performance["download_performance"] = downloadResult[0]
	}

	return performance
}

func (as *AnalyticsService) getDetailedRevenueTrend(ctx context.Context, startDate time.Time, groupBy, currency string) []map[string]interface{} {
	var groupStage bson.M
	switch groupBy {
	case "hour":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
				"hour":  bson.M{"$hour": "$created_at"},
			},
		}
	case "week":
		groupStage = bson.M{
			"_id": bson.M{
				"year": bson.M{"$year": "$created_at"},
				"week": bson.M{"$week": "$created_at"},
			},
		}
	case "month":
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
			},
		}
	default: // day
		groupStage = bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
			},
		}
	}

	groupStage["revenue"] = bson.M{"$sum": "$amount"}
	groupStage["transaction_count"] = bson.M{"$sum": 1}
	groupStage["avg_transaction"] = bson.M{"$avg": "$amount"}

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":     "completed",
				"currency":   currency,
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": groupStage,
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := as.collections.Payments().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var trend []map[string]interface{}
	cursor.All(ctx, &trend)
	return trend
}

func (as *AnalyticsService) getRevenueByPlan(ctx context.Context, startDate time.Time, currency string) []map[string]interface{} {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":     "completed",
				"currency":   currency,
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "subscriptions",
				"localField":   "subscription_id",
				"foreignField": "_id",
				"as":           "subscription",
			},
		},
		{
			"$unwind": "$subscription",
		},
		{
			"$lookup": bson.M{
				"from":         "plans",
				"localField":   "subscription.plan_id",
				"foreignField": "_id",
				"as":           "plan",
			},
		},
		{
			"$unwind": "$plan",
		},
		{
			"$group": bson.M{
				"_id":               "$plan.name",
				"revenue":           bson.M{"$sum": "$amount"},
				"transaction_count": bson.M{"$sum": 1},
				"plan_price":        bson.M{"$first": "$plan.price"},
			},
		},
		{
			"$sort": bson.M{"revenue": -1},
		},
	}

	cursor, err := as.collections.Payments().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var revenue []map[string]interface{}
	cursor.All(ctx, &revenue)
	return revenue
}

func (as *AnalyticsService) getMRR(ctx context.Context, currency string) float64 {
	// Calculate Monthly Recurring Revenue
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":        "active",
				"billing_cycle": "monthly",
				"currency":      currency,
			},
		},
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": "$price"},
			},
		},
	}

	cursor, err := database.GetCollection("subscriptions").Aggregate(ctx, pipeline)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	if len(result) > 0 {
		if total, ok := result[0]["total"].(float64); ok {
			return total
		}
	}

	return 0
}

func (as *AnalyticsService) getARR(ctx context.Context, currency string) float64 {
	// Calculate Annual Recurring Revenue
	mrr := as.getMRR(ctx, currency)

	// Add yearly subscriptions
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":        "active",
				"billing_cycle": "yearly",
				"currency":      currency,
			},
		},
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": "$price"},
			},
		},
	}

	cursor, err := database.GetCollection("subscriptions").Aggregate(ctx, pipeline)
	if err != nil {
		return mrr * 12
	}
	defer cursor.Close(ctx)

	var result []bson.M
	cursor.All(ctx, &result)

	yearlyRevenue := float64(0)
	if len(result) > 0 {
		if total, ok := result[0]["total"].(float64); ok {
			yearlyRevenue = total
		}
	}

	return (mrr * 12) + yearlyRevenue
}

func (as *AnalyticsService) getCustomerLifetimeValue(ctx context.Context, currency string) map[string]interface{} {
	// Simplified CLV calculation
	totalRevenue := as.getTotalRevenue(ctx)
	totalCustomers, _ := as.collections.Users().CountDocuments(ctx, bson.M{})

	avgRevenue := float64(0)
	if totalCustomers > 0 {
		avgRevenue = totalRevenue / float64(totalCustomers)
	}

	return map[string]interface{}{
		"avg_revenue_per_customer": avgRevenue,
		"total_revenue":            totalRevenue,
		"total_customers":          totalCustomers,
		"currency":                 currency,
	}
}

func (as *AnalyticsService) getChurnAnalysis(ctx context.Context, startDate time.Time) map[string]interface{} {
	// Calculate churn rate
	activeStart, _ := database.GetCollection("subscriptions").CountDocuments(ctx, bson.M{
		"status":     "active",
		"created_at": bson.M{"$lt": startDate},
	})

	churned, _ := database.GetCollection("subscriptions").CountDocuments(ctx, bson.M{
		"status":     bson.M{"$in": []string{"cancelled", "expired"}},
		"updated_at": bson.M{"$gte": startDate},
	})

	churnRate := float64(0)
	if activeStart > 0 {
		churnRate = (float64(churned) / float64(activeStart)) * 100
	}

	return map[string]interface{}{
		"churn_rate":        churnRate,
		"churned_customers": churned,
		"active_at_start":   activeStart,
		"period_start":      startDate,
	}
}

func (as *AnalyticsService) getPaymentMethodDistribution(ctx context.Context, startDate time.Time) []map[string]interface{} {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":     "completed",
				"created_at": bson.M{"$gte": startDate},
			},
		},
		{
			"$group": bson.M{
				"_id":               "$payment_method",
				"transaction_count": bson.M{"$sum": 1},
				"total_amount":      bson.M{"$sum": "$amount"},
			},
		},
		{
			"$sort": bson.M{"transaction_count": -1},
		},
	}

	cursor, err := as.collections.Payments().Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var distribution []map[string]interface{}
	cursor.All(ctx, &distribution)
	return distribution
}

func (as *AnalyticsService) getRevenueForecast(ctx context.Context, days int, currency string) map[string]interface{} {
	// Simple forecast based on recent trends
	currentMRR := as.getMRR(ctx, currency)

	// Calculate growth rate from last 3 months
	threeMonthsAgo := time.Now().AddDate(0, -3, 0)
	recentTrend := as.getDetailedRevenueTrend(ctx, threeMonthsAgo, "month", currency)

	growthRate := float64(0)
	if len(recentTrend) >= 2 {
		oldRevenue := recentTrend[0]["revenue"].(float64)
		newRevenue := recentTrend[len(recentTrend)-1]["revenue"].(float64)
		if oldRevenue > 0 {
			growthRate = ((newRevenue - oldRevenue) / oldRevenue) * 100
		}
	}

	forecaseMonths := days / 30
	forecastRevenue := currentMRR * float64(forecaseMonths)

	if growthRate > 0 {
		forecastRevenue = forecastRevenue * (1 + (growthRate / 100))
	}

	return map[string]interface{}{
		"forecast_period_days": days,
		"current_mrr":          currentMRR,
		"growth_rate":          growthRate,
		"forecast_revenue":     forecastRevenue,
		"currency":             currency,
	}
}

func (as *AnalyticsService) getSystemLoad(ctx context.Context) map[string]interface{} {
	// Get current system metrics
	now := time.Now()
	fiveMinutesAgo := now.Add(-5 * time.Minute)

	recentActivity, _ := as.collections.Activities().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": fiveMinutesAgo},
	})

	recentUploads, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": fiveMinutesAgo},
	})

	activeConnections, _ := as.collections.Sessions().CountDocuments(ctx, bson.M{
		"last_activity": bson.M{"$gte": fiveMinutesAgo},
		"is_active":     true,
	})

	return map[string]interface{}{
		"recent_activity":    recentActivity,
		"recent_uploads":     recentUploads,
		"active_connections": activeConnections,
		"cpu_usage":          "85%", // Would be from system monitoring
		"memory_usage":       "72%", // Would be from system monitoring
		"disk_usage":         "45%", // Would be from system monitoring
		"timestamp":          now,
	}
}

// Helper functions for ExportAnalytics
func (as *AnalyticsService) exportUserData(ctx context.Context, period, groupBy string) (interface{}, error) {
	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}
	startDate := time.Now().AddDate(0, 0, -days)

	cursor, err := as.collections.Users().Find(ctx,
		bson.M{"created_at": bson.M{"$gte": startDate}},
		options.Find().SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []bson.M
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (as *AnalyticsService) exportFileData(ctx context.Context, period, groupBy string) (interface{}, error) {
	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}
	startDate := time.Now().AddDate(0, 0, -days)

	cursor, err := as.collections.Files().Find(ctx,
		bson.M{"created_at": bson.M{"$gte": startDate}},
		options.Find().SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var files []bson.M
	if err = cursor.All(ctx, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (as *AnalyticsService) exportStorageData(ctx context.Context, period, groupBy string) (interface{}, error) {
	return as.GetStorageAnalytics(period, groupBy, "")
}

func (as *AnalyticsService) exportRevenueData(ctx context.Context, period, groupBy string) (interface{}, error) {
	return as.GetRevenueAnalytics(period, groupBy, "USD")
}

func (as *AnalyticsService) generateExportFile(data interface{}, format, dataType, period string) (string, error) {
	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_export_%s_%s.%s", dataType, period, timestamp, format)

	// Create exports directory if it doesn't exist
	exportDir := "./exports"
	os.MkdirAll(exportDir, 0755)
	filePath := filepath.Join(exportDir, fileName)

	switch format {
	case "csv":
		return fileName, as.generateCSVFile(data, filePath)
	case "excel":
		return fileName, as.generateExcelFile(data, filePath)
	case "pdf":
		return fileName, as.generatePDFFile(data, filePath)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func (as *AnalyticsService) generateCSVFile(data interface{}, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Convert data to CSV format based on type
	switch v := data.(type) {
	case []bson.M:
		if len(v) > 0 {
			// Write headers
			var headers []string
			for key := range v[0] {
				headers = append(headers, key)
			}
			writer.Write(headers)

			// Write data
			for _, record := range v {
				var row []string
				for _, header := range headers {
					if val, exists := record[header]; exists {
						row = append(row, fmt.Sprintf("%v", val))
					} else {
						row = append(row, "")
					}
				}
				writer.Write(row)
			}
		}
	case map[string]interface{}:
		// Write key-value pairs
		writer.Write([]string{"Key", "Value"})
		for key, value := range v {
			writer.Write([]string{key, fmt.Sprintf("%v", value)})
		}
	}

	return nil
}

func (as *AnalyticsService) generateExcelFile(data interface{}, filePath string) error {
	// For now, just generate CSV and rename extension
	csvPath := strings.Replace(filePath, ".excel", ".csv", 1)
	err := as.generateCSVFile(data, csvPath)
	if err != nil {
		return err
	}
	return os.Rename(csvPath, filePath)
}

func (as *AnalyticsService) generatePDFFile(data interface{}, filePath string) error {
	// Simple text file for PDF placeholder
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("PDF Export - Data would be formatted here\n")
	return err
}

func (as *AnalyticsService) sendExportEmail(email, fileName, dataType, format string) error {
	// Email sending logic placeholder
	// In a real implementation, this would use an email service
	return nil
}

// Helper functions for GetSystemMetrics
func (as *AnalyticsService) getDatabaseMetrics(ctx context.Context) (map[string]interface{}, error) {
	// Get database statistics
	dbStats := as.collections.Users().Database().RunCommand(ctx, bson.M{"dbStats": 1})
	var dbInfo bson.M
	if err := dbStats.Decode(&dbInfo); err != nil {
		return nil, err
	}

	// Get collection stats
	collections := []string{"users", "files", "plans", "activities"}
	collectionStats := make(map[string]interface{})

	for _, collName := range collections {
		coll := as.collections.Analytics().Database().Collection(collName)
		count, _ := coll.EstimatedDocumentCount(ctx)
		collectionStats[collName] = map[string]interface{}{
			"document_count": count,
		}
	}

	return map[string]interface{}{
		"database_size": dbInfo["dataSize"],
		"index_size":    dbInfo["indexSize"],
		"collections":   collectionStats,
		"connections":   dbInfo["connections"],
	}, nil
}

func (as *AnalyticsService) getAPIMetrics(ctx context.Context, startTime time.Time) (map[string]interface{}, error) {
	// Get API request metrics from logs
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startTime},
				"type":       "api_request",
			},
		},
		{
			"$group": bson.M{
				"_id":               "$endpoint",
				"request_count":     bson.M{"$sum": 1},
				"avg_response_time": bson.M{"$avg": "$response_time"},
				"error_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$gte": []interface{}{"$status_code", 400}},
							"then": 1,
							"else": 0,
						},
					},
				},
			},
		},
		{
			"$sort": bson.M{"request_count": -1},
		},
		{
			"$limit": 20,
		},
	}

	cursor, err := as.collections.Logs().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var endpointStats []bson.M
	if err = cursor.All(ctx, &endpointStats); err != nil {
		return nil, err
	}

	// Get overall API metrics
	totalRequests, _ := as.collections.Logs().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startTime},
		"type":       "api_request",
	})

	errorRequests, _ := as.collections.Logs().CountDocuments(ctx, bson.M{
		"created_at":  bson.M{"$gte": startTime},
		"type":        "api_request",
		"status_code": bson.M{"$gte": 400},
	})

	errorRate := float64(0)
	if totalRequests > 0 {
		errorRate = float64(errorRequests) / float64(totalRequests) * 100
	}

	return map[string]interface{}{
		"total_requests": totalRequests,
		"error_requests": errorRequests,
		"error_rate":     errorRate,
		"endpoint_stats": endpointStats,
	}, nil
}

func (as *AnalyticsService) getStorageMetrics(ctx context.Context, startTime time.Time) (map[string]interface{}, error) {
	// Storage operations metrics
	uploadCount, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startTime},
	})

	downloadCount, _ := as.collections.Files().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startTime},
		"action":     "download",
	})

	// Storage usage by provider
	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":        "$storage_provider",
				"file_count": bson.M{"$sum": 1},
				"total_size": bson.M{"$sum": "$size"},
			},
		},
	}

	cursor, err := as.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var providerStats []bson.M
	if err = cursor.All(ctx, &providerStats); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"uploads_count":   uploadCount,
		"downloads_count": downloadCount,
		"provider_stats":  providerStats,
	}, nil
}

func (as *AnalyticsService) getResourceMetrics(ctx context.Context, startTime time.Time) map[string]interface{} {
	// Basic resource metrics (placeholder for actual system monitoring)
	return map[string]interface{}{
		"cpu_usage":    "N/A", // Would integrate with system monitoring
		"memory_usage": "N/A",
		"disk_usage":   "N/A",
		"uptime":       "N/A",
		"note":         "Requires system monitoring integration",
	}
}

func (as *AnalyticsService) getErrorMetrics(ctx context.Context, startTime time.Time) (map[string]interface{}, error) {
	// Get error metrics from logs
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"created_at": bson.M{"$gte": startTime},
				"level":      bson.M{"$in": []string{"error", "fatal"}},
			},
		},
		{
			"$group": bson.M{
				"_id":   "$level",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := as.collections.Logs().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var errorStats []bson.M
	if err = cursor.All(ctx, &errorStats); err != nil {
		return nil, err
	}

	// Get total error count
	totalErrors, _ := as.collections.Logs().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startTime},
		"level":      bson.M{"$in": []string{"error", "fatal"}},
	})

	return map[string]interface{}{
		"total_errors":    totalErrors,
		"error_breakdown": errorStats,
	}, nil
}

func (as *AnalyticsService) getConnectionMetrics(ctx context.Context) (map[string]interface{}, error) {
	// Active sessions in last 5 minutes
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	activeSessions, _ := as.collections.Sessions().CountDocuments(ctx, bson.M{
		"last_activity": bson.M{"$gte": fiveMinutesAgo},
		"is_active":     true,
	})

	// Total sessions today
	today := time.Now().Truncate(24 * time.Hour)
	todaySessions, _ := as.collections.Sessions().CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": today},
	})

	return map[string]interface{}{
		"active_sessions": activeSessions,
		"today_sessions":  todaySessions,
	}, nil
}

func (as *AnalyticsService) getCacheMetrics(ctx context.Context, startTime time.Time) map[string]interface{} {
	// Cache metrics (placeholder - would integrate with actual cache system like Redis)
	return map[string]interface{}{
		"hit_rate":   "N/A",
		"miss_rate":  "N/A",
		"cache_size": "N/A",
		"note":       "Requires cache system integration (Redis, Memcached, etc.)",
	}
}

// Helper function to calculate growth rate percentage
func calculateGrowthRate(previous, current int64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0 // 100% growth from zero
		}
		return 0.0 // No growth if both are zero
	}

	growth := float64(current-previous) / float64(previous) * 100
	return math.Round(growth*100) / 100 // Round to 2 decimal places
}

// Alternative version if you need to handle different input types
func calculateGrowthRateFloat(previous, current float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}

	growth := (current - previous) / previous * 100
	return math.Round(growth*100) / 100
}

// Helper function to format growth rate with sign
func formatGrowthRate(rate float64) string {
	if rate > 0 {
		return fmt.Sprintf("+%.2f%%", rate)
	} else if rate < 0 {
		return fmt.Sprintf("%.2f%%", rate)
	}
	return "0.00%"
}
