package totp

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/internal/core"
)

// ExampleIntegration demonstrates how to integrate TOTP into TUNNEL
func ExampleIntegration() {
	// 1. Initialize credential store
	store, err := core.NewCredentialStore("keyring", "tunnel", "", "")
	if err != nil {
		log.Fatalf("Failed to create credential store: %v", err)
	}

	// 2. Create TOTP manager
	manager := NewManager(store, "TUNNEL")

	// 3. Create HTTP handler
	handler := NewHandler(manager)

	// 4. Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "TUNNEL API",
	})

	// 5. Register public API routes
	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// 6. Setup authentication middleware
	accountExtractor := (&AccountExtractor{}).FromHeader

	// 7. Protect sensitive routes with TOTP
	protected := api.Group("/admin")
	protected.Use(handler.AuthMiddleware(accountExtractor))

	// Protected admin routes
	protected.Get("/settings", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Admin settings (TOTP protected)",
		})
	})

	// 8. Optional TOTP for regular users
	userRoutes := api.Group("/user")
	userRoutes.Use(handler.OptionalAuthMiddleware(accountExtractor))

	userRoutes.Get("/profile", func(c *fiber.Ctx) error {
		totpAuth := c.Locals("totp_authenticated")
		if totpAuth == true {
			return c.JSON(fiber.Map{
				"message": "Full profile (TOTP authenticated)",
				"level":   "full",
			})
		}
		return c.JSON(fiber.Map{
			"message": "Basic profile (no TOTP)",
			"level":   "basic",
		})
	})

	// 9. Start server
	log.Fatal(app.Listen(":8080"))
}

// ExampleUsage demonstrates basic TOTP operations
func ExampleUsage() {
	store, _ := core.NewCredentialStore("keyring", "tunnel", "", "")
	manager := NewManager(store, "TUNNEL")

	// Enroll a new user
	accountName := "user@example.com"
	bundle, err := manager.EnrollUser(accountName)
	if err != nil {
		log.Fatalf("Enrollment failed: %v", err)
	}

	fmt.Printf("User enrolled: %s\n", bundle.Account)
	fmt.Printf("Secret: %s\n", bundle.Secret)
	fmt.Printf("QR Code URL: %s\n", bundle.URL)

	// Generate current valid code
	code, err := manager.GenerateCurrentCode(accountName)
	if err != nil {
		log.Fatalf("Code generation failed: %v", err)
	}

	fmt.Printf("Current code: %s\n", code)

	// Validate the code
	valid, err := manager.ValidateCode(accountName, code, 1)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Printf("Code valid: %v\n", valid)

	// Get time remaining in current window
	remaining, err := manager.GetTimeRemaining(accountName)
	if err != nil {
		log.Fatalf("Failed to get time remaining: %v", err)
	}

	fmt.Printf("Time remaining in window: %d seconds\n", remaining)

	// List all enrolled users
	users, err := manager.ListEnrolledUsers()
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}

	fmt.Printf("Enrolled users: %v\n", users)

	// Check enrollment status
	enrolled, err := manager.IsEnrolled(accountName)
	if err != nil {
		log.Fatalf("Failed to check enrollment: %v", err)
	}

	fmt.Printf("User enrolled: %v\n", enrolled)

	// Regenerate secret (if user lost authenticator)
	newBundle, err := manager.RegenerateSecret(accountName)
	if err != nil {
		log.Fatalf("Secret regeneration failed: %v", err)
	}

	fmt.Printf("New secret generated: %s\n", newBundle.Secret)

	// Delete user enrollment
	err = manager.DeleteUser(accountName)
	if err != nil {
		log.Fatalf("Deletion failed: %v", err)
	}

	fmt.Println("User enrollment deleted")
}

// ExampleWithMiddleware demonstrates middleware usage
func ExampleWithMiddleware() {
	store, _ := core.NewCredentialStore("keyring", "tunnel", "", "")
	manager := NewManager(store, "TUNNEL")
	handler := NewHandler(manager)

	app := fiber.New()

	// Different authentication strategies
	accountExtractor := (&AccountExtractor{}).FromHeader

	// 1. Required TOTP authentication
	adminGroup := app.Group("/admin")
	adminGroup.Use(handler.AuthMiddleware(accountExtractor))
	adminGroup.Get("/settings", func(c *fiber.Ctx) error {
		return c.SendString("Admin settings - TOTP required")
	})

	// 2. Optional TOTP authentication
	userGroup := app.Group("/user")
	userGroup.Use(handler.OptionalAuthMiddleware(accountExtractor))
	userGroup.Get("/profile", func(c *fiber.Ctx) error {
		if c.Locals("totp_authenticated") == true {
			return c.SendString("Full profile - TOTP authenticated")
		}
		return c.SendString("Basic profile - No TOTP")
	})

	// 3. Custom account extraction
	customExtractor := (&AccountExtractor{}).Custom(func(c *fiber.Ctx) string {
		// Extract account from JWT token, session, etc.
		return c.Locals("user_id")
	})

	apiGroup := app.Group("/api")
	apiGroup.Use(handler.AuthMiddleware(customExtractor))
	apiGroup.Get("/data", func(c *fiber.Ctx) error {
		return c.SendString("API data - Custom extraction")
	})

	// 4. Route-specific TOTP requirements
	app.Get("/public", func(c *fiber.Ctx) error {
		return c.SendString("Public endpoint - No authentication")
	})

	app.Get("/sensitive", handler.AuthMiddleware(accountExtractor), func(c *fiber.Ctx) error {
		return c.SendString("Sensitive data - TOTP required")
	})

	// 5. Combining with other authentication methods
	app.Post("/login", func(c *fiber.Ctx) error {
		// First validate password/other auth
		// Then check if user has TOTP enabled
		accountName := c.FormValue("username")
		enrolled, _ := manager.IsEnrolled(accountName)

		if enrolled {
			// Require TOTP code
			totpCode := c.FormValue("totp_code")
			valid, err := manager.ValidateCode(accountName, totpCode, 1)
			if err != nil || !valid {
				return c.Status(401).SendString("Invalid TOTP code")
			}
		}

		return c.SendString("Login successful")
	})

	log.Fatal(app.Listen(":8080"))
}

// ExampleErrorResponseHandling demonstrates proper error handling
func ExampleErrorResponseHandling() {
	store, _ := core.NewCredentialStore("keyring", "tunnel", "", "")
	manager := NewManager(store, "TUNNEL")
	handler := NewHandler(manager)

	app := fiber.New()

	// Register TOTP routes
	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Custom error handling middleware
	app.Use(func(c *fiber.Ctx) error {
		// Your error handling logic
		return c.Next()
	})

	// Example: Proper error responses for different scenarios
	app.Post("/custom-endpoint", func(c *fiber.Ctx) error {
		accountName := c.Get("X-Account-Name")
		totpCode := c.Get("X-TOTP-Code")

		// Check if user exists
		enrolled, err := manager.IsEnrolled(accountName)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Internal server error",
				"message": "Failed to verify enrollment status",
			})
		}

		if !enrolled {
			return c.Status(401).JSON(fiber.Map{
				"error":   "TOTP not enrolled",
				"message": "Please enable TOTP first",
				"action":  "enroll_totp",
			})
		}

		// Validate TOTP code
		valid, err := manager.ValidateCode(accountName, totpCode, 1)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Validation error",
				"message": "Failed to validate TOTP code",
			})
		}

		if !valid {
			return c.Status(401).JSON(fiber.Map{
				"error":   "Invalid code",
				"message": "TOTP code is incorrect",
				"retry":   true,
			})
		}

		// Success
		return c.JSON(fiber.Map{
			"message": "Authentication successful",
		})
	})

	log.Fatal(app.Listen(":8080"))
}
