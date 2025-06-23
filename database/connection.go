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

// Connect establishes connection to MongoDB
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
	return database.Collection(collectionName)
}

// Ping checks the database connection
func Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.Ping(ctx, readpref.Primary())
}

// GetStats returns database statistics
func GetStats() (bson.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	// Users collection indexes
	usersCollection := GetCollection("users")
	userIndexes := []mongo.IndexModel{
		{
			Keys:    map[string]int{"email": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    map[string]int{"username": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]int{"created_at": -1},
		},
		{
			Keys: map[string]int{"plan_id": 1},
		},
	}

	if _, err := usersCollection.Indexes().CreateMany(ctx, userIndexes); err != nil {
		return fmt.Errorf("failed to create user indexes: %v", err)
	}

	// Files collection indexes
	filesCollection := GetCollection("files")
	fileIndexes := []mongo.IndexModel{
		{
			Keys: map[string]int{"user_id": 1, "created_at": -1},
		},
		{
			Keys: map[string]int{"folder_id": 1},
		},
		{
			Keys: map[string]int{"hash": 1},
		},
		{
			Keys: map[string]int{"storage_provider": 1},
		},
		{
			Keys: map[string]int{"is_public": 1},
		},
		{
			Keys:    map[string]int{"share_token": 1},
			Options: options.Index().SetSparse(true),
		},
		{
			Keys: map[string]int{"mime_type": 1},
		},
	}

	if _, err := filesCollection.Indexes().CreateMany(ctx, fileIndexes); err != nil {
		return fmt.Errorf("failed to create file indexes: %v", err)
	}

	// Folders collection indexes
	foldersCollection := GetCollection("folders")
	folderIndexes := []mongo.IndexModel{
		{
			Keys: map[string]int{"user_id": 1, "parent_id": 1},
		},
		{
			Keys:    map[string]int{"user_id": 1, "name": 1, "parent_id": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    map[string]int{"share_token": 1},
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
			Keys:    map[string]int{"slug": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]int{"is_active": 1, "sort_order": 1},
		},
	}

	if _, err := plansCollection.Indexes().CreateMany(ctx, planIndexes); err != nil {
		return fmt.Errorf("failed to create plan indexes: %v", err)
	}

	// Admin collection indexes
	adminsCollection := GetCollection("admins")
	adminIndexes := []mongo.IndexModel{
		{
			Keys:    map[string]int{"email": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    map[string]int{"username": 1},
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
			Keys:    map[string]int{"key": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: map[string]int{"group": 1},
		},
	}

	if _, err := settingsCollection.Indexes().CreateMany(ctx, settingIndexes); err != nil {
		return fmt.Errorf("failed to create setting indexes: %v", err)
	}

	log.Println("Successfully created database indexes")
	return nil
}
