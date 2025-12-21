package core

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// MockProvider is a simple mock provider for testing
type MockProvider struct {
	name        string
	failureRate float64 // Probability of connection failure (0.0 - 1.0)
	baseLatency time.Duration
}

// NewMockProvider creates a new mock provider
func NewMockProvider(name string, failureRate float64, baseLatency time.Duration) *MockProvider {
	return &MockProvider{
		name:        name,
		failureRate: failureRate,
		baseLatency: baseLatency,
	}
}

// Name returns the provider name
func (p *MockProvider) Name() string {
	return p.name
}

// Connect simulates establishing a connection
func (p *MockProvider) Connect(ctx context.Context, config *Config) (*Connection, error) {
	// Simulate connection delay
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	// Randomly fail based on failure rate
	if rand.Float64() < p.failureRate {
		return nil, fmt.Errorf("mock connection failed (simulated failure)")
	}

	// Create connection ID
	connID := fmt.Sprintf("%s-%d", p.name, time.Now().UnixNano())

	conn := NewConnection(connID, p.name, config.LocalPort, config.RemoteHost, config.RemotePort)
	conn.State = StateConnected
	conn.StartedAt = time.Now()
	conn.PID = rand.Intn(10000) + 1000 // Simulate PID
	conn.Config = config

	// Set initial latency
	conn.Metrics.Latency = p.baseLatency + time.Duration(rand.Intn(50))*time.Millisecond

	return conn, nil
}

// Disconnect simulates tearing down a connection
func (p *MockProvider) Disconnect(conn *Connection) error {
	// Simulate disconnection delay
	time.Sleep(50 * time.Millisecond)

	conn.SetState(StateDisconnected)
	conn.PID = 0

	return nil
}

// IsHealthy checks if a connection is healthy
func (p *MockProvider) IsHealthy(conn *Connection) bool {
	if conn.GetState() != StateConnected {
		return false
	}

	// Simulate occasional unhealthy state
	return rand.Float64() > p.failureRate/2
}
