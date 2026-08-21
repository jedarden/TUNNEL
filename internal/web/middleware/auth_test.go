package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestBearerAuth(t *testing.T) {
	const token = "correct-control-token"

	tests := []struct {
		name       string
		header     string
		query      string
		wantStatus int
	}{
		{name: "missing", wantStatus: fiber.StatusUnauthorized},
		{name: "malformed", header: token, wantStatus: fiber.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic " + token, wantStatus: fiber.StatusUnauthorized},
		{name: "wrong token", header: "Bearer wrong", wantStatus: fiber.StatusUnauthorized},
		{name: "query token rejected", query: "?token=" + token, wantStatus: fiber.StatusUnauthorized},
		{name: "valid", header: "Bearer " + token, wantStatus: fiber.StatusNoContent},
		{name: "case insensitive scheme", header: "bearer " + token, wantStatus: fiber.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/api/control", BearerAuth(token), func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest("GET", "/api/control"+tt.query, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			response, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestBearerAuthAcceptsWebSocketSubprotocolToken(t *testing.T) {
	const token = "correct-control-token"
	app := fiber.New()
	app.Get("/api/ws", BearerAuth(token), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest("GET", "/api/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Protocol", WebSocketAuthProtocol+", "+token)
	response, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
	}
}

func TestBearerAuthFailsClosedWithoutConfiguredToken(t *testing.T) {
	app := fiber.New()
	app.Get("/api/control", BearerAuth(""), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest("GET", "/api/control", nil)
	req.Header.Set("Authorization", "Bearer anything")
	response, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusServiceUnavailable)
	}
}
