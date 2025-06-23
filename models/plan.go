package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Plan struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name             string             `bson:"name" json:"name" validate:"required"`
	Slug             string             `bson:"slug" json:"slug"`
	Description      string             `bson:"description" json:"description"`
	ShortDescription string             `bson:"short_description" json:"short_description"`
	StorageLimit     int64              `bson:"storage_limit" json:"storage_limit"`     // in bytes
	BandwidthLimit   int64              `bson:"bandwidth_limit" json:"bandwidth_limit"` // in bytes per month
	FilesLimit       int                `bson:"files_limit" json:"files_limit"`
	FoldersLimit     int                `bson:"folders_limit" json:"folders_limit"`
	Price            float64            `bson:"price" json:"price"`
	OriginalPrice    float64            `bson:"original_price" json:"original_price"`
	Currency         string             `bson:"currency" json:"currency"`
	BillingCycle     string             `bson:"billing_cycle" json:"billing_cycle"` // daily, weekly, monthly, yearly
	MaxFileSize      int64              `bson:"max_file_size" json:"max_file_size"`
	AllowedTypes     []string           `bson:"allowed_types" json:"allowed_types"`
	Features         []string           `bson:"features" json:"features"`
	Limitations      []string           `bson:"limitations" json:"limitations"`
	PopularBadge     bool               `bson:"popular_badge" json:"popular_badge"`
	IsActive         bool               `bson:"is_active" json:"is_active"`
	IsDefault        bool               `bson:"is_default" json:"is_default"`
	IsFree           bool               `bson:"is_free" json:"is_free"`
	SortOrder        int                `bson:"sort_order" json:"sort_order"`
	TrialDays        int                `bson:"trial_days" json:"trial_days"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}

type UserPlan struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID        primitive.ObjectID `bson:"user_id" json:"user_id"`
	PlanID        primitive.ObjectID `bson:"plan_id" json:"plan_id"`
	Status        string             `bson:"status" json:"status"` // active, expired, cancelled
	PaymentMethod string             `bson:"payment_method" json:"payment_method"`
	TransactionID string             `bson:"transaction_id" json:"transaction_id"`
	Amount        float64            `bson:"amount" json:"amount"`
	Currency      string             `bson:"currency" json:"currency"`
	StartDate     time.Time          `bson:"start_date" json:"start_date"`
	EndDate       time.Time          `bson:"end_date" json:"end_date"`
	IsRecurring   bool               `bson:"is_recurring" json:"is_recurring"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}
