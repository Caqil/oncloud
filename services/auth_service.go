// services/auth_service.go
package services

import (
	"context"
	"errors"
	"fmt"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthService struct {
	*BaseService
}

func NewAuthService() *AuthService {
	return &AuthService{
		BaseService: NewBaseService(),
	}
}

// Register creates a new user account
func (as *AuthService) Register(req *models.RegisterRequest) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if user already exists
	var existingUser models.User
	err := as.collections.Users().FindOne(ctx, bson.M{
		"$or": []bson.M{
			{"email": req.Email},
			{"username": req.Username},
		},
	}).Decode(&existingUser)

	if err == nil {
		if existingUser.Email == req.Email {
			return nil, errors.New("user with this email already exists")
		}
		return nil, errors.New("username already taken")
	} else if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("database error: %v", err)
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	// Get default plan
	defaultPlan, err := as.getDefaultPlan()
	if err != nil {
		return nil, fmt.Errorf("failed to get default plan: %v", err)
	}

	// Create user
	user := &models.User{
		ID:            primitive.NewObjectID(),
		Username:      req.Username,
		Email:         req.Email,
		Password:      hashedPassword,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PlanID:        defaultPlan.ID,
		StorageUsed:   0,
		BandwidthUsed: 0,
		FilesCount:    0,
		FoldersCount:  0,
		IsActive:      true,
		IsVerified:    false,
		IsPremium:     !defaultPlan.IsFree,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Insert user
	_, err = as.collections.Users().InsertOne(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	// Send verification email if required
	if as.isEmailVerificationRequired() {
		err = as.sendVerificationEmail(user)
		if err != nil {
			// Log error but don't fail registration
			fmt.Printf("Failed to send verification email: %v\n", err)
		}
	} else {
		// Mark as verified if email verification is disabled
		user.IsVerified = true
		user.EmailVerifiedAt = &user.CreatedAt
		as.collections.Users().UpdateOne(ctx,
			bson.M{"_id": user.ID},
			bson.M{"$set": bson.M{
				"is_verified":       true,
				"email_verified_at": user.EmailVerifiedAt,
			}},
		)
	}

	// Clear password before returning
	user.Password = ""
	return user, nil
}

// Login authenticates a user
func (as *AuthService) Login(email, password string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user by email
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid credentials")
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	// Check password
	if !utils.CheckPasswordHash(password, user.Password) {
		return nil, errors.New("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("account is deactivated")
	}

	// Update last login
	as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{"last_login_at": time.Now()}},
	)

	// Clear password before returning
	user.Password = ""
	return &user, nil
}

// SendPasswordResetEmail sends password reset email
func (as *AuthService) SendPasswordResetEmail(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user by email
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Don't reveal if email exists for security
			return nil
		}
		return fmt.Errorf("database error: %v", err)
	}

	// Generate reset token
	resetToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %v", err)
	}

	// Store reset token with expiration
	resetExpiry := time.Now().Add(24 * time.Hour)
	_, err = as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{
			"reset_token":            resetToken,
			"reset_token_expires_at": resetExpiry,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to store reset token: %v", err)
	}

	// Send email (implement email service)
	return as.sendPasswordResetEmailNotification(&user, resetToken)
}

// ResetPassword resets user password using token
func (as *AuthService) ResetPassword(token, newPassword string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user by reset token
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{
		"reset_token":            token,
		"reset_token_expires_at": bson.M{"$gt": time.Now()},
	}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("invalid or expired reset token")
		}
		return fmt.Errorf("database error: %v", err)
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Update password and clear reset token
	_, err = as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{
			"$set": bson.M{
				"password":   hashedPassword,
				"updated_at": time.Now(),
			},
			"$unset": bson.M{
				"reset_token":            "",
				"reset_token_expires_at": "",
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	return nil
}

// VerifyEmail verifies user email using token
func (as *AuthService) VerifyEmail(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user by verification token
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{
		"verification_token": token,
		"is_verified":        false,
	}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("invalid verification token")
		}
		return fmt.Errorf("database error: %v", err)
	}

	// Mark as verified
	verifiedAt := time.Now()
	_, err = as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{
			"$set": bson.M{
				"is_verified":       true,
				"email_verified_at": verifiedAt,
				"updated_at":        verifiedAt,
			},
			"$unset": bson.M{
				"verification_token": "",
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to verify email: %v", err)
	}

	return nil
}

// ResendVerificationEmail resends verification email
func (as *AuthService) ResendVerificationEmail(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find user by email
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{
		"email":       email,
		"is_verified": false,
	}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user not found or already verified")
		}
		return fmt.Errorf("database error: %v", err)
	}

	// Send verification email
	return as.sendVerificationEmail(&user)
}

// ChangePassword changes user password
func (as *AuthService) ChangePassword(userID primitive.ObjectID, currentPassword, newPassword string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	// Verify current password
	if !utils.CheckPasswordHash(currentPassword, user.Password) {
		return errors.New("current password is incorrect")
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Update password
	_, err = as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	return nil
}

// DeleteAccount deletes user account
func (as *AuthService) DeleteAccount(userID primitive.ObjectID, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user
	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	// Verify password
	if !utils.CheckPasswordHash(password, user.Password) {
		return errors.New("password is incorrect")
	}

	// Mark account as deleted (soft delete)
	_, err = as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"is_active":  false,
			"deleted_at": time.Now(),
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to delete account: %v", err)
	}

	// Schedule data cleanup (implement async cleanup)
	go as.scheduleAccountCleanup(userID)

	return nil
}

// IsRegistrationAllowed checks if registration is enabled
func (as *AuthService) IsRegistrationAllowed() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var setting models.AdminSettings
	err := as.collections.Settings().FindOne(ctx, bson.M{"key": "allow_registration"}).Decode(&setting)
	if err != nil {
		// Default to true if setting not found
		return true
	}

	if allowed, ok := setting.Value.(bool); ok {
		return allowed
	}
	return true
}

// Helper methods

func (as *AuthService) getDefaultPlan() (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var plan models.Plan
	err := as.collections.Plans().FindOne(ctx, bson.M{
		"is_default": true,
		"is_active":  true,
	}).Decode(&plan)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// If no default plan, get the first free plan
			err = as.collections.Plans().FindOne(ctx, bson.M{
				"is_free":   true,
				"is_active": true,
			}).Decode(&plan)
			if err != nil {
				return nil, errors.New("no default plan available")
			}
		} else {
			return nil, err
		}
	}
	return &plan, nil
}

func (as *AuthService) isEmailVerificationRequired() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var setting models.AdminSettings
	err := as.collections.Settings().FindOne(ctx, bson.M{"key": "email_verification"}).Decode(&setting)
	if err != nil {
		// Default to true if setting not found
		return true
	}

	if required, ok := setting.Value.(bool); ok {
		return required
	}
	return true
}

func (as *AuthService) sendVerificationEmail(user *models.User) error {
	// Generate verification token
	verificationToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %v", err)
	}

	// Store verification token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = as.collections.Users().UpdateOne(ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{"verification_token": verificationToken}},
	)
	if err != nil {
		return fmt.Errorf("failed to store verification token: %v", err)
	}

	// Send email (implement email service)
	return as.sendEmailNotification(user.Email, "verify", map[string]string{
		"name":  user.FirstName + " " + user.LastName,
		"token": verificationToken,
	})
}

func (as *AuthService) sendPasswordResetEmailNotification(user *models.User, token string) error {
	// Send email (implement email service)
	return as.sendEmailNotification(user.Email, "reset", map[string]string{
		"name":  user.FirstName + " " + user.LastName,
		"token": token,
	})
}

func (as *AuthService) sendEmailNotification(email, template string, data map[string]string) error {
	// Implement email service integration
	// This would integrate with services like SendGrid, AWS SES, etc.
	fmt.Printf("Sending %s email to %s with data: %v\n", template, email, data)
	return nil
}

func (as *AuthService) scheduleAccountCleanup(userID primitive.ObjectID) {
	// Implement async account cleanup
	// This would delete user files, folders, and eventually the user record
	fmt.Printf("Scheduling account cleanup for user: %s\n", userID.Hex())
}

// AdminLogin handles admin authentication
func (as *AuthService) AdminLogin(email, password string) (*models.Admin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminCollection := database.GetCollection("admins")

	// Find admin by email
	var admin models.Admin
	err := adminCollection.FindOne(ctx, bson.M{"email": email}).Decode(&admin)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid credentials")
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	// Check password
	if !utils.CheckPasswordHash(password, admin.Password) {
		return nil, errors.New("invalid credentials")
	}

	// Check if admin is active
	if !admin.IsActive {
		return nil, errors.New("admin account is deactivated")
	}

	// Update last login
	adminCollection.UpdateOne(ctx,
		bson.M{"_id": admin.ID},
		bson.M{"$set": bson.M{"last_login_at": time.Now()}},
	)

	// Clear password before returning
	admin.Password = ""
	return &admin, nil
}

// ValidateToken validates JWT token and returns user
func (as *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	claims, err := utils.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err = as.collections.Users().FindOne(ctx, bson.M{"_id": claims.UserID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	if !user.IsActive {
		return nil, errors.New("user account is deactivated")
	}

	user.Password = ""
	return &user, nil
}

// ValidateAdminToken validates admin JWT token and returns admin
func (as *AuthService) ValidateAdminToken(tokenString string) (*models.Admin, error) {
	claims, err := utils.ValidateAdminToken(tokenString)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	adminCollection := database.GetCollection("admins")
	var admin models.Admin
	err = adminCollection.FindOne(ctx, bson.M{"_id": claims.AdminID}).Decode(&admin)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %v", err)
	}

	if !admin.IsActive {
		return nil, errors.New("admin account is deactivated")
	}

	admin.Password = ""
	return &admin, nil
}

// GetUserByID retrieves user by ID
func (as *AuthService) GetUserByID(userID primitive.ObjectID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := as.collections.Users().FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	user.Password = ""
	return &user, nil
}

// GetAdminByID retrieves admin by ID
func (as *AuthService) GetAdminByID(adminID primitive.ObjectID) (*models.Admin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	adminCollection := database.GetCollection("admins")
	var admin models.Admin
	err := adminCollection.FindOne(ctx, bson.M{"_id": adminID}).Decode(&admin)
	if err != nil {
		return nil, fmt.Errorf("admin not found: %v", err)
	}

	admin.Password = ""
	return &admin, nil
}
