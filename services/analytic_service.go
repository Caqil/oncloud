package services

import (
	"context"
	"fmt"
	"oncloud/database"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AnalyticsService struct {
	userCollection      *mongo.Collection
	fileCollection      *mongo.Collection
	planCollection      *mongo.Collection
	paymentCollection   *mongo.Collection
	sessionCollection   *mongo.Collection
	activityCollection  *mongo.Collection
	analyticsCollection *mongo.Collection
}

func NewAnalyticsService() *AnalyticsService {
	return &AnalyticsService{
		userCollection:      database.GetCollection("users"),
		fileCollection:      database.GetCollection("files"),
		planCollection:      database.GetCollection("plans"),
		paymentCollection:   database.GetCollection("payments"),
		sessionCollection:   database.GetCollection("sessions"),
		activityCollection:  database.GetCollection("activities"),
		analyticsCollection: database.GetCollection("analytics"),
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
	totalUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{})
	totalFiles, _ := as.fileCollection.CountDocuments(ctx, bson.M{"is_deleted": false})
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
	onlineUsers, _ := as.sessionCollection.CountDocuments(ctx, bson.M{
		"last_activity": bson.M{"$gte": fiveMinutesAgo},
		"is_active":     true,
	})

	// Today's activity
	today := time.Now().Truncate(24 * time.Hour)
	todayUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": today},
	})

	todayUploads, _ := as.fileCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": today},
	})

	// Recent uploads (last hour)
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	recentUploads, _ := as.fileCollection.CountDocuments(ctx, bson.M{
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

	cursor, err := as.activityCollection.Aggregate(ctx, pipeline)
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

	_, err := as.analyticsCollection.InsertOne(ctx, event)
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

	cursor, err := as.paymentCollection.Aggregate(ctx, pipeline)
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
	count, _ := as.userCollection.CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": startDate},
	})
	return count
}

func (as *AnalyticsService) getGrowthMetrics(ctx context.Context, startDate time.Time, period string) map[string]interface{} {
	endDate := time.Now()

	newUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	})

	newFiles, _ := as.fileCollection.CountDocuments(ctx, bson.M{
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

	prevUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{
			"$gte": previousStart,
			"$lt":  startDate,
		},
	})

	prevFiles, _ := as.fileCollection.CountDocuments(ctx, bson.M{
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

func (as *AnalyticsService) getRecentActivity(ctx context.Context, limit int) []map[string]interface{} {
	cursor, err := as.activityCollection.Find(ctx, bson.M{},
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.paymentCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.userCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer cursor.Close(ctx)

	var trend []map[string]interface{}
	cursor.All(ctx, &trend)
	return trend
}

func (as *AnalyticsService) getUserActivityMetrics(ctx context.Context, startDate time.Time) map[string]interface{} {
	activeUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": startDate},
	})

	totalSessions, _ := as.sessionCollection.CountDocuments(ctx, bson.M{
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

	cursor, _ := as.sessionCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.userCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.userCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.userCollection.Aggregate(ctx, pipeline)
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
	totalUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": startDate},
	})

	activeUsers, _ := as.userCollection.CountDocuments(ctx, bson.M{
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.activityCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.activityCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.activityCollection.Aggregate(ctx, pipeline)
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
	totalFiles, _ := as.fileCollection.CountDocuments(ctx, bson.M{"is_deleted": false})
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
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

	uploadCursor, _ := as.activityCollection.Aggregate(ctx, uploadPipeline)
	downloadCursor, _ := as.activityCollection.Aggregate(ctx, downloadPipeline)

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

	cursor, err := as.paymentCollection.Aggregate(ctx, pipeline)
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

	cursor, err := as.paymentCollection.Aggregate(ctx, pipeline)
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
	totalCustomers, _ := as.userCollection.CountDocuments(ctx, bson.M{})

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

	cursor, err := as.paymentCollection.Aggregate(ctx, pipeline)
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

	recentActivity, _ := as.activityCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": fiveMinutesAgo},
	})

	recentUploads, _ := as.fileCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": fiveMinutesAgo},
	})

	activeConnections, _ := as.sessionCollection.CountDocuments(ctx, bson.M{
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
