package utils

import (
	"math"
	"net/http"
	"oncloud/models"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SuccessResponse sends a successful API response
func SuccessResponse(c *gin.Context, message string, data interface{}) {
	response := models.APIResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusOK, response)
}

// CreatedResponse sends a 201 created response
func CreatedResponse(c *gin.Context, message string, data interface{}) {
	response := models.APIResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusCreated, response)
}

// ErrorResponse sends an error API response
func ErrorResponse(c *gin.Context, statusCode int, message string, details map[string]interface{}) {
	response := models.APIResponse{
		Success: false,
		Message: message,
		Error: &models.APIError{
			Code:    http.StatusText(statusCode),
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
	}
	c.JSON(statusCode, response)
}

// ValidationErrorResponse sends a validation error response
func ValidationErrorResponse(c *gin.Context, err error) {
	ErrorResponse(c, http.StatusUnprocessableEntity, "Validation failed", map[string]interface{}{
		"validation_errors": err.Error(),
	})
}

// UnauthorizedResponse sends an unauthorized response
func UnauthorizedResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Unauthorized access"
	}
	ErrorResponse(c, http.StatusUnauthorized, message, nil)
}

// ForbiddenResponse sends a forbidden response
func ForbiddenResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Access forbidden"
	}
	ErrorResponse(c, http.StatusForbidden, message, nil)
}

// NotFoundResponse sends a not found response
func NotFoundResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Resource not found"
	}
	ErrorResponse(c, http.StatusNotFound, message, nil)
}

// ConflictResponse sends a conflict response
func ConflictResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusConflict, message, nil)
}

// InternalServerErrorResponse sends an internal server error response
func InternalServerErrorResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Internal server error"
	}
	ErrorResponse(c, http.StatusInternalServerError, message, nil)
}

// BadRequestResponse sends a bad request response
func BadRequestResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusBadRequest, message, nil)
}

// TooManyRequestsResponse sends a rate limit exceeded response
func TooManyRequestsResponse(c *gin.Context, message string) {
	if message == "" {
		message = "Rate limit exceeded"
	}
	ErrorResponse(c, http.StatusTooManyRequests, message, nil)
}

// PaginatedResponse sends a paginated response
func PaginatedResponse(c *gin.Context, message string, data interface{}, page, limit, total int) {
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	response := models.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta: &models.Meta{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusOK, response)
}

// FileUploadResponse sends a file upload response
func FileUploadResponse(c *gin.Context, message string, file *models.File, uploadURL string) {
	response := models.UploadResponse{
		File:      file,
		UploadURL: uploadURL,
	}
	SuccessResponse(c, message, response)
}

// AbortWithError aborts request with error response
func AbortWithError(c *gin.Context, statusCode int, message string) {
	ErrorResponse(c, statusCode, message, nil)
	c.Abort()
}

// AbortWithValidationError aborts request with validation error
func AbortWithValidationError(c *gin.Context, err error) {
	ValidationErrorResponse(c, err)
	c.Abort()
}

// GetUserFromContext gets user from gin context
func GetUserFromContext(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	userModel, ok := user.(*models.User)
	return userModel, ok
}

// GetUserIDFromContext gets user ID from gin context
func GetUserIDFromContext(c *gin.Context) (primitive.ObjectID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return primitive.NilObjectID, false
	}
	id, ok := userID.(primitive.ObjectID)
	return id, ok
}

// GetAdminFromContext gets admin from gin context
func GetAdminFromContext(c *gin.Context) (*models.Admin, bool) {
	admin, exists := c.Get("admin")
	if !exists {
		return nil, false
	}
	adminModel, ok := admin.(*models.Admin)
	return adminModel, ok
}

// SetUserInContext sets user in gin context
func SetUserInContext(c *gin.Context, user *models.User) {
	c.Set("user", user)
	c.Set("user_id", user.ID)
}

// SetAdminInContext sets admin in gin context
func SetAdminInContext(c *gin.Context, admin *models.Admin) {
	c.Set("admin", admin)
	c.Set("admin_id", admin.ID)
}
