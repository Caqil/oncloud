package database

import "go.mongodb.org/mongo-driver/mongo"

// Collection names as constants to prevent typos
const (
	UsersCollection             = "users"
	FilesCollection             = "files"
	FoldersCollection           = "folders"
	PlansCollection             = "plans"
	AdminsCollection            = "admins"
	SettingsCollection          = "settings"
	SubscriptionsCollection     = "subscriptions"
	PaymentsCollection          = "payments"
	SessionsCollection          = "sessions"
	APIKeysCollection           = "api_keys"
	ActivitiesCollection        = "activities"
	NotificationsCollection     = "notifications"
	AnalyticsCollection         = "analytics"
	ExportsCollection           = "exports"
	LogsCollection              = "logs"
	StorageProvidersCollection  = "storage_providers"
	StorageSyncCollection       = "storage_sync"
	BackupsCollection           = "backups"
	StorageActivitiesCollection = "storage_activities"
	FileSharesCollection        = "file_shares"
	FileVersionsCollection      = "file_versions"
	UsageTrackingCollection     = "usage_tracking"
	BillingHistoryCollection    = "billing_history"
	InvoicesCollection          = "invoices"
	CDNInvalidationsCollection  = "cdn_invalidations"
	OptimizationJobsCollection  = "optimization_jobs"
	RestoreJobsCollection       = "restore_jobs"
)

// Collections provides typed access to all collections
type Collections struct {
	manager *Manager
}

// NewCollections creates a new collections instance
func NewCollections() *Collections {
	return &Collections{
		manager: GetManager(),
	}
}

// Core collections
func (c *Collections) Users() *mongo.Collection {
	return c.manager.GetCollection(UsersCollection)
}

func (c *Collections) Files() *mongo.Collection {
	return c.manager.GetCollection(FilesCollection)
}

func (c *Collections) Folders() *mongo.Collection {
	return c.manager.GetCollection(FoldersCollection)
}

func (c *Collections) Plans() *mongo.Collection {
	return c.manager.GetCollection(PlansCollection)
}

func (c *Collections) Admins() *mongo.Collection {
	return c.manager.GetCollection(AdminsCollection)
}

func (c *Collections) Settings() *mongo.Collection {
	return c.manager.GetCollection(SettingsCollection)
}

// Payment and subscription collections
func (c *Collections) Subscriptions() *mongo.Collection {
	return c.manager.GetCollection(SubscriptionsCollection)
}

func (c *Collections) Payments() *mongo.Collection {
	return c.manager.GetCollection(PaymentsCollection)
}

func (c *Collections) UsageTracking() *mongo.Collection {
	return c.manager.GetCollection(UsageTrackingCollection)
}

func (c *Collections) BillingHistory() *mongo.Collection {
	return c.manager.GetCollection(BillingHistoryCollection)
}

func (c *Collections) Invoices() *mongo.Collection {
	return c.manager.GetCollection(InvoicesCollection)
}

// Auth and session collections
func (c *Collections) Sessions() *mongo.Collection {
	return c.manager.GetCollection(SessionsCollection)
}

func (c *Collections) APIKeys() *mongo.Collection {
	return c.manager.GetCollection(APIKeysCollection)
}

// Activity and logging collections
func (c *Collections) Activities() *mongo.Collection {
	return c.manager.GetCollection(ActivitiesCollection)
}

func (c *Collections) Notifications() *mongo.Collection {
	return c.manager.GetCollection(NotificationsCollection)
}

func (c *Collections) Analytics() *mongo.Collection {
	return c.manager.GetCollection(AnalyticsCollection)
}

func (c *Collections) Exports() *mongo.Collection {
	return c.manager.GetCollection(ExportsCollection)
}

func (c *Collections) Logs() *mongo.Collection {
	return c.manager.GetCollection(LogsCollection)
}

// Storage collections
func (c *Collections) StorageProviders() *mongo.Collection {
	return c.manager.GetCollection(StorageProvidersCollection)
}

func (c *Collections) StorageSync() *mongo.Collection {
	return c.manager.GetCollection(StorageSyncCollection)
}

func (c *Collections) Backups() *mongo.Collection {
	return c.manager.GetCollection(BackupsCollection)
}

func (c *Collections) StorageActivities() *mongo.Collection {
	return c.manager.GetCollection(StorageActivitiesCollection)
}

// File-related collections
func (c *Collections) FileShares() *mongo.Collection {
	return c.manager.GetCollection(FileSharesCollection)
}

func (c *Collections) FileVersions() *mongo.Collection {
	return c.manager.GetCollection(FileVersionsCollection)
}

// Job and task collections
func (c *Collections) CDNInvalidations() *mongo.Collection {
	return c.manager.GetCollection(CDNInvalidationsCollection)
}

func (c *Collections) OptimizationJobs() *mongo.Collection {
	return c.manager.GetCollection(OptimizationJobsCollection)
}

func (c *Collections) RestoreJobs() *mongo.Collection {
	return c.manager.GetCollection(RestoreJobsCollection)
}
