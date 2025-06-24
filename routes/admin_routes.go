package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func AdminRoutes(r *gin.RouterGroup) {
	adminController := controllers.NewAdminController()
	userAdminController := controllers.NewUserAdminController()
	fileAdminController := controllers.NewFileAdminController()
	settingsController := controllers.NewSettingsController()
	analyticsController := controllers.NewAnalyticsController()

	// Admin authentication
	r.POST("/login", adminController.Login)
	r.POST("/logout", adminController.Logout)

	// Protected admin routes
	api := r.Group("/api")
	api.Use(middleware.AdminAuthMiddleware())
	{
		// Dashboard and analytics
		api.GET("/dashboard", analyticsController.GetDashboard)
		api.GET("/analytics/users", analyticsController.GetUserAnalytics)
		api.GET("/analytics/files", analyticsController.GetFileAnalytics)
		api.GET("/analytics/storage", analyticsController.GetStorageAnalytics)
		api.GET("/analytics/revenue", analyticsController.GetRevenueAnalytics)

		// User management
		users := api.Group("/users")
		{
			users.GET("/", userAdminController.GetUsers)
			users.GET("/:id", userAdminController.GetUser)
			users.POST("/", userAdminController.CreateUser)
			users.PUT("/:id", userAdminController.UpdateUser)
			users.DELETE("/:id", userAdminController.DeleteUser)
			users.POST("/:id/suspend", userAdminController.SuspendUser)
			users.POST("/:id/unsuspend", userAdminController.UnsuspendUser)
			users.POST("/:id/verify", userAdminController.VerifyUser)
			users.POST("/:id/reset-password", userAdminController.ResetUserPassword)
			users.GET("/:id/files", userAdminController.GetUserFiles)
			users.GET("/:id/activity", userAdminController.GetUserActivity)
		}

		// File management
		files := api.Group("/files")
		{
			files.GET("/", fileAdminController.GetFiles)
			files.GET("/:id", fileAdminController.GetFile)
			files.DELETE("/:id", fileAdminController.DeleteFile)
			files.POST("/:id/restore", fileAdminController.RestoreFile)
			files.PUT("/:id/moderate", fileAdminController.ModerateFile)
			files.GET("/reported", fileAdminController.GetReportedFiles)
			files.POST("/:id/scan", fileAdminController.ScanFile)
		}

		// Plan management
		plans := api.Group("/plans")
		{
			plans.GET("/", adminController.GetPlans)
			plans.GET("/:id", adminController.GetPlan)
			plans.POST("/", adminController.CreatePlan)
			plans.PUT("/:id", adminController.UpdatePlan)
			plans.DELETE("/:id", adminController.DeletePlan)
			plans.POST("/:id/activate", adminController.ActivatePlan)
			plans.POST("/:id/deactivate", adminController.DeactivatePlan)
		}

		// Storage provider management
		providers := api.Group("/storage-providers")
		{
			providers.GET("/", adminController.GetStorageProviders)
			providers.GET("/:id", adminController.GetStorageProvider)
			providers.POST("/", adminController.CreateStorageProvider)
			providers.PUT("/:id", adminController.UpdateStorageProvider)
			providers.DELETE("/:id", adminController.DeleteStorageProvider)
			providers.POST("/:id/test", adminController.TestStorageProvider)
			providers.POST("/:id/sync", adminController.SyncStorageProvider)
		}

		// System settings
		settings := api.Group("/settings")
		{
			settings.GET("/", settingsController.GetSettings)
			settings.PUT("/", settingsController.UpdateSettings)
			settings.GET("/groups", settingsController.GetSettingGroups)
			settings.GET("/:group", settingsController.GetSettingsByGroup)
			settings.PUT("/:key", settingsController.UpdateSetting)
			settings.POST("/backup", settingsController.BackupSettings)
			settings.POST("/restore", settingsController.RestoreSettings)
		}

		// System maintenance
		system := api.Group("/system")
		{
			system.GET("/info", adminController.GetSystemInfo)
			system.POST("/cache/clear", adminController.ClearCache)
			system.POST("/logs/clear", adminController.ClearLogs)
			system.GET("/logs", adminController.GetLogs)
			system.POST("/backup", adminController.CreateSystemBackup)
			system.GET("/backups", adminController.GetSystemBackups)
		}
	}
}

// Admin panel HTML routes
func AdminPanelRoutes(r *gin.Engine) {
	adminController := controllers.NewDashboardController()

	admin := r.Group("/admin")
	{
		// Login page (public)
		admin.GET("/login", adminController.LoginPage)

		// Protected admin panel pages
		protected := admin.Group("/")
		protected.Use(middleware.AdminPanelMiddleware())
		{
			protected.GET("/", adminController.Dashboard)
			protected.GET("/dashboard", adminController.Dashboard)

			// User management pages
			protected.GET("/users", adminController.UsersPage)
			protected.GET("/users/:id", adminController.UserDetailPage)
			protected.GET("/users/:id/edit", adminController.EditUserPage)

			// File management pages
			protected.GET("/files", adminController.FilesPage)
			protected.GET("/files/:id", adminController.FileDetailPage)

			// Plan management pages
			protected.GET("/plans", adminController.PlansPage)
			protected.GET("/plans/create", adminController.CreatePlanPage)
			protected.GET("/plans/:id/edit", adminController.EditPlanPage)

			// Settings pages
			protected.GET("/settings", adminController.SettingsPage)
			protected.GET("/settings/general", adminController.GeneralSettingsPage)
			protected.GET("/settings/storage", adminController.StorageSettingsPage)
			protected.GET("/settings/pricing", adminController.PricingSettingsPage)

			// Analytics pages
			protected.GET("/analytics", adminController.AnalyticsPage)
			protected.GET("/reports", adminController.ReportsPage)
		}
	}
}
