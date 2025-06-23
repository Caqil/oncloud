package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username        string            `bson:"username" json:"username" validate:"required,min=3,max=50"`
	Email           string            `bson:"email" json:"email" validate:"required,email"`
	Password        string            `bson:"password" json:"-" validate:"required,min=6"`
	FirstName       string            `bson:"first_name" json:"first_name" validate:"required"`
	LastName        string            `bson:"last_name" json:"last_name" validate:"required"`
	Avatar          string            `bson:"avatar" json:"avatar"`
	Phone           string            `bson:"phone" json:"phone"`
	Country         string            `bson:"country" json:"country"`
	PlanID          primitive.ObjectID `bson:"plan_id" json:"plan_id"`
	StorageUsed     int64             `bson:"storage_used" json:"storage_used"` // in bytes
	BandwidthUsed   int64             `bson:"bandwidth_used" json:"bandwidth_used"` // in bytes
	FilesCount      int               `bson:"files_count" json:"files_count"`
	FoldersCount    int               `bson:"folders_count" json:"folders_count"`
	IsActive        bool              `bson:"is_active" json:"is_active"`
	IsVerified      bool              `bson:"is_verified" json:"is_verified"`
	IsPremium       bool              `bson:"is_premium" json:"is_premium"`
	EmailVerifiedAt *time.Time        `bson:"email_verified_at,omitempty" json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time        `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	PlanExpiresAt   *time.Time        `bson:"plan_expires_at,omitempty" json:"plan_expires_at,omitempty"`
	CreatedAt       time.Time         `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time         `bson:"updated_at" json:"updated_at"`
}

type UserProfile struct {
	ID        primitive.ObjectID `json:"id"`
	Username  string            `json:"username"`
	Email     string            `json:"email"`
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Avatar    string            `json:"avatar"`
	Plan      *Plan             `json:"plan,omitempty"`
}

type UserStats struct {
	StorageUsed     int64 `json:"storage_used"`
	StorageLimit    int64 `json:"storage_limit"`
	BandwidthUsed   int64 `json:"bandwidth_used"`
	BandwidthLimit  int64 `json:"bandwidth_limit"`
	FilesCount      int   `json:"files_count"`
	FoldersCount    int   `json:"folders_count"`
	StoragePercent  float64 `json:"storage_percent"`
	BandwidthPercent float64 `json:"bandwidth_percent"`
}
