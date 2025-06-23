package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func UserRoutes(r *gin.RouterGroup) {
	userController := controllers.NewUserController()

	users := r.Group("/users")
	users.Use(middleware.AuthMiddleware())
	{
		// User profile management
		users.GET("/profile", userController.GetProfile)
		users.PUT("/profile", userController.UpdateProfile)
		users.POST("/avatar", userController.UploadAvatar)
		users.DELETE("/avatar", userController.DeleteAvatar)

		// User statistics and dashboard
		users.GET("/stats", userController.GetUserStats)
		users.GET("/dashboard", userController.GetDashboard)
		users.GET("/activity", userController.GetActivity)
		users.GET("/notifications", userController.GetNotifications)
		users.PUT("/notifications/:id/read", userController.MarkNotificationRead)

		// User settings
		users.GET("/settings", userController.GetSettings)
		users.PUT("/settings", userController.UpdateSettings)
		users.GET("/sessions", userController.GetActiveSessions)
		users.DELETE("/sessions/:id", userController.RevokeSession)

		// API keys management
		users.GET("/api-keys", userController.GetAPIKeys)
		users.POST("/api-keys", userController.CreateAPIKey)
		users.PUT("/api-keys/:id", userController.UpdateAPIKey)
		users.DELETE("/api-keys/:id", userController.DeleteAPIKey)

		// Two-factor authentication
		users.GET("/2fa/status", userController.Get2FAStatus)
		users.POST("/2fa/enable", userController.Enable2FA)
		users.POST("/2fa/verify", userController.Verify2FA)
		users.POST("/2fa/disable", userController.Disable2FA)
		users.GET("/2fa/backup-codes", userController.GetBackupCodes)
		users.POST("/2fa/backup-codes/regenerate", userController.RegenerateBackupCodes)
	}
}
