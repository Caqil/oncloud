package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserService struct {
	*BaseService
}

type UserFilters struct {
	Search    string
	Status    string
	PlanID    string
	SortBy    string
	SortOrder string
}

func NewUserService() *UserService {
	return &UserService{
		BaseService: NewBaseService(),
	}
}
func (us *UserService) GetByID(userID primitive.ObjectID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := us.collections.Users().FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	user.Password = ""
	return &user, nil
}

// UpdateUser updates user fields
func (us *UserService) UpdateUser(userID primitive.ObjectID, updates bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": updates},
	)
	return err
}

// UpdateLastLogin updates user's last login time
func (us *UserService) UpdateLastLogin(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{"last_login_at": time.Now()}},
	)
	return err
}

// GetUserPlan retrieves user's current plan
func (us *UserService) GetUserPlan(userID primitive.ObjectID) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get user to find plan ID
	var user models.User
	err := us.collections.Users().FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get plan details
	var plan models.Plan
	err = us.collections.Plans().FindOne(ctx, bson.M{"_id": user.PlanID}).Decode(&plan)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %v", err)
	}

	return &plan, nil
}

// UpdateProfile updates user profile information
func (us *UserService) UpdateProfile(userID primitive.ObjectID, firstName, lastName, phone, country string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates := bson.M{
		"first_name": firstName,
		"last_name":  lastName,
		"phone":      phone,
		"country":    country,
		"updated_at": time.Now(),
	}

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %v", err)
	}

	return us.GetByID(userID)
}

// UploadAvatar uploads and sets user avatar
func (us *UserService) UploadAvatar(userID primitive.ObjectID, file multipart.File, header *multipart.FileHeader) (string, error) {
	// Process avatar upload
	uploadConfig := &utils.UploadConfig{
		MaxFileSize:  5 * 1024 * 1024, // 5MB
		AllowedTypes: []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
	}

	fileInfo, err := utils.ProcessFileUpload(header, uploadConfig)
	if err != nil {
		return "", fmt.Errorf("invalid avatar file: %v", err)
	}

	// Generate avatar URL (implement storage service integration)
	avatarURL := fmt.Sprintf("/uploads/avatars/%s", fileInfo.Name)

	// Update user avatar
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"avatar":     avatarURL,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return "", fmt.Errorf("failed to update avatar: %v", err)
	}

	return avatarURL, nil
}

// DeleteAvatar removes user avatar
func (us *UserService) DeleteAvatar(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"avatar":     "",
			"updated_at": time.Now(),
		}},
	)
	return err
}

// GetUserStats calculates user statistics
func (us *UserService) GetUserStats(userID primitive.ObjectID) (*models.UserStats, error) {
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user and plan
	user, err := us.GetByID(userID)
	if err != nil {
		return nil, err
	}

	plan, err := us.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// Calculate percentages
	storagePercent := float64(0)
	if plan.StorageLimit > 0 {
		storagePercent = utils.CalculateStorageUsage(user.StorageUsed, plan.StorageLimit)
	}

	bandwidthPercent := float64(0)
	if plan.BandwidthLimit > 0 {
		bandwidthPercent = utils.CalculateStorageUsage(user.BandwidthUsed, plan.BandwidthLimit)
	}

	return &models.UserStats{
		StorageUsed:      user.StorageUsed,
		StorageLimit:     plan.StorageLimit,
		BandwidthUsed:    user.BandwidthUsed,
		BandwidthLimit:   plan.BandwidthLimit,
		FilesCount:       user.FilesCount,
		FoldersCount:     user.FoldersCount,
		StoragePercent:   storagePercent,
		BandwidthPercent: bandwidthPercent,
	}, nil
}

// GetDashboardData returns dashboard data for user
func (us *UserService) GetDashboardData(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user stats
	stats, err := us.GetUserStats(userID)
	if err != nil {
		return nil, err
	}

	// Get recent files
	recentFiles, err := us.getRecentFiles(ctx, userID, 5)
	if err != nil {
		return nil, err
	}

	// Get recent folders
	recentFolders, err := us.getRecentFolders(ctx, userID, 5)
	if err != nil {
		return nil, err
	}

	// Get storage usage by type
	storageByType, err := us.getStorageByType(ctx, userID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"stats":           stats,
		"recent_files":    recentFiles,
		"recent_folders":  recentFolders,
		"storage_by_type": storageByType,
		"last_updated":    time.Now(),
	}, nil
}

// GetUserActivity returns user activity log
func (us *UserService) GetUserActivity(userID primitive.ObjectID, page, limit int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calculate skip
	skip := (page - 1) * limit

	// Get activities
	cursor, err := us.collections.Activities().Find(ctx,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.M{"created_at": -1}).SetSkip(int64(skip)).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var activities []map[string]interface{}
	if err = cursor.All(ctx, &activities); err != nil {
		return nil, 0, err
	}

	// Get total count
	total, err := us.collections.Activities().CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, 0, err
	}

	return activities, int(total), nil
}

// GetNotifications returns user notifications
func (us *UserService) GetNotifications(userID primitive.ObjectID, page, limit int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := us.collections.Notifications().Find(ctx,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.M{"created_at": -1}).SetSkip(int64(skip)).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var notifications []map[string]interface{}
	if err = cursor.All(ctx, &notifications); err != nil {
		return nil, 0, err
	}

	total, err := us.collections.Notifications().CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, 0, err
	}

	return notifications, int(total), nil
}

// MarkNotificationRead marks notification as read
func (us *UserService) MarkNotificationRead(userID, notificationID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Notifications().UpdateOne(ctx,
		bson.M{"_id": notificationID, "user_id": userID},
		bson.M{"$set": bson.M{
			"is_read": true,
			"read_at": time.Now(),
		}},
	)
	return err
}

// GetUserSettings returns user settings
func (us *UserService) GetUserSettings(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Default user settings
	settings := map[string]interface{}{
		"email_notifications": true,
		"push_notifications":  true,
		"auto_sync":           true,
		"public_profile":      false,
		"theme":               "light",
		"language":            "en",
		"timezone":            "UTC",
		"two_factor_enabled":  false,
	}

	// Get user-specific settings from database if they exist
	settingsCollection := database.GetCollection("user_settings")
	var userSettings map[string]interface{}
	err := settingsCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&userSettings)
	if err == nil {
		// Merge with defaults
		for key, value := range userSettings {
			if key != "_id" && key != "user_id" {
				settings[key] = value
			}
		}
	}

	return settings, nil
}

// UpdateUserSettings updates user settings
func (us *UserService) UpdateUserSettings(userID primitive.ObjectID, settings map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	settingsCollection := database.GetCollection("user_settings")

	// Add metadata
	settings["user_id"] = userID
	settings["updated_at"] = time.Now()

	// Upsert settings
	_, err := settingsCollection.ReplaceOne(ctx,
		bson.M{"user_id": userID},
		settings,
		options.Replace().SetUpsert(true),
	)
	return err
}

// GetActiveSessions returns user's active sessions
func (us *UserService) GetActiveSessions(userID primitive.ObjectID) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := us.collections.Sessions().Find(ctx,
		bson.M{
			"user_id":    userID,
			"is_active":  true,
			"expires_at": bson.M{"$gt": time.Now()},
		},
		options.Find().SetSort(bson.M{"last_activity": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []map[string]interface{}
	if err = cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

// RevokeSession revokes a user session
func (us *UserService) RevokeSession(userID primitive.ObjectID, sessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Sessions().UpdateOne(ctx,
		bson.M{"_id": sessionID, "user_id": userID},
		bson.M{"$set": bson.M{
			"is_active":  false,
			"revoked_at": time.Now(),
		}},
	)
	return err
}

// API Keys management
func (us *UserService) GetAPIKeys(userID primitive.ObjectID) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := us.collections.APIKeys().Find(ctx,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var apiKeys []map[string]interface{}
	if err = cursor.All(ctx, &apiKeys); err != nil {
		return nil, err
	}

	// Hide actual keys, only show masked versions
	for i, key := range apiKeys {
		if keyStr, ok := key["api_key"].(string); ok && len(keyStr) > 8 {
			apiKeys[i]["api_key"] = keyStr[:4] + "****" + keyStr[len(keyStr)-4:]
		}
	}

	return apiKeys, nil
}

// CreateAPIKey creates a new API key
func (us *UserService) CreateAPIKey(userID primitive.ObjectID, name string, permissions []string, expiresAt *int64) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate API key
	apiKey, err := utils.GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %v", err)
	}

	// Prepare expiration
	var expiry *time.Time
	if expiresAt != nil {
		t := time.Unix(*expiresAt, 0)
		expiry = &t
	}

	// Create API key record
	keyRecord := map[string]interface{}{
		"_id":         primitive.NewObjectID(),
		"user_id":     userID,
		"name":        name,
		"api_key":     apiKey,
		"permissions": permissions,
		"expires_at":  expiry,
		"is_active":   true,
		"created_at":  time.Now(),
		"last_used":   nil,
	}

	_, err = us.collections.APIKeys().InsertOne(ctx, keyRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %v", err)
	}

	// Return key with actual value (only time it's shown)
	return keyRecord, nil
}

// UpdateAPIKey updates API key
func (us *UserService) UpdateAPIKey(userID, keyID primitive.ObjectID, name string, permissions []string, isActive *bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates := bson.M{"updated_at": time.Now()}

	if name != "" {
		updates["name"] = name
	}
	if permissions != nil {
		updates["permissions"] = permissions
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	_, err := us.collections.APIKeys().UpdateOne(ctx,
		bson.M{"_id": keyID, "user_id": userID},
		bson.M{"$set": updates},
	)
	return err
}

// DeleteAPIKey deletes API key
func (us *UserService) DeleteAPIKey(userID, keyID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.APIKeys().DeleteOne(ctx, bson.M{"_id": keyID, "user_id": userID})
	return err
}

// 2FA methods
func (us *UserService) Get2FAStatus(userID primitive.ObjectID) (map[string]interface{}, error) {
	_, err := us.GetByID(userID)
	if err != nil {
		return nil, err
	}

	// Check if 2FA is enabled (implement 2FA table)
	return map[string]interface{}{
		"enabled":      false,
		"backup_codes": 0,
		"last_used":    nil,
		"setup_date":   nil,
	}, nil
}

func (us *UserService) Enable2FA(userID primitive.ObjectID) (string, string, error) {
	// Generate TOTP secret and QR code
	// This would integrate with a TOTP library
	secret := "JBSWY3DPEHPK3PXP" // Example secret
	qrCode := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

	return qrCode, secret, nil
}

func (us *UserService) Verify2FA(userID primitive.ObjectID, code string) ([]string, error) {
	// Verify TOTP code and enable 2FA
	// Generate backup codes
	backupCodes := []string{
		"12345678", "87654321", "11111111", "22222222", "33333333",
		"44444444", "55555555", "66666666", "77777777", "88888888",
	}

	return backupCodes, nil
}

func (us *UserService) Disable2FA(userID primitive.ObjectID, code string) error {
	// Verify code and disable 2FA
	return nil
}

func (us *UserService) GetBackupCodes(userID primitive.ObjectID) ([]string, error) {
	// Get remaining backup codes
	return []string{"12345678", "87654321"}, nil
}

func (us *UserService) RegenerateBackupCodes(userID primitive.ObjectID) ([]string, error) {
	// Generate new backup codes
	newCodes := []string{
		"98765432", "12348765", "55544433", "66677788", "99900011",
		"22211100", "33322211", "44433322", "55566644", "66699977",
	}

	return newCodes, nil
}

// Admin methods for user management
func (us *UserService) GetUsersForAdmin(page, limit int, filters *UserFilters) ([]models.User, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build filter query
	filter := bson.M{}

	if filters.Search != "" {
		filter["$or"] = []bson.M{
			{"username": bson.M{"$regex": filters.Search, "$options": "i"}},
			{"email": bson.M{"$regex": filters.Search, "$options": "i"}},
			{"first_name": bson.M{"$regex": filters.Search, "$options": "i"}},
			{"last_name": bson.M{"$regex": filters.Search, "$options": "i"}},
		}
	}

	if filters.Status != "" && filters.Status != "all" {
		if filters.Status == "active" {
			filter["is_active"] = true
		} else if filters.Status == "inactive" {
			filter["is_active"] = false
		}
	}

	if filters.PlanID != "" && utils.IsValidObjectID(filters.PlanID) {
		planObjID, _ := utils.StringToObjectID(filters.PlanID)
		filter["plan_id"] = planObjID
	}

	// Set sort options
	sortField := "created_at"
	if filters.SortBy != "" {
		sortField = filters.SortBy
	}

	sortOrder := -1
	if filters.SortOrder == "asc" {
		sortOrder = 1
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Get users
	cursor, err := us.collections.Users().Find(ctx, filter,
		options.Find().
			SetSort(bson.M{sortField: sortOrder}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	// Clear passwords
	for i := range users {
		users[i].Password = ""
	}

	// Get total count
	total, err := us.collections.Users().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return users, int(total), nil
}

func (us *UserService) GetUserForAdmin(userID primitive.ObjectID) (*models.User, error) {
	return us.GetByID(userID)
}

func (us *UserService) CreateUserByAdmin(req interface{}) (*models.User, error) {
	// Implement admin user creation
	return nil, errors.New("not implemented")
}

func (us *UserService) UpdateUserByAdmin(userID primitive.ObjectID, updates map[string]interface{}) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, err
	}

	return us.GetByID(userID)
}

func (us *UserService) DeleteUserByAdmin(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"is_active":  false,
			"deleted_at": time.Now(),
		}},
	)
	return err
}

func (us *UserService) SuspendUser(userID primitive.ObjectID, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"is_active":         false,
			"suspension_reason": reason,
			"suspended_at":      time.Now(),
		}},
	)
	return err
}

func (us *UserService) UnsuspendUser(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{
			"$set": bson.M{"is_active": true},
			"$unset": bson.M{
				"suspension_reason": "",
				"suspended_at":      "",
			},
		},
	)
	return err
}

func (us *UserService) VerifyUserByAdmin(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"is_verified":       true,
			"email_verified_at": time.Now(),
		}},
	)
	return err
}

func (us *UserService) ResetUserPasswordByAdmin(userID primitive.ObjectID, newPassword string, sendEmail bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = us.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		}},
	)
	return err
}

func (us *UserService) GetUserFilesForAdmin(userID primitive.ObjectID, page, limit int) ([]models.File, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := us.collections.Files().Find(ctx,
		bson.M{"user_id": userID},
		options.Find().
			SetSort(bson.M{"created_at": -1}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var files []models.File
	if err = cursor.All(ctx, &files); err != nil {
		return nil, 0, err
	}

	total, err := us.collections.Files().CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, 0, err
	}

	return files, int(total), nil
}

func (us *UserService) GetUserActivityForAdmin(userID primitive.ObjectID, days, page, limit int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Filter by date
	startDate := time.Now().AddDate(0, 0, -days)
	filter := bson.M{
		"user_id":    userID,
		"created_at": bson.M{"$gte": startDate},
	}

	skip := (page - 1) * limit

	cursor, err := us.collections.Activities().Find(ctx, filter,
		options.Find().
			SetSort(bson.M{"created_at": -1}).
			SetSkip(int64(skip)).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var activities []map[string]interface{}
	if err = cursor.All(ctx, &activities); err != nil {
		return nil, 0, err
	}

	total, err := us.collections.Activities().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return activities, int(total), nil
}

// Helper methods
func (us *UserService) getRecentFiles(ctx context.Context, userID primitive.ObjectID, limit int) ([]models.File, error) {
	cursor, err := us.collections.Files().Find(ctx,
		bson.M{"user_id": userID, "is_deleted": false},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var files []models.File
	if err = cursor.All(ctx, &files); err != nil {
		return nil, err
	}

	return files, nil
}

func (us *UserService) getRecentFolders(ctx context.Context, userID primitive.ObjectID, limit int) ([]models.Folder, error) {
	cursor, err := us.collections.Folders().Find(ctx,
		bson.M{"user_id": userID, "is_deleted": false},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var folders []models.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

func (us *UserService) getStorageByType(ctx context.Context, userID primitive.ObjectID) (map[string]interface{}, error) {
	// Aggregate storage usage by file type
	pipeline := []bson.M{
		{"$match": bson.M{"user_id": userID, "is_deleted": false}},
		{"$group": bson.M{
			"_id":   "$mime_type",
			"size":  bson.M{"$sum": "$size"},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"size": -1}},
	}

	cursor, err := us.collections.Files().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Categorize by type
	categories := map[string]map[string]interface{}{
		"images":    {"size": int64(0), "count": 0},
		"videos":    {"size": int64(0), "count": 0},
		"documents": {"size": int64(0), "count": 0},
		"audio":     {"size": int64(0), "count": 0},
		"other":     {"size": int64(0), "count": 0},
	}

	for _, result := range results {
		mimeType := result["_id"].(string)
		size := result["size"].(int64)
		count := result["count"].(int32)

		category := "other"
		if strings.HasPrefix(mimeType, "image/") {
			category = "images"
		} else if strings.HasPrefix(mimeType, "video/") {
			category = "videos"
		} else if strings.HasPrefix(mimeType, "audio/") {
			category = "audio"
		} else if strings.Contains(mimeType, "pdf") || strings.Contains(mimeType, "document") || strings.Contains(mimeType, "text") {
			category = "documents"
		}

		categories[category]["size"] = categories[category]["size"].(int64) + size
		categories[category]["count"] = categories[category]["count"].(int) + int(count)
	}

	return map[string]interface{}{"categories": categories}, nil
}
