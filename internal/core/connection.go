package core

import (
	"sync"
	"time"
)

// ConnectionState represents the current state of a connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateFailed
)

// String returns the string representation of ConnectionState
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateReconnecting:
		return "Reconnecting"
	case StateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// ConnectionMetrics holds performance and usage metrics for a connection
type ConnectionMetrics struct {
	mu            sync.RWMutex
	BytesSent     int64
	BytesReceived int64
	Latency       time.Duration
	LastActive    time.Time
	Uptime        time.Duration
	FailureCount  int
	LastError     error
}

// Update safely updates metrics
func (m *ConnectionMetrics) Update(sent, received int64, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.BytesSent += sent
	m.BytesReceived += received
	m.Latency = latency
	m.LastActive = time.Now()
}

// GetLatency safely retrieves latency
func (m *ConnectionMetrics) GetLatency() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Latency
}

// RecordFailure increments failure count
func (m *ConnectionMetrics) RecordFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailureCount++
	m.LastError = err
}

// GetStats returns a copy of current stats
func (m *ConnectionMetrics) GetStats() (sent, received int64, latency time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.BytesSent, m.BytesReceived, m.Latency
}

// Connection represents a single SSH tunnel connection
type Connection struct {
	mu         sync.RWMutex
	ID         string
	Method     string // Provider name (e.g., "cloudflare", "tailscale", "ngrok")
	State      ConnectionState
	LocalPort  int
	RemoteHost string
	RemotePort int
	StartedAt  time.Time
	PID        int // Process ID of the tunnel process
	Metrics    *ConnectionMetrics
	Priority   int           // For failover ordering (lower = higher priority)
	IsPrimary  bool          // Is this the primary connection
	Config     interface{}   // Provider-specific configuration
	cancel     chan struct{} // For cancellation
}

// NewConnection creates a new connection instance
func NewConnection(id, method string, localPort int, remoteHost string, remotePort int) *Connection {
	return &Connection{
		ID:         id,
		Method:     method,
		State:      StateDisconnected,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Metrics:    &ConnectionMetrics{LastActive: time.Now()},
		cancel:     make(chan struct{}),
	}
}

// GetState safely retrieves the connection state
func (c *Connection) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.State
}

// SetState safely updates the connection state
func (c *Connection) SetState(state ConnectionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.State = state
}

// GetPriority safely retrieves the connection priority
func (c *Connection) GetPriority() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Priority
}

// SetPriority safely updates the connection priority
func (c *Connection) SetPriority(priority int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Priority = priority
}

// IsPrimaryConnection safely checks if this is the primary connection
func (c *Connection) IsPrimaryConnection() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.IsPrimary
}

// SetPrimaryConnection safely sets the primary flag
func (c *Connection) SetPrimaryConnection(isPrimary bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.IsPrimary = isPrimary
}

// GetUptime calculates the connection uptime
func (c *Connection) GetUptime() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.State == StateConnected && !c.StartedAt.IsZero() {
		return time.Since(c.StartedAt)
	}
	return 0
}

// Cancel signals the connection to cancel
func (c *Connection) Cancel() {
	close(c.cancel)
}

// Done returns the cancellation channel
func (c *Connection) Done() <-chan struct{} {
	return c.cancel
}

// Clone creates a deep copy of the connection (for safe reading)
func (c *Connection) Clone() *Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sent, received, latency := c.Metrics.GetStats()

	return &Connection{
		ID:         c.ID,
		Method:     c.Method,
		State:      c.State,
		LocalPort:  c.LocalPort,
		RemoteHost: c.RemoteHost,
		RemotePort: c.RemotePort,
		StartedAt:  c.StartedAt,
		PID:        c.PID,
		Priority:   c.Priority,
		IsPrimary:  c.IsPrimary,
		Metrics: &ConnectionMetrics{
			BytesSent:     sent,
			BytesReceived: received,
			Latency:       latency,
		},
	}
}

// Config represents the configuration for establishing connections
type Config struct {
	RemoteHost          string
	RemotePort          int
	LocalPort           int
	SSHKey              string
	SSHUser             string
	Timeout             time.Duration
	RetryAttempts       int
	RetryDelay          time.Duration
	HealthCheckInterval time.Duration
	ProviderConfigs     map[string]interface{} // Provider-specific configurations
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		RemoteHost:          "localhost",
		RemotePort:          22,
		LocalPort:           8080,
		Timeout:             30 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          5 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		ProviderConfigs:     make(map[string]interface{}),
	}
}
