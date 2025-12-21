package providers

import (
	"time"
)

// Category represents the type of provider
type Category string

const (
	CategoryVPN    Category = "vpn"
	CategoryTunnel Category = "tunnel"
	CategoryDirect Category = "direct"
)

// Provider defines the interface that all network providers must implement
type Provider interface {
	Name() string
	Category() Category

	// Lifecycle
	Install() error
	Uninstall() error
	IsInstalled() bool

	// Configuration
	Configure(config *ProviderConfig) error
	GetConfig() (*ProviderConfig, error)
	ValidateConfig(config *ProviderConfig) error

	// Connection
	Connect() error
	Disconnect() error
	IsConnected() bool
	GetConnectionInfo() (*ConnectionInfo, error)

	// Health
	HealthCheck() (*HealthStatus, error)
	GetLogs(since time.Time) ([]LogEntry, error)
}

// ProviderConfig holds configuration for a provider
type ProviderConfig struct {
	Name       string            `json:"name"`
	AuthToken  string            `json:"auth_token,omitempty"`
	AuthKey    string            `json:"auth_key,omitempty"`
	NetworkID  string            `json:"network_id,omitempty"`
	TunnelName string            `json:"tunnel_name,omitempty"`
	RemoteHost string            `json:"remote_host,omitempty"`
	RemotePort int               `json:"remote_port,omitempty"`
	LocalPort  int               `json:"local_port,omitempty"`
	ConfigFile string            `json:"config_file,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// ConnectionInfo contains information about the current connection
type ConnectionInfo struct {
	Status        string                 `json:"status"`
	ConnectedAt   time.Time              `json:"connected_at,omitempty"`
	LocalIP       string                 `json:"local_ip,omitempty"`
	RemoteIP      string                 `json:"remote_ip,omitempty"`
	TunnelURL     string                 `json:"tunnel_url,omitempty"`
	InterfaceName string                 `json:"interface_name,omitempty"`
	Peers         []string               `json:"peers,omitempty"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// HealthStatus represents the health of the provider
type HealthStatus struct {
	Healthy       bool                   `json:"healthy"`
	Status        string                 `json:"status"`
	Message       string                 `json:"message,omitempty"`
	LastCheck     time.Time              `json:"last_check"`
	Latency       time.Duration          `json:"latency,omitempty"`
	BytesSent     uint64                 `json:"bytes_sent,omitempty"`
	BytesReceived uint64                 `json:"bytes_received,omitempty"`
	Metrics       map[string]interface{} `json:"metrics,omitempty"`
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	name     string
	category Category
	config   *ProviderConfig
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(name string, category Category) *BaseProvider {
	return &BaseProvider{
		name:     name,
		category: category,
		config:   &ProviderConfig{Name: name},
	}
}

// Name returns the provider name
func (b *BaseProvider) Name() string {
	return b.name
}

// Category returns the provider category
func (b *BaseProvider) Category() Category {
	return b.category
}

// Configure sets the provider configuration
func (b *BaseProvider) Configure(config *ProviderConfig) error {
	if config == nil {
		return ErrInvalidConfig
	}
	b.config = config
	return nil
}

// GetConfig returns the current configuration
func (b *BaseProvider) GetConfig() (*ProviderConfig, error) {
	if b.config == nil {
		return nil, ErrNoConfig
	}
	return b.config, nil
}

// ValidateConfig validates the configuration
func (b *BaseProvider) ValidateConfig(config *ProviderConfig) error {
	if config == nil {
		return ErrInvalidConfig
	}
	if config.Name == "" {
		return ErrMissingName
	}
	return nil
}
