package middleware

import (
	"fmt"
	"oncloud/utils"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	visitors map[string]*Visitor
	mutex    sync.RWMutex
	rate     time.Duration
	burst    int
}

type Visitor struct {
	limiter  *TokenBucket
	lastSeen time.Time
}

type TokenBucket struct {
	tokens     int
	capacity   int
	refillRate time.Duration
	lastRefill time.Time
	mutex      sync.Mutex
}

var (
	rateLimiters = map[string]*RateLimiter{
		"global":   NewRateLimiter(time.Minute, 60),   // 60 requests per minute
		"auth":     NewRateLimiter(time.Minute, 10),   // 10 auth requests per minute
		"upload":   NewRateLimiter(time.Minute, 30),   // 30 uploads per minute
		"download": NewRateLimiter(time.Minute, 100),  // 100 downloads per minute
		"api":      NewRateLimiter(time.Minute, 1000), // 1000 API calls per minute
	}
)

func NewRateLimiter(rate time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
		rate:     rate,
		burst:    burst,
	}

	// Clean up expired visitors every 10 minutes
	go rl.cleanupVisitors()

	return rl
}

func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Refill tokens based on elapsed time
	if elapsed >= tb.refillRate {
		tokensToAdd := int(elapsed / tb.refillRate)
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	visitor, exists := rl.visitors[key]
	if !exists {
		visitor = &Visitor{
			limiter:  NewTokenBucket(rl.burst, rl.rate),
			lastSeen: time.Now(),
		}
		rl.visitors[key] = visitor
	}

	visitor.lastSeen = time.Now()
	return visitor.limiter.Allow()
}

func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(10 * time.Minute)

		rl.mutex.Lock()
		for key, visitor := range rl.visitors {
			if time.Since(visitor.lastSeen) > time.Hour {
				delete(rl.visitors, key)
			}
		}
		rl.mutex.Unlock()
	}
}

// RateLimitMiddleware applies rate limiting based on client IP
func RateLimitMiddleware() gin.HandlerFunc {
	return RateLimitWithType("global")
}

// RateLimitWithType applies specific rate limiting type
func RateLimitWithType(limitType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter, exists := rateLimiters[limitType]
		if !exists {
			limiter = rateLimiters["global"]
		}

		// Get client identifier (IP or user ID if authenticated)
		clientID := getClientID(c)

		if !limiter.Allow(clientID) {
			// Set rate limit headers
			c.Header("X-RateLimit-Limit", strconv.Itoa(limiter.burst))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(limiter.rate).Unix(), 10))

			utils.TooManyRequestsResponse(c, "Rate limit exceeded")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthRateLimitMiddleware applies strict rate limiting for auth endpoints
func AuthRateLimitMiddleware() gin.HandlerFunc {
	return RateLimitWithType("auth")
}

// UploadRateLimitMiddleware applies rate limiting for upload endpoints
func UploadRateLimitMiddleware() gin.HandlerFunc {
	return RateLimitWithType("upload")
}

// getClientID returns client identifier for rate limiting
func getClientID(c *gin.Context) string {
	// Try to get user ID from context first
	if userID, exists := utils.GetUserIDFromContext(c); exists {
		return fmt.Sprintf("user:%s", userID.Hex())
	}

	// Fall back to IP address
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
