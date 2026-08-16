package reversessh

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
	if provider.cmd != nil {
		t.Error("cmd should be nil initially")
	}
}

func TestName(t *testing.T) {
	provider := New()
	expected := "reverse-ssh"
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
				Extra: map[string]string{
					"relayServer": "relay.example.com",
				},
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "valid config with name only",
			config: &providers.ProviderConfig{
				Name: "reverse-ssh",
			},
			wantErr: false,
		},
		{
			name: "valid config with relay server",
			config: &providers.ProviderConfig{
				Name:  "reverse-ssh",
				Extra: map[string]string{
					"relayServer": "relay.example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with full relay settings",
			config: &providers.ProviderConfig{
				Name:  "reverse-ssh",
				Extra: map[string]string{
					"relayServer":    "relay.example.com",
					"relayPort":      "2222",
					"relayUsername":  "tunneluser",
					"remotePort":     "2223",
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

func TestGetConnectionInfo_GracefulHandling(t *testing.T) {
	provider := New()

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Errorf("GetConnectionInfo() unexpected error = %v", err)
	}
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil info")
	}
	// Status will be "connected" if reverse SSH is running, "disconnected" otherwise
	// Both states are valid, just check that a status is set
	if info.Status == "" {
		t.Error("Status is empty")
	}
	if info.Extra == nil {
		t.Fatal("Extra map is nil")
	}
}

func TestGetConnectionInfo_Connected(t *testing.T) {
	provider := New()

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Errorf("GetConnectionInfo() unexpected error = %v", err)
	}
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil info")
	}

	// If reverse SSH is running and connected, check for expected fields
	if info.Status == "connected" {
		if info.Extra["type"] != "reverse-ssh-tunnel" {
			t.Errorf("Extra[type] = %v, want %v", info.Extra["type"], "reverse-ssh-tunnel")
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

	// If reverse SSH is running and connected, verify the response
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
		Name:  "reverse-ssh",
		Extra: map[string]string{
			"relayServer": "relay.example.com",
			"relayPort":   "2222",
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
	if retrieved.Extra == nil {
		t.Fatal("Config.Extra is nil")
	}
	if retrieved.Extra["relayServer"] != config.Extra["relayServer"] {
		t.Errorf("Config.Extra[relayServer] = %q, want %q", retrieved.Extra["relayServer"], config.Extra["relayServer"])
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
