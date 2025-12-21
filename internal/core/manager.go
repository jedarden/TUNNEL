package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConnectionManager defines the interface for managing tunnel connections
type ConnectionManager interface {
	// Single connection operations
	Start(method string, config *Config) (*Connection, error)
	Stop(connID string) error
	Restart(connID string) error
	Status(connID string) (*Connection, error)

	// Multi-connection for redundancy
	StartMultiple(methods []string, config *Config) ([]*Connection, error)
	StopAll() error

	// List and monitor
	List() ([]*Connection, error)
	Monitor(connID string) <-chan *ConnectionEvent

	// Failover
	SetPrimary(connID string) error
	GetPrimary() (*Connection, error)
	EnableAutoFailover(enabled bool)
}

// DefaultConnectionManager implements ConnectionManager
type DefaultConnectionManager struct {
	mu               sync.RWMutex
	connections      map[string]*Connection
	providers        map[string]ConnectionProvider // Provider implementations
	eventPublisher   *EventPublisher
	metricsCollector *DefaultMetricsCollector
	failoverManager  *FailoverManager
	config           *ManagerConfig
	ctx              context.Context
	cancel           context.CancelFunc
}

// ManagerConfig holds configuration for the connection manager
type ManagerConfig struct {
	EnableMetrics   bool
	EnableFailover  bool
	FailoverConfig  *FailoverConfig
	MetricsInterval time.Duration
	EventBufferSize int
}

// DefaultManagerConfig returns a manager config with sensible defaults
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		EnableMetrics:   true,
		EnableFailover:  true,
		FailoverConfig:  DefaultFailoverConfig(),
		MetricsInterval: 10 * time.Second,
		EventBufferSize: 100,
	}
}

// ConnectionProvider defines the interface for connection providers
type ConnectionProvider interface {
	// Name returns the provider name
	Name() string

	// Connect establishes a connection
	Connect(ctx context.Context, config *Config) (*Connection, error)

	// Disconnect tears down a connection
	Disconnect(conn *Connection) error

	// IsHealthy checks if a connection is healthy
	IsHealthy(conn *Connection) bool
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config *ManagerConfig) *DefaultConnectionManager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	publisher := NewEventPublisher(config.EventBufferSize)
	collector := NewMetricsCollector()

	var failover *FailoverManager
	if config.EnableFailover {
		failover = NewFailoverManager(config.FailoverConfig, publisher, collector)
	}

	manager := &DefaultConnectionManager{
		connections:      make(map[string]*Connection),
		providers:        make(map[string]ConnectionProvider),
		eventPublisher:   publisher,
		metricsCollector: collector,
		failoverManager:  failover,
		config:           config,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Start metrics collection
	if config.EnableMetrics {
		collector.Start(ctx, config.MetricsInterval)
	}

	// Start failover monitoring
	if config.EnableFailover && failover != nil {
		failover.Start()
	}

	return manager
}

// RegisterProvider registers a connection provider
func (m *DefaultConnectionManager) RegisterProvider(provider ConnectionProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.Name()] = provider
}

// Start establishes a new connection using the specified method
func (m *DefaultConnectionManager) Start(method string, config *Config) (*Connection, error) {
	m.mu.Lock()
	provider, exists := m.providers[method]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("provider %s not registered", method)
	}

	// Create connection using provider
	conn, err := provider.Connect(m.ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to start connection: %w", err)
	}

	// Register with manager
	m.mu.Lock()
	m.connections[conn.ID] = conn
	m.mu.Unlock()

	// Register with metrics collector
	if m.config.EnableMetrics {
		m.metricsCollector.RegisterConnection(conn)
	}

	// Register with failover manager
	if m.config.EnableFailover && m.failoverManager != nil {
		m.failoverManager.RegisterConnection(conn)
	}

	// Publish connected event
	event := NewEvent(EventConnected, conn.ID, conn,
		fmt.Sprintf("Connection %s started using %s", conn.ID, method))
	m.eventPublisher.Publish(event)

	return conn, nil
}

// Stop terminates a connection
func (m *DefaultConnectionManager) Stop(connID string) error {
	m.mu.Lock()
	conn, exists := m.connections[connID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("connection %s not found", connID)
	}

	provider, providerExists := m.providers[conn.Method]
	m.mu.Unlock()

	if !providerExists {
		return fmt.Errorf("provider %s not found", conn.Method)
	}

	// Disconnect using provider
	if err := provider.Disconnect(conn); err != nil {
		return fmt.Errorf("failed to stop connection: %w", err)
	}

	// Unregister from failover
	if m.config.EnableFailover && m.failoverManager != nil {
		m.failoverManager.UnregisterConnection(connID)
	}

	// Unregister from metrics
	if m.config.EnableMetrics {
		m.metricsCollector.UnregisterConnection(connID)
	}

	// Remove from manager
	m.mu.Lock()
	delete(m.connections, connID)
	m.mu.Unlock()

	// Publish disconnected event
	event := NewEvent(EventDisconnected, connID, nil,
		fmt.Sprintf("Connection %s stopped", connID))
	m.eventPublisher.Publish(event)

	return nil
}

// Restart reconnects an existing connection
func (m *DefaultConnectionManager) Restart(connID string) error {
	m.mu.RLock()
	conn, exists := m.connections[connID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}

	// Get the config from the connection
	config, ok := conn.Config.(*Config)
	if !ok || config == nil {
		config = DefaultConfig()
		config.RemoteHost = conn.RemoteHost
		config.RemotePort = conn.RemotePort
		config.LocalPort = conn.LocalPort
	}

	method := conn.Method

	// Stop the old connection
	if err := m.Stop(connID); err != nil {
		return fmt.Errorf("failed to stop connection during restart: %w", err)
	}

	// Start a new connection
	newConn, err := m.Start(method, config)
	if err != nil {
		return fmt.Errorf("failed to start connection during restart: %w", err)
	}

	// Publish reconnecting event
	event := NewEvent(EventReconnecting, newConn.ID, newConn,
		fmt.Sprintf("Connection %s restarted as %s", connID, newConn.ID))
	m.eventPublisher.Publish(event)

	return nil
}

// Status retrieves the status of a connection
func (m *DefaultConnectionManager) Status(connID string) (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[connID]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", connID)
	}

	return conn.Clone(), nil
}

// StartMultiple starts multiple connections for redundancy
func (m *DefaultConnectionManager) StartMultiple(methods []string, config *Config) ([]*Connection, error) {
	if len(methods) == 0 {
		return nil, fmt.Errorf("no methods specified")
	}

	// Pre-allocate with exact size to maintain order
	connections := make([]*Connection, len(methods))
	errors := make([]error, len(methods))

	var wg sync.WaitGroup

	// Start all connections concurrently
	for i, method := range methods {
		wg.Add(1)
		go func(idx int, methodName string) {
			defer wg.Done()

			conn, err := m.Start(methodName, config)

			if err != nil {
				errors[idx] = fmt.Errorf("%s: %w", methodName, err)
			} else {
				// Set priority based on order (first = highest priority)
				conn.SetPriority(idx)

				// First connection is primary by default
				if idx == 0 {
					conn.SetPrimaryConnection(true)
					if m.config.EnableFailover && m.failoverManager != nil {
						m.failoverManager.mu.Lock()
						m.failoverManager.primaryConnID = conn.ID
						m.failoverManager.mu.Unlock()
					}
				}

				connections[idx] = conn
			}
		}(i, method)
	}

	wg.Wait()

	// Filter out nil connections and collect errors
	validConnections := make([]*Connection, 0, len(connections))
	var collectedErrors []error
	for i, conn := range connections {
		if conn != nil {
			validConnections = append(validConnections, conn)
		} else if errors[i] != nil {
			collectedErrors = append(collectedErrors, errors[i])
		}
	}

	if len(validConnections) == 0 {
		return nil, fmt.Errorf("failed to start any connections: %v", collectedErrors)
	}

	return validConnections, nil
}

// StopAll terminates all connections
func (m *DefaultConnectionManager) StopAll() error {
	m.mu.RLock()
	connIDs := make([]string, 0, len(m.connections))
	for id := range m.connections {
		connIDs = append(connIDs, id)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	errorsChan := make(chan error, len(connIDs))

	// Stop all connections concurrently
	for _, id := range connIDs {
		wg.Add(1)
		go func(connID string) {
			defer wg.Done()
			if err := m.Stop(connID); err != nil {
				errorsChan <- err
			}
		}(id)
	}

	wg.Wait()
	close(errorsChan)

	// Collect errors
	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping connections: %v", errors)
	}

	return nil
}

// List returns all active connections
func (m *DefaultConnectionManager) List() ([]*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connections := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		connections = append(connections, conn.Clone())
	}

	return connections, nil
}

// Monitor subscribes to events for a specific connection
func (m *DefaultConnectionManager) Monitor(connID string) <-chan *ConnectionEvent {
	// Create a filter for this specific connection
	filter := func(event *ConnectionEvent) bool {
		return event.ConnID == connID
	}

	subscriber := m.eventPublisher.Subscribe(connID, filter)
	return subscriber.Channel
}

// SetPrimary manually sets the primary connection
func (m *DefaultConnectionManager) SetPrimary(connID string) error {
	if m.failoverManager == nil {
		return fmt.Errorf("failover not enabled")
	}

	return m.failoverManager.SetPrimary(connID)
}

// GetPrimary returns the current primary connection
func (m *DefaultConnectionManager) GetPrimary() (*Connection, error) {
	if m.failoverManager == nil {
		return nil, fmt.Errorf("failover not enabled")
	}

	primaryID := m.failoverManager.GetPrimary()
	if primaryID == "" {
		return nil, fmt.Errorf("no primary connection set")
	}

	return m.Status(primaryID)
}

// EnableAutoFailover enables or disables automatic failover
func (m *DefaultConnectionManager) EnableAutoFailover(enabled bool) {
	if m.failoverManager == nil {
		return
	}

	m.failoverManager.mu.Lock()
	defer m.failoverManager.mu.Unlock()

	m.failoverManager.config.Enabled = enabled

	if enabled && !m.failoverManager.running {
		go m.failoverManager.Start()
	} else if !enabled && m.failoverManager.running {
		m.failoverManager.Stop()
	}
}

// Shutdown gracefully shuts down the connection manager
func (m *DefaultConnectionManager) Shutdown() error {
	// Stop failover
	if m.failoverManager != nil {
		m.failoverManager.Stop()
	}

	// Stop metrics collection
	if m.metricsCollector != nil {
		m.metricsCollector.Stop()
	}

	// Stop all connections
	if err := m.StopAll(); err != nil {
		return err
	}

	// Close event publisher
	m.eventPublisher.Close()

	// Cancel context
	m.cancel()

	return nil
}

// GetMetrics exports current metrics
func (m *DefaultConnectionManager) GetMetrics() map[string]interface{} {
	if m.metricsCollector == nil {
		return nil
	}
	return m.metricsCollector.Export()
}

// GetEventPublisher returns the event publisher for external subscription
func (m *DefaultConnectionManager) GetEventPublisher() *EventPublisher {
	return m.eventPublisher
}
