package api

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/internal/auth/fido2"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/web/middleware"
	"github.com/jedarden/tunnel/pkg/config"
	"github.com/jedarden/tunnel/pkg/tunnel"
)

// Server holds the API server state and dependencies
type Server struct {
	manager        *tunnel.Manager
	registry       *tunnel.Registry
	logger         *log.Logger
	config         *ServerConfig
	authMiddleware fiber.Handler
	// FIDO2 authentication
	fido2Manager *fido2.Manager
	fido2Handler *fido2.Handler
	// Credential store
	credentialStore core.CredentialStore
	appConfig       *config.Config
}

// ServerConfig holds configuration for the API server
type ServerConfig struct {
	Manager   *tunnel.Manager
	Registry  *tunnel.Registry
	Logger    *log.Logger
	DevMode   bool
	AuthToken string
	// Authentication support
	CredentialStore core.CredentialStore
	AppConfig       *config.Config
}

// NewServer creates a new API server instance
func NewServer(config *ServerConfig) *Server {
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	server := &Server{
		manager:         config.Manager,
		registry:        config.Registry,
		logger:          config.Logger,
		config:          config,
		authMiddleware:  middleware.BearerAuth(config.AuthToken),
		credentialStore: config.CredentialStore,
		appConfig:       config.AppConfig,
	}

	// Initialize FIDO2 if credential store is available
	if config.CredentialStore != nil && config.AppConfig != nil {
		if err := server.initFIDO2(); err != nil {
			config.Logger.Printf("Warning: Failed to initialize FIDO2: %v", err)
		}
	}

	return server
}

// initFIDO2 initializes the FIDO2 manager and handler
func (s *Server) initFIDO2() error {
	// Get FIDO2 configuration from app config
	fido2Config, ok := s.appConfig.Methods["fido2"]
	if !ok || !fido2Config.Enabled {
		// FIDO2 not configured or not enabled, skip initialization
		return nil
	}

	// Create FIDO2 provider configuration
	providerConfig := &fido2.ProviderConfig{
		RPID:          "localhost", // Default, should be configurable
		RPOrigin:      "http://localhost:8080",
		RPDisplayName: "TUNNEL",
		Timeout:       60000,
	}

	// Create FIDO2 manager
	manager, err := fido2.NewManager(s.credentialStore, providerConfig)
	if err != nil {
		return err
	}

	s.fido2Manager = manager
	s.fido2Handler = fido2.NewHandler(manager)

	s.logger.Printf("FIDO2 authentication initialized")
	return nil
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

// GetFIDO2Handler returns the FIDO2 handler if initialized
func (s *Server) GetFIDO2Handler() *fido2.Handler {
	return s.fido2Handler
}

// GetFIDO2Manager returns the FIDO2 manager if initialized
func (s *Server) GetFIDO2Manager() *fido2.Manager {
	return s.fido2Manager
}

// Close performs cleanup when the server is shutting down
func (s *Server) Close() error {
	if s.manager != nil {
		return s.manager.Shutdown()
	}
	return nil
}
