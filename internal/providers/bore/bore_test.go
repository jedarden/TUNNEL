package bore

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
	if provider.tunnelURL != "" {
		t.Errorf("tunnelURL = %q, want empty string", provider.tunnelURL)
	}
}

func TestName(t *testing.T) {
	provider := New()
	expected := "bore"
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
				LocalPort: 22,
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "valid config with name only",
			config: &providers.ProviderConfig{
				Name: "bore",
			},
			wantErr: false,
		},
		{
			name: "valid config with local port",
			config: &providers.ProviderConfig{
				Name:      "bore",
				LocalPort: 8080,
			},
			wantErr: false,
		},
		{
			name: "valid config with remote host and port",
			config: &providers.ProviderConfig{
				Name:       "bore",
				LocalPort:  22,
				RemoteHost: "bore.pub",
				RemotePort: 12345,
			},
			wantErr: false,
		},
		{
			name: "valid config with custom remote host",
			config: &providers.ProviderConfig{
				Name:       "bore",
				RemoteHost: "custom.example.com",
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
		Name:       "bore",
		LocalPort:  22,
		RemoteHost: "bore.pub",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Get connection info
	// This will fail if bore is not installed, which is expected
	info, err := provider.GetConnectionInfo()
	if err != nil {
		// If bore is not installed, we should get ErrNotInstalled
		if err != providers.ErrNotInstalled {
			t.Logf("GetConnectionInfo() error = %v (expected if bore not installed)", err)
		}
		return
	}

	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil without error")
	}

	// Status should be disconnected (assuming bore is not running in test environment)
	expectedStatus := "disconnected"
	if info.Status != expectedStatus && info.Status != "connected" {
		t.Errorf("GetConnectionInfo() status = %q, want %q or 'connected'", info.Status, expectedStatus)
	}

	// Extra map should exist
	if info.Extra == nil {
		t.Error("GetConnectionInfo() Extra map is nil")
	}

	// If disconnected, extra fields should still be populated
	if info.Status == "disconnected" {
		if localPort, ok := info.Extra["local_port"]; ok {
			if localPort != config.LocalPort {
				t.Errorf("Extra[local_port] = %v, want %v", localPort, config.LocalPort)
			}
		}
		if remoteHost, ok := info.Extra["remote_host"]; ok {
			if remoteHost != config.RemoteHost {
				t.Errorf("Extra[remote_host] = %v, want %v", remoteHost, config.RemoteHost)
			}
		}
	}
}

func TestGetConnectionInfo_WithTunnelURL(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:       "bore",
		LocalPort:  22,
		RemoteHost: "bore.pub",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Simulate a tunnel URL being set
	provider.tunnelURL = "bore.pub:12345"

	// Get connection info
	// This will fail if bore is not installed, which is expected
	info, err := provider.GetConnectionInfo()
	if err != nil {
		// If bore is not installed, we should get ErrNotInstalled
		if err != providers.ErrNotInstalled {
			t.Logf("GetConnectionInfo() error = %v (expected if bore not installed)", err)
		}
		return
	}

	// If the process is running, the URL should be included
	if provider.tunnelURL != "" && info.Status == "connected" {
		if info.TunnelURL != provider.tunnelURL {
			t.Errorf("TunnelURL = %q, want %q", info.TunnelURL, provider.tunnelURL)
		}

		// RemoteIP should be extracted
		expectedIP := "bore.pub"
		if info.RemoteIP != expectedIP {
			t.Errorf("RemoteIP = %q, want %q", info.RemoteIP, expectedIP)
		}
	}
}

func TestHealthCheck(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:       "bore",
		LocalPort:  22,
		RemoteHost: "bore.pub",
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

func TestHealthCheck_WithTunnelURL(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:       "bore",
		LocalPort:  22,
		RemoteHost: "bore.pub",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Simulate a tunnel URL
	testURL := "bore.pub:54321"
	provider.tunnelURL = testURL

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	// If connected and has tunnel URL, message should include it
	if health.Status == "connected" && provider.tunnelURL != "" {
		expectedMessage := "bore tunnel active at " + testURL
		if health.Message != expectedMessage {
			t.Errorf("HealthCheck() message = %q, want %q", health.Message, expectedMessage)
		}
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
				Name:       "bore",
				LocalPort:  8080,
				RemoteHost: "bore.pub",
				RemotePort: 12345,
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

	if config.Name != "bore" {
		t.Errorf("GetConfig() name = %q, want %q", config.Name, "bore")
	}
}

func TestDisconnect_ClearsTunnelURL(t *testing.T) {
	provider := New()

	// Check if bore is installed
	if !provider.IsInstalled() {
		t.Skip("bore not installed, skipping test")
	}

	// Set a tunnel URL
	provider.tunnelURL = "bore.pub:12345"

	if provider.tunnelURL == "" {
		t.Fatal("Failed to set initial tunnelURL")
	}

	// Disconnect should clear the tunnel URL
	// (Note: This will attempt to kill the process, which may fail in tests)
	err := provider.Disconnect()
	if err != nil {
		t.Logf("Disconnect() error = %v (expected if bore not running)", err)
	}

	if provider.tunnelURL != "" {
		t.Errorf("Disconnect() did not clear tunnelURL, got %q", provider.tunnelURL)
	}
}
