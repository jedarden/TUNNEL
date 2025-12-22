package tailscale

import (
	"encoding/json"
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
	expected := "tailscale"
	if got := provider.Name(); got != expected {
		t.Errorf("Name() = %q, want %q", got, expected)
	}
}

func TestCategory(t *testing.T) {
	provider := New()
	expected := providers.CategoryVPN
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
				AuthKey: "test-key",
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "valid config with name only",
			config: &providers.ProviderConfig{
				Name: "tailscale",
			},
			wantErr: false,
		},
		{
			name: "valid config with auth key",
			config: &providers.ProviderConfig{
				Name:    "tailscale",
				AuthKey: "tskey-auth-123456",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: &providers.ProviderConfig{
				Name:    "tailscale",
				AuthKey: "tskey-auth-123456",
				Extra: map[string]string{
					"hostname": "my-device",
				},
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
		Name: "tailscale",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Get connection info
	// This will fail if tailscale is not installed, which is expected in many test environments
	info, err := provider.GetConnectionInfo()

	if err != nil {
		// If tailscale is not installed, we should get ErrNotInstalled or ErrCommandFailed
		if err != providers.ErrNotInstalled && !isCommandError(err) {
			t.Logf("GetConnectionInfo() error = %v (expected if tailscale not installed)", err)
		}
		return
	}

	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil without error")
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
		Name: "tailscale",
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
		"error":         true,
		"Running":       true,
		"Stopped":       true,
		"NeedsLogin":    true,
		"NoState":       true,
		"Starting":      true,
	}
	if !validStatuses[health.Status] {
		t.Logf("HealthCheck() status = %q (might be valid tailscale status)", health.Status)
	}

	// Message should not be empty
	if health.Message == "" {
		t.Error("HealthCheck() message is empty")
	}

	// If not installed, healthy should be false
	if health.Status == "not_installed" && health.Healthy {
		t.Error("HealthCheck() healthy = true when status is not_installed")
	}

	// If Running, healthy should be true
	if health.Status == "Running" && !health.Healthy {
		t.Error("HealthCheck() healthy = false when status is Running")
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
				Name:    "tailscale",
				AuthKey: "tskey-test-123",
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

	if config.Name != "tailscale" {
		t.Errorf("GetConfig() name = %q, want %q", config.Name, "tailscale")
	}
}

func TestTailscaleStatus_Marshal(t *testing.T) {
	// Test that TailscaleStatus can be marshaled/unmarshaled
	status := TailscaleStatus{
		BackendState: "Running",
	}
	status.Self.HostName = "test-host"
	status.Self.DNSName = "test-host.tailnet.ts.net"
	status.Self.TailscaleIPs = []string{"100.64.0.1"}

	status.Peer = map[string]struct {
		HostName string `json:"HostName"`
		DNSName  string `json:"DNSName"`
	}{
		"peer1": {
			HostName: "peer-host",
			DNSName:  "peer-host.tailnet.ts.net",
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded TailscaleStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.BackendState != status.BackendState {
		t.Errorf("decoded BackendState = %q, want %q", decoded.BackendState, status.BackendState)
	}
	if decoded.Self.HostName != status.Self.HostName {
		t.Errorf("decoded Self.HostName = %q, want %q", decoded.Self.HostName, status.Self.HostName)
	}
	if decoded.Self.DNSName != status.Self.DNSName {
		t.Errorf("decoded Self.DNSName = %q, want %q", decoded.Self.DNSName, status.Self.DNSName)
	}
	if len(decoded.Self.TailscaleIPs) != len(status.Self.TailscaleIPs) {
		t.Errorf("decoded Self.TailscaleIPs length = %d, want %d", len(decoded.Self.TailscaleIPs), len(status.Self.TailscaleIPs))
	}
}

func TestIsConnected(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name: "tailscale",
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// IsConnected checks if status is "Running"
	connected := provider.IsConnected()

	// This is expected to be false in most test environments
	// unless tailscale is actually running
	if connected {
		t.Log("IsConnected() = true (tailscale is running)")
	} else {
		t.Log("IsConnected() = false (expected if tailscale not running)")
	}

	// The result should be deterministic for the current state
	connected2 := provider.IsConnected()
	if connected != connected2 {
		t.Error("IsConnected() returned inconsistent results")
	}
}

// Helper function to check if error is a command error
func isCommandError(err error) bool {
	if err == nil {
		return false
	}
	return err == providers.ErrCommandFailed ||
		   err == providers.ErrInvalidResponse ||
		   containsString(err.Error(), "command execution failed") ||
		   containsString(err.Error(), "invalid response") ||
		   containsString(err.Error(), "failed to get status")
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
