package models

import "time"

type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Meta      *Meta       `json:"meta,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type APIError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type Meta struct {
	Page       int `json:"page,omitempty"`
	Limit      int `json:"limit,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RegisterRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=50"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=6"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" validate:"required"`
}

type UploadResponse struct {
	File      *File  `json:"file"`
	UploadURL string `json:"upload_url,omitempty"`
}

type DashboardStats struct {
	TotalUsers        int     `json:"total_users"`
	TotalFiles        int     `json:"total_files"`
	TotalStorage      int64   `json:"total_storage"`
	TotalBandwidth    int64   `json:"total_bandwidth"`
	NewUsersToday     int     `json:"new_users_today"`
	UploadsToday      int     `json:"uploads_today"`
	DownloadsToday    int     `json:"downloads_today"`
	Revenue           float64 `json:"revenue"`
	ActiveSubscriptions int   `json:"active_subscriptions"`
	StorageProviders  []StorageStats `json:"storage_providers"`
}

type FileUploadRequest struct {
	FolderID    string            `form:"folder_id"`
	Name        string            `form:"name"`
	Description string            `form:"description"`
	IsPublic    bool              `form:"is_public"`
	Tags        []string          `form:"tags"`
	Metadata    map[string]string `form:"metadata"`
}

type FolderCreateRequest struct {
	Name        string `json:"name" validate:"required"`
	ParentID    string `json:"parent_id,omitempty"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	IsPublic    bool   `json:"is_public"`
}

type ShareRequest struct {
	Password     string     `json:"password,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxDownloads int        `json:"max_downloads,omitempty"`
}