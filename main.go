package main

import (
	"context"
	"log"
	"net/http"
	"oncloud/config"
	"oncloud/database"
	"oncloud/routes"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize application
	app, err := NewApplication()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start the application
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}

// Application represents the main application structure
type Application struct {
	config         *config.Config
	server         *http.Server
	dbManager      *config.DatabaseManager
	storageManager *config.StorageManager
	router         *gin.Engine
}

// NewApplication creates and initializes a new application instance
func NewApplication() (*Application, error) {
	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.ValidateConfig(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize database manager
	dbManager := config.NewDatabaseManager(cfg)

	// Initialize router
	router := setupRouter(cfg)

	// Create application instance (storage manager will be initialized later after DB connection)
	app := &Application{
		config:         cfg,
		dbManager:      dbManager,
		storageManager: nil, // Will be initialized after database connection
		router:         router,
		server: &http.Server{
			Addr:         cfg.GetServerAddress(),
			Handler:      router,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	return app, nil
}

// Start initializes all components and starts the HTTP server
func (app *Application) Start() error {
	log.Printf("Starting %s v%s in %s mode",
		app.config.AppName,
		app.config.AppVersion,
		app.config.Environment)

	// Log startup info
	app.logStartupInfo()

	// Initialize database
	if err := app.initializeDatabase(); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}

	// Initialize storage (after database is ready)
	if err := app.initializeStorage(); err != nil {
		log.Fatalf("Storage initialization failed: %v", err)
	}

	// Setup routes
	app.setupRoutes()

	// Start background jobs
	app.startBackgroundJobs()

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s", app.server.Addr)
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	app.waitForShutdown()

	return nil
}

// initializeDatabase sets up database connection and runs migrations
func (app *Application) initializeDatabase() error {
	log.Println("Initializing database...")

	// Connect to database first
	if err := app.dbManager.Initialize(); err != nil {
		return err
	}

	// Setup database (this will set global database connection, create indexes and run migrations)
	if err := app.dbManager.SetupDatabase(); err != nil {
		return err
	}

	log.Println("Database initialization completed successfully")
	return nil
}

// initializeStorage sets up storage providers and file handling
func (app *Application) initializeStorage() error {
	log.Println("Initializing storage subsystem...")

	// Now create the storage manager after database is connected
	app.storageManager = config.NewStorageManager(app.config)

	if err := app.storageManager.Initialize(); err != nil {
		return err
	}

	log.Println("Storage subsystem initialization completed successfully")
	return nil
}

// setupRoutes configures all application routes and middleware
func (app *Application) setupRoutes() {
	routes.SetupRoutes(app.router)
	log.Println("Routes configured successfully")
}

func setupRouter(config *config.Config) *gin.Engine {
	router := gin.New()

	// Trust proxies for proper client IP detection
	router.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// Global middleware (order matters)
	router.Use(gin.Recovery())

	// Health check endpoint (before other middleware)
	router.GET("/health", healthCheckHandler())
	router.GET("/version", versionHandler())

	// Configure template loading if admin panel is enabled
	if config.AdminPanelEnabled {
		router.LoadHTMLGlob("admin/templates/**/*")
		router.Static("/admin/static", "./admin/static")
	}

	// Static file serving - ALL static routes handled here
	router.Static("/uploads", config.UploadPath)
	router.Static("/public", "./public")

	return router
}

// waitForShutdown waits for interrupt signal and gracefully shuts down the server
func (app *Application) waitForShutdown() {
	// Create channel to receive OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	<-quit
	log.Println("Shutdown signal received...")

	// Gracefully shutdown with timeout
	app.shutdown()
}

// shutdown gracefully shuts down the application
func (app *Application) shutdown() {
	log.Println("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := app.server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if err := app.dbManager.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	// Additional cleanup can be added here
	log.Println("Server shutdown complete")
}

// Health check handler for monitoring
func healthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Basic health check
		health := gin.H{
			"status":    "ok",
			"service":   "cloud-storage",
			"version":   config.AppConfig.AppVersion,
			"timestamp": time.Now().Unix(),
		}

		// Add database health check
		if database.GetDatabase() != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := database.GetDatabase().Client().Ping(ctx, nil); err != nil {
				health["status"] = "degraded"
				health["database"] = "unhealthy"
			} else {
				health["database"] = "healthy"
			}
		}

		c.JSON(http.StatusOK, health)
	}
}

// Version handler
func versionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":        config.AppConfig.AppName,
			"version":     config.AppConfig.AppVersion,
			"environment": config.AppConfig.Environment,
			"build_time":  time.Now().Format(time.RFC3339),
		})
	}
}

func (app *Application) startBackgroundJobs() {
	// Database cleanup job
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Println("Running periodic cleanup tasks...")
				if err := app.dbManager.CleanupOldData(); err != nil {
					log.Printf("Database cleanup failed: %v", err)
				}
			}
		}
	}()

	// Storage health monitoring
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if app.config.Debug {
					results := app.storageManager.HealthCheck()
					for provider, healthy := range results {
						if !healthy {
							log.Printf("Storage provider %s is unhealthy", provider)
						}
					}
				}
			}
		}
	}()

	log.Println("Background jobs started successfully")
}

// logStartupInfo logs important startup information
func (app *Application) logStartupInfo() {
	log.Printf("=== %s v%s ===", app.config.AppName, app.config.AppVersion)
	log.Printf("Environment: %s", app.config.Environment)
	log.Printf("Database: %s", app.config.DBName)
	log.Printf("Upload Path: %s", app.config.UploadPath)
	log.Printf("Max Upload Size: %d bytes", app.config.MaxUploadSize)
	log.Printf("Default Storage Provider: %s", app.config.DefaultStorageProvider)
	log.Printf("Admin Panel: %t", app.config.AdminPanelEnabled)
	log.Printf("Rate Limiting: %t", app.config.RateLimitEnabled)
	if app.config.Debug {
		log.Println("Debug mode enabled")
	}
	log.Println("=========================")
}
