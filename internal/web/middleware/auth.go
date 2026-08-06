package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	// AuthService is the service name used for storing the bearer token in the credential store
	AuthService = "tunnel-auth"
	// AuthKey is the key used for storing the bearer token
	AuthKey = "bearer-token"
	// TokenLength is the length of the random token bytes (before base64 encoding)
	TokenLength = 32
)

// TokenStore defines the interface for storing and retrieving the auth token
type TokenStore interface {
	GetToken() (string, error)
	SetToken(token string) error
}

// BearerAuth creates a middleware that validates bearer tokens
func BearerAuth(tokenStore TokenStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Allow /api/system/info without authentication
		if c.Path() == "/api/system/info" {
			return c.Next()
		}

		// Get Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		// Check if it's a bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format. Expected: Bearer <token>",
			})
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Get stored token
		storedToken, err := tokenStore.GetToken()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to retrieve authentication token",
			})
		}

		// Validate token
		if token != storedToken {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authentication token",
			})
		}

		// Token is valid, proceed to next handler
		return c.Next()
	}
}

// GenerateRandomToken generates a cryptographically secure random token
func GenerateRandomToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GetTokenFromHeader extracts the bearer token from the Authorization header
func GetTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header required")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("invalid authorization header format. Expected: Bearer <token>")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}

	return token, nil
}
