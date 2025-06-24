package services

import (
	"oncloud/database"

	"go.mongodb.org/mongo-driver/mongo"
)

// BaseService provides common database access for all services
type BaseService struct {
	collections *database.Collections
	manager     *database.Manager
}

// NewBaseService creates a new base service instance
func NewBaseService() *BaseService {
	return &BaseService{
		collections: database.NewCollections(),
		manager:     database.GetManager(),
	}
}

// GetDatabase returns the database instance
func (bs *BaseService) GetDatabase() *mongo.Database {
	return bs.manager.GetDatabase()
}

// GetCollections returns the collections accessor
func (bs *BaseService) GetCollections() *database.Collections {
	return bs.collections
}
