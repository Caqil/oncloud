package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Folder struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID      primitive.ObjectID  `bson:"user_id" json:"user_id"`
	ParentID    *primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id,omitempty"`
	Name        string              `bson:"name" json:"name" validate:"required"`
	Description string              `bson:"description" json:"description"`
	Path        string              `bson:"path" json:"path"`
	Color       string              `bson:"color" json:"color"`
	Icon        string              `bson:"icon" json:"icon"`
	IsPublic    bool                `bson:"is_public" json:"is_public"`
	IsShared    bool                `bson:"is_shared" json:"is_shared"`
	IsFavorite  bool                `bson:"is_favorite" json:"is_favorite"`
	IsDeleted   bool                `bson:"is_deleted" json:"is_deleted"`
	FilesCount  int                 `bson:"files_count" json:"files_count"`
	Size        int64               `bson:"size" json:"size"`
	ShareToken  string              `bson:"share_token" json:"share_token"`
	Tags        []string            `bson:"tags" json:"tags"`
	CreatedAt   time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time           `bson:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

type FolderTree struct {
	Folder   *Folder       `json:"folder"`
	Children []*FolderTree `json:"children,omitempty"`
	Files    []*File       `json:"files,omitempty"`
}
