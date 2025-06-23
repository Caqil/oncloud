package utils

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// StringToObjectID converts string to MongoDB ObjectID
func StringToObjectID(s string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(s)
}

// ObjectIDToString converts MongoDB ObjectID to string
func ObjectIDToString(id primitive.ObjectID) string {
	return id.Hex()
}

// IsValidObjectID checks if string is valid MongoDB ObjectID
func IsValidObjectID(s string) bool {
	_, err := primitive.ObjectIDFromHex(s)
	return err == nil
}

// generateRandomString generates random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}

	return string(result)
}

// FormatFileSize formats file size in human-readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ParseFileSize parses human-readable file size to bytes
func ParseFileSize(size string) (int64, error) {
	size = strings.TrimSpace(strings.ToUpper(size))

	if size == "" {
		return 0, fmt.Errorf("empty size")
	}

	// Extract number and unit
	var numStr string
	var unit string

	for i, char := range size {
		if char >= '0' && char <= '9' || char == '.' {
			numStr += string(char)
		} else {
			unit = size[i:]
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("invalid size format")
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	multiplier := int64(1)
	switch unit {
	case "", "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(num * float64(multiplier)), nil
}

// CalculateStorageUsage calculates storage usage percentage
func CalculateStorageUsage(used, total int64) float64 {
	if total == 0 {
		return 0
	}
	return math.Round((float64(used)/float64(total))*100*100) / 100
}

// TimeAgo returns human-readable time difference
func TimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / (24 * 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// SliceContains checks if slice contains element
func SliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SliceUnique removes duplicates from string slice
func SliceUnique(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// ToJSON converts interface to JSON string
func ToJSON(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

// FromJSON parses JSON string to interface
func FromJSON(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// GenerateSlug generates URL-friendly slug from string
func GenerateSlug(text string) string {
	// Convert to lowercase
	slug := strings.ToLower(text)

	// Replace spaces and special characters with hyphens
	replacer := strings.NewReplacer(
		" ", "-",
		"_", "-",
		".", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		";", "-",
		"@", "-",
		"#", "-",
		"$", "-",
		"%", "-",
		"^", "-",
		"&", "-",
		"*", "-",
		"(", "-",
		")", "-",
		"+", "-",
		"=", "-",
		"?", "-",
		"!", "-",
		"~", "-",
		"`", "-",
		"'", "-",
		"\"", "-",
		"[", "-",
		"]", "-",
		"{", "-",
		"}", "-",
		"|", "-",
		"<", "-",
		">", "-",
		",", "-",
	)

	slug = replacer.Replace(slug)

	// Remove multiple consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

// TruncateString truncates string to specified length
func TruncateString(str string, length int) string {
	if len(str) <= length {
		return str
	}
	return str[:length] + "..."
}

// IsImageFile checks if file is an image based on extension
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg"}
	return SliceContains(imageExts, ext)
}

// IsVideoFile checks if file is a video based on extension
func IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	videoExts := []string{".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v"}
	return SliceContains(videoExts, ext)
}

// IsAudioFile checks if file is audio based on extension
func IsAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	audioExts := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a"}
	return SliceContains(audioExts, ext)
}

// GenerateRandomString generates a random string of specified length
func GenerateRandomString(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		result[i] = letters[num.Int64()]
	}

	return string(result)
}
func CalculatePercentage(used, limit int64) float64 {
	if limit == 0 {
		return 0
	}
	return (float64(used) / float64(limit)) * 100
}

// IsValidEmail validates email format
func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// IsValidURL validates URL format
func IsValidURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsedURL.Scheme != "" && parsedURL.Host != ""
}

// ToFloat64 converts various numeric types to float64
func ToFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

// MatchRegex checks if a string matches a regex pattern
func MatchRegex(pattern, text string) (bool, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return regex.MatchString(text), nil
}

// GenerateTOTPSecret generates a base32 encoded secret for TOTP
func GenerateTOTPSecret() string {
	secretBytes := make([]byte, 20) // 160 bits
	_, err := rand.Read(secretBytes)
	if err != nil {
		// Fallback to a simple generation method
		secretBytes = []byte("DEFAULTSECRET1234567")
	}
	return base32.StdEncoding.EncodeToString(secretBytes)
}
