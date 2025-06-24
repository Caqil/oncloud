package config

import (
	"context"
	"fmt"
	"log"
	"oncloud/database"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// DatabaseManager handles database initialization and management
type DatabaseManager struct {
	client   *mongo.Client
	database *mongo.Database
	config   *Config
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(cfg *Config) *DatabaseManager {
	return &DatabaseManager{
		config: cfg,
	}
}

// Initialize initializes the database connection
func (dm *DatabaseManager) Initialize() error {
	log.Println("Initializing database connection...")

	// Set connection options with production-ready settings
	clientOptions := options.Client().
		ApplyURI(dm.config.MongoURI).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(10 * time.Second).
		SetSocketTimeout(10 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetRetryWrites(true).
		SetRetryReads(true)

	// Add authentication if credentials are in the URI
	if dm.config.IsDevelopment() {
		// Enable more verbose logging in development
		clientOptions.SetLoggerOptions(&options.LoggerOptions{
			ComponentLevels: map[options.LogComponent]options.LogLevel{
				options.LogComponentCommand: options.LogLevelDebug,
			},
		})
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	dm.client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Ping the database to verify connection
	if err = dm.client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	// Set database
	dm.database = dm.client.Database(dm.config.DBName)

	// Set global database connection for the database package
	// database.SetClient(dm.client)
	// database.SetDatabase(dm.database)

	log.Printf("Successfully connected to MongoDB database: %s", dm.config.DBName)
	return nil
}

// SetupDatabase performs initial database setup
func (dm *DatabaseManager) SetupDatabase() error {
	log.Println("Setting up database...")

	// Create indexes
	if err := dm.CreateIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %v", err)
	}

	// Run migrations
	if err := dm.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	log.Println("Database setup completed successfully")
	return nil
}

// CreateIndexes creates all necessary database indexes
func (dm *DatabaseManager) CreateIndexes() error {
	log.Println("Creating database indexes...")

	// Use the existing CreateIndexes function from database package
	return database.CreateIndexes()
}

// RunMigrations runs all database migrations
func (dm *DatabaseManager) RunMigrations() error {
	log.Println("Running database migrations...")

	// Use the existing RunMigrations function from database package
	return database.RunMigrations()
}

// HealthCheck performs a database health check
func (dm *DatabaseManager) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return dm.client.Ping(ctx, readpref.Primary())
}

// GetStats returns database statistics
func (dm *DatabaseManager) GetStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stats bson.M
	result := dm.database.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}})
	if err := result.Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to get database stats: %v", err)
	}

	// Get collection stats
	collections := []string{"users", "files", "folders", "plans", "admins", "settings", "storage_providers"}
	collectionStats := make(map[string]interface{})

	for _, collName := range collections {
		var collStats bson.M
		collResult := dm.database.RunCommand(ctx, bson.D{
			{Key: "collStats", Value: collName},
		})
		if err := collResult.Decode(&collStats); err == nil {
			collectionStats[collName] = collStats
		}
	}

	return map[string]interface{}{
		"database":    stats,
		"collections": collectionStats,
		"server_time": time.Now(),
	}, nil
}

// GetCollectionSizes returns sizes of all collections
func (dm *DatabaseManager) GetCollectionSizes() (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collectionNames, err := dm.database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %v", err)
	}

	sizes := make(map[string]int64)
	for _, name := range collectionNames {
		count, err := dm.database.Collection(name).EstimatedDocumentCount(ctx)
		if err != nil {
			log.Printf("Warning: Could not get size for collection %s: %v", name, err)
			continue
		}
		sizes[name] = count
	}

	return sizes, nil
}

// Backup creates a logical backup of the database
func (dm *DatabaseManager) Backup(outputPath string) error {
	log.Printf("Creating database backup to %s...", outputPath)

	// This is a simplified backup - in production you might want to use mongodump
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	collections := []string{"users", "files", "folders", "plans", "admins", "settings", "storage_providers"}

	for _, collName := range collections {
		collection := dm.database.Collection(collName)
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("failed to backup collection %s: %v", collName, err)
		}

		var documents []bson.M
		if err = cursor.All(ctx, &documents); err != nil {
			cursor.Close(ctx)
			return fmt.Errorf("failed to read collection %s: %v", collName, err)
		}
		cursor.Close(ctx)

		log.Printf("Backed up %d documents from collection %s", len(documents), collName)
	}

	log.Println("Database backup completed")
	return nil
}

// CleanupOldData removes old data based on retention policies
func (dm *DatabaseManager) CleanupOldData() error {
	log.Println("Starting database cleanup...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Clean up old logs (keep last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	logsCollection := dm.database.Collection("logs")

	result, err := logsCollection.DeleteMany(ctx, bson.M{
		"created_at": bson.M{"$lt": thirtyDaysAgo},
	})
	if err != nil {
		log.Printf("Warning: Failed to cleanup old logs: %v", err)
	} else {
		log.Printf("Cleaned up %d old log entries", result.DeletedCount)
	}

	// Clean up expired file shares
	sharesCollection := dm.database.Collection("file_shares")
	result, err = sharesCollection.DeleteMany(ctx, bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
		"is_active":  false,
	})
	if err != nil {
		log.Printf("Warning: Failed to cleanup expired shares: %v", err)
	} else {
		log.Printf("Cleaned up %d expired file shares", result.DeletedCount)
	}

	// Clean up old session data if you have a sessions collection
	sessionsCollection := dm.database.Collection("sessions")
	result, err = sessionsCollection.DeleteMany(ctx, bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
	})
	if err != nil {
		log.Printf("Warning: Failed to cleanup old sessions: %v", err)
	} else if result.DeletedCount > 0 {
		log.Printf("Cleaned up %d expired sessions", result.DeletedCount)
	}

	log.Println("Database cleanup completed")
	return nil
}

// Close closes the database connection
func (dm *DatabaseManager) Close() error {
	if dm.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := dm.client.Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to disconnect from MongoDB: %v", err)
		}
		log.Println("Disconnected from MongoDB")
	}
	return nil
}

// GetClient returns the MongoDB client
func (dm *DatabaseManager) GetClient() *mongo.Client {
	return dm.client
}

// GetDatabase returns the MongoDB database
func (dm *DatabaseManager) GetDatabase() *mongo.Database {
	return dm.database
}

// MonitorConnection monitors the database connection and logs status
func (dm *DatabaseManager) MonitorConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := dm.HealthCheck(); err != nil {
			log.Printf("Database health check failed: %v", err)
		} else if dm.config.Debug {
			log.Println("Database connection healthy")
		}
	}
}
