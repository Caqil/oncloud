package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminSettings struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Key         string             `bson:"key" json:"key" validate:"required"`
	Value       interface{}        `bson:"value" json:"value"`
	Type        string             `bson:"type" json:"type"` // string, int, bool, json, array
	Group       string             `bson:"group" json:"group"`
	Label       string             `bson:"label" json:"label"`
	Description string             `bson:"description" json:"description"`
	Options     []SettingOption    `bson:"options" json:"options"`
	Rules       []string           `bson:"rules" json:"rules"`
	IsPublic    bool               `bson:"is_public" json:"is_public"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

type SettingOption struct {
	Label string      `bson:"label" json:"label"`
	Value interface{} `bson:"value" json:"value"`
}

type SystemSettings struct {
	SiteName               string   `json:"site_name"`
	SiteDescription        string   `json:"site_description"`
	SiteUrl                string   `json:"site_url"`
	SiteLogo               string   `json:"site_logo"`
	SiteFavicon            string   `json:"site_favicon"`
	ContactEmail           string   `json:"contact_email"`
	SupportEmail           string   `json:"support_email"`
	AllowRegistration      bool     `json:"allow_registration"`
	EmailVerification      bool     `json:"email_verification"`
	MaintenanceMode        bool     `json:"maintenance_mode"`
	DefaultPlanID          string   `json:"default_plan_id"`
	DefaultStorageProvider string   `json:"default_storage_provider"`
	MaxUploadSize          int64    `json:"max_upload_size"`
	AllowedFileTypes       []string `json:"allowed_file_types"`
}

type Admin struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username    string             `bson:"username" json:"username" validate:"required"`
	Email       string             `bson:"email" json:"email" validate:"required,email"`
	Password    string             `bson:"password" json:"-" validate:"required"`
	FirstName   string             `bson:"first_name" json:"first_name"`
	LastName    string             `bson:"last_name" json:"last_name"`
	Avatar      string             `bson:"avatar" json:"avatar"`
	Role        string             `bson:"role" json:"role"` // super_admin, admin, moderator
	Permissions []string           `bson:"permissions" json:"permissions"`
	IsActive    bool               `bson:"is_active" json:"is_active"`
	LastLoginAt *time.Time         `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
