package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func StorageRoutes(r *gin.RouterGroup) {
	storageController := controllers.NewStorageController()

	storage := r.Group("/storage")
	storage.Use(middleware.AuthMiddleware())
	{
		// Storage provider information
		storage.GET("/providers", storageController.GetProviders)
		storage.GET("/providers/:id", storageController.GetProvider)
		storage.GET("/stats", storageController.GetStorageStats)
		storage.GET("/usage", storageController.GetStorageUsage)

		// File operations across providers
		storage.POST("/sync", storageController.SyncFiles)
		storage.POST("/migrate", storageController.MigrateFiles)
		storage.GET("/health", storageController.CheckProvidersHealth)

		// Upload operations
		storage.POST("/upload/url", storageController.GetUploadURL)
		storage.POST("/upload/multipart", storageController.InitiateMultipartUpload)
		storage.PUT("/upload/multipart/:upload_id/part/:part_number", storageController.UploadPart)
		storage.POST("/upload/multipart/:upload_id/complete", storageController.CompleteMultipartUpload)
		storage.DELETE("/upload/multipart/:upload_id", storageController.AbortMultipartUpload)

		// CDN and optimization
		storage.POST("/cdn/invalidate", storageController.InvalidateCDN)
		storage.GET("/cdn/stats", storageController.GetCDNStats)
		storage.POST("/optimize/images", storageController.OptimizeImages)

		// Backup and restore
		storage.POST("/backup", storageController.CreateBackup)
		storage.GET("/backups", storageController.GetBackups)
		storage.POST("/restore/:backup_id", storageController.RestoreBackup)
		storage.DELETE("/backups/:backup_id", storageController.DeleteBackup)
	}
}
