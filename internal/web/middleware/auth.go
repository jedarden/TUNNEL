package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// WebSocketAuthProtocol is offered by browser WebSocket clients immediately
// before the bearer token. Browsers cannot set an Authorization header during
// the WebSocket handshake, so the token is carried in Sec-WebSocket-Protocol.
const WebSocketAuthProtocol = "tunnel-auth"

// BearerAuth creates a middleware that validates bearer tokens
func BearerAuth(expectedToken string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		providedToken := ""
		if authHeader != "" {
			providedToken = tokenFromAuthorization(authHeader)
		} else if isWebSocketRequest(c) {
			providedToken = tokenFromWebSocketProtocols(c.Get("Sec-WebSocket-Protocol"))
		}

		if expectedToken == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "API authentication is not configured",
			})
		}

		if providedToken == "" || subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) != 1 {
			c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="tunnel-api"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Valid bearer token required",
			})
		}

		return c.Next()
	}
}

func tokenFromAuthorization(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func isWebSocketRequest(c *fiber.Ctx) bool {
	return c.Path() == "/api/ws" && strings.EqualFold(c.Get(fiber.HeaderUpgrade), "websocket")
}

func tokenFromWebSocketProtocols(header string) string {
	protocols := strings.Split(header, ",")
	for i := 0; i+1 < len(protocols); i++ {
		if strings.TrimSpace(protocols[i]) == WebSocketAuthProtocol {
			return strings.TrimSpace(protocols[i+1])
		}
	}
	return ""
}
