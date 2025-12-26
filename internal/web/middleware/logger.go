package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RequestLogger is a custom logging middleware
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Log request details
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		ip := c.IP()

		// Determine log level based on status code
		level := "INFO"
		if status >= 400 && status < 500 {
			level = "WARN"
		} else if status >= 500 {
			level = "ERROR"
		}

		// Format log message
		fmt.Printf("[%s] %s | %d | %s | %s %s | %s\n",
			level,
			time.Now().Format("2006-01-02 15:04:05"),
			status,
			duration,
			method,
			path,
			ip,
		)

		return err
	}
}

// ErrorLogger logs errors that occur during request processing
func ErrorLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		if err != nil {
			// Log the error
			fmt.Printf("[ERROR] %s | %s %s | Error: %v\n",
				time.Now().Format("2006-01-02 15:04:05"),
				c.Method(),
				c.Path(),
				err,
			)
		}

		return err
	}
}
