package api

import (
	"log"

	"github.com/jedarden/tunnel/pkg/tunnel"
)

// Server holds the API server state and dependencies
type Server struct {
	manager  *tunnel.Manager
	registry *tunnel.Registry
	logger   *log.Logger
	config   *ServerConfig
}

// ServerConfig holds configuration for the API server
type ServerConfig struct {
	Manager  *tunnel.Manager
	Registry *tunnel.Registry
	Logger   *log.Logger
	DevMode  bool
}

// NewServer creates a new API server instance
func NewServer(config *ServerConfig) *Server {
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	return &Server{
		manager:  config.Manager,
		registry: config.Registry,
		logger:   config.Logger,
		config:   config,
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
