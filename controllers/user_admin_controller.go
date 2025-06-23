package controllers

import (
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserAdminController struct {
	userService  *services.UserService
	adminService *services.AdminService
}

func NewUserAdminController() *UserAdminController {
	return &UserAdminController{
		userService:  services.NewUserService(),
		adminService: services.NewAdminService(),
	}
}

// GetUsers returns list of users for admin
func (uac *UserAdminController) GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")
	status := c.Query("status") // active, inactive, all
	planID := c.Query("plan_id")
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	filters := &services.UserFilters{
		Search:    search,
		Status:    status,
		PlanID:    planID,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	users, total, err := uac.userService.GetUsersForAdmin(page, limit, filters)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get users")
		return
	}

	utils.PaginatedResponse(c, "Users retrieved successfully", users, page, limit, total)
}

// GetUser returns a specific user for admin
func (uac *UserAdminController) GetUser(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	user, err := uac.userService.GetUserForAdmin(objID)
	if err != nil {
		utils.NotFoundResponse(c, "User not found")
		return
	}

	utils.SuccessResponse(c, "User retrieved successfully", user)
}

// CreateUser creates a new user (admin only)
func (uac *UserAdminController) CreateUser(c *gin.Context) {
	var req struct {
		Username   string `json:"username" validate:"required,min=3,max=50"`
		Email      string `json:"email" validate:"required,email"`
		Password   string `json:"password" validate:"required,min=6"`
		FirstName  string `json:"first_name" validate:"required"`
		LastName   string `json:"last_name" validate:"required"`
		PlanID     string `json:"plan_id"`
		IsActive   bool   `json:"is_active"`
		IsVerified bool   `json:"is_verified"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	user, err := uac.userService.CreateUserByAdmin(&req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create user")
		return
	}

	utils.CreatedResponse(c, "User created successfully", user)
}

// UpdateUser updates user information (admin only)
func (uac *UserAdminController) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	updatedUser, err := uac.userService.UpdateUserByAdmin(objID, updates)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update user")
		return
	}

	utils.SuccessResponse(c, "User updated successfully", updatedUser)
}

// DeleteUser deletes a user (admin only)
func (uac *UserAdminController) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	err := uac.userService.DeleteUserByAdmin(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete user")
		return
	}

	utils.SuccessResponse(c, "User deleted successfully", nil)
}

// SuspendUser suspends a user account
func (uac *UserAdminController) SuspendUser(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	err := uac.userService.SuspendUser(objID, req.Reason)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to suspend user")
		return
	}

	utils.SuccessResponse(c, "User suspended successfully", nil)
}

// UnsuspendUser reactivates a suspended user account
func (uac *UserAdminController) UnsuspendUser(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	err := uac.userService.UnsuspendUser(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to unsuspend user")
		return
	}

	utils.SuccessResponse(c, "User unsuspended successfully", nil)
}

// VerifyUser manually verifies a user account
func (uac *UserAdminController) VerifyUser(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	err := uac.userService.VerifyUserByAdmin(objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to verify user")
		return
	}

	utils.SuccessResponse(c, "User verified successfully", nil)
}

// ResetUserPassword resets user password
func (uac *UserAdminController) ResetUserPassword(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	var req struct {
		NewPassword string `json:"new_password" validate:"required,min=6"`
		SendEmail   bool   `json:"send_email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	objID, _ := utils.StringToObjectID(userID)
	err := uac.userService.ResetUserPasswordByAdmin(objID, req.NewPassword, req.SendEmail)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to reset user password")
		return
	}

	utils.SuccessResponse(c, "User password reset successfully", nil)
}

// GetUserFiles returns files for a specific user
func (uac *UserAdminController) GetUserFiles(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	objID, _ := utils.StringToObjectID(userID)
	files, total, err := uac.userService.GetUserFilesForAdmin(objID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user files")
		return
	}

	utils.PaginatedResponse(c, "User files retrieved successfully", files, page, limit, total)
}

// GetUserActivity returns activity log for a specific user
func (uac *UserAdminController) GetUserActivity(c *gin.Context) {
	userID := c.Param("id")
	if !utils.IsValidObjectID(userID) {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	objID, _ := utils.StringToObjectID(userID)
	activities, total, err := uac.userService.GetUserActivityForAdmin(objID, days, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user activity")
		return
	}

	utils.PaginatedResponse(c, "User activity retrieved successfully", activities, page, limit, total)
}
