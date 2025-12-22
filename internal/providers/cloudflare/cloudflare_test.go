package cloudflare

import (
	"testing"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

func TestNew(t *testing.T) {
	provider := New()
	if provider == nil {
		t.Fatal("New() returned nil")
	}
	if provider.BaseProvider == nil {
		t.Fatal("BaseProvider is nil")
	}
}

func TestName(t *testing.T) {
	provider := New()
	expected := "cloudflare"
	if got := provider.Name(); got != expected {
		t.Errorf("Name() = %q, want %q", got, expected)
	}
}

func TestCategory(t *testing.T) {
	provider := New()
	expected := providers.CategoryTunnel
	if got := provider.Category(); got != expected {
		t.Errorf("Category() = %q, want %q", got, expected)
	}
}

func TestValidateConfig(t *testing.T) {
	provider := New()

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "invalid configuration",
		},
		{
			name: "missing name",
			config: &providers.ProviderConfig{
				TunnelName: "my-tunnel",
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "missing tunnel name",
			config: &providers.ProviderConfig{
				Name: "cloudflare",
			},
			wantErr: true,
			errMsg:  "tunnel_name is required for Cloudflare Tunnel",
		},
		{
			name: "valid config with tunnel name",
			config: &providers.ProviderConfig{
				Name:       "cloudflare",
				TunnelName: "my-tunnel",
			},
			wantErr: false,
		},
		{
			name: "valid config with tunnel name and auth token",
			config: &providers.ProviderConfig{
				Name:       "cloudflare",
				TunnelName: "my-tunnel",
				AuthToken:  "test-token-123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("ValidateConfig() error message = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestGetConnectionInfo_Disconnected(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:       "cloudflare",
		TunnelName: "test-tunnel",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Get connection info
	// This will fail if cloudflared is not installed, which is expected
	info, err := provider.GetConnectionInfo()
	if err != nil {
		// If cloudflared is not installed, we should get ErrNotInstalled
		if err != providers.ErrNotInstalled {
			t.Logf("GetConnectionInfo() error = %v (expected if cloudflared not installed)", err)
		}
		return
	}

	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil without error")
	}

	// Status should be disconnected (assuming cloudflared is not running in test environment)
	expectedStatus := "disconnected"
	if info.Status != expectedStatus && info.Status != "connected" {
		t.Errorf("GetConnectionInfo() status = %q, want %q or 'connected'", info.Status, expectedStatus)
	}

	// Extra map should exist
	if info.Extra == nil {
		t.Error("GetConnectionInfo() Extra map is nil")
	}
}

func TestHealthCheck(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:       "cloudflare",
		TunnelName: "test-tunnel",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	if health == nil {
		t.Fatal("HealthCheck() returned nil")
	}

	// Verify LastCheck is recent
	if time.Since(health.LastCheck) > time.Second {
		t.Errorf("HealthCheck() LastCheck is too old: %v", health.LastCheck)
	}

	// Status should be one of the expected values
	validStatuses := map[string]bool{
		"not_installed": true,
		"disconnected":  true,
		"connected":     true,
	}
	if !validStatuses[health.Status] {
		t.Errorf("HealthCheck() status = %q, want one of: not_installed, disconnected, connected", health.Status)
	}

	// Message should not be empty
	if health.Message == "" {
		t.Error("HealthCheck() message is empty")
	}

	// If not installed, healthy should be false
	if health.Status == "not_installed" && health.Healthy {
		t.Error("HealthCheck() healthy = true when status is not_installed")
	}

	// If disconnected, healthy should be false
	if health.Status == "disconnected" && health.Healthy {
		t.Error("HealthCheck() healthy = true when status is disconnected")
	}
}

func TestGetLogs(t *testing.T) {
	provider := New()

	// GetLogs should not return an error even if logs are empty
	logs, err := provider.GetLogs(time.Now().Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("GetLogs() error = %v", err)
	}

	if logs == nil {
		t.Fatal("GetLogs() returned nil")
	}

	// For now, logs are expected to be empty (simplified implementation)
	if len(logs) != 0 {
		t.Logf("GetLogs() returned %d logs (expected 0 for simplified implementation)", len(logs))
	}
}

func TestConfigure(t *testing.T) {
	provider := New()

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &providers.ProviderConfig{
				Name:       "cloudflare",
				TunnelName: "my-tunnel",
				AuthToken:  "test-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.Configure(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify config was set
				config, err := provider.GetConfig()
				if err != nil {
					t.Errorf("GetConfig() error = %v", err)
				}
				if config == nil {
					t.Error("GetConfig() returned nil after Configure()")
				}
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	provider := New()

	// GetConfig should return a default config (name only)
	config, err := provider.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}

	if config.Name != "cloudflare" {
		t.Errorf("GetConfig() name = %q, want %q", config.Name, "cloudflare")
	}
}

func TestListTunnels(t *testing.T) {
	provider := New()

	// ListTunnels will likely fail if cloudflared is not installed or not configured
	tunnels, err := provider.ListTunnels()

	// If cloudflared is not installed, we should get ErrNotInstalled
	if err != nil {
		if err != providers.ErrNotInstalled && !isCommandFailedError(err) {
			t.Logf("ListTunnels() error = %v (expected if cloudflared not installed)", err)
		}
		return
	}

	// If it succeeds, tunnels should not be nil
	if tunnels == nil {
		t.Error("ListTunnels() returned nil tunnels without error")
	}
}

// Helper function to check if error is a command failed error
func isCommandFailedError(err error) bool {
	if err == nil {
		return false
	}
	return err == providers.ErrCommandFailed ||
		   err == providers.ErrInvalidResponse ||
		   containsString(err.Error(), "command execution failed") ||
		   containsString(err.Error(), "invalid response")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr || len(s) > len(substr) &&
		   (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		    len(s) > len(substr)+2 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
