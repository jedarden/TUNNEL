// Package tunnel provides a public API for the tunnel connection manager
package tunnel

import (
	"context"
	"time"

	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
)

// Re-export core types
type (
	ConnectionManager = core.ConnectionManager
	Connection        = core.Connection
	ConnectionState   = core.ConnectionState
	ConnectionMetrics = core.ConnectionMetrics
	Config            = core.Config
	ConnectionEvent   = core.ConnectionEvent
	EventType         = core.EventType
	EventPublisher    = core.EventPublisher
	EventSubscriber   = core.EventSubscriber
)

// Re-export provider types
type (
	Provider         = providers.Provider
	ProviderConfig   = providers.ProviderConfig
	ConnectionInfo   = providers.ConnectionInfo
	HealthStatus     = providers.HealthStatus
	Category         = providers.Category
)

// Re-export registry types
type (
	Registry     = registry.Registry
	ProviderInfo = registry.ProviderInfo
)

// Connection states
const (
	StateDisconnected  = core.StateDisconnected
	StateConnecting    = core.StateConnecting
	StateConnected     = core.StateConnected
	StateReconnecting  = core.StateReconnecting
	StateFailed        = core.StateFailed
)

// Event types
const (
	EventConnected     = core.EventConnected
	EventDisconnected  = core.EventDisconnected
	EventReconnecting  = core.EventReconnecting
	EventFailover      = core.EventFailover
	EventMetricsUpdate = core.EventMetricsUpdate
	EventError         = core.EventError
	EventStateChange   = core.EventStateChange
	EventPrimaryChange = core.EventPrimaryChange
)

// Provider categories
const (
	CategoryVPN    = providers.CategoryVPN
	CategoryTunnel = providers.CategoryTunnel
	CategoryDirect = providers.CategoryDirect
)

// ManagerConfig wraps the internal manager config
type ManagerConfig struct {
	EnableMetrics   bool
	EnableFailover  bool
	FailoverConfig  *core.FailoverConfig
	MetricsInterval time.Duration
	EventBufferSize int
}

// DefaultManagerConfig returns a manager config with sensible defaults
func DefaultManagerConfig() *ManagerConfig {
	cfg := core.DefaultManagerConfig()
	return &ManagerConfig{
		EnableMetrics:   cfg.EnableMetrics,
		EnableFailover:  cfg.EnableFailover,
		FailoverConfig:  cfg.FailoverConfig,
		MetricsInterval: cfg.MetricsInterval,
		EventBufferSize: cfg.EventBufferSize,
	}
}

// Manager wraps the internal connection manager
type Manager struct {
	*core.DefaultConnectionManager
}

// NewManager creates a new connection manager
func NewManager(config *ManagerConfig) *Manager {
	var internalConfig *core.ManagerConfig
	if config == nil {
		internalConfig = core.DefaultManagerConfig()
	} else {
		internalConfig = &core.ManagerConfig{
			EnableMetrics:   config.EnableMetrics,
			EnableFailover:  config.EnableFailover,
			FailoverConfig:  config.FailoverConfig,
			MetricsInterval: config.MetricsInterval,
			EventBufferSize: config.EventBufferSize,
		}
	}

	return &Manager{
		DefaultConnectionManager: core.NewConnectionManager(internalConfig),
	}
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return core.DefaultConfig()
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return registry.NewRegistry()
}

// NewConnection creates a new connection instance
func NewConnection(id, method string, localPort int, remoteHost string, remotePort int) *Connection {
	return core.NewConnection(id, method, localPort, remoteHost, remotePort)
}

// ConnectionProvider wraps the internal connection provider interface
type ConnectionProvider interface {
	Name() string
	Connect(ctx context.Context, config *Config) (*Connection, error)
	Disconnect(conn *Connection) error
	IsHealthy(conn *Connection) bool
}

// RegisterProvider registers a connection provider with the manager
func (m *Manager) RegisterProvider(provider ConnectionProvider) {
	m.DefaultConnectionManager.RegisterProvider(&providerAdapter{provider})
}

// providerAdapter adapts a public ConnectionProvider to the internal interface
type providerAdapter struct {
	provider ConnectionProvider
}

func (a *providerAdapter) Name() string {
	return a.provider.Name()
}

func (a *providerAdapter) Connect(ctx context.Context, config *core.Config) (*core.Connection, error) {
	return a.provider.Connect(ctx, config)
}

func (a *providerAdapter) Disconnect(conn *core.Connection) error {
	return a.provider.Disconnect(conn)
}

func (a *providerAdapter) IsHealthy(conn *core.Connection) bool {
	return a.provider.IsHealthy(conn)
}
