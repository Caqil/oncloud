// ============================================================================
// database/connection.go - Complete file with all required functions
// ============================================================================
package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	client   *mongo.Client
	database *mongo.Database
	dbName   = os.Getenv("DB_NAME")
)

// SetClient sets the global MongoDB client (called by DatabaseManager)
func SetClient(c *mongo.Client) {
	client = c
}

// SetDatabase sets the global MongoDB database (called by DatabaseManager)
func SetDatabase(db *mongo.Database) {
	database = db
}

// Connect establishes connection to MongoDB (legacy - now handled by DatabaseManager)
func Connect() error {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	if dbName == "" {
		dbName = "cloudstorage"
	}

	// Set connection options
	clientOptions := options.Client().
		ApplyURI(mongoURI).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(10 * time.Second).
		SetSocketTimeout(10 * time.Second).
		SetConnectTimeout(10 * time.Second)

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Ping the database to verify connection
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	// Set database
	database = client.Database(dbName)

	log.Printf("Successfully connected to MongoDB database: %s", dbName)
	return nil
}

// Disconnect closes the MongoDB connection
func Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if client != nil {
		if err := client.Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to disconnect from MongoDB: %v", err)
		}
		log.Println("Disconnected from MongoDB")
	}

	return nil
}

// GetClient returns the MongoDB client
func GetClient() *mongo.Client {
	return client
}

// GetDatabase returns the MongoDB database
func GetDatabase() *mongo.Database {
	return database
}

// GetCollection returns a MongoDB collection
func GetCollection(collectionName string) *mongo.Collection {
	if database == nil {
		panic(fmt.Sprintf("database not initialized when trying to get collection: %s. Make sure DatabaseManager.Initialize() is called first.", collectionName))
	}
	return database.Collection(collectionName)
}

// Ping checks the database connection
func Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if client == nil {
		return fmt.Errorf("database client not initialized")
	}

	return client.Ping(ctx, readpref.Primary())
}

// GetStats returns database statistics
func GetStats() (bson.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if database == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var stats bson.M
	result := database.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}})
	err := result.Decode(&stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// CreateIndexes creates necessary database indexes
func CreateIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Creating database indexes...")

	// Users collection indexes
	usersCollection := GetCollection("users")
	userIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"username", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"created_at", -1}},
		},
		{
			Keys: bson.D{{"plan_id", 1}},
		},
	}

	if _, err := usersCollection.Indexes().CreateMany(ctx, userIndexes); err != nil {
		return fmt.Errorf("failed to create user indexes: %v", err)
	}

	// Files collection indexes
	filesCollection := GetCollection("files")
	fileIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {"created_at", -1}},
		},
		{
			Keys: bson.D{{"folder_id", 1}},
		},
		{
			Keys: bson.D{{"hash", 1}},
		},
		{
			Keys: bson.D{{"storage_provider", 1}},
		},
		{
			Keys: bson.D{{"is_public", 1}},
		},
		{
			Keys:    bson.D{{"share_token", 1}},
			Options: options.Index().SetSparse(true),
		},
		{
			Keys: bson.D{{"mime_type", 1}},
		},
	}

	if _, err := filesCollection.Indexes().CreateMany(ctx, fileIndexes); err != nil {
		return fmt.Errorf("failed to create file indexes: %v", err)
	}

	// Folders collection indexes
	foldersCollection := GetCollection("folders")
	folderIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{"user_id", 1}, {"parent_id", 1}},
		},
		{
			Keys:    bson.D{{"user_id", 1}, {"name", 1}, {"parent_id", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"share_token", 1}},
			Options: options.Index().SetSparse(true),
		},
	}

	if _, err := foldersCollection.Indexes().CreateMany(ctx, folderIndexes); err != nil {
		return fmt.Errorf("failed to create folder indexes: %v", err)
	}

	// Plans collection indexes
	plansCollection := GetCollection("plans")
	planIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"slug", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"is_active", 1}, {"sort_order", 1}},
		},
	}

	if _, err := plansCollection.Indexes().CreateMany(ctx, planIndexes); err != nil {
		return fmt.Errorf("failed to create plan indexes: %v", err)
	}

	// Admin collection indexes
	adminsCollection := GetCollection("admins")
	adminIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"username", 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	if _, err := adminsCollection.Indexes().CreateMany(ctx, adminIndexes); err != nil {
		return fmt.Errorf("failed to create admin indexes: %v", err)
	}

	// Settings collection indexes
	settingsCollection := GetCollection("settings")
	settingIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"key", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"group", 1}},
		},
	}

	if _, err := settingsCollection.Indexes().CreateMany(ctx, settingIndexes); err != nil {
		return fmt.Errorf("failed to create setting indexes: %v", err)
	}

	// Storage providers collection indexes
	storageProvidersCollection := GetCollection("storage_providers")
	storageProviderIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"name", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"type", 1}, {"is_active", 1}},
		},
		{
			Keys: bson.D{{"is_default", 1}},
		},
	}

	if _, err := storageProvidersCollection.Indexes().CreateMany(ctx, storageProviderIndexes); err != nil {
		return fmt.Errorf("failed to create storage provider indexes: %v", err)
	}

	// Sessions collection indexes
	sessionsCollection := GetCollection("sessions")
	sessionIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"session_id", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"user_id", 1}},
		},
		{
			Keys: bson.D{{"expires_at", 1}},
		},
	}

	if _, err := sessionsCollection.Indexes().CreateMany(ctx, sessionIndexes); err != nil {
		return fmt.Errorf("failed to create session indexes: %v", err)
	}

	// API Keys collection indexes
	apiKeysCollection := GetCollection("api_keys")
	apiKeyIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"key_hash", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"user_id", 1}},
		},
		{
			Keys: bson.D{{"is_active", 1}},
		},
	}

	if _, err := apiKeysCollection.Indexes().CreateMany(ctx, apiKeyIndexes); err != nil {
		return fmt.Errorf("failed to create API key indexes: %v", err)
	}

	// File shares collection indexes
	fileSharesCollection := GetCollection("file_shares")
	fileShareIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"share_token", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"file_id", 1}},
		},
		{
			Keys: bson.D{{"created_by", 1}},
		},
		{
			Keys: bson.D{{"expires_at", 1}},
		},
	}

	if _, err := fileSharesCollection.Indexes().CreateMany(ctx, fileShareIndexes); err != nil {
		return fmt.Errorf("failed to create file share indexes: %v", err)
	}

	// Subscriptions collection indexes
	subscriptionsCollection := GetCollection("subscriptions")
	subscriptionIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{"user_id", 1}, {"status", 1}},
		},
		{
			Keys: bson.D{{"plan_id", 1}},
		},
		{
			Keys: bson.D{{"status", 1}, {"next_billing_date", 1}},
		},
	}

	if _, err := subscriptionsCollection.Indexes().CreateMany(ctx, subscriptionIndexes); err != nil {
		return fmt.Errorf("failed to create subscription indexes: %v", err)
	}

	// Activities collection indexes
	activitiesCollection := GetCollection("activities")
	activityIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{"user_id", 1}, {"created_at", -1}},
		},
		{
			Keys: bson.D{{"action", 1}},
		},
		{
			Keys: bson.D{{"resource_type", 1}, {"resource_id", 1}},
		},
	}

	if _, err := activitiesCollection.Indexes().CreateMany(ctx, activityIndexes); err != nil {
		return fmt.Errorf("failed to create activity indexes: %v", err)
	}

	log.Println("Database indexes created successfully")
	return nil
}
