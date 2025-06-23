// utils/jwt.go
package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Claims struct {
	UserID   primitive.ObjectID `json:"user_id"`
	Email    string             `json:"email"`
	Username string             `json:"username"`
	Role     string             `json:"role"`
	PlanID   primitive.ObjectID `json:"plan_id"`
	jwt.RegisteredClaims
}

type AdminClaims struct {
	AdminID     primitive.ObjectID `json:"admin_id"`
	Email       string             `json:"email"`
	Username    string             `json:"username"`
	Role        string             `json:"role"`
	Permissions []string           `json:"permissions"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

var (
	jwtSecret        = []byte(getEnv("JWT_SECRET", "your-secret-key"))
	jwtRefreshSecret = []byte(getEnv("JWT_REFRESH_SECRET", "your-refresh-secret-key"))
	accessTokenTTL   = 24 * time.Hour
	refreshTokenTTL  = 7 * 24 * time.Hour
)

// GenerateTokenPair generates both access and refresh tokens
func GenerateTokenPair(userID primitive.ObjectID, email, username, role string, planID primitive.ObjectID) (*TokenPair, error) {
	// Generate access token
	accessToken, err := GenerateAccessToken(userID, email, username, role, planID)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken, err := GenerateRefreshToken(userID, email)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// GenerateAccessToken creates a new JWT access token
func GenerateAccessToken(userID primitive.ObjectID, email, username, role string, planID primitive.ObjectID) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Email:    email,
		Username: username,
		Role:     role,
		PlanID:   planID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "cloudstorage",
			Subject:   userID.Hex(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateRefreshToken creates a new JWT refresh token
func GenerateRefreshToken(userID primitive.ObjectID, email string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "cloudstorage",
			Subject:   userID.Hex(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtRefreshSecret)
}

// GenerateAdminToken creates a new JWT token for admin users
func GenerateAdminToken(adminID primitive.ObjectID, email, username, role string, permissions []string) (string, error) {
	claims := &AdminClaims{
		AdminID:     adminID,
		Email:       email,
		Username:    username,
		Role:        role,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "cloudstorage-admin",
			Subject:   adminID.Hex(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken validates and parses JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidateRefreshToken validates refresh token
func ValidateRefreshToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtRefreshSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid refresh token")
}

// ValidateAdminToken validates admin JWT token
func ValidateAdminToken(tokenString string) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid admin token")
}
