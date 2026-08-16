package wireguard

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
	if provider.interfaceName != "wg0" {
		t.Errorf("interfaceName = %q, want %q", provider.interfaceName, "wg0")
	}
}

func TestName(t *testing.T) {
	provider := New()
	expected := "wireguard"
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
				ConfigFile: "/etc/wireguard/wg0.conf",
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "valid config with name only",
			config: &providers.ProviderConfig{
				Name: "wireguard",
			},
			wantErr: false,
		},
		{
			name: "valid config with interface",
			config: &providers.ProviderConfig{
				Name:  "wireguard",
				Extra: map[string]string{
					"interface": "wg0",
				},
			},
			wantErr: false,
		},
		{
			name: "config file not found",
			config: &providers.ProviderConfig{
				Name:       "wireguard",
				ConfigFile: "/nonexistent/path/to/wg0.conf",
			},
			wantErr: true,
			errMsg:  "config file not found",
		},
		{
			name: "valid config with peers",
			config: &providers.ProviderConfig{
				Name:  "wireguard",
				Extra: map[string]string{
					"peers": "peer1,peer2,peer3",
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

	// If WireGuard is not installed, it should return ErrNotInstalled
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

func TestGetConnectionInfo_Disconnected(t *testing.T) {
	provider := New()

	// If WireGuard is installed but not connected
	if provider.IsInstalled() && !provider.IsConnected() {
		info, err := provider.GetConnectionInfo()
		if err != nil {
			t.Errorf("GetConnectionInfo() unexpected error = %v", err)
		}
		if info == nil {
			t.Fatal("GetConnectionInfo() returned nil info")
		}
		if info.Status != "disconnected" {
			t.Errorf("Status = %q, want %q", info.Status, "disconnected")
		}
		if info.InterfaceName != provider.interfaceName {
			t.Errorf("InterfaceName = %q, want %q", info.InterfaceName, provider.interfaceName)
		}
		if info.Extra == nil {
			t.Fatal("Extra map is nil")
		}
	}
}

func TestGetConnectionInfo_Connected(t *testing.T) {
	provider := New()

	// If WireGuard is installed and connected
	if provider.IsInstalled() && provider.IsConnected() {
		info, err := provider.GetConnectionInfo()
		if err != nil {
			t.Errorf("GetConnectionInfo() unexpected error = %v", err)
		}
		if info == nil {
			t.Fatal("GetConnectionInfo() returned nil info")
		}
		if info.Status != "connected" {
			t.Errorf("Status = %q, want %q", info.Status, "connected")
		}
		if info.InterfaceName != provider.interfaceName {
			t.Errorf("InterfaceName = %q, want %q", info.InterfaceName, provider.interfaceName)
		}
		// Just check that Extra map is initialized; actual fields depend on system state
		if info.Extra == nil {
			t.Fatal("Extra map is nil")
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

	// If WireGuard is not installed, verify the response reflects that
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

func TestHealthCheck_Disconnected(t *testing.T) {
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

	// If WireGuard is installed but not connected
	if provider.IsInstalled() && !provider.IsConnected() {
		if health.Healthy != false {
			t.Errorf("Healthy = %v, want false when disconnected", health.Healthy)
		}
		if health.Status != "disconnected" {
			t.Errorf("Status = %q, want %q", health.Status, "disconnected")
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

	// If WireGuard is running and connected
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
		// Metrics map should be initialized
		if health.Metrics == nil {
			t.Error("Metrics map is nil")
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
		Name:  "wireguard",
		Extra: map[string]string{
			"interface": "wg0",
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
