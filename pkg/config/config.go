package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Version     string                  `yaml:"version"`
	Settings    Settings                `yaml:"settings"`
	Credentials CredentialConfig        `yaml:"credentials"`
	Methods     map[string]MethodConfig `yaml:"methods"`
	SSH         SSHConfig               `yaml:"ssh"`
	Monitoring  MonitoringConfig        `yaml:"monitoring"`

	mu       sync.RWMutex
	filePath string
	watcher  *fsnotify.Watcher
	onChange []func(*Config)
}

// Settings contains general application settings
type Settings struct {
	DefaultMethod string `yaml:"default_method"`
	AutoReconnect bool   `yaml:"auto_reconnect"`
	LogLevel      string `yaml:"log_level"`
	Theme         string `yaml:"theme"`
}

// CredentialConfig contains credential store configuration
type CredentialConfig struct {
	Store      string `yaml:"store"`      // keyring, file, env
	BaseDir    string `yaml:"base_dir"`   // For file store
	Passphrase string `yaml:"passphrase"` // For file store encryption
}

// MethodConfig contains configuration for each authentication method
type MethodConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Priority   int               `yaml:"priority"`     // For failover ordering
	AuthKeyRef string            `yaml:"auth_key_ref"` // Reference to credential store
	ExtraArgs  []string          `yaml:"extra_args"`
	Settings   map[string]string `yaml:"settings"`
}

// SSHConfig contains SSH-specific configuration
type SSHConfig struct {
	Port                 int      `yaml:"port"`
	HostKeyPath          string   `yaml:"host_key_path"`
	AuthorizedKeys       string   `yaml:"authorized_keys"`
	AllowedUsers         []string `yaml:"allowed_users"`
	MaxSessions          int      `yaml:"max_sessions"`
	IdleTimeout          int      `yaml:"idle_timeout"` // seconds
	KeepAlive            int      `yaml:"keep_alive"`   // seconds
	AllowTCPForwarding   bool     `yaml:"allow_tcp_forwarding"`
	AllowAgentForwarding bool     `yaml:"allow_agent_forwarding"`
}

// MonitoringConfig contains monitoring and audit configuration
type MonitoringConfig struct {
	Enabled        bool   `yaml:"enabled"`
	AuditLog       string `yaml:"audit_log"`
	Syslog         bool   `yaml:"syslog"`
	SyslogServer   string `yaml:"syslog_server"`
	MetricsEnabled bool   `yaml:"metrics_enabled"`
	MetricsPort    int    `yaml:"metrics_port"`
}

var (
	defaultConfigPath = filepath.Join(os.Getenv("HOME"), ".config", "tunnel", "config.yaml")
)

// Load loads configuration from the specified path
func Load(path string) (*Config, error) {
	if path == "" {
		path = defaultConfigPath
	}

	// Ensure config directory exists
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}

	// If config doesn't exist, create default
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := createDefaultConfig(path); err != nil {
			return nil, fmt.Errorf("create default config: %w", err)
		}
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.filePath = path

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// validateConfig performs validation without locking
func validateConfig(c *Config) error {
	// Validate version
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.Settings.LogLevel] {
		return fmt.Errorf("invalid log level: %s", c.Settings.LogLevel)
	}

	// Validate default method exists
	if c.Settings.DefaultMethod != "" {
		if _, ok := c.Methods[c.Settings.DefaultMethod]; !ok {
			return fmt.Errorf("default method %s not found in methods", c.Settings.DefaultMethod)
		}
	}

	// Validate credential store type
	validStores := map[string]bool{
		"keyring": true, "file": true, "env": true,
	}
	if !validStores[c.Credentials.Store] {
		return fmt.Errorf("invalid credential store: %s", c.Credentials.Store)
	}

	// Validate SSH port
	if c.SSH.Port < 1 || c.SSH.Port > 65535 {
		return fmt.Errorf("invalid SSH port: %d", c.SSH.Port)
	}

	// Validate monitoring metrics port if enabled
	if c.Monitoring.MetricsEnabled {
		if c.Monitoring.MetricsPort < 1 || c.Monitoring.MetricsPort > 65535 {
			return fmt.Errorf("invalid metrics port: %d", c.Monitoring.MetricsPort)
		}
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return validateConfig(c)
}

// Save saves the current configuration to file
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(c.filePath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// Watch starts watching the config file for changes
func (c *Config) Watch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	c.watcher = watcher

	if err := watcher.Add(c.filePath); err != nil {
		return fmt.Errorf("watch config file: %w", err)
	}

	go c.watchLoop()

	return nil
}

// watchLoop handles file system events
func (c *Config) watchLoop() {
	debounce := time.NewTimer(0)
	<-debounce.C // drain initial timer

	for {
		select {
		case event, ok := <-c.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				// Debounce rapid changes
				debounce.Reset(100 * time.Millisecond)
			}

		case <-debounce.C:
			// Reload configuration
			if err := c.Reload(); err != nil {
				// Log error but don't stop watching
				fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
			}

		case err, ok := <-c.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "Config watcher error: %v\n", err)
		}
	}
}

// Reload reloads configuration from file
func (c *Config) Reload() error {
	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var newCfg Config
	if err := yaml.Unmarshal(data, &newCfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Validate without locking (newCfg is a local variable)
	if err := validateConfig(&newCfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	c.mu.Lock()
	// Update config fields individually to preserve mutex
	c.Version = newCfg.Version
	c.Settings = newCfg.Settings
	c.Credentials = newCfg.Credentials
	c.Methods = newCfg.Methods
	c.SSH = newCfg.SSH
	c.Monitoring = newCfg.Monitoring
	// filePath, watcher, onChange, and mu are preserved automatically

	// Save onChange callbacks before unlock
	callbacks := make([]func(*Config), len(c.onChange))
	copy(callbacks, c.onChange)
	c.mu.Unlock()

	// Notify listeners
	for _, callback := range callbacks {
		callback(c)
	}

	return nil
}

// OnChange registers a callback to be called when configuration changes
func (c *Config) OnChange(callback func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChange = append(c.onChange, callback)
}

// Close closes the config watcher
func (c *Config) Close() error {
	if c.watcher != nil {
		return c.watcher.Close()
	}
	return nil
}

// GetMethod returns the configuration for a specific method
func (c *Config) GetMethod(name string) (MethodConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	method, ok := c.Methods[name]
	return method, ok
}

// GetEnabledMethods returns all enabled methods sorted by priority
func (c *Config) GetEnabledMethods() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	type methodPriority struct {
		name     string
		priority int
	}

	var methods []methodPriority
	for name, config := range c.Methods {
		if config.Enabled {
			methods = append(methods, methodPriority{
				name:     name,
				priority: config.Priority,
			})
		}
	}

	// Sort by priority (higher first)
	for i := 0; i < len(methods); i++ {
		for j := i + 1; j < len(methods); j++ {
			if methods[j].priority > methods[i].priority {
				methods[i], methods[j] = methods[j], methods[i]
			}
		}
	}

	result := make([]string, len(methods))
	for i, m := range methods {
		result[i] = m.name
	}

	return result
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(path string) error {
	cfg := GetDefaultConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}

	return nil
}
