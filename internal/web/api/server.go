package api

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/pkg/tunnel"
)

// Server holds the API server state and dependencies
type Server struct {
	manager        *tunnel.Manager
	registry       *tunnel.Registry
	logger         *log.Logger
	config         *ServerConfig
	tokenStore     TokenStore
	authMiddleware fiber.Handler
}

// TokenStore defines the interface for storing and retrieving the auth token
type TokenStore interface {
	GetToken() (string, error)
	SetToken(token string) error
}

// ServerConfig holds configuration for the API server
type ServerConfig struct {
	Manager    *tunnel.Manager
	Registry   *tunnel.Registry
	Logger     *log.Logger
	DevMode    bool
	TokenStore TokenStore
}

// NewServer creates a new API server instance
func NewServer(config *ServerConfig) *Server {
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	server := &Server{
		manager:  config.Manager,
		registry: config.Registry,
		logger:   config.Logger,
		config:   config,
		tokenStore: config.TokenStore,
	}

	// Initialize auth middleware if token store is provided
	if config.TokenStore != nil {
		server.authMiddleware = createAuthMiddleware(config.TokenStore)
	}

	return server
}

// createAuthMiddleware creates the authentication middleware
func createAuthMiddleware(tokenStore TokenStore) fiber.Handler {
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
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format. Expected: Bearer <token>",
			})
		}

		// Extract token
		token := authHeader[7:]

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

// GetManager returns the connection manager
func (s *Server) GetManager() *tunnel.Manager {
	return s.manager
}

// GetRegistry returns the provider registry
func (s *Server) GetRegistry() *tunnel.Registry {
	return s.registry
}

// GetLogger returns the logger
func (s *Server) GetLogger() *log.Logger {
	return s.logger
}

// IsDevMode returns true if running in development mode
func (s *Server) IsDevMode() bool {
	return s.config.DevMode
}

// Close performs cleanup when the server is shutting down
func (s *Server) Close() error {
	if s.manager != nil {
		return s.manager.Shutdown()
	}
	return nil
}
