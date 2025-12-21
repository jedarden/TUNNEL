package registry_test

import (
	"testing"

	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
)

func TestNewRegistry(t *testing.T) {
	r := registry.NewRegistry()

	providerList := r.ListProviders()
	if len(providerList) == 0 {
		t.Error("expected providers to be registered, got 0")
	}

	// Check that default providers are registered
	expectedProviders := []string{
		"tailscale",
		"wireguard",
		"zerotier",
		"cloudflare",
		"ngrok",
		"bore",
	}

	for _, name := range expectedProviders {
		_, err := r.GetProvider(name)
		if err != nil {
			t.Errorf("expected provider '%s' to be registered", name)
		}
	}
}

func TestGetProvider(t *testing.T) {
	r := registry.NewRegistry()

	// Test getting an existing provider
	provider, err := r.GetProvider("tailscale")
	if err != nil {
		t.Errorf("failed to get tailscale provider: %v", err)
	}

	if provider.Name() != "tailscale" {
		t.Errorf("expected provider name 'tailscale', got '%s'", provider.Name())
	}

	// Test getting a non-existent provider
	_, err = r.GetProvider("nonexistent")
	if err == nil {
		t.Error("expected error when getting non-existent provider")
	}
}

func TestListByCategory(t *testing.T) {
	r := registry.NewRegistry()

	vpnProviders := r.ListByCategory(providers.CategoryVPN)
	tunnelProviders := r.ListByCategory(providers.CategoryTunnel)

	if len(vpnProviders) == 0 {
		t.Error("expected VPN providers to be registered")
	}

	if len(tunnelProviders) == 0 {
		t.Error("expected Tunnel providers to be registered")
	}

	// Verify VPN providers
	expectedVPN := map[string]bool{
		"tailscale": true,
		"wireguard": true,
		"zerotier":  true,
	}

	for _, provider := range vpnProviders {
		if !expectedVPN[provider.Name()] {
			t.Errorf("unexpected VPN provider: %s", provider.Name())
		}
	}

	// Verify Tunnel providers
	expectedTunnel := map[string]bool{
		"cloudflare": true,
		"ngrok":      true,
		"bore":       true,
	}

	for _, provider := range tunnelProviders {
		if !expectedTunnel[provider.Name()] {
			t.Errorf("unexpected Tunnel provider: %s", provider.Name())
		}
	}
}

func TestGetProviderInfo(t *testing.T) {
	r := registry.NewRegistry()

	info := r.GetProviderInfo()

	if len(info) == 0 {
		t.Error("expected provider info to be returned")
	}

	for _, i := range info {
		if i.Name == "" {
			t.Error("provider info missing name")
		}
		if i.Category == "" {
			t.Error("provider info missing category")
		}
	}
}

func TestGlobalFunctions(t *testing.T) {
	// Test global registry functions
	providers := registry.ListProviders()
	if len(providers) == 0 {
		t.Error("expected providers from global registry")
	}

	provider, err := registry.GetProvider("tailscale")
	if err != nil {
		t.Errorf("failed to get provider from global registry: %v", err)
	}

	if provider.Name() != "tailscale" {
		t.Errorf("expected provider name 'tailscale', got '%s'", provider.Name())
	}
}
