package utils

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validations
	validate.RegisterValidation("file_extension", validateFileExtension)
	validate.RegisterValidation("file_size", validateFileSize)
	validate.RegisterValidation("storage_provider", validateStorageProvider)
	validate.RegisterValidation("plan_type", validatePlanType)
	validate.RegisterValidation("strong_password", validateStrongPassword)
	validate.RegisterValidation("username", validateUsername)
	validate.RegisterValidation("folder_name", validateFolderName)

	// Register custom tag name function
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// ValidateStruct validates a struct using validator tags
func ValidateStruct(s interface{}) error {
	err := validate.Struct(s)
	if err != nil {
		return formatValidationErrors(err)
	}
	return nil
}

// ValidateVar validates a single variable
func ValidateVar(field interface{}, tag string) error {
	return validate.Var(field, tag)
}

// formatValidationErrors formats validation errors for better readability
func formatValidationErrors(err error) error {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		var messages []string
		for _, e := range validationErrors {
			message := getValidationMessage(e)
			messages = append(messages, message)
		}
		return errors.New(strings.Join(messages, "; "))
	}
	return err
}

// getValidationMessage returns a user-friendly validation message
func getValidationMessage(e validator.FieldError) string {
	field := e.Field()

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters long", field, e.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters long", field, e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, e.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, e.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, e.Param())
	case "file_extension":
		return fmt.Sprintf("%s has an invalid file extension", field)
	case "file_size":
		return fmt.Sprintf("%s exceeds maximum file size", field)
	case "storage_provider":
		return fmt.Sprintf("%s must be a valid storage provider", field)
	case "strong_password":
		return fmt.Sprintf("%s must contain at least 8 characters with uppercase, lowercase, number and special character", field)
	case "username":
		return fmt.Sprintf("%s must contain only letters, numbers, and underscores", field)
	case "folder_name":
		return fmt.Sprintf("%s contains invalid characters", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// Custom validation functions
func validateFileExtension(fl validator.FieldLevel) bool {
	ext := fl.Field().String()
	allowedExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", // Images
		".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", // Videos
		".mp3", ".wav", ".flac", ".aac", ".ogg", // Audio
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", // Documents
		".txt", ".csv", ".json", ".xml", ".html", ".css", ".js", // Text
		".zip", ".rar", ".7z", ".tar", ".gz", // Archives
	}

	for _, allowed := range allowedExtensions {
		if strings.ToLower(ext) == allowed {
			return true
		}
	}
	return false
}

func validateFileSize(fl validator.FieldLevel) bool {
	size := fl.Field().Int()
	maxSize := int64(100 * 1024 * 1024) // 100MB default
	return size <= maxSize
}

func validateStorageProvider(fl validator.FieldLevel) bool {
	provider := fl.Field().String()
	allowedProviders := []string{"s3", "wasabi", "r2", "local"}

	for _, allowed := range allowedProviders {
		if provider == allowed {
			return true
		}
	}
	return false
}

func validatePlanType(fl validator.FieldLevel) bool {
	planType := fl.Field().String()
	allowedTypes := []string{"free", "basic", "premium", "enterprise"}

	for _, allowed := range allowedTypes {
		if planType == allowed {
			return true
		}
	}
	return false
}

func validateStrongPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	if len(password) < 8 {
		return false
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	return hasUpper && hasLower && hasNumber && hasSpecial
}

func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	return matched
}

func validateFolderName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	// Disallow special characters that might cause issues
	matched, _ := regexp.MatchString(`^[^<>:"/\\|?*]+$`, name)
	return matched
}
