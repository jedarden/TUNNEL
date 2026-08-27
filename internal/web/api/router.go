package api

import (
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, server *Server) {
	// API group
	api := app.Group("/api")

	// Keep only the minimal system information probe public. Route order is
	// significant in Fiber: this handler is registered before the group auth.
	api.Get("/system/info", server.getSystemInfo)

	// Every other /api route, including unmatched paths, fails closed.
	api.Use(server.authMiddleware)

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
	providers.Post("/:name/health", server.providerHealthCheckWithConfig)

	// Connection routes
	connections := api.Group("/connections")
	connections.Get("/", server.listConnections)
	connections.Post("/", server.createConnection)
	connections.Get("/:id", server.getConnection)
	connections.Delete("/:id", server.deleteConnection)
	connections.Post("/:id/restart", server.restartConnection)
	connections.Get("/:id/metrics", server.getConnectionMetrics)
	connections.Get("/:id/history", server.getConnectionHistory)
	connections.Get("/:id/stats", server.getConnectionStats)

	// Failover routes
	failover := api.Group("/failover")
	failover.Get("/primary", server.getPrimaryConnection)
	failover.Post("/primary/:id", server.setPrimaryConnection)
	failover.Post("/enable", server.enableAutoFailover)
	failover.Post("/disable", server.disableAutoFailover)
	failover.Get("/events", server.getFailoverEvents)

	// Metrics routes
	metrics := api.Group("/metrics")
	metrics.Get("/", server.getGlobalMetrics)
	metrics.Get("/export", server.exportMetrics)
	metrics.Get("/connections/stats", server.getAllConnectionsStats)

	// WebSocket route
	api.Get("/ws", server.handleWebSocket)

	// System routes
	system := api.Group("/system")
	system.Get("/status", server.getSystemStatus)
}
