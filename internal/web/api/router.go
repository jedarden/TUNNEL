package api

import (
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, server *Server) {
	// API group
	api := app.Group("/api")

	// Public endpoint to get the bearer token (must be before auth middleware)
	api.Get("/auth/token", func(c *fiber.Ctx) error {
		if server.tokenStore == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Authentication not available",
			})
		}
		token, err := server.tokenStore.GetToken()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to retrieve authentication token",
			})
		}
		return c.JSON(fiber.Map{
			"token": token,
		})
	})

	// Apply authentication middleware to all other API routes
	if server.authMiddleware != nil {
		api.Use(server.authMiddleware)
	}

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
