package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/pkg/tunnel"
)

// Provider handlers

func (s *Server) listProviders(c *fiber.Ctx) error {
	providerList := s.registry.ListProviders()

	result := make([]map[string]interface{}, 0, len(providerList))
	for _, p := range providerList {
		result = append(result, map[string]interface{}{
			"name":      p.Name(),
			"category":  p.Category(),
			"installed": p.IsInstalled(),
			"connected": p.IsConnected(),
		})
	}

	return c.JSON(fiber.Map{
		"providers": result,
		"count":     len(result),
	})
}

func (s *Server) getProvider(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	return c.JSON(fiber.Map{
		"name":      provider.Name(),
		"category":  provider.Category(),
		"installed": provider.IsInstalled(),
		"connected": provider.IsConnected(),
	})
}

func (s *Server) getProviderStatus(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	status := "disconnected"
	if provider.IsConnected() {
		status = "connected"
	} else if provider.IsInstalled() {
		status = "ready"
	} else {
		status = "not_installed"
	}

	return c.JSON(fiber.Map{
		"name":   provider.Name(),
		"status": status,
	})
}

func (s *Server) installProvider(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	if err := provider.Install(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to install provider: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Provider %s installed successfully", name),
	})
}

func (s *Server) uninstallProvider(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	if err := provider.Uninstall(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to uninstall provider: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Provider %s uninstalled successfully", name),
	})
}

func (s *Server) connectProvider(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	// Parse config from request body
	var config tunnel.ProviderConfig
	if err := c.BodyParser(&config); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid configuration")
	}

	// Configure and connect
	if err := provider.Configure(&config); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Configuration error: %v", err))
	}

	if err := provider.Connect(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to connect: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Provider %s connected successfully", name),
	})
}

func (s *Server) disconnectProvider(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	if err := provider.Disconnect(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to disconnect: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Provider %s disconnected successfully", name),
	})
}

func (s *Server) providerHealthCheck(c *fiber.Ctx) error {
	name := c.Params("name")

	provider, err := s.registry.GetProvider(name)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Provider %s not found", name))
	}

	health, err := provider.HealthCheck()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Health check failed: %v", err))
	}

	return c.JSON(health)
}

// Connection handlers

func (s *Server) listConnections(c *fiber.Ctx) error {
	connections, err := s.manager.List()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to list connections: %v", err))
	}

	result := make([]map[string]interface{}, 0, len(connections))
	for _, conn := range connections {
		result = append(result, connectionToMap(conn))
	}

	return c.JSON(fiber.Map{
		"connections": result,
		"count":       len(result),
	})
}

func (s *Server) createConnection(c *fiber.Ctx) error {
	var req struct {
		Method     string                 `json:"method"`
		LocalPort  int                    `json:"local_port"`
		RemoteHost string                 `json:"remote_host"`
		RemotePort int                    `json:"remote_port"`
		Config     map[string]interface{} `json:"config"`
	}

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Method == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Method is required")
	}

	config := &tunnel.Config{
		RemoteHost:      req.RemoteHost,
		RemotePort:      req.RemotePort,
		LocalPort:       req.LocalPort,
		Timeout:         30 * time.Second,
		ProviderConfigs: req.Config,
	}

	conn, err := s.manager.Start(req.Method, config)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to create connection: %v", err))
	}

	return c.Status(fiber.StatusCreated).JSON(connectionToMap(conn))
}

func (s *Server) getConnection(c *fiber.Ctx) error {
	id := c.Params("id")

	conn, err := s.manager.Status(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Connection %s not found", id))
	}

	return c.JSON(connectionToMap(conn))
}

func (s *Server) deleteConnection(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := s.manager.Stop(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to delete connection: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Connection %s deleted successfully", id),
	})
}

func (s *Server) restartConnection(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := s.manager.Restart(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to restart connection: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Connection %s restarted successfully", id),
	})
}

func (s *Server) getConnectionMetrics(c *fiber.Ctx) error {
	id := c.Params("id")

	conn, err := s.manager.Status(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("Connection %s not found", id))
	}

	sent, received, latency := conn.Metrics.GetStats()

	return c.JSON(fiber.Map{
		"connection_id":  id,
		"bytes_sent":     sent,
		"bytes_received": received,
		"latency":        latency.String(),
		"uptime":         conn.GetUptime().String(),
		"state":          conn.GetState().String(),
	})
}

// Failover handlers

func (s *Server) getPrimaryConnection(c *fiber.Ctx) error {
	conn, err := s.manager.GetPrimary()
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("No primary connection: %v", err))
	}

	return c.JSON(connectionToMap(conn))
}

func (s *Server) setPrimaryConnection(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := s.manager.SetPrimary(id); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to set primary: %v", err))
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Connection %s set as primary", id),
	})
}

func (s *Server) enableAutoFailover(c *fiber.Ctx) error {
	s.manager.EnableAutoFailover(true)
	return c.JSON(fiber.Map{
		"message": "Auto-failover enabled",
	})
}

func (s *Server) disableAutoFailover(c *fiber.Ctx) error {
	s.manager.EnableAutoFailover(false)
	return c.JSON(fiber.Map{
		"message": "Auto-failover disabled",
	})
}

// Metrics handlers

func (s *Server) getGlobalMetrics(c *fiber.Ctx) error {
	metrics := s.manager.GetMetrics()
	return c.JSON(fiber.Map{
		"metrics": metrics,
	})
}

func (s *Server) exportMetrics(c *fiber.Ctx) error {
	metrics := s.manager.GetMetrics()

	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", "attachment; filename=tunnel-metrics.json")

	return c.JSON(metrics)
}

// System handlers

func (s *Server) getSystemInfo(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version":     "1.0.0",
		"go_version":  "1.24",
		"platform":    "linux",
		"server_time": time.Now().UTC(),
	})
}

func (s *Server) getSystemStatus(c *fiber.Ctx) error {
	connections, _ := s.manager.List()
	providers := s.registry.ListProviders()

	installedCount := 0
	connectedCount := 0
	for _, p := range providers {
		if p.IsInstalled() {
			installedCount++
		}
		if p.IsConnected() {
			connectedCount++
		}
	}

	return c.JSON(fiber.Map{
		"status":              "operational",
		"connections_count":   len(connections),
		"providers_total":     len(providers),
		"providers_installed": installedCount,
		"providers_connected": connectedCount,
		"uptime":              time.Since(time.Now()).String(),
	})
}

// Helper functions

func connectionToMap(conn *tunnel.Connection) map[string]interface{} {
	sent, received, latency := conn.Metrics.GetStats()

	return map[string]interface{}{
		"id":          conn.ID,
		"method":      conn.Method,
		"state":       conn.GetState().String(),
		"local_port":  conn.LocalPort,
		"remote_host": conn.RemoteHost,
		"remote_port": conn.RemotePort,
		"started_at":  conn.StartedAt,
		"uptime":      conn.GetUptime().String(),
		"is_primary":  conn.IsPrimaryConnection(),
		"priority":    conn.GetPriority(),
		"metrics": map[string]interface{}{
			"bytes_sent":     sent,
			"bytes_received": received,
			"latency":        latency.String(),
		},
	}
}
