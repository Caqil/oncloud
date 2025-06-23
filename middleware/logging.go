package middleware

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"oncloud/utils"
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// LoggingMiddleware logs HTTP requests and responses
func LoggingMiddleware() gin.HandlerFunc {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Read request body
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Create response writer wrapper
		w := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = w

		// Process request
		c.Next()

		// Calculate processing time
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		// Get user info if available
		var userID string
		var username string
		if user, exists := utils.GetUserFromContext(c); exists {
			userID = user.ID.Hex()
			username = user.Username
		}

		// Create log entry
		logEntry := logger.WithFields(logrus.Fields{
			"status_code":  statusCode,
			"latency":      latency.String(),
			"client_ip":    clientIP,
			"method":       method,
			"path":         path,
			"user_agent":   c.Request.UserAgent(),
			"user_id":      userID,
			"username":     username,
			"request_id":   c.GetHeader("X-Request-ID"),
			"content_type": c.ContentType(),
		})

		// Add request body for non-GET requests (but not for file uploads)
		if method != "GET" && !isFileUpload(c) && len(requestBody) > 0 && len(requestBody) < 1024 {
			logEntry = logEntry.WithField("request_body", string(requestBody))
		}

		// Add response body for errors (but limit size)
		if statusCode >= 400 && w.body.Len() > 0 && w.body.Len() < 1024 {
			logEntry = logEntry.WithField("response_body", w.body.String())
		}

		// Log based on status code
		message := fmt.Sprintf("%s %s %d", method, path, statusCode)

		switch {
		case statusCode >= 500:
			logEntry.Error(message)
		case statusCode >= 400:
			logEntry.Warn(message)
		case statusCode >= 300:
			logEntry.Info(message)
		default:
			logEntry.Info(message)
		}
	}
}

// isFileUpload checks if request is a file upload
func isFileUpload(c *gin.Context) bool {
	contentType := c.ContentType()
	return contentType == "multipart/form-data" ||
		contentType == "application/octet-stream"
}

// RequestIDMiddleware adds unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID, _ = utils.GenerateSecureToken(16)
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}
