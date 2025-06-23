package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func FileRoutes(r *gin.RouterGroup) {
	fileController := controllers.NewFileController()

	files := r.Group("/files")
	files.Use(middleware.AuthMiddleware())
	{
		// File CRUD operations
		files.GET("/", fileController.GetFiles)
		files.GET("/:id", fileController.GetFile)
		files.POST("/upload", fileController.Upload)
		files.POST("/upload/chunk", fileController.ChunkUpload)
		files.POST("/upload/complete", fileController.CompleteChunkUpload)
		files.PUT("/:id", fileController.UpdateFile)
		files.DELETE("/:id", fileController.DeleteFile)
		files.POST("/:id/restore", fileController.RestoreFile)
		files.DELETE("/:id/permanent", fileController.PermanentDelete)

		// File operations
		files.GET("/:id/download", fileController.Download)
		files.GET("/:id/stream", fileController.Stream)
		files.GET("/:id/preview", fileController.Preview)
		files.GET("/:id/thumbnail", fileController.GetThumbnail)
		files.POST("/:id/thumbnail", fileController.GenerateThumbnail)

		// File sharing
		files.POST("/:id/share", fileController.CreateShare)
		files.GET("/:id/share", fileController.GetShare)
		files.PUT("/:id/share", fileController.UpdateShare)
		files.DELETE("/:id/share", fileController.DeleteShare)
		files.GET("/:id/share/url", fileController.GetShareURL)

		// File organization
		files.POST("/:id/copy", fileController.CopyFile)
		files.POST("/:id/move", fileController.MoveFile)
		files.POST("/:id/favorite", fileController.AddToFavorites)
		files.DELETE("/:id/favorite", fileController.RemoveFromFavorites)
		files.PUT("/:id/tags", fileController.UpdateTags)

		// File versions
		files.GET("/:id/versions", fileController.GetVersions)
		files.POST("/:id/versions", fileController.CreateVersion)
		files.GET("/:id/versions/:version", fileController.GetVersion)
		files.POST("/:id/versions/:version/restore", fileController.RestoreVersion)
		files.DELETE("/:id/versions/:version", fileController.DeleteVersion)

		// Bulk operations
		files.POST("/bulk/delete", fileController.BulkDelete)
		files.POST("/bulk/move", fileController.BulkMove)
		files.POST("/bulk/copy", fileController.BulkCopy)
		files.POST("/bulk/download", fileController.BulkDownload)
		files.POST("/bulk/share", fileController.BulkShare)
	}

	// Public file access (no auth required)
	r.GET("/public/:token", fileController.PublicDownload)
	r.GET("/shared/:token", fileController.SharedDownload)
	r.POST("/shared/:token/password", fileController.VerifySharePassword)
}
