package routes

import (
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Global middleware
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.LoggingMiddleware())
	r.Use(gin.Recovery())

	// API v1 routes
	v1 := r.Group("/api/v1")
	v1.Use(middleware.RateLimitMiddleware())
	{
		// Public routes
		AuthRoutes(v1)

		// Protected routes
		UserRoutes(v1)
		FileRoutes(v1)
		FolderRoutes(v1)
		PlanRoutes(v1)
		StorageRoutes(v1)
	}

	// Admin routes
	admin := r.Group("/admin")
	admin.Use(middleware.AdminMiddleware())
	{
		AdminRoutes(admin)
	}

	// // Static files and uploads
	// r.Static("/uploads", "./uploads")
	// r.Static("/static", "./admin/static")

	// // Admin panel HTML routes
	// r.LoadHTMLGlob("admin/templates/**/*")
	AdminPanelRoutes(r)
}
