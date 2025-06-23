package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func AuthRoutes(r *gin.RouterGroup) {
	authController := controllers.NewAuthController()

	auth := r.Group("/auth")
	{
		// Public authentication routes
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
		auth.POST("/forgot-password", authController.ForgotPassword)
		auth.POST("/reset-password", authController.ResetPassword)
		auth.GET("/verify-email/:token", authController.VerifyEmail)
		auth.POST("/resend-verification", authController.ResendVerification)

		// Social authentication
		auth.GET("/google", authController.GoogleAuth)
		auth.GET("/google/callback", authController.GoogleCallback)
		auth.GET("/facebook", authController.FacebookAuth)
		auth.GET("/facebook/callback", authController.FacebookCallback)

		// Protected authentication routes
		protected := auth.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.POST("/logout", authController.Logout)
			protected.POST("/refresh", authController.RefreshToken)
			protected.POST("/change-password", authController.ChangePassword)
			protected.GET("/me", authController.GetProfile)
			protected.PUT("/profile", authController.UpdateProfile)
			protected.DELETE("/account", authController.DeleteAccount)
		}
	}
}
