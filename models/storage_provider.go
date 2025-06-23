package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StorageProvider struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Name         string                 `bson:"name" json:"name" validate:"required"`
	Type         string                 `bson:"type" json:"type"` // s3, wasabi, r2, local
	Region       string                 `bson:"region" json:"region"`
	Endpoint     string                 `bson:"endpoint" json:"endpoint"`
	Bucket       string                 `bson:"bucket" json:"bucket"`
	AccessKey    string                 `bson:"access_key" json:"access_key"`
	SecretKey    string                 `bson:"secret_key" json:"-"`
	CDNUrl       string                 `bson:"cdn_url" json:"cdn_url"`
	MaxFileSize  int64                  `bson:"max_file_size" json:"max_file_size"`
	AllowedTypes []string               `bson:"allowed_types" json:"allowed_types"`
	Settings     map[string]interface{} `bson:"settings" json:"settings"`
	IsActive     bool                   `bson:"is_active" json:"is_active"`
	IsDefault    bool                   `bson:"is_default" json:"is_default"`
	Priority     int                    `bson:"priority" json:"priority"`
	StorageUsed  int64                  `bson:"storage_used" json:"storage_used"`
	FilesCount   int                    `bson:"files_count" json:"files_count"`
	LastSyncAt   *time.Time             `bson:"last_sync_at,omitempty" json:"last_sync_at,omitempty"`
	CreatedAt    time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time              `bson:"updated_at" json:"updated_at"`
}

type StorageStats struct {
	TotalStorage   int64   `json:"total_storage"`
	UsedStorage    int64   `json:"used_storage"`
	FreeStorage    int64   `json:"free_storage"`
	TotalFiles     int     `json:"total_files"`
	StoragePercent float64 `json:"storage_percent"`
}
