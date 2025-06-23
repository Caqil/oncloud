package middleware

import (
	"context"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuthMiddleware validates JWT tokens for user authentication
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.UnauthorizedResponse(c, "Authorization header required")
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			utils.UnauthorizedResponse(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		token := tokenParts[1]
		claims, err := utils.ValidateToken(token)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Get user from database
		user, err := getUserByID(claims.UserID)
		if err != nil {
			utils.UnauthorizedResponse(c, "User not found")
			c.Abort()
			return
		}

		// Check if user is active
		if !user.IsActive {
			utils.UnauthorizedResponse(c, "Account is deactivated")
			c.Abort()
			return
		}

		// Set user in context
		utils.SetUserInContext(c, user)
		c.Set("token_claims", claims)

		c.Next()
	}
}

// OptionalAuthMiddleware provides optional authentication (doesn't abort if no token)
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		token := tokenParts[1]
		claims, err := utils.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		user, err := getUserByID(claims.UserID)
		if err != nil || !user.IsActive {
			c.Next()
			return
		}

		utils.SetUserInContext(c, user)
		c.Set("token_claims", claims)
		c.Next()
	}
}

// AdminMiddleware validates admin JWT tokens
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.UnauthorizedResponse(c, "Authorization header required")
			c.Abort()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			utils.UnauthorizedResponse(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		token := tokenParts[1]
		claims, err := utils.ValidateAdminToken(token)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid or expired admin token")
			c.Abort()
			return
		}

		// Get admin from database
		admin, err := getAdminByID(claims.AdminID)
		if err != nil {
			utils.UnauthorizedResponse(c, "Admin not found")
			c.Abort()
			return
		}

		if !admin.IsActive {
			utils.UnauthorizedResponse(c, "Admin account is deactivated")
			c.Abort()
			return
		}

		utils.SetAdminInContext(c, admin)
		c.Set("admin_claims", claims)

		c.Next()
	}
}

// AdminAuthMiddleware for API routes (different from panel middleware)
func AdminAuthMiddleware() gin.HandlerFunc {
	return AdminMiddleware()
}

// RequirePermission checks if admin has specific permission
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		admin, exists := utils.GetAdminFromContext(c)
		if !exists {
			utils.ForbiddenResponse(c, "Admin context not found")
			c.Abort()
			return
		}

		// Super admin has all permissions
		if admin.Role == "super_admin" {
			c.Next()
			return
		}

		// Check if admin has required permission
		if !utils.SliceContains(admin.Permissions, permission) {
			utils.ForbiddenResponse(c, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminPanelMiddleware for HTML admin panel authentication
func AdminPanelMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for session cookie
		sessionCookie, err := c.Cookie("admin_session")
		if err != nil {
			c.Redirect(302, "/admin/login")
			c.Abort()
			return
		}

		// Validate session token
		claims, err := utils.ValidateAdminToken(sessionCookie)
		if err != nil {
			c.SetCookie("admin_session", "", -1, "/admin", "", false, true)
			c.Redirect(302, "/admin/login")
			c.Abort()
			return
		}

		// Get admin from database
		admin, err := getAdminByID(claims.AdminID)
		if err != nil || !admin.IsActive {
			c.SetCookie("admin_session", "", -1, "/admin", "", false, true)
			c.Redirect(302, "/admin/login")
			c.Abort()
			return
		}

		utils.SetAdminInContext(c, admin)
		c.Next()
	}
}

// PlanLimitMiddleware checks user plan limits
func PlanLimitMiddleware(limitType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := utils.GetUserFromContext(c)
		if !exists {
			utils.UnauthorizedResponse(c, "User context not found")
			c.Abort()
			return
		}

		// Get user's plan
		plan, err := getPlanByID(user.PlanID)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to get user plan")
			c.Abort()
			return
		}

		// Check different types of limits
		switch limitType {
		case "storage":
			if user.StorageUsed >= plan.StorageLimit {
				utils.ForbiddenResponse(c, "Storage limit exceeded")
				c.Abort()
				return
			}
		case "files":
			if user.FilesCount >= plan.FilesLimit {
				utils.ForbiddenResponse(c, "File limit exceeded")
				c.Abort()
				return
			}
		case "folders":
			if user.FoldersCount >= plan.FoldersLimit {
				utils.ForbiddenResponse(c, "Folder limit exceeded")
				c.Abort()
				return
			}
		}

		c.Set("user_plan", plan)
		c.Next()
	}
}

// Helper functions for database operations
func getUserByID(userID primitive.ObjectID) (*models.User, error) {
	collection := database.GetCollection("users")
	var user models.User

	err := collection.FindOne(context.Background(), bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func getAdminByID(adminID primitive.ObjectID) (*models.Admin, error) {
	collection := database.GetCollection("admins")
	var admin models.Admin

	err := collection.FindOne(context.Background(), bson.M{"_id": adminID}).Decode(&admin)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func getPlanByID(planID primitive.ObjectID) (*models.Plan, error) {
	collection := database.GetCollection("plans")
	var plan models.Plan

	err := collection.FindOne(context.Background(), bson.M{"_id": planID}).Decode(&plan)
	if err != nil {
		return nil, err
	}

	return &plan, nil
}
