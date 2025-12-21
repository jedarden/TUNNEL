/*
Package core provides the core connection management functionality for the TUNNEL TUI application.

# Overview

The core package implements a robust connection manager with support for:
  - Multiple concurrent connections using different providers
  - Automatic failover between connections
  - Health monitoring and metrics collection
  - Event-driven architecture for connection state changes

# Key Components

Connection Management:
  - ConnectionManager: Main interface for managing tunnel connections
  - Connection: Represents a single SSH tunnel connection
  - ConnectionProvider: Interface for implementing connection providers

Failover & Redundancy:
  - FailoverManager: Automatic failover between connections
  - HealthStatus: Tracks connection health
  - Priority-based failover ordering

Metrics & Monitoring:
  - MetricsCollector: Collects connection metrics (latency, throughput, uptime)
  - LatencyMonitor: Monitors and alerts on latency issues
  - ConnectionMetrics: Stores performance data

Event System:
  - EventPublisher: Publishes connection events
  - EventSubscriber: Subscribes to specific events
  - ConnectionEvent: Event data structure

# Usage Example

	// Create connection manager
	config := DefaultManagerConfig()
	manager := NewConnectionManager(config)

	// Register providers
	manager.RegisterProvider(NewMockProvider("cloudflare", 0.1, 50*time.Millisecond))
	manager.RegisterProvider(NewMockProvider("tailscale", 0.05, 30*time.Millisecond))

	// Start multiple connections for redundancy
	connConfig := DefaultConfig()
	connections, err := manager.StartMultiple(
		[]string{"cloudflare", "tailscale"},
		connConfig,
	)

	// Monitor events
	eventSub := manager.GetEventPublisher().Subscribe("monitor", nil)
	go func() {
		for event := range eventSub.Channel {
			fmt.Printf("Event: %s - %s\n", event.Type, event.Message)
		}
	}()

	// Enable automatic failover
	manager.EnableAutoFailover(true)

	// Get metrics
	metrics := manager.GetMetrics()

	// Shutdown gracefully
	manager.Shutdown()

# Architecture

The package follows a layered architecture:

	┌─────────────────────────────────────┐
	│     ConnectionManager Interface     │
	└─────────────────────────────────────┘
	              │
	    ┌─────────┴──────────┐
	    │                    │
	┌───▼────────┐    ┌──────▼──────┐
	│  Failover  │    │   Metrics   │
	│  Manager   │    │  Collector  │
	└───┬────────┘    └──────┬──────┘
	    │                    │
	    └─────────┬──────────┘
	              │
	    ┌─────────▼──────────┐
	    │  Event Publisher   │
	    └────────────────────┘

# Concurrency

The package is designed for concurrent use:
  - All exported methods are thread-safe
  - Connection monitoring runs in separate goroutines
  - Health checks are performed in parallel
  - Metrics collection is concurrent

# Extensibility

New connection providers can be added by implementing the ConnectionProvider interface:

	type MyProvider struct{}

	func (p *MyProvider) Name() string {
		return "my-provider"
	}

	func (p *MyProvider) Connect(ctx context.Context, config *Config) (*Connection, error) {
		// Implementation
	}

	func (p *MyProvider) Disconnect(conn *Connection) error {
		// Implementation
	}

	func (p *MyProvider) IsHealthy(conn *Connection) bool {
		// Implementation
	}
*/
package core
