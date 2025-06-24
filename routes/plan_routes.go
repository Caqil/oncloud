package routes

import (
	"oncloud/controllers"
	"oncloud/middleware"

	"github.com/gin-gonic/gin"
)

func PlanRoutes(r *gin.RouterGroup) {
	planController := controllers.NewPlanController()

	plans := r.Group("/plans")
	{
		// Public plan routes
		plans.GET("/", planController.GetPlans)
		plans.GET("/:id", planController.GetPlan)
		plans.GET("/compare", planController.ComparePlans)
		plans.GET("/pricing", planController.GetPricing)

		// Protected plan routes
		protected := plans.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			// User subscription management
			protected.GET("/my-plan", planController.GetUserPlan)
			protected.POST("/subscribe", planController.Subscribe)
			protected.POST("/upgrade", planController.UpgradePlan)
			protected.POST("/downgrade", planController.DowngradePlan)
			protected.POST("/cancel", planController.CancelSubscription)
			protected.POST("/renew", planController.RenewSubscription)

			// Payment and billing
			protected.GET("/billing-history", planController.GetBillingHistory)
			protected.GET("/invoices", planController.GetInvoices)
			protected.GET("/invoices/:id/download", planController.DownloadInvoice)
			protected.POST("/payment-methods", planController.AddPaymentMethod)
			protected.GET("/payment-methods", planController.GetPaymentMethods)
			protected.PUT("/payment-methods/:id", planController.UpdatePaymentMethod)
			protected.DELETE("/payment-methods/:id", planController.DeletePaymentMethod)

			// Usage tracking
			protected.GET("/usage", planController.GetUsage)
			protected.GET("/usage/history", planController.GetUsageHistory)
			protected.GET("/limits", planController.GetLimits)
		}
	}

	// Webhook endpoints for payment processors
	r.POST("/webhooks/stripe", planController.StripeWebhook)
	// r.POST("/webhooks/paypal", planController.PayPalWebhook)
	// r.POST("/webhooks/razorpay", planController.RazorpayWebhook)
}
