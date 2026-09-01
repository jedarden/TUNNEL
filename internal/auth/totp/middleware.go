package totp

import (
	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware creates Fiber middleware for TOTP authentication
func (h *Handler) AuthMiddleware(accountExtractor func(c *fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract account name from request
		accountName := accountExtractor(c)
		if accountName == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No account identifier provided",
			})
		}

		// Check if user is enrolled
		enrolled, err := h.manager.IsEnrolled(accountName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check TOTP enrollment",
			})
		}

		// If not enrolled, reject request
		if !enrolled {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "TOTP not enabled for this account",
			})
		}

		// Extract TOTP code from request
		// Try header first, then query param, then body
		totpCode := c.Get("X-TOTP-Code")
		if totpCode == "" {
			totpCode = c.Query("totp_code")
		}
		if totpCode == "" {
			// Try to parse from body if JSON
			if c.Is("json") {
				body := make(map[string]string)
				if err := c.BodyParser(&body); err == nil {
					totpCode = body["totp_code"]
				}
			}
		}

		if totpCode == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "TOTP code required",
			})
		}

		// Validate TOTP code
		valid, err := h.manager.ValidateCode(accountName, totpCode, 1)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to validate TOTP code",
			})
		}

		if !valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid TOTP code",
			})
		}

		// Store account name in context for downstream handlers
		c.Locals("totp_account", accountName)
		c.Locals("totp_authenticated", true)

		return c.Next()
	}
}

// OptionalAuthMiddleware creates optional TOTP authentication middleware
// If TOTP is enabled for the account, it validates the code
// If TOTP is not enabled, it allows the request to proceed
func (h *Handler) OptionalAuthMiddleware(accountExtractor func(c *fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract account name from request
		accountName := accountExtractor(c)
		if accountName == "" {
			return c.Next()
		}

		// Check if user is enrolled
		enrolled, err := h.manager.IsEnrolled(accountName)
		if err != nil {
			return c.Next() // Don't block on error
		}

		// If not enrolled, allow request to proceed
		if !enrolled {
			return c.Next()
		}

		// Extract TOTP code
		totpCode := c.Get("X-TOTP-Code")
		if totpCode == "" {
			totpCode = c.Query("totp_code")
		}
		if totpCode == "" {
			// Allow request to proceed if no code provided
			// Downstream handlers can check if TOTP was validated
			c.Locals("totp_authenticated", false)
			return c.Next()
		}

		// Validate TOTP code
		valid, err := h.manager.ValidateCode(accountName, totpCode, 1)
		if err != nil {
			c.Locals("totp_authenticated", false)
			return c.Next()
		}

		// Store authentication status in context
		c.Locals("totp_account", accountName)
		c.Locals("totp_authenticated", valid)

		return c.Next()
	}
}

// WebSocketAuthMiddleware creates TOTP middleware for WebSocket connections
// For WebSocket, the code is expected in Sec-WebSocket-Protocol
func (h *Handler) WebSocketAuthMiddleware(accountExtractor func(c *fiber.Ctx) string) fiber.Handler {
	const wsProtocol = "totp-auth"

	return func(c *fiber.Ctx) error {
		// Only apply to WebSocket upgrade requests
		if !c.Is("websocket") {
			return c.Next()
		}

		// Extract account name
		accountName := accountExtractor(c)
		if accountName == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No account identifier provided",
			})
		}

		// Check if user is enrolled
		enrolled, err := h.manager.IsEnrolled(accountName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check TOTP enrollment",
			})
		}

		if !enrolled {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "TOTP not enabled for this account",
			})
		}

		// Extract TOTP code from Sec-WebSocket-Protocol
		protocols := c.Get("Sec-WebSocket-Protocol")
		var totpCode string
		for _, protocol := range splitProtocols(protocols) {
			if protocol == wsProtocol {
				// Next protocol should be the code
				// This is simplified - in production you'd parse properly
				protocolsList := splitProtocols(protocols)
				for i, p := range protocolsList {
					if p == wsProtocol && i+1 < len(protocolsList) {
						totpCode = protocolsList[i+1]
						break
					}
				}
				break
			}
		}

		if totpCode == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "TOTP code required in WebSocket protocol",
			})
		}

		// Validate TOTP code
		valid, err := h.manager.ValidateCode(accountName, totpCode, 1)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to validate TOTP code",
			})
		}

		if !valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid TOTP code",
			})
		}

		// Store authentication status in context
		c.Locals("totp_account", accountName)
		c.Locals("totp_authenticated", true)

		return c.Next()
	}
}

// splitProtocols splits WebSocket protocol string by comma
func splitProtocols(protocols string) []string {
	var result []string
	current := ""
	for _, c := range protocols {
		if c == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if c != ' ' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// AccountExtractor provides common account extraction strategies
type AccountExtractor struct{}

// FromHeader extracts account from X-Account-Name header
func (ae *AccountExtractor) FromHeader(c *fiber.Ctx) string {
	return c.Get("X-Account-Name")
}

// FromQuery extracts account from account query parameter
func (ae *AccountExtractor) FromQuery(c *fiber.Ctx) string {
	return c.Query("account")
}

// FromBasicAuth extracts account from Basic Auth username
func (ae *AccountExtractor) FromBasicAuth(c *fiber.Ctx) string {
	user, _, ok := c.BasicAuth()
	if !ok {
		return ""
	}
	return user
}

// FromBearerToken extracts account from Bearer token (assumes token is account name)
// In production, you'd validate the token and extract account from claims
func (ae *AccountExtractor) FromBearerToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// Custom creates a custom account extractor
func (ae *AccountExtractor) Custom(extractor func(c *fiber.Ctx) string) func(c *fiber.Ctx) string {
	return extractor
}
