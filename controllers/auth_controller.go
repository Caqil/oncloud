package controllers

import (
	"oncloud/models"
	"oncloud/services"
	"oncloud/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type AuthController struct {
	authService *services.AuthService
	userService *services.UserService
}

func NewAuthController() *AuthController {
	return &AuthController{
		authService: services.NewAuthService(),
		userService: services.NewUserService(),
	}
}

// Register handles user registration
func (ac *AuthController) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Check if registration is allowed
	if !ac.authService.IsRegistrationAllowed() {
		utils.ForbiddenResponse(c, "Registration is currently disabled")
		return
	}

	// Create user
	user, err := ac.authService.Register(&req)
	if err != nil {
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
		return
	}

	// Generate tokens
	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, user.Username, "user", user.PlanID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate tokens")
		return
	}

	utils.CreatedResponse(c, "Registration successful", gin.H{
		"user":   user,
		"tokens": tokens,
	})
}

// Login handles user authentication
func (ac *AuthController) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Authenticate user
	user, err := ac.authService.Login(req.Email, req.Password)
	if err != nil {
		utils.UnauthorizedResponse(c, "Invalid credentials")
		return
	}

	// Check if user is active
	if !user.IsActive {
		utils.UnauthorizedResponse(c, "Account is deactivated")
		return
	}

	// Update last login
	ac.userService.UpdateLastLogin(user.ID)

	// Generate tokens
	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, user.Username, "user", user.PlanID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate tokens")
		return
	}

	utils.SuccessResponse(c, "Login successful", gin.H{
		"user":   user,
		"tokens": tokens,
	})
}

// Logout handles user logout
func (ac *AuthController) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side
	// Here we could implement token blacklisting if needed
	utils.SuccessResponse(c, "Logout successful", nil)
}

// RefreshToken handles token refresh
func (ac *AuthController) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate refresh token
	claims, err := utils.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		utils.UnauthorizedResponse(c, "Invalid refresh token")
		return
	}

	// Get user
	user, err := ac.userService.GetByID(claims.UserID)
	if err != nil || !user.IsActive {
		utils.UnauthorizedResponse(c, "User not found or inactive")
		return
	}

	// Generate new tokens
	tokens, err := utils.GenerateTokenPair(user.ID, user.Email, user.Username, "user", user.PlanID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate tokens")
		return
	}

	utils.SuccessResponse(c, "Token refreshed successfully", tokens)
}

// ForgotPassword handles password reset request
func (ac *AuthController) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := ac.authService.SendPasswordResetEmail(req.Email)
	if err != nil {
		// Don't reveal if email exists for security
		utils.SuccessResponse(c, "If the email exists, a reset link has been sent", nil)
		return
	}

	utils.SuccessResponse(c, "Password reset email sent", nil)
}

// ResetPassword handles password reset
func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := ac.authService.ResetPassword(req.Token, req.NewPassword)
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Password reset successful", nil)
}

// VerifyEmail handles email verification
func (ac *AuthController) VerifyEmail(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		utils.BadRequestResponse(c, "Verification token is required")
		return
	}

	err := ac.authService.VerifyEmail(token)
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Email verified successfully", nil)
}

// ResendVerification resends email verification
func (ac *AuthController) ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := ac.authService.ResendVerificationEmail(req.Email)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Verification email sent", nil)
}

// ChangePassword handles password change for authenticated users
func (ac *AuthController) ChangePassword(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Check if new password matches confirmation
	if req.NewPassword != req.ConfirmPassword {
		utils.BadRequestResponse(c, "New password and confirmation do not match")
		return
	}

	err := ac.authService.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Password changed successfully", nil)
}

// GetProfile returns current user profile
func (ac *AuthController) GetProfile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	profile := &models.UserProfile{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Avatar:    user.Avatar,
	}

	utils.SuccessResponse(c, "Profile retrieved successfully", profile)
}

// UpdateProfile updates user profile
func (ac *AuthController) UpdateProfile(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		FirstName string `json:"first_name" validate:"required"`
		LastName  string `json:"last_name" validate:"required"`
		Phone     string `json:"phone"`
		Country   string `json:"country"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Update user
	updates := bson.M{
		"first_name": req.FirstName,
		"last_name":  req.LastName,
		"phone":      req.Phone,
		"country":    req.Country,
		"updated_at": time.Now(),
	}

	err := ac.userService.UpdateUser(user.ID, updates)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update profile")
		return
	}

	utils.SuccessResponse(c, "Profile updated successfully", nil)
}

// DeleteAccount handles account deletion
func (ac *AuthController) DeleteAccount(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Password string `json:"password" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	err := ac.authService.DeleteAccount(user.ID, req.Password)
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Account deleted successfully", nil)
}

// Google OAuth handlers
func (ac *AuthController) GoogleAuth(c *gin.Context) {
	// Implement Google OAuth redirect
	utils.ErrorResponse(c, http.StatusNotImplemented, "Google OAuth not implemented", nil)
}

func (ac *AuthController) GoogleCallback(c *gin.Context) {
	// Implement Google OAuth callback
	utils.ErrorResponse(c, http.StatusNotImplemented, "Google OAuth callback not implemented", nil)
}

// Facebook OAuth handlers
func (ac *AuthController) FacebookAuth(c *gin.Context) {
	// Implement Facebook OAuth redirect
	utils.ErrorResponse(c, http.StatusNotImplemented, "Facebook OAuth not implemented", nil)
}

func (ac *AuthController) FacebookCallback(c *gin.Context) {
	// Implement Facebook OAuth callback
	utils.ErrorResponse(c, http.StatusNotImplemented, "Facebook OAuth callback not implemented", nil)
}
