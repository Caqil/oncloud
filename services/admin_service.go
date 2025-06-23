package services

import (
	"context"
	"fmt"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"time"

	"golang.org/x/crypto/bcrypt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AdminService struct {
	adminCollection    *mongo.Collection
	userCollection     *mongo.Collection
	fileCollection     *mongo.Collection
	planCollection     *mongo.Collection
	settingsCollection *mongo.Collection
	logCollection      *mongo.Collection
}

func NewAdminService() *AdminService {
	return &AdminService{
		adminCollection:    database.GetCollection("admins"),
		userCollection:     database.GetCollection("users"),
		fileCollection:     database.GetCollection("files"),
		planCollection:     database.GetCollection("plans"),
		settingsCollection: database.GetCollection("settings"),
		logCollection:      database.GetCollection("logs"),
	}
}

// Admin Service - Login Function
func (as *AdminService) Login(email, password string) (*models.Admin, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var admin models.Admin
	err := as.adminCollection.FindOne(ctx, bson.M{
		"email":     email,
		"is_active": true,
	}).Decode(&admin)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, "", fmt.Errorf("invalid credentials")
		}
		return nil, "", fmt.Errorf("login failed: %v", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)); err != nil {
		return nil, "", fmt.Errorf("invalid credentials")
	}

	// Generate JWT token
	token, err := utils.GenerateAdminToken(admin.ID, admin.ID.Hex(), admin.Role, "super_admin", []string{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %v", err)
	}

	// Update last login
	_, err = as.adminCollection.UpdateOne(ctx,
		bson.M{"_id": admin.ID},
		bson.M{"$set": bson.M{
			"last_login_at": time.Now(),
			"updated_at":    time.Now(),
		}},
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to update login time: %v", err)
	}

	return &admin, token, nil
}

// Admin Management
func (as *AdminService) GetAllAdmins(page, limit int) ([]models.Admin, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := as.adminCollection.Find(ctx, bson.M{},
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var admins []models.Admin
	if err = cursor.All(ctx, &admins); err != nil {
		return nil, 0, err
	}

	total, err := as.adminCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	return admins, int(total), nil
}

func (as *AdminService) GetAdminByID(adminID primitive.ObjectID) (*models.Admin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var admin models.Admin
	err := as.adminCollection.FindOne(ctx, bson.M{"_id": adminID}).Decode(&admin)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %v", err)
	}

	return &admin, nil
}

func (as *AdminService) CreateAdmin(admin *models.Admin) (*models.Admin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if admin email already exists
	count, err := as.adminCollection.CountDocuments(ctx, bson.M{"email": admin.Email})
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("admin with email %s already exists", admin.Email)
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(admin.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	admin.ID = primitive.NewObjectID()
	admin.Password = hashedPassword
	admin.IsActive = true
	admin.CreatedAt = time.Now()
	admin.UpdatedAt = time.Now()

	_, err = as.adminCollection.InsertOne(ctx, admin)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin: %v", err)
	}

	// Clear password from response
	admin.Password = ""
	return admin, nil
}

func (as *AdminService) UpdateAdmin(adminID primitive.ObjectID, updates map[string]interface{}) (*models.Admin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Handle password update
	if password, exists := updates["password"]; exists {
		hashedPassword, err := utils.HashPassword(password.(string))
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
		updates["password"] = hashedPassword
	}

	updates["updated_at"] = time.Now()

	_, err := as.adminCollection.UpdateOne(ctx,
		bson.M{"_id": adminID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update admin: %v", err)
	}

	return as.GetAdminByID(adminID)
}

func (as *AdminService) DeleteAdmin(adminID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := as.adminCollection.DeleteOne(ctx, bson.M{"_id": adminID})
	if err != nil {
		return fmt.Errorf("failed to delete admin: %v", err)
	}

	return nil
}

func (as *AdminService) ActivateAdmin(adminID primitive.ObjectID) error {
	return as.UpdateAdminStatus(adminID, true)
}

func (as *AdminService) DeactivateAdmin(adminID primitive.ObjectID) error {
	return as.UpdateAdminStatus(adminID, false)
}

func (as *AdminService) UpdateAdminStatus(adminID primitive.ObjectID, isActive bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := as.adminCollection.UpdateOne(ctx,
		bson.M{"_id": adminID},
		bson.M{"$set": bson.M{
			"is_active":  isActive,
			"updated_at": time.Now(),
		}},
	)
	return err
}

// Dashboard and System Management
func (as *AdminService) GetDashboardStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats := make(map[string]interface{})

	// Users count
	userCount, err := as.userCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats["total_users"] = userCount

	// Active users (logged in last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	activeUserCount, err := as.userCollection.CountDocuments(ctx, bson.M{
		"last_login_at": bson.M{"$gte": thirtyDaysAgo},
	})
	if err != nil {
		return nil, err
	}
	stats["active_users"] = activeUserCount

	// Files count
	fileCount, err := as.fileCollection.CountDocuments(ctx, bson.M{
		"is_deleted": false,
	})
	if err != nil {
		return nil, err
	}
	stats["total_files"] = fileCount

	// Storage usage
	pipeline := []bson.M{
		{
			"$match": bson.M{"is_deleted": false},
		},
		{
			"$group": bson.M{
				"_id":         nil,
				"total_size":  bson.M{"$sum": "$size"},
				"total_files": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := as.fileCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var storageStats []bson.M
	if err = cursor.All(ctx, &storageStats); err != nil {
		return nil, err
	}

	if len(storageStats) > 0 {
		stats["total_storage_used"] = storageStats[0]["total_size"]
	} else {
		stats["total_storage_used"] = 0
	}

	// Plans count
	planCount, err := as.planCollection.CountDocuments(ctx, bson.M{
		"is_active": true,
	})
	if err != nil {
		return nil, err
	}
	stats["total_plans"] = planCount

	// Recent registrations (last 7 days)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	recentUsers, err := as.userCollection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": sevenDaysAgo},
	})
	if err != nil {
		return nil, err
	}
	stats["recent_registrations"] = recentUsers

	return stats, nil
}

func (as *AdminService) GetSystemInfo() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info := make(map[string]interface{})

	// Database info
	dbStats := as.adminCollection.Database().RunCommand(ctx, bson.M{"dbStats": 1})
	var dbInfo bson.M
	if err := dbStats.Decode(&dbInfo); err == nil {
		info["database"] = dbInfo
	}

	// Server status
	serverStatus := as.adminCollection.Database().RunCommand(ctx, bson.M{"serverStatus": 1})
	var serverInfo bson.M
	if err := serverStatus.Decode(&serverInfo); err == nil {
		info["server"] = serverInfo
	}

	info["timestamp"] = time.Now()
	return info, nil
}

func (as *AdminService) GetSystemHealth() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health := make(map[string]interface{})

	// Database health
	err := as.adminCollection.Database().Client().Ping(ctx, nil)
	health["database"] = map[string]interface{}{
		"status": "healthy",
		"error":  nil,
	}
	if err != nil {
		health["database"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	// Overall status
	overallHealthy := err == nil
	status := "unhealthy"
	if overallHealthy {
		status = "healthy"
	}
	health["overall"] = map[string]interface{}{
		"status":    status,
		"timestamp": time.Now(),
	}

	return health, nil
}

// System Maintenance
func (as *AdminService) ClearCache() error {
	// Implementation for cache clearing
	// This would depend on your caching strategy (Redis, in-memory, etc.)
	return nil
}

func (as *AdminService) ClearLogs() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Keep logs from last 30 days only
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	_, err := as.logCollection.DeleteMany(ctx, bson.M{
		"created_at": bson.M{"$lt": thirtyDaysAgo},
	})
	return err
}

func (as *AdminService) GetLogs(page, limit int, level string) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if level != "" && level != "all" {
		filter["level"] = level
	}

	skip := (page - 1) * limit

	cursor, err := as.logCollection.Find(ctx, filter,
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []map[string]interface{}
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	total, err := as.logCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return logs, int(total), nil
}

func (as *AdminService) CreateSystemBackup() (map[string]interface{}, error) {
	_, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	backupID := primitive.NewObjectID()
	backup := map[string]interface{}{
		"_id":        backupID,
		"status":     "initiated",
		"created_at": time.Now(),
		"size":       0,
		"collections": []string{
			"users", "files", "plans", "admins", "settings",
		},
	}

	// Implementation would create actual backup
	// This is a placeholder for the backup logic
	backup["status"] = "completed"
	backup["size"] = 1024 * 1024 // 1MB placeholder

	return backup, nil
}

func (as *AdminService) GetSystemBackups() ([]map[string]interface{}, error) {
	// Implementation would retrieve actual backups
	// This is a placeholder
	backups := []map[string]interface{}{
		{
			"id":         primitive.NewObjectID(),
			"created_at": time.Now().AddDate(0, 0, -1),
			"size":       1024 * 1024,
			"status":     "completed",
		},
	}

	return backups, nil
}

// Authentication
func (as *AdminService) AuthenticateAdmin(email, password string) (*models.Admin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var admin models.Admin
	err := as.adminCollection.FindOne(ctx, bson.M{
		"email":     email,
		"is_active": true,
	}).Decode(&admin)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !utils.CheckPasswordHash(password, admin.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	as.adminCollection.UpdateOne(ctx,
		bson.M{"_id": admin.ID},
		bson.M{"$set": bson.M{"last_login_at": time.Now()}},
	)

	// Clear password from response
	admin.Password = ""
	return &admin, nil
}

func (as *AdminService) UpdateLastLogin(adminID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := as.adminCollection.UpdateOne(ctx,
		bson.M{"_id": adminID},
		bson.M{"$set": bson.M{"last_login_at": time.Now()}},
	)
	return err
}
