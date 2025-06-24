package main

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server Configuration
	Port        string
	Environment string
	Debug       bool

	// Database Configuration
	MongoURI string
	DBName   string

	// JWT Configuration
	JWTSecret        string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration

	// Storage Configuration
	DefaultStorageProvider string
	UploadPath             string
	MaxUploadSize          int64
	AllowedFileTypes       []string

	// Security Configuration
	CORSAllowedOrigins []string
	RateLimitEnabled   bool
	RateLimitRequests  int
	RateLimitWindow    time.Duration

	// Email Configuration
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// Application Configuration
	AppName    string
	AppVersion string
	AppURL     string

	// File Storage Limits
	DefaultPlanStorageLimit int64
	DefaultPlanFilesLimit   int
	DefaultPlanFoldersLimit int

	// Session Configuration
	SessionSecret string
	SessionMaxAge int

	// Admin Configuration
	AdminPanelEnabled bool
	AdminDefaultEmail string
	AdminDefaultPass  string
}

var AppConfig *Config

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := &Config{
		// Server Configuration
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		Debug:       getEnvAsBool("DEBUG", true),

		// Database Configuration
		MongoURI: getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:   getEnv("DB_NAME", "cloudstorage"),

		// JWT Configuration
		JWTSecret:        getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-super-secret-refresh-key-change-in-production"),
		AccessTokenTTL:   getEnvAsDuration("ACCESS_TOKEN_TTL", "24h"),
		RefreshTokenTTL:  getEnvAsDuration("REFRESH_TOKEN_TTL", "168h"), // 7 days

		// Storage Configuration
		DefaultStorageProvider: getEnv("DEFAULT_STORAGE_PROVIDER", "local"),
		UploadPath:             getEnv("UPLOAD_PATH", "./uploads"),
		MaxUploadSize:          getEnvAsInt64("MAX_UPLOAD_SIZE", 104857600), // 100MB
		AllowedFileTypes:       getEnvAsSlice("ALLOWED_FILE_TYPES", []string{}),

		// Security Configuration
		CORSAllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:8080",
		}),
		RateLimitEnabled:  getEnvAsBool("RATE_LIMIT_ENABLED", true),
		RateLimitRequests: getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   getEnvAsDuration("RATE_LIMIT_WINDOW", "1h"),

		// Email Configuration
		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@yourdomain.com"),

		// Application Configuration
		AppName:    getEnv("APP_NAME", "CloudStorage"),
		AppVersion: getEnv("APP_VERSION", "1.0.0"),
		AppURL:     getEnv("APP_URL", "http://localhost:8080"),

		// File Storage Limits
		DefaultPlanStorageLimit: getEnvAsInt64("DEFAULT_PLAN_STORAGE_LIMIT", 1073741824), // 1GB
		DefaultPlanFilesLimit:   getEnvAsInt("DEFAULT_PLAN_FILES_LIMIT", 1000),
		DefaultPlanFoldersLimit: getEnvAsInt("DEFAULT_PLAN_FOLDERS_LIMIT", 100),

		// Session Configuration
		SessionSecret: getEnv("SESSION_SECRET", "your-session-secret-change-in-production"),
		SessionMaxAge: getEnvAsInt("SESSION_MAX_AGE", 86400), // 24 hours

		// Admin Configuration
		AdminPanelEnabled: getEnvAsBool("ADMIN_PANEL_ENABLED", true),
		AdminDefaultEmail: getEnv("ADMIN_DEFAULT_EMAIL", "admin@example.com"),
		AdminDefaultPass:  getEnv("ADMIN_DEFAULT_PASS", "admin123"),
	}

	// Set global config
	AppConfig = config

	// Log configuration in development
	if config.Debug {
		log.Printf("Configuration loaded: Environment=%s, Port=%s, Database=%s",
			config.Environment, config.Port, config.DBName)
	}

	return config
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	if parsed, err := time.ParseDuration(defaultValue); err == nil {
		return parsed
	}
	return 24 * time.Hour // fallback
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing
		// You might want to use a more sophisticated parser
		var result []string
		for _, item := range []string{value} {
			if item != "" {
				result = append(result, item)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// GetServerAddress returns the server address for listening
func (c *Config) GetServerAddress() string {
	return ":" + c.Port
}

// ValidateConfig validates the configuration
func (c *Config) ValidateConfig() error {
	if c.MongoURI == "" {
		log.Fatal("MONGO_URI environment variable is required")
	}

	if c.JWTSecret == "your-super-secret-jwt-key-change-in-production" && c.IsProduction() {
		log.Fatal("JWT_SECRET must be changed in production")
	}

	if c.JWTRefreshSecret == "your-super-secret-refresh-key-change-in-production" && c.IsProduction() {
		log.Fatal("JWT_REFRESH_SECRET must be changed in production")
	}

	if c.SessionSecret == "your-session-secret-change-in-production" && c.IsProduction() {
		log.Fatal("SESSION_SECRET must be changed in production")
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(c.UploadPath, 0755); err != nil {
		log.Printf("Warning: Could not create upload directory %s: %v", c.UploadPath, err)
	}

	return nil
}
