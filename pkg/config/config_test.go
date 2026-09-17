package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Load config (should create default)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify default values
	if cfg.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", cfg.Version)
	}

	if cfg.Settings.DefaultMethod != "ssh-key" {
		t.Errorf("Expected default method ssh-key, got %s", cfg.Settings.DefaultMethod)
	}

	if cfg.SSH.Port != 2222 {
		t.Errorf("Expected SSH port 2222, got %d", cfg.SSH.Port)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name:      "valid config",
			config:    GetDefaultConfig(),
			expectErr: false,
		},
		{
			name: "invalid log level",
			config: &Config{
				Version: "1.0.0",
				Settings: Settings{
					LogLevel: "invalid",
				},
				SSH: SSHConfig{Port: 2222},
			},
			expectErr: true,
		},
		{
			name: "invalid SSH port",
			config: &Config{
				Version: "1.0.0",
				Settings: Settings{
					LogLevel: "info",
				},
				SSH: SSHConfig{Port: 70000},
			},
			expectErr: true,
		},
		{
			name: "invalid credential store",
			config: &Config{
				Version: "1.0.0",
				Settings: Settings{
					LogLevel: "info",
				},
				Credentials: CredentialConfig{
					Store: "invalid",
				},
				SSH: SSHConfig{Port: 2222},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create and save config
	cfg := GetDefaultConfig()
	cfg.filePath = configPath
	cfg.Settings.DefaultMethod = "password"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Settings.DefaultMethod != "password" {
		t.Errorf("Expected default method password, got %s", loaded.Settings.DefaultMethod)
	}
}

func TestGetEnabledMethods(t *testing.T) {
	cfg := GetDefaultConfig()

	// Enable specific methods with priorities
	cfg.Methods = map[string]MethodConfig{
		"ssh-key": {
			Enabled:  true,
			Priority: 100,
		},
		"password": {
			Enabled:  true,
			Priority: 90,
		},
		"totp": {
			Enabled:  false,
			Priority: 80,
		},
		"fido2": {
			Enabled:  true,
			Priority: 95,
		},
	}

	enabled := cfg.GetEnabledMethods()

	// Should return enabled methods sorted by priority (highest first)
	expected := []string{"ssh-key", "fido2", "password"}
	if len(enabled) != len(expected) {
		t.Fatalf("Expected %d methods, got %d", len(expected), len(enabled))
	}

	for i, method := range enabled {
		if method != expected[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expected[i], method)
		}
	}
}

func TestConfigOnChange(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Register change callback
	changed := false
	cfg.OnChange(func(c *Config) {
		changed = true
	})

	// Modify and save
	cfg.Settings.DefaultMethod = "password"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload
	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify callback was called
	if !changed {
		t.Error("OnChange callback was not called")
	}
}

func TestConfigWatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Start watching
	if err := cfg.Watch(); err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	defer cfg.Close()

	// Register change callback
	changed := make(chan bool, 1)
	cfg.OnChange(func(c *Config) {
		changed <- true
	})

	// Modify config file
	cfg.Settings.LogLevel = "debug"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Wait for change notification (with timeout)
	select {
	case <-changed:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Config change was not detected")
	}
}

func TestMigrateConfig(t *testing.T) {
	cfg := &Config{
		Version: "",
	}

	if err := MigrateConfig(cfg, "", "1.0.0"); err != nil {
		t.Fatalf("MigrateConfig failed: %v", err)
	}

	// Verify defaults were set
	if cfg.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", cfg.Version)
	}

	if cfg.Settings.LogLevel != "info" {
		t.Errorf("Expected log level info, got %s", cfg.Settings.LogLevel)
	}

	if cfg.SSH.Port != 2222 {
		t.Errorf("Expected SSH port 2222, got %d", cfg.SSH.Port)
	}
}

func TestMetricsPortValidation(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		enabled   bool
		expectErr bool
	}{
		{name: "default port, disabled", port: 9090, enabled: false, expectErr: false},
		{name: "default port, enabled", port: 9090, enabled: true, expectErr: false},
		{name: "high valid port, disabled", port: 65535, enabled: false, expectErr: false},
		{name: "port 1, enabled", port: 1, enabled: true, expectErr: false},
		// 0 means "unset" and is normalized to the default by Load/Reload;
		// Validate itself accepts it as such.
		{name: "unset port, disabled", port: 0, enabled: false, expectErr: false},
		{name: "unset port, enabled", port: 0, enabled: true, expectErr: false},
		{name: "negative port, disabled", port: -1, enabled: false, expectErr: true},
		{name: "negative port, enabled", port: -1, enabled: true, expectErr: true},
		{name: "port above range, disabled", port: 65536, enabled: false, expectErr: true},
		{name: "port above range, enabled", port: 70000, enabled: true, expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetDefaultConfig()
			cfg.Monitoring.MetricsPort = tt.port
			cfg.Monitoring.MetricsEnabled = tt.enabled

			err := cfg.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

// TestMetricsPortDefaultAppliedOnLoad pins the endpoint's documented
// defaulting: a config file that enables metrics without choosing a port
// loads successfully and serves on DefaultMetricsPort.
func TestMetricsPortDefaultAppliedOnLoad(t *testing.T) {
	configYAML := `version: "1.0.0"
settings:
  log_level: info
credentials:
  store: keyring
ssh:
  port: 2222
monitoring:
  metrics_enabled: true
`
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.Monitoring.MetricsEnabled {
		t.Fatal("metrics_enabled did not load as true")
	}
	if got := cfg.Monitoring.MetricsPort; got != DefaultMetricsPort {
		t.Errorf("MetricsPort = %d, want default %d", got, DefaultMetricsPort)
	}
}

// TestMetricsPortInvalidRejectedOnLoad pins that a typo'd metrics port fails
// config load even with the endpoint disabled, rather than resurfacing later
// as an endpoint that can never bind.
func TestMetricsPortInvalidRejectedOnLoad(t *testing.T) {
	configYAML := `version: "1.0.0"
settings:
  log_level: info
credentials:
  store: keyring
ssh:
  port: 2222
monitoring:
  metrics_enabled: false
  metrics_port: 70000
`
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	if _, err := Load(configPath); err == nil {
		t.Error("Load accepted metrics_port 70000 with metrics disabled; want validation error")
	}
}

// TestMetricsPortDefaultAppliedOnReload covers the reload path, which
// validates incoming config without going through Load.
func TestMetricsPortDefaultAppliedOnReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	enabledYAML := `version: "1.0.0"
settings:
  log_level: info
credentials:
  store: keyring
ssh:
  port: 2222
monitoring:
  metrics_enabled: true
`
	if err := os.WriteFile(configPath, []byte(enabledYAML), 0644); err != nil {
		t.Fatalf("rewriting config: %v", err)
	}

	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if got := cfg.Monitoring.MetricsPort; got != DefaultMetricsPort {
		t.Errorf("MetricsPort after reload = %d, want default %d", got, DefaultMetricsPort)
	}
}
