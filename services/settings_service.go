package services

import (
	"context"
	"encoding/json"
	"fmt"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SettingsService struct {
	settingsCollection       *mongo.Collection
	userSettingsCollection   *mongo.Collection
	settingsBackupCollection *mongo.Collection
	userCollection           *mongo.Collection
	planCollection           *mongo.Collection
	cacheExpiry              time.Duration
	cache                    map[string]interface{}
	lastCacheUpdate          time.Time
}

type SettingsBackup struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Name        string                 `bson:"name" json:"name"`
	Description string                 `bson:"description" json:"description"`
	Settings    map[string]interface{} `bson:"settings" json:"settings"`
	CreatedBy   primitive.ObjectID     `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
}

func NewSettingsService() *SettingsService {
	return &SettingsService{
		settingsCollection:       database.GetCollection("settings"),
		userSettingsCollection:   database.GetCollection("user_settings"),
		settingsBackupCollection: database.GetCollection("settings_backups"),
		userCollection:           database.GetCollection("users"),
		planCollection:           database.GetCollection("plans"),
		cacheExpiry:              5 * time.Minute,
		cache:                    make(map[string]interface{}),
	}
}

// Admin Settings Management
func (ss *SettingsService) GetSettings(group string, includePrivate bool) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if group != "" {
		filter["group"] = group
	}
	if !includePrivate {
		filter["is_public"] = true
	}

	cursor, err := ss.settingsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	settings := make(map[string]interface{})
	for cursor.Next(ctx) {
		var setting models.AdminSettings
		if err := cursor.Decode(&setting); err != nil {
			continue
		}
		settings[setting.Key] = map[string]interface{}{
			"value":       setting.Value,
			"type":        setting.Type,
			"group":       setting.Group,
			"label":       setting.Label,
			"description": setting.Description,
			"options":     setting.Options,
			"is_public":   setting.IsPublic,
		}
	}

	return settings, nil
}

func (ss *SettingsService) GetSettingsByGroup(group string) (map[string]interface{}, error) {
	return ss.GetSettings(group, true)
}

func (ss *SettingsService) GetSettingGroups() ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$group",
			"count": bson.M{"$sum": 1},
			"label": bson.M{"$first": "$group"},
		}},
		{"$sort": bson.M{"_id": 1}},
	}

	cursor, err := ss.settingsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groups []map[string]interface{}
	if err = cursor.All(ctx, &groups); err != nil {
		return nil, err
	}

	return groups, nil
}

func (ss *SettingsService) GetSetting(key string) (interface{}, error) {
	// Check cache first
	if ss.isCacheValid() {
		if value, exists := ss.cache[key]; exists {
			return value, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var setting models.AdminSettings
	err := ss.settingsCollection.FindOne(ctx, bson.M{"key": key}).Decode(&setting)
	if err != nil {
		return nil, fmt.Errorf("setting not found: %s", key)
	}

	// Update cache
	ss.cache[key] = setting.Value
	ss.lastCacheUpdate = time.Now()

	return setting.Value, nil
}

func (ss *SettingsService) UpdateSetting(key string, value interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate the value based on setting type
	var setting models.AdminSettings
	err := ss.settingsCollection.FindOne(ctx, bson.M{"key": key}).Decode(&setting)
	if err != nil {
		return fmt.Errorf("setting not found: %s", key)
	}

	// Type validation
	if err := ss.validateSettingValue(setting.Type, value); err != nil {
		return fmt.Errorf("invalid value for setting %s: %v", key, err)
	}

	// Rule validation
	if err := ss.validateSettingRules(setting.Rules, value); err != nil {
		return fmt.Errorf("value violates rules for setting %s: %v", key, err)
	}

	// Update setting
	_, err = ss.settingsCollection.UpdateOne(ctx,
		bson.M{"key": key},
		bson.M{"$set": bson.M{
			"value":      value,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update setting: %v", err)
	}

	// Update cache
	ss.cache[key] = value
	ss.lastCacheUpdate = time.Now()

	// Handle special settings that require additional actions
	if err := ss.handleSpecialSettings(key, value); err != nil {
		return fmt.Errorf("failed to handle special setting %s: %v", key, err)
	}

	return nil
}

func (ss *SettingsService) UpdateSettings(settings map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start transaction for atomic updates
	session, err := database.GetClient().StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		for key, value := range settings {
			err := ss.UpdateSetting(key, value)
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	_, err = session.WithTransaction(ctx, callback)
	if err != nil {
		return fmt.Errorf("failed to update settings: %v", err)
	}

	// Clear cache to force refresh
	ss.clearCache()

	return nil
}

func (ss *SettingsService) CreateSetting(key, label, description, settingType, group string, value interface{}, isPublic bool, options []models.SettingOption, rules []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if setting already exists
	count, err := ss.settingsCollection.CountDocuments(ctx, bson.M{"key": key})
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("setting with key %s already exists", key)
	}

	// Validate value
	if err := ss.validateSettingValue(settingType, value); err != nil {
		return fmt.Errorf("invalid value: %v", err)
	}

	setting := &models.AdminSettings{
		ID:          primitive.NewObjectID(),
		Key:         key,
		Value:       value,
		Type:        settingType,
		Group:       group,
		Label:       label,
		Description: description,
		Options:     options,
		Rules:       rules,
		IsPublic:    isPublic,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err = ss.settingsCollection.InsertOne(ctx, setting)
	if err != nil {
		return fmt.Errorf("failed to create setting: %v", err)
	}

	// Update cache
	ss.cache[key] = value

	return nil
}

func (ss *SettingsService) DeleteSetting(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if setting is protected
	protectedSettings := []string{
		"site_name", "site_url", "default_plan_id", "allow_registration",
		"email_verification", "maintenance_mode",
	}

	for _, protected := range protectedSettings {
		if key == protected {
			return fmt.Errorf("cannot delete protected setting: %s", key)
		}
	}

	_, err := ss.settingsCollection.DeleteOne(ctx, bson.M{"key": key})
	if err != nil {
		return fmt.Errorf("failed to delete setting: %v", err)
	}

	// Remove from cache
	delete(ss.cache, key)

	return nil
}

// User Settings Management
func (ss *SettingsService) GetUserSettings(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Default user settings
	defaults := map[string]interface{}{
		"email_notifications":    true,
		"push_notifications":     true,
		"auto_sync":              true,
		"public_profile":         false,
		"theme":                  "light",
		"language":               "en",
		"timezone":               "UTC",
		"two_factor_enabled":     false,
		"storage_quota_alerts":   true,
		"login_alerts":           true,
		"security_notifications": true,
		"marketing_emails":       false,
		"data_export_format":     "json",
		"auto_backup":            true,
		"file_versioning":        true,
		"link_expiry_days":       7,
	}

	// Get user-specific settings
	var userSettings map[string]interface{}
	err := ss.userSettingsCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&userSettings)
	if err == nil {
		// Merge with defaults
		for key, value := range userSettings {
			if key != "_id" && key != "user_id" && key != "created_at" && key != "updated_at" {
				defaults[key] = value
			}
		}
	}

	return defaults, nil
}

func (ss *SettingsService) UpdateUserSettings(userID primitive.ObjectID, settings map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate settings
	validKeys := []string{
		"email_notifications", "push_notifications", "auto_sync", "public_profile",
		"theme", "language", "timezone", "two_factor_enabled", "storage_quota_alerts",
		"login_alerts", "security_notifications", "marketing_emails", "data_export_format",
		"auto_backup", "file_versioning", "link_expiry_days",
	}

	for key := range settings {
		valid := false
		for _, validKey := range validKeys {
			if key == validKey {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid setting key: %s", key)
		}
	}

	// Add metadata
	settings["user_id"] = userID
	settings["updated_at"] = time.Now()

	// If this is the first time settings are being saved, add created_at
	existing := ss.userSettingsCollection.FindOne(ctx, bson.M{"user_id": userID})
	if existing.Err() == mongo.ErrNoDocuments {
		settings["created_at"] = time.Now()
	}

	// Upsert settings
	_, err := ss.userSettingsCollection.ReplaceOne(ctx,
		bson.M{"user_id": userID},
		settings,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to update user settings: %v", err)
	}

	// Handle special user settings
	if err := ss.handleSpecialUserSettings(userID, settings); err != nil {
		return fmt.Errorf("failed to handle special user settings: %v", err)
	}

	return nil
}

func (ss *SettingsService) ResetUserSettings(userID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ss.userSettingsCollection.DeleteOne(ctx, bson.M{"user_id": userID})
	return err
}

// Settings Backup and Restore
func (ss *SettingsService) BackupSettings(name, description string) (*SettingsBackup, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all current settings
	settings, err := ss.GetSettings("", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get current settings: %v", err)
	}

	backup := &SettingsBackup{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Description: description,
		Settings:    settings,
		CreatedAt:   time.Now(),
	}

	_, err = ss.settingsBackupCollection.InsertOne(ctx, backup)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %v", err)
	}

	return backup, nil
}

func (ss *SettingsService) RestoreSettings(backupID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get backup
	var backup SettingsBackup
	err := ss.settingsBackupCollection.FindOne(ctx, bson.M{"_id": backupID}).Decode(&backup)
	if err != nil {
		return fmt.Errorf("backup not found: %v", err)
	}

	// Start transaction
	session, err := database.GetClient().StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Restore each setting
		for key, settingData := range backup.Settings {
			if settingMap, ok := settingData.(map[string]interface{}); ok {
				if value, exists := settingMap["value"]; exists {
					err := ss.UpdateSetting(key, value)
					if err != nil {
						return nil, fmt.Errorf("failed to restore setting %s: %v", key, err)
					}
				}
			}
		}
		return nil, nil
	}

	_, err = session.WithTransaction(ctx, callback)
	if err != nil {
		return fmt.Errorf("failed to restore settings: %v", err)
	}

	// Clear cache
	ss.clearCache()

	return nil
}

func (ss *SettingsService) GetBackups() ([]SettingsBackup, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := ss.settingsBackupCollection.Find(ctx, bson.M{},
		options.Find().SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var backups []SettingsBackup
	if err = cursor.All(ctx, &backups); err != nil {
		return nil, err
	}

	return backups, nil
}

func (ss *SettingsService) DeleteBackup(backupID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ss.settingsBackupCollection.DeleteOne(ctx, bson.M{"_id": backupID})
	return err
}

// System Configuration
func (ss *SettingsService) GetSystemSettings() (*models.SystemSettings, error) {
	settings, err := ss.GetSettings("", false)
	if err != nil {
		return nil, err
	}

	systemSettings := &models.SystemSettings{}

	// Map settings to system settings struct
	if val, exists := settings["site_name"]; exists {
		if settingData, ok := val.(map[string]interface{}); ok {
			if value, ok := settingData["value"].(string); ok {
				systemSettings.SiteName = value
			}
		}
	}

	if val, exists := settings["site_description"]; exists {
		if settingData, ok := val.(map[string]interface{}); ok {
			if value, ok := settingData["value"].(string); ok {
				systemSettings.SiteDescription = value
			}
		}
	}

	if val, exists := settings["site_url"]; exists {
		if settingData, ok := val.(map[string]interface{}); ok {
			if value, ok := settingData["value"].(string); ok {
				systemSettings.SiteUrl = value
			}
		}
	}

	if val, exists := settings["allow_registration"]; exists {
		if settingData, ok := val.(map[string]interface{}); ok {
			if value, ok := settingData["value"].(bool); ok {
				systemSettings.AllowRegistration = value
			}
		}
	}

	if val, exists := settings["email_verification"]; exists {
		if settingData, ok := val.(map[string]interface{}); ok {
			if value, ok := settingData["value"].(bool); ok {
				systemSettings.EmailVerification = value
			}
		}
	}

	// Add more field mappings as needed...

	return systemSettings, nil
}

// Validation and Helper Methods
func (ss *SettingsService) validateSettingValue(settingType string, value interface{}) error {
	switch settingType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "int":
		switch v := value.(type) {
		case int, int32, int64, float64:
			// These are all acceptable for int type
		default:
			return fmt.Errorf("expected integer, got %T", v)
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "json":
		// Try to marshal and unmarshal to validate JSON
		if _, err := json.Marshal(value); err != nil {
			return fmt.Errorf("invalid JSON: %v", err)
		}
	case "array":
		// Check if it's a slice or array
		switch value.(type) {
		case []interface{}, []string, []int, []float64:
			// Valid array types
		default:
			return fmt.Errorf("expected array, got %T", value)
		}
	}
	return nil
}

func (ss *SettingsService) validateSettingRules(rules []string, value interface{}) error {
	for _, rule := range rules {
		switch {
		case strings.HasPrefix(rule, "min:"):
			if err := ss.validateMin(rule, value); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "max:"):
			if err := ss.validateMax(rule, value); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "regex:"):
			if err := ss.validateRegex(rule, value); err != nil {
				return err
			}
		case rule == "required":
			if value == nil || value == "" {
				return fmt.Errorf("value is required")
			}
		case rule == "email":
			if str, ok := value.(string); ok {
				if !utils.IsValidEmail(str) {
					return fmt.Errorf("invalid email format")
				}
			}
		case rule == "url":
			if str, ok := value.(string); ok {
				if !utils.IsValidURL(str) {
					return fmt.Errorf("invalid URL format")
				}
			}
		}
	}
	return nil
}

func (ss *SettingsService) validateMin(rule string, value interface{}) error {
	// Implementation for min validation
	return nil
}

func (ss *SettingsService) validateMax(rule string, value interface{}) error {
	// Implementation for max validation
	return nil
}

func (ss *SettingsService) validateRegex(rule string, value interface{}) error {
	// Implementation for regex validation
	return nil
}

func (ss *SettingsService) handleSpecialSettings(key string, value interface{}) error {
	switch key {
	case "maintenance_mode":
		if enabled, ok := value.(bool); ok && enabled {
			// Log maintenance mode activation
			// Could also send notifications to admins
		}
	case "default_plan_id":
		// Validate that the plan exists
		if planID, ok := value.(string); ok {
			if utils.IsValidObjectID(planID) {
				objID, _ := utils.StringToObjectID(planID)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				count, err := ss.planCollection.CountDocuments(ctx, bson.M{"_id": objID, "is_active": true})
				if err != nil || count == 0 {
					return fmt.Errorf("invalid plan ID")
				}
			}
		}
	case "max_upload_size":
		// Clear upload validation cache
		ps.clearCache()
	case "allowed_file_types":
		// Clear file type validation cache
		ps.clearCache()
	case "site_url":
		// Validate URL format
		if url, ok := value.(string); ok {
			if !utils.IsValidURL(url) {
				return fmt.Errorf("invalid site URL format")
			}
		}
	}
	return nil
}

func (ps *SettingsService) handleSpecialUserSettings(userID primitive.ObjectID, settings map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Handle two-factor authentication
	if twoFAEnabled, exists := settings["two_factor_enabled"]; exists {
		if enabled, ok := twoFAEnabled.(bool); ok && enabled {
			// Generate 2FA secret if not exists
			var user models.User
			err := ps.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
			if err == nil {
				// Check if user already has 2FA secret
				if user.TwoFactorSecret == "" {
					secret := utils.GenerateTOTPSecret()
					_, err = ps.userCollection.UpdateOne(ctx,
						bson.M{"_id": userID},
						bson.M{"$set": bson.M{
							"two_factor_secret": secret,
							"updated_at":        time.Now(),
						}},
					)
					if err != nil {
						return fmt.Errorf("failed to generate 2FA secret: %v", err)
					}
				}
			}
		}
	}

	// Handle timezone changes
	if timezone, exists := settings["timezone"]; exists {
		if tz, ok := timezone.(string); ok {
			// Validate timezone
			if !ps.isValidTimezone(tz) {
				return fmt.Errorf("invalid timezone: %s", tz)
			}
		}
	}

	// Handle language changes
	if language, exists := settings["language"]; exists {
		if lang, ok := language.(string); ok {
			// Validate language code
			if !ps.isValidLanguageCode(lang) {
				return fmt.Errorf("invalid language code: %s", lang)
			}
		}
	}

	return nil
}

// Cache Management
func (ps *SettingsService) isCacheValid() bool {
	return time.Since(ps.lastCacheUpdate) < ps.cacheExpiry
}

func (ps *SettingsService) clearCache() {
	ps.cache = make(map[string]interface{})
	ps.lastCacheUpdate = time.Time{}
}

func (ps *SettingsService) preloadCache() error {
	settings, err := ps.GetSettings("", false)
	if err != nil {
		return err
	}

	ps.cache = make(map[string]interface{})
	for key, settingData := range settings {
		if settingMap, ok := settingData.(map[string]interface{}); ok {
			if value, exists := settingMap["value"]; exists {
				ps.cache[key] = value
			}
		}
	}
	ps.lastCacheUpdate = time.Now()

	return nil
}

// Settings Import/Export
func (ps *SettingsService) ExportSettings(group string) (map[string]interface{}, error) {
	settings, err := ps.GetSettings(group, true)
	if err != nil {
		return nil, err
	}

	export := map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now(),
		"group":       group,
		"settings":    settings,
	}

	return export, nil
}

func (ps *SettingsService) ImportSettings(importData map[string]interface{}) error {
	settingsData, ok := importData["settings"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid import data format")
	}

	// Extract only values for import
	settingsToImport := make(map[string]interface{})
	for key, settingData := range settingsData {
		if settingMap, ok := settingData.(map[string]interface{}); ok {
			if value, exists := settingMap["value"]; exists {
				settingsToImport[key] = value
			}
		}
	}

	return ps.UpdateSettings(settingsToImport)
}

// Settings Validation Helpers
func (ps *SettingsService) validateMin(rule string, value interface{}) error {
	minStr := strings.TrimPrefix(rule, "min:")
	minVal, err := strconv.ParseFloat(minStr, 64)
	if err != nil {
		return fmt.Errorf("invalid min rule: %s", rule)
	}

	switch v := value.(type) {
	case string:
		if float64(len(v)) < minVal {
			return fmt.Errorf("value must be at least %g characters", minVal)
		}
	case int, int32, int64:
		val := utils.ToFloat64(v)
		if val < minVal {
			return fmt.Errorf("value must be at least %g", minVal)
		}
	case float64:
		if v < minVal {
			return fmt.Errorf("value must be at least %g", minVal)
		}
	case []interface{}:
		if float64(len(v)) < minVal {
			return fmt.Errorf("array must have at least %g items", minVal)
		}
	}
	return nil
}

func (ps *SettingsService) validateMax(rule string, value interface{}) error {
	maxStr := strings.TrimPrefix(rule, "max:")
	maxVal, err := strconv.ParseFloat(maxStr, 64)
	if err != nil {
		return fmt.Errorf("invalid max rule: %s", rule)
	}

	switch v := value.(type) {
	case string:
		if float64(len(v)) > maxVal {
			return fmt.Errorf("value must be at most %g characters", maxVal)
		}
	case int, int32, int64:
		val := utils.ToFloat64(v)
		if val > maxVal {
			return fmt.Errorf("value must be at most %g", maxVal)
		}
	case float64:
		if v > maxVal {
			return fmt.Errorf("value must be at most %g", maxVal)
		}
	case []interface{}:
		if float64(len(v)) > maxVal {
			return fmt.Errorf("array must have at most %g items", maxVal)
		}
	}
	return nil
}

func (ps *SettingsService) validateRegex(rule string, value interface{}) error {
	regexPattern := strings.TrimPrefix(rule, "regex:")
	if str, ok := value.(string); ok {
		matched, err := utils.MatchRegex(regexPattern, str)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %s", regexPattern)
		}
		if !matched {
			return fmt.Errorf("value does not match required pattern")
		}
	}
	return nil
}

// Validation helpers
func (ps *SettingsService) isValidTimezone(tz string) bool {
	validTimezones := []string{
		"UTC", "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
		"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Moscow",
		"Asia/Tokyo", "Asia/Shanghai", "Asia/Kolkata", "Asia/Dubai",
		"Australia/Sydney", "Pacific/Auckland",
	}

	for _, validTz := range validTimezones {
		if tz == validTz {
			return true
		}
	}
	return false
}

func (ps *SettingsService) isValidLanguageCode(lang string) bool {
	validLanguages := []string{
		"en", "es", "fr", "de", "it", "pt", "ru", "zh", "ja", "ko", "ar", "hi",
	}

	for _, validLang := range validLanguages {
		if lang == validLang {
			return true
		}
	}
	return false
}

// Settings Groups Management
func (ps *SettingsService) CreateSettingGroup(name, label, description string) error {
	// Groups are created implicitly when settings are added
	// This method can be used for validation or metadata
	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	validGroups := []string{
		"general", "auth", "storage", "email", "payment", "security",
		"api", "backup", "notification", "ui", "advanced",
	}

	for _, validGroup := range validGroups {
		if name == validGroup {
			return nil
		}
	}

	return fmt.Errorf("invalid group name: %s", name)
}

// Settings Templates
func (ps *SettingsService) GetSettingTemplate(templateName string) (map[string]interface{}, error) {
	templates := map[string]map[string]interface{}{
		"default": {
			"site_name":          "CloudStorage",
			"site_description":   "Secure cloud storage for your files",
			"allow_registration": true,
			"email_verification": true,
			"maintenance_mode":   false,
			"max_upload_size":    104857600, // 100MB
			"default_currency":   "USD",
			"tax_rate":           0.0,
		},
		"restrictive": {
			"allow_registration":     false,
			"email_verification":     true,
			"maintenance_mode":       false,
			"max_upload_size":        52428800, // 50MB
			"require_admin_approval": true,
		},
		"development": {
			"allow_registration": true,
			"email_verification": false,
			"maintenance_mode":   false,
			"debug_mode":         true,
			"max_upload_size":    1073741824, // 1GB
		},
	}

	template, exists := templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	return template, nil
}

func (ps *SettingsService) ApplySettingTemplate(templateName string) error {
	template, err := ps.GetSettingTemplate(templateName)
	if err != nil {
		return err
	}

	return ps.UpdateSettings(template)
}

// Settings Health Check
func (ps *SettingsService) HealthCheck() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status":         "healthy",
		"last_updated":   ps.lastCacheUpdate,
		"cache_valid":    ps.isCacheValid(),
		"settings_count": 0,
		"issues":         []string{},
	}

	// Count total settings
	count, err := ps.settingsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		health["status"] = "unhealthy"
		health["issues"] = append(health["issues"].([]string), "Failed to count settings")
	} else {
		health["settings_count"] = count
	}

	// Check critical settings
	criticalSettings := []string{"site_name", "default_plan_id", "allow_registration"}
	for _, setting := range criticalSettings {
		_, err := ps.GetSetting(setting)
		if err != nil {
			health["status"] = "degraded"
			health["issues"] = append(health["issues"].([]string),
				fmt.Sprintf("Missing critical setting: %s", setting))
		}
	}

	// Check database connection
	err = database.GetCollection("settings").Database().Client().Ping(ctx, nil)
	if err != nil {
		health["status"] = "unhealthy"
		health["issues"] = append(health["issues"].([]string), "Database connection failed")
	}

	return health, nil
}

// Settings Monitoring
func (ps *SettingsService) GetSettingsStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Count settings by group
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$group",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := ps.settingsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groupStats []map[string]interface{}
	if err = cursor.All(ctx, &groupStats); err != nil {
		return nil, err
	}

	// Count settings by type
	typePipeline := []bson.M{
		{"$group": bson.M{
			"_id":   "$type",
			"count": bson.M{"$sum": 1},
		}},
	}

	typeCursor, err := ps.settingsCollection.Aggregate(ctx, typePipeline)
	if err != nil {
		return nil, err
	}
	defer typeCursor.Close(ctx)

	var typeStats []map[string]interface{}
	if err = typeCursor.All(ctx, &typeStats); err != nil {
		return nil, err
	}

	// Total counts
	totalSettings, _ := ps.settingsCollection.CountDocuments(ctx, bson.M{})
	publicSettings, _ := ps.settingsCollection.CountDocuments(ctx, bson.M{"is_public": true})
	privateSettings, _ := ps.settingsCollection.CountDocuments(ctx, bson.M{"is_public": false})

	stats := map[string]interface{}{
		"total_settings":   totalSettings,
		"public_settings":  publicSettings,
		"private_settings": privateSettings,
		"groups":           groupStats,
		"types":            typeStats,
		"cache_status": map[string]interface{}{
			"enabled":      true,
			"valid":        ps.isCacheValid(),
			"last_update":  ps.lastCacheUpdate,
			"expiry":       ps.cacheExpiry,
			"cached_items": len(ps.cache),
		},
	}

	return stats, nil
}

// Settings Migration
func (ps *SettingsService) MigrateSettings(fromVersion, toVersion string) error {
	// Settings migration logic for version upgrades
	migrations := map[string]func() error{
		"1.0_to_1.1": ps.migrateV1ToV11,
		"1.1_to_1.2": ps.migrateV11ToV12,
	}

	migrationKey := fmt.Sprintf("%s_to_%s", fromVersion, toVersion)
	if migration, exists := migrations[migrationKey]; exists {
		return migration()
	}

	return fmt.Errorf("no migration available from %s to %s", fromVersion, toVersion)
}

func (ps *SettingsService) migrateV1ToV11() error {
	// Example migration: Add new settings for v1.1
	newSettings := map[string]interface{}{
		"api_rate_limit":   1000,
		"session_timeout":  3600,
		"backup_retention": 30,
	}

	for key, value := range newSettings {
		err := ps.CreateSetting(key, strings.Title(strings.ReplaceAll(key, "_", " ")),
			"", "int", "api", value, false, nil, nil)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

func (ps *SettingsService) migrateV11ToV12() error {
	// Example migration: Update existing settings for v1.2
	updates := map[string]interface{}{
		"max_upload_size": 209715200, // Increase to 200MB
	}

	return ps.UpdateSettings(updates)
}

// Settings Synchronization
func (ps *SettingsService) SyncSettings() error {
	// Force reload all settings from database
	ps.clearCache()
	return ps.preloadCache()
}

// Settings Cleanup
func (ps *SettingsService) CleanupOrphanedSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Remove settings that are no longer used
	orphanedKeys := []string{
		"deprecated_setting_1",
		"old_payment_gateway",
		"legacy_storage_config",
	}

	for _, key := range orphanedKeys {
		_, err := ps.settingsCollection.DeleteOne(ctx, bson.M{"key": key})
		if err != nil {
			return fmt.Errorf("failed to delete orphaned setting %s: %v", key, err)
		}
	}

	return nil
}
