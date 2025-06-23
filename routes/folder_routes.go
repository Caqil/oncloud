package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func FolderRoutes(r *gin.RouterGroup) {
	folderController := controllers.NewFolderController()

	folders := r.Group("/folders")
	folders.Use(middleware.AuthMiddleware())
	{
		// Folder CRUD operations
		folders.GET("/", folderController.GetFolders)
		folders.GET("/:id", folderController.GetFolder)
		folders.POST("/", folderController.CreateFolder)
		folders.PUT("/:id", folderController.UpdateFolder)
		folders.DELETE("/:id", folderController.DeleteFolder)
		folders.POST("/:id/restore", folderController.RestoreFolder)
		folders.DELETE("/:id/permanent", folderController.PermanentDelete)

		// Folder navigation
		folders.GET("/:id/contents", folderController.GetFolderContents)
		folders.GET("/:id/tree", folderController.GetFolderTree)
		folders.GET("/:id/breadcrumb", folderController.GetBreadcrumb)
		folders.GET("/root", folderController.GetRootFolder)
		folders.GET("/recent", folderController.GetRecentFolders)
		folders.GET("/favorites", folderController.GetFavoriteFolders)
		folders.GET("/trash", folderController.GetDeletedFolders)

		// Folder operations
		folders.POST("/:id/copy", folderController.CopyFolder)
		folders.POST("/:id/move", folderController.MoveFolder)
		folders.POST("/:id/favorite", folderController.AddToFavorites)
		folders.DELETE("/:id/favorite", folderController.RemoveFromFavorites)
		folders.PUT("/:id/tags", folderController.UpdateTags)

		// Folder sharing
		folders.POST("/:id/share", folderController.CreateShare)
		folders.GET("/:id/share", folderController.GetShare)
		folders.PUT("/:id/share", folderController.UpdateShare)
		folders.DELETE("/:id/share", folderController.DeleteShare)
		folders.GET("/:id/share/url", folderController.GetShareURL)

		// Folder statistics
		folders.GET("/:id/stats", folderController.GetFolderStats)
		folders.GET("/:id/size", folderController.GetFolderSize)

		// Bulk operations
		folders.POST("/bulk/delete", folderController.BulkDelete)
		folders.POST("/bulk/move", folderController.BulkMove)
		folders.POST("/bulk/copy", folderController.BulkCopy)
		folders.POST("/bulk/share", folderController.BulkShare)
	}

	// Public folder access
	r.GET("/public/folder/:token", folderController.PublicFolderAccess)
	r.GET("/shared/folder/:token", folderController.SharedFolderAccess)
}
