package api

import (
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, server *Server) {
	// API group
	api := app.Group("/api")

	// Provider routes
	providers := api.Group("/providers")
	providers.Get("/", server.listProviders)
	providers.Get("/:name", server.getProvider)
	providers.Get("/:name/status", server.getProviderStatus)
	providers.Post("/:name/install", server.installProvider)
	providers.Post("/:name/uninstall", server.uninstallProvider)
	providers.Post("/:name/connect", server.connectProvider)
	providers.Post("/:name/disconnect", server.disconnectProvider)
	providers.Get("/:name/health", server.providerHealthCheck)

	// Connection routes
	connections := api.Group("/connections")
	connections.Get("/", server.listConnections)
	connections.Post("/", server.createConnection)
	connections.Get("/:id", server.getConnection)
	connections.Delete("/:id", server.deleteConnection)
	connections.Post("/:id/restart", server.restartConnection)
	connections.Get("/:id/metrics", server.getConnectionMetrics)

	// Failover routes
	failover := api.Group("/failover")
	failover.Get("/primary", server.getPrimaryConnection)
	failover.Post("/primary/:id", server.setPrimaryConnection)
	failover.Post("/enable", server.enableAutoFailover)
	failover.Post("/disable", server.disableAutoFailover)

	// Metrics routes
	metrics := api.Group("/metrics")
	metrics.Get("/", server.getGlobalMetrics)
	metrics.Get("/export", server.exportMetrics)

	// WebSocket route
	api.Get("/ws", server.handleWebSocket)

	// System routes
	system := api.Group("/system")
	system.Get("/info", server.getSystemInfo)
	system.Get("/status", server.getSystemStatus)
}
