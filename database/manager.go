package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Manager handles all database operations with proper connection pooling
type Manager struct {
	client      *mongo.Client
	database    *mongo.Database
	collections map[string]*mongo.Collection
	mu          sync.RWMutex
	config      *Config
}

type Config struct {
	MongoURI        string
	DatabaseName    string
	MaxPoolSize     uint64
	MinPoolSize     uint64
	MaxConnIdleTime time.Duration
	ConnectTimeout  time.Duration
	ServerTimeout   time.Duration
	SocketTimeout   time.Duration
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager returns the singleton database manager instance
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			collections: make(map[string]*mongo.Collection),
		}
	})
	return instance
}

// Initialize sets up the database connection with proper configuration
func (m *Manager) Initialize(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		return fmt.Errorf("database already initialized")
	}

	m.config = config

	// Set optimized connection options
	clientOptions := options.Client().
		ApplyURI(config.MongoURI).
		SetMaxPoolSize(config.MaxPoolSize).
		SetMinPoolSize(config.MinPoolSize).
		SetMaxConnIdleTime(config.MaxConnIdleTime).
		SetServerSelectionTimeout(config.ServerTimeout).
		SetSocketTimeout(config.SocketTimeout).
		SetConnectTimeout(config.ConnectTimeout).
		SetRetryWrites(true).
		SetRetryReads(true)

	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	var err error
	m.client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Verify connection
	if err = m.client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	m.database = m.client.Database(config.DatabaseName)

	log.Printf("Successfully connected to MongoDB database: %s", config.DatabaseName)
	return nil
}

// GetCollection returns a cached collection instance
func (m *Manager) GetCollection(name string) *mongo.Collection {
	m.mu.RLock()
	if collection, exists := m.collections[name]; exists {
		m.mu.RUnlock()
		return collection
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check pattern
	if collection, exists := m.collections[name]; exists {
		return collection
	}

	collection := m.database.Collection(name)
	m.collections[name] = collection
	return collection
}

// GetDatabase returns the database instance
func (m *Manager) GetDatabase() *mongo.Database {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.database
}

// GetClient returns the MongoDB client
func (m *Manager) GetClient() *mongo.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// Close gracefully closes the database connection
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := m.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect from MongoDB: %v", err)
	}

	m.client = nil
	m.database = nil
	m.collections = make(map[string]*mongo.Collection)

	log.Println("Database connection closed successfully")
	return nil
}

// HealthCheck verifies database connectivity
func (m *Manager) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if m.client == nil {
		return fmt.Errorf("database not initialized")
	}

	return m.client.Ping(ctx, readpref.Primary())
}
