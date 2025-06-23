package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware configures CORS for the application
func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:8080",
			"https://yourdomain.com",
		},
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Length",
			"Content-Type",
			"Authorization",
			"Accept",
			"Accept-Encoding",
			"Accept-Language",
			"Connection",
			"Host",
			"Referer",
			"User-Agent",
			"X-Requested-With",
			"X-CSRF-Token",
			"X-Upload-Content-Type",
			"X-Upload-Content-Length",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"Content-Disposition",
			"X-Total-Count",
			"X-Page-Count",
		},
		AllowCredentials: true,
		AllowWildcard:    false,
		MaxAge:           12 * time.Hour,
	}

	// Allow all origins in development
	if gin.Mode() == gin.DebugMode {
		config.AllowAllOrigins = true
		config.AllowWildcard = true
	}

	return cors.New(config)
}
