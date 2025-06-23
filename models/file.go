package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type File struct {
	ID              primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	UserID          primitive.ObjectID     `bson:"user_id" json:"user_id"`
	FolderID        *primitive.ObjectID    `bson:"folder_id,omitempty" json:"folder_id,omitempty"`
	Name            string                 `bson:"name" json:"name" validate:"required"`
	OriginalName    string                 `bson:"original_name" json:"original_name"`
	DisplayName     string                 `bson:"display_name" json:"display_name"`
	Description     string                 `bson:"description" json:"description"`
	Path            string                 `bson:"path" json:"path"`
	Size            int64                  `bson:"size" json:"size"`
	MimeType        string                 `bson:"mime_type" json:"mime_type"`
	Extension       string                 `bson:"extension" json:"extension"`
	Hash            string                 `bson:"hash" json:"hash"` // for duplicate detection
	StorageProvider string                 `bson:"storage_provider" json:"storage_provider"`
	StorageKey      string                 `bson:"storage_key" json:"storage_key"`
	StorageBucket   string                 `bson:"storage_bucket" json:"storage_bucket"`
	PublicURL       string                 `bson:"public_url" json:"public_url"`
	ThumbnailURL    string                 `bson:"thumbnail_url" json:"thumbnail_url"`
	IsPublic        bool                   `bson:"is_public" json:"is_public"`
	IsShared        bool                   `bson:"is_shared" json:"is_shared"`
	IsFavorite      bool                   `bson:"is_favorite" json:"is_favorite"`
	IsDeleted       bool                   `bson:"is_deleted" json:"is_deleted"`
	Downloads       int                    `bson:"downloads" json:"downloads"`
	Views           int                    `bson:"views" json:"views"`
	ShareToken      string                 `bson:"share_token" json:"share_token"`
	ShareExpiresAt  *time.Time             `bson:"share_expires_at,omitempty" json:"share_expires_at,omitempty"`
	Tags            []string               `bson:"tags" json:"tags"`
	Metadata        map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedAt       time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time              `bson:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time             `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

type FileShare struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FileID       primitive.ObjectID `bson:"file_id" json:"file_id"`
	UserID       primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token        string             `bson:"token" json:"token"`
	Password     string             `bson:"password" json:"password,omitempty"`
	Downloads    int                `bson:"downloads" json:"downloads"`
	MaxDownloads int                `bson:"max_downloads" json:"max_downloads"`
	ExpiresAt    *time.Time         `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
	IsActive     bool               `bson:"is_active" json:"is_active"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

type FileVersion struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FileID        primitive.ObjectID `bson:"file_id" json:"file_id"`
	VersionNumber int                `bson:"version_number" json:"version_number"`
	Size          int64              `bson:"size" json:"size"`
	StorageKey    string             `bson:"storage_key" json:"storage_key"`
	Hash          string             `bson:"hash" json:"hash"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
}
