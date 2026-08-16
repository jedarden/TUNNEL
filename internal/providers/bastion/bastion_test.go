package bastion

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
	expected := "bastion"
	if got := provider.Name(); got != expected {
		t.Errorf("Name() = %q, want %q", got, expected)
	}
}

func TestCategory(t *testing.T) {
	provider := New()
	expected := providers.CategorySSH
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
				Name: "bastion",
			},
			wantErr: false,
		},
		{
			name: "valid config with SSH port",
			config: &providers.ProviderConfig{
				Name:      "bastion",
				LocalPort: 22,
			},
			wantErr: false,
		},
		{
			name: "valid config with remote host",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "bastion.example.com",
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
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGetConnectionInfo_GracefulHandling(t *testing.T) {
	provider := New()

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Errorf("GetConnectionInfo() unexpected error = %v", err)
	}
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil info")
	}
	// Status will be "connected" if SSH is running, "disconnected" otherwise
	// Both states are valid, just check that a status is set
	if info.Status == "" {
		t.Error("Status is empty")
	}
	if info.Extra == nil {
		t.Fatal("Extra map is nil")
	}
}

func TestGetConnectionInfo_Connected(t *testing.T) {
	// This test will pass even if SSH is not installed
	// The method gracefully handles the not installed case
	provider := New()

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Errorf("GetConnectionInfo() unexpected error = %v", err)
	}
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil info")
	}

	// If SSH is running and connected, check for expected fields
	if info.Status == "connected" {
		if info.Extra["type"] != "bastion-host" {
			t.Errorf("Extra[type] = %v, want %v", info.Extra["type"], "bastion-host")
		}
		if info.Extra["mode"] != "jump-server" {
			t.Errorf("Extra[mode] = %v, want %v", info.Extra["mode"], "jump-server")
		}
		if info.Extra["port"] != 22 {
			t.Errorf("Extra[port] = %v, want %v", info.Extra["port"], 22)
		}
	}
}

func TestHealthCheck_NotInstalled(t *testing.T) {
	provider := New()

	health, err := provider.HealthCheck()
	if err != nil {
		t.Errorf("HealthCheck() unexpected error = %v", err)
	}
	if health == nil {
		t.Fatal("HealthCheck() returned nil health")
	}

	// Check timestamp is recent
	if time.Since(health.LastCheck) > 5*time.Second {
		t.Errorf("LastCheck = %v, too old", health.LastCheck)
	}

	// If SSH is not installed, verify the response reflects that
	if !provider.IsInstalled() {
		if health.Healthy != false {
			t.Errorf("Healthy = %v, want false when not installed", health.Healthy)
		}
		if health.Status != "not_installed" {
			t.Errorf("Status = %q, want %q", health.Status, "not_installed")
		}
		if health.Message == "" {
			t.Error("Message is empty")
		}
	}
}

func TestHealthCheck_Ready(t *testing.T) {
	provider := New()

	health, err := provider.HealthCheck()
	if err != nil {
		t.Errorf("HealthCheck() unexpected error = %v", err)
	}
	if health == nil {
		t.Fatal("HealthCheck() returned nil health")
	}

	// Check timestamp is recent
	if time.Since(health.LastCheck) > 5*time.Second {
		t.Errorf("LastCheck = %v, too old", health.LastCheck)
	}

	// If SSH is installed but not connected, verify status
	if provider.IsInstalled() && !provider.IsConnected() {
		if health.Status != "ready" {
			t.Errorf("Status = %q, want %q", health.Status, "ready")
		}
		if health.Message == "" {
			t.Error("Message is empty")
		}
	}
}

func TestHealthCheck_Connected(t *testing.T) {
	provider := New()

	health, err := provider.HealthCheck()
	if err != nil {
		t.Errorf("HealthCheck() unexpected error = %v", err)
	}
	if health == nil {
		t.Fatal("HealthCheck() returned nil health")
	}

	// If SSH is installed and running, verify the response
	// Check both IsInstalled() and IsConnected() to avoid false positives
	// where pgrep finds sshd but which sshd fails (not properly installed)
	if provider.IsInstalled() && provider.IsConnected() {
		if health.Healthy != true {
			t.Errorf("Healthy = %v, want true when connected", health.Healthy)
		}
		if health.Status != "connected" {
			t.Errorf("Status = %q, want %q", health.Status, "connected")
		}
		if health.Message == "" {
			t.Error("Message is empty")
		}
	}
}

func TestGetLogs(t *testing.T) {
	provider := New()

	since := time.Now().Add(-1 * time.Hour)
	logs, err := provider.GetLogs(since)

	if err != nil {
		t.Errorf("GetLogs() unexpected error = %v", err)
	}
	if logs == nil {
		t.Fatal("GetLogs() returned nil logs")
	}
}

func TestConfigure(t *testing.T) {
	provider := New()

	config := &providers.ProviderConfig{
		Name:      "bastion",
		LocalPort: 22,
	}

	err := provider.Configure(config)
	if err != nil {
		t.Errorf("Configure() unexpected error = %v", err)
	}

	retrieved, err := provider.GetConfig()
	if err != nil {
		t.Errorf("GetConfig() unexpected error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetConfig() returned nil")
	}
	if retrieved.Name != config.Name {
		t.Errorf("Config.Name = %q, want %q", retrieved.Name, config.Name)
	}
	if retrieved.LocalPort != config.LocalPort {
		t.Errorf("Config.LocalPort = %d, want %d", retrieved.LocalPort, config.LocalPort)
	}
}

func TestConfigure_Nil(t *testing.T) {
	provider := New()

	err := provider.Configure(nil)
	if err == nil {
		t.Error("Configure(nil) should return error")
	}
}

func TestGetConfig_Default(t *testing.T) {
	provider := New()

	config, err := provider.GetConfig()
	if err != nil {
		t.Errorf("GetConfig() unexpected error = %v", err)
	}
	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
