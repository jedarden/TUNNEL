package config

import (
	"os"
	"path/filepath"
)

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "tunnel")

	return &Config{
		Version: "1.0.0",

		Settings: Settings{
			DefaultMethod: "ssh-key",
			AutoReconnect: true,
			LogLevel:      "info",
			Theme:         "default",
		},

		Credentials: CredentialConfig{
			Store:      "keyring",
			BaseDir:    filepath.Join(configDir, "credentials"),
			Passphrase: "", // Will be prompted if needed
		},

		Methods: map[string]MethodConfig{
			"ssh-key": {
				Enabled:    true,
				Priority:   100,
				AuthKeyRef: "",
				ExtraArgs:  []string{},
				Settings:   map[string]string{},
			},
			"password": {
				Enabled:    true,
				Priority:   90,
				AuthKeyRef: "tunnel:ssh-password",
				ExtraArgs:  []string{},
				Settings:   map[string]string{},
			},
			"fido2": {
				Enabled:    false,
				Priority:   80,
				AuthKeyRef: "",
				ExtraArgs:  []string{},
				Settings: map[string]string{
					"require_user_verification": "true",
				},
			},
			"totp": {
				Enabled:    false,
				Priority:   70,
				AuthKeyRef: "tunnel:totp-secret",
				ExtraArgs:  []string{},
				Settings: map[string]string{
					"window": "1",
					"period": "30",
				},
			},
			"oauth": {
				Enabled:    false,
				Priority:   60,
				AuthKeyRef: "tunnel:oauth-token",
				ExtraArgs:  []string{},
				Settings: map[string]string{
					"provider":     "github",
					"client_id":    "",
					"redirect_uri": "http://localhost:8080/callback",
				},
			},
			"wireguard": {
				Enabled:    false,
				Priority:   50,
				AuthKeyRef: "tunnel:wg-private-key",
				ExtraArgs:  []string{},
				Settings: map[string]string{
					"endpoint":    "",
					"allowed_ips": "0.0.0.0/0",
				},
			},
			"tailscale": {
				Enabled:    false,
				Priority:   40,
				AuthKeyRef: "tunnel:tailscale-key",
				ExtraArgs:  []string{},
				Settings: map[string]string{
					"control_url": "https://controlplane.tailscale.com",
				},
			},
		},

		SSH: SSHConfig{
			Port:                 2222,
			HostKeyPath:          filepath.Join(configDir, "ssh_host_key"),
			AuthorizedKeys:       filepath.Join(homeDir, ".ssh", "authorized_keys"),
			AllowedUsers:         []string{},
			MaxSessions:          10,
			IdleTimeout:          300, // 5 minutes
			KeepAlive:            60,  // 1 minute
			AllowTCPForwarding:   true,
			AllowAgentForwarding: true,
		},

		Monitoring: MonitoringConfig{
			Enabled:        true,
			AuditLog:       filepath.Join(configDir, "audit.log"),
			Syslog:         false,
			SyslogServer:   "",
			MetricsEnabled: false,
			MetricsPort:    9090,
		},
	}
}

// MigrateConfig migrates configuration from older versions
func MigrateConfig(cfg *Config, fromVersion, toVersion string) error {
	// Add migration logic here as versions evolve
	// For now, just ensure all required fields are present

	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}

	if cfg.Settings.LogLevel == "" {
		cfg.Settings.LogLevel = "info"
	}

	if cfg.Settings.Theme == "" {
		cfg.Settings.Theme = "default"
	}

	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 2222
	}

	if cfg.SSH.MaxSessions == 0 {
		cfg.SSH.MaxSessions = 10
	}

	if cfg.SSH.IdleTimeout == 0 {
		cfg.SSH.IdleTimeout = 300
	}

	if cfg.SSH.KeepAlive == 0 {
		cfg.SSH.KeepAlive = 60
	}

	if cfg.Credentials.Store == "" {
		cfg.Credentials.Store = "keyring"
	}

	if cfg.Monitoring.MetricsPort == 0 {
		cfg.Monitoring.MetricsPort = 9090
	}

	// Ensure all default methods are present
	defaults := GetDefaultConfig()
	for name, method := range defaults.Methods {
		if _, ok := cfg.Methods[name]; !ok {
			if cfg.Methods == nil {
				cfg.Methods = make(map[string]MethodConfig)
			}
			cfg.Methods[name] = method
		}
	}

	return nil
}

// ValidateAndMigrate validates and migrates configuration if needed
func ValidateAndMigrate(cfg *Config) error {
	// Check if migration is needed
	if cfg.Version != "1.0.0" {
		if err := MigrateConfig(cfg, cfg.Version, "1.0.0"); err != nil {
			return err
		}
	}

	return cfg.Validate()
}
