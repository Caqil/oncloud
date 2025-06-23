package database

import (
	"context"
	"log"
	"oncloud/models"
	"oncloud/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RunMigrations executes all database migrations
func RunMigrations() error {
	log.Println("Running database migrations...")

	if err := createDefaultAdmin(); err != nil {
		return err
	}

	if err := createDefaultPlans(); err != nil {
		return err
	}

	if err := createDefaultSettings(); err != nil {
		return err
	}

	if err := createDefaultStorageProvider(); err != nil {
		return err
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// createDefaultAdmin creates the default super admin user
func createDefaultAdmin() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := GetCollection("admins")

	// Check if any admin exists
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Admin users already exist, skipping default admin creation")
		return nil
	}

	// Create default admin
	hashedPassword, err := utils.HashPassword("admin123")
	if err != nil {
		return err
	}

	admin := models.Admin{
		ID:        primitive.NewObjectID(),
		Username:  "admin",
		Email:     "admin@example.com",
		Password:  hashedPassword,
		FirstName: "Super",
		LastName:  "Admin",
		Role:      "super_admin",
		Permissions: []string{
			"users.read", "users.write", "users.delete",
			"files.read", "files.write", "files.delete",
			"plans.read", "plans.write", "plans.delete",
			"settings.read", "settings.write",
			"analytics.read",
			"system.manage",
		},
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = collection.InsertOne(ctx, admin)
	if err != nil {
		return err
	}

	log.Printf("Created default admin user: %s (password: admin123)", admin.Email)
	return nil
}

// createDefaultPlans creates default pricing plans
func createDefaultPlans() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := GetCollection("plans")

	// Check if any plans exist
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Plans already exist, skipping default plans creation")
		return nil
	}

	// Create default plans
	plans := []models.Plan{
		{
			ID:               primitive.NewObjectID(),
			Name:             "Free",
			Slug:             "free",
			Description:      "Perfect for personal use with basic features",
			ShortDescription: "Basic features for personal use",
			StorageLimit:     1024 * 1024 * 1024,     // 1GB
			BandwidthLimit:   5 * 1024 * 1024 * 1024, // 5GB
			FilesLimit:       100,
			FoldersLimit:     10,
			Price:            0,
			OriginalPrice:    0,
			Currency:         "USD",
			BillingCycle:     "monthly",
			MaxFileSize:      10 * 1024 * 1024, // 10MB
			AllowedTypes:     []string{".jpg", ".jpeg", ".png", ".gif", ".pdf", ".txt", ".doc", ".docx"},
			Features:         []string{"1GB Storage", "5GB Bandwidth", "100 Files", "10 Folders", "Basic Support"},
			Limitations:      []string{"10MB max file size", "Limited file types", "No API access"},
			PopularBadge:     false,
			IsActive:         true,
			IsDefault:        true,
			IsFree:           true,
			SortOrder:        1,
			TrialDays:        0,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
		{
			ID:               primitive.NewObjectID(),
			Name:             "Basic",
			Slug:             "basic",
			Description:      "Great for small teams and growing businesses",
			ShortDescription: "Enhanced features for small teams",
			StorageLimit:     10 * 1024 * 1024 * 1024, // 10GB
			BandwidthLimit:   50 * 1024 * 1024 * 1024, // 50GB
			FilesLimit:       1000,
			FoldersLimit:     100,
			Price:            9.99,
			OriginalPrice:    12.99,
			Currency:         "USD",
			BillingCycle:     "monthly",
			MaxFileSize:      100 * 1024 * 1024, // 100MB
			AllowedTypes:     []string{},        // All types allowed
			Features:         []string{"10GB Storage", "50GB Bandwidth", "1000 Files", "100 Folders", "Email Support", "API Access"},
			Limitations:      []string{"100MB max file size"},
			PopularBadge:     true,
			IsActive:         true,
			IsDefault:        false,
			IsFree:           false,
			SortOrder:        2,
			TrialDays:        7,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
		{
			ID:               primitive.NewObjectID(),
			Name:             "Premium",
			Slug:             "premium",
			Description:      "Perfect for large teams and enterprises",
			ShortDescription: "Advanced features for enterprises",
			StorageLimit:     100 * 1024 * 1024 * 1024, // 100GB
			BandwidthLimit:   500 * 1024 * 1024 * 1024, // 500GB
			FilesLimit:       -1,                       // Unlimited
			FoldersLimit:     -1,                       // Unlimited
			Price:            29.99,
			OriginalPrice:    39.99,
			Currency:         "USD",
			BillingCycle:     "monthly",
			MaxFileSize:      1024 * 1024 * 1024, // 1GB
			AllowedTypes:     []string{},         // All types allowed
			Features:         []string{"100GB Storage", "500GB Bandwidth", "Unlimited Files", "Unlimited Folders", "Priority Support", "Advanced API", "Custom Branding"},
			Limitations:      []string{},
			PopularBadge:     false,
			IsActive:         true,
			IsDefault:        false,
			IsFree:           false,
			SortOrder:        3,
			TrialDays:        14,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}

	// Insert plans
	for _, plan := range plans {
		_, err := collection.InsertOne(ctx, plan)
		if err != nil {
			return err
		}
		log.Printf("Created default plan: %s", plan.Name)
	}

	return nil
}

// createDefaultSettings creates default system settings
func createDefaultSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := GetCollection("settings")

	// Check if any settings exist
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Settings already exist, skipping default settings creation")
		return nil
	}

	// Create default settings
	settings := []models.AdminSettings{
		{
			ID:          primitive.NewObjectID(),
			Key:         "site_name",
			Value:       "CloudStorage",
			Type:        "string",
			Group:       "general",
			Label:       "Site Name",
			Description: "The name of your cloud storage service",
			IsPublic:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			Key:         "site_description",
			Value:       "Secure cloud storage for your files",
			Type:        "string",
			Group:       "general",
			Label:       "Site Description",
			Description: "Brief description of your service",
			IsPublic:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			Key:         "allow_registration",
			Value:       true,
			Type:        "bool",
			Group:       "auth",
			Label:       "Allow Registration",
			Description: "Allow new users to register",
			IsPublic:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			Key:         "email_verification",
			Value:       true,
			Type:        "bool",
			Group:       "auth",
			Label:       "Email Verification",
			Description: "Require email verification for new accounts",
			IsPublic:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          primitive.NewObjectID(),
			Key:         "max_upload_size",
			Value:       104857600, // 100MB
			Type:        "int",
			Group:       "files",
			Label:       "Max Upload Size",
			Description: "Maximum file upload size in bytes",
			IsPublic:    false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Insert settings
	for _, setting := range settings {
		_, err := collection.InsertOne(ctx, setting)
		if err != nil {
			return err
		}
		log.Printf("Created default setting: %s", setting.Key)
	}

	return nil
}

// createDefaultStorageProvider creates default local storage provider
func createDefaultStorageProvider() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := GetCollection("storage_providers")

	// Check if any providers exist
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Storage providers already exist, skipping default provider creation")
		return nil
	}

	// Create default local storage provider
	provider := models.StorageProvider{
		ID:           primitive.NewObjectID(),
		Name:         "Local Storage",
		Type:         "local",
		Region:       "local",
		Endpoint:     "",
		Bucket:       "uploads",
		MaxFileSize:  1024 * 1024 * 1024, // 1GB
		AllowedTypes: []string{},         // All types allowed
		Settings: map[string]interface{}{
			"base_path": "./uploads",
		},
		IsActive:  true,
		IsDefault: true,
		Priority:  1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = collection.InsertOne(ctx, provider)
	if err != nil {
		return err
	}

	log.Printf("Created default storage provider: %s", provider.Name)
	return nil
}
