package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/pkg/tunnel"
)

func TestSetupRoutesProtectsAPIExceptSystemInfo(t *testing.T) {
	const token = "test-control-token"
	server := NewServer(&ServerConfig{
		Manager:   tunnel.NewManager(nil),
		Registry:  tunnel.NewRegistry(),
		AuthToken: token,
	})
	defer server.Close()

	app := fiber.New()
	SetupRoutes(app, server)

	publicRequest := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
	publicResponse, err := app.Test(publicRequest)
	if err != nil {
		t.Fatalf("public app.Test() error = %v", err)
	}
	if publicResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("public status = %d, want %d", publicResponse.StatusCode, fiber.StatusOK)
	}

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/providers/cloudflare/install"},
		{http.MethodPost, "/api/providers/cloudflare/uninstall"},
		{http.MethodPost, "/api/providers/cloudflare/connect"},
		{http.MethodPost, "/api/providers/cloudflare/disconnect"},
		{http.MethodDelete, "/api/connections/connection-id"},
		{http.MethodPost, "/api/connections/connection-id/restart"},
		{http.MethodPost, "/api/failover/enable"},
		{http.MethodPost, "/api/failover/disable"},
		{http.MethodPost, "/api/failover/primary/connection-id"},
		{http.MethodGet, "/api/ws"},
		{http.MethodPost, "/api/system/info"},
		{http.MethodGet, "/api/auth/token"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if response.StatusCode != fiber.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnauthorized)
			}
		})
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer "+token)
	authorizedResponse, err := app.Test(authorizedRequest)
	if err != nil {
		t.Fatalf("authorized app.Test() error = %v", err)
	}
	if authorizedResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("authorized status = %d, want %d", authorizedResponse.StatusCode, fiber.StatusOK)
	}

	removedTokenEndpoint := httptest.NewRequest(http.MethodGet, "/api/auth/token", nil)
	removedTokenEndpoint.Header.Set("Authorization", "Bearer "+token)
	removedTokenResponse, err := app.Test(removedTokenEndpoint)
	if err != nil {
		t.Fatalf("removed token endpoint app.Test() error = %v", err)
	}
	if removedTokenResponse.StatusCode != fiber.StatusNotFound {
		t.Fatalf("removed token endpoint status = %d, want %d", removedTokenResponse.StatusCode, fiber.StatusNotFound)
	}
}
