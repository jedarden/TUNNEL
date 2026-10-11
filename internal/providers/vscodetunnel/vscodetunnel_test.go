package vscodetunnel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	expected := "vscode-tunnel"
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
					"machineName": "my-machine",
				},
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "wrong provider name",
			config: &providers.ProviderConfig{
				Name: "ssh-forward",
			},
			wantErr: true,
			errMsg:  "provider name must be \"vscode-tunnel\"",
		},
		{
			name: "invalid license setting",
			config: &providers.ProviderConfig{
				Name:  "vscode-tunnel",
				Extra: map[string]string{"acceptLicense": "sometimes"},
			},
			wantErr: true,
			errMsg:  "acceptLicense must be a boolean",
		},
		{
			name: "valid config with name only",
			config: &providers.ProviderConfig{
				Name: "vscode-tunnel",
			},
			wantErr: false,
		},
		{
			name: "valid config with machine name",
			config: &providers.ProviderConfig{
				Name: "vscode-tunnel",
				Extra: map[string]string{
					"machineName": "my-machine",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with machine and host",
			config: &providers.ProviderConfig{
				Name:       "vscode-tunnel",
				RemoteHost: "tunnel.example.com",
				Extra: map[string]string{
					"machineName": "dev-machine",
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
	// Status will be "connected" if VS Code tunnel is running, "disconnected" otherwise
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

	// If VS Code tunnel is running and connected, check for expected fields
	if info.Status == "connected" {
		if info.Extra["type"] != "vscode-tunnel" {
			t.Errorf("Extra[type] = %v, want %v", info.Extra["type"], "vscode-tunnel")
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

	// If VS Code CLI is not installed, verify the response reflects that
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

	// If VS Code CLI is installed but not connected, verify status
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

	// If VS Code tunnel is running and connected, verify the response
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
		Name: "vscode-tunnel",
		Extra: map[string]string{
			"machineName": "my-dev-machine",
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
	if retrieved.Extra["machineName"] != config.Extra["machineName"] {
		t.Errorf("Config.Extra[machineName] = %q, want %q", retrieved.Extra["machineName"], config.Extra["machineName"])
	}
}

func TestConfigure_Nil(t *testing.T) {
	provider := New()

	err := provider.Configure(nil)
	if err == nil {
		t.Error("Configure(nil) should return error")
	}
}

func TestConfigureRejectsInvalidConfigWithoutApplyingIt(t *testing.T) {
	provider := New()

	err := provider.Configure(&providers.ProviderConfig{
		Name: "ssh-forward",
	})
	if err == nil {
		t.Fatal("Configure() accepted a config for a different provider")
	}

	config, err := provider.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if config.Name != "vscode-tunnel" {
		t.Fatalf("invalid config was applied: Name = %q", config.Name)
	}

	err = provider.Configure(&providers.ProviderConfig{
		Name:  "vscode-tunnel",
		Extra: map[string]string{"acceptLicense": "sometimes"},
	})
	if err == nil {
		t.Fatal("Configure() accepted a non-boolean acceptLicense value")
	}
	config, err = provider.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if config.Name != "vscode-tunnel" || len(config.Extra) != 0 {
		t.Fatalf("invalid config was applied: %#v", config)
	}
}

func TestMissingCodeExecutableIsReported(t *testing.T) {
	previous := codeBinary
	codeBinary = filepath.Join(t.TempDir(), "missing-code")
	t.Cleanup(func() { codeBinary = previous })

	provider := New()
	if provider.IsInstalled() {
		t.Fatal("IsInstalled() = true for a missing executable")
	}
	if err := provider.Connect(); !errors.Is(err, providers.ErrNotInstalled) {
		t.Fatalf("Connect() error = %v, want ErrNotInstalled", err)
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if health.Status != "not_installed" || health.Healthy || !strings.Contains(health.Message, "VS Code CLI") {
		t.Fatalf("HealthCheck() = %#v, want a clear not_installed status", health)
	}
}

func TestCodeExecutableIsDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "code")
	if err := os.WriteFile(path, []byte(`#!/bin/sh
if [ "$1" = "--version" ]; then
	echo 1.99.0
	exit 0
fi
exit 0
`), 0o700); err != nil {
		t.Fatalf("write fake code executable: %v", err)
	}
	previous := codeBinary
	codeBinary = path
	t.Cleanup(func() { codeBinary = previous })

	provider := New()
	if !provider.IsInstalled() {
		t.Fatal("IsInstalled() = false for a present code executable")
	}
	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if health.Status != "ready" || !strings.Contains(health.Message, "1.99.0") {
		t.Fatalf("HealthCheck() = %#v, want ready status with CLI version", health)
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
