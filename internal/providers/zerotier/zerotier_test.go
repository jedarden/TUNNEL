package zerotier

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
	expected := "zerotier"
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
				NetworkID: "8056c2e21c000001",
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "missing network_id",
			config: &providers.ProviderConfig{
				Name: "zerotier",
			},
			wantErr: true,
			errMsg:  "network_id is required",
		},
		{
			name: "invalid network_id length",
			config: &providers.ProviderConfig{
				Name:      "zerotier",
				NetworkID: "abc",
			},
			wantErr: true,
			errMsg:  "network_id must be 16 characters",
		},
		{
			name: "valid config with network_id",
			config: &providers.ProviderConfig{
				Name:      "zerotier",
				NetworkID: "8056c2e21c000001",
			},
			wantErr: false,
		},
		{
			name: "valid config with full settings",
			config: &providers.ProviderConfig{
				Name:      "zerotier",
				NetworkID: "8056c2e21c000001",
				Extra: map[string]string{
					"network_name": "my-network",
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
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestGetConnectionInfo_NotInstalled(t *testing.T) {
	provider := New()

	// If ZeroTier is not installed, it should return ErrNotInstalled
	if !provider.IsInstalled() {
		info, err := provider.GetConnectionInfo()
		if err != providers.ErrNotInstalled {
			t.Errorf("GetConnectionInfo() error = %v, want ErrNotInstalled", err)
		}
		if info != nil {
			t.Error("GetConnectionInfo() should return nil info when not installed")
		}
	}
}

func TestGetConnectionInfo_Installed(t *testing.T) {
	provider := New()

	// If ZeroTier is installed
	if provider.IsInstalled() {
		info, err := provider.GetConnectionInfo()
		if err != nil {
			t.Errorf("GetConnectionInfo() unexpected error = %v", err)
		}
		if info == nil {
			t.Fatal("GetConnectionInfo() returned nil info")
		}
		// Status could be "ok", "requesting configuration", "disconnected", etc.
		// Just check that Extra map is initialized
		if info.Extra == nil {
			t.Fatal("Extra map is nil")
		}
	}
}

func TestGetConnectionInfo_Connected(t *testing.T) {
	provider := New()

	// If ZeroTier is installed and connected to a network
	if provider.IsInstalled() && provider.IsConnected() {
		info, err := provider.GetConnectionInfo()
		if err != nil {
			t.Errorf("GetConnectionInfo() unexpected error = %v", err)
		}
		if info == nil {
			t.Fatal("GetConnectionInfo() returned nil info")
		}
		// Status should be "ok" when connected (converted to lowercase)
		if info.Status != "ok" && info.Status != "connected" {
			t.Logf("Status = %q (note: may vary based on network state)", info.Status)
		}
		// Check for expected fields in Extra
		if info.Extra["network_id"] == nil && info.Extra["type"] == nil {
			t.Error("Extra map should contain network_id or type when connected")
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

	// If ZeroTier is not installed, verify the response reflects that
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

func TestHealthCheck_ServiceError(t *testing.T) {
	provider := New()

	// This test verifies the health check handles service errors gracefully
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

	// If service is not running but installed
	if provider.IsInstalled() && health.Status == "error" {
		if health.Healthy != false {
			t.Errorf("Healthy = %v, want false when service error", health.Healthy)
		}
		if health.Message == "" {
			t.Error("Message should not be empty when service has error")
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

	// If ZeroTier is running and connected
	if provider.IsConnected() {
		if health.Healthy != true {
			t.Errorf("Healthy = %v, want true when connected", health.Healthy)
		}
		if health.Status != "connected" {
			t.Errorf("Status = %q, want %q", health.Status, "connected")
		}
		if health.Message == "" {
			t.Error("Message is empty")
		}
		// Message should contain node ID when connected
		if !containsString(health.Message, "ZeroTier node") {
			t.Logf("Message = %q (may not contain node ID in all cases)", health.Message)
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
		Name:      "zerotier",
		NetworkID: "8056c2e21c000001",
		Extra: map[string]string{
			"network_name": "test-network",
		},
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
	if retrieved.NetworkID != config.NetworkID {
		t.Errorf("Config.NetworkID = %q, want %q", retrieved.NetworkID, config.NetworkID)
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
