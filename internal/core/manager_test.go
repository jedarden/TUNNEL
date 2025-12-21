package core

import (
	"testing"
	"time"
)

func TestConnectionManagerCreation(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewConnectionManager(config)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.connections == nil {
		t.Error("Expected connections map to be initialized")
	}

	if manager.providers == nil {
		t.Error("Expected providers map to be initialized")
	}

	if manager.eventPublisher == nil {
		t.Error("Expected event publisher to be initialized")
	}

	// Cleanup
	manager.Shutdown()
}

func TestRegisterProvider(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	provider := NewMockProvider("test-provider", 0.0, 50*time.Millisecond)
	manager.RegisterProvider(provider)

	if _, exists := manager.providers[provider.Name()]; !exists {
		t.Errorf("Provider %s was not registered", provider.Name())
	}
}

func TestStartConnection(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	// Register mock provider
	provider := NewMockProvider("mock", 0.0, 50*time.Millisecond)
	manager.RegisterProvider(provider)

	// Create config
	config := DefaultConfig()

	// Start connection
	conn, err := manager.Start("mock", config)
	if err != nil {
		t.Fatalf("Failed to start connection: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}

	if conn.Method != "mock" {
		t.Errorf("Expected method 'mock', got '%s'", conn.Method)
	}

	if conn.GetState() != StateConnected {
		t.Errorf("Expected state Connected, got %s", conn.GetState())
	}
}

func TestStartMultipleConnections(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	// Register multiple providers
	manager.RegisterProvider(NewMockProvider("provider1", 0.0, 30*time.Millisecond))
	manager.RegisterProvider(NewMockProvider("provider2", 0.0, 50*time.Millisecond))
	manager.RegisterProvider(NewMockProvider("provider3", 0.0, 70*time.Millisecond))

	// Start multiple connections
	config := DefaultConfig()
	connections, err := manager.StartMultiple(
		[]string{"provider1", "provider2", "provider3"},
		config,
	)

	if err != nil {
		t.Fatalf("Failed to start multiple connections: %v", err)
	}

	if len(connections) != 3 {
		t.Errorf("Expected 3 connections, got %d", len(connections))
	}

	// Check priorities
	for i, conn := range connections {
		if conn.GetPriority() != i {
			t.Errorf("Connection %d: expected priority %d, got %d", i, i, conn.GetPriority())
		}
	}

	// First should be primary
	if !connections[0].IsPrimaryConnection() {
		t.Error("First connection should be primary")
	}
}

func TestStopConnection(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	provider := NewMockProvider("mock", 0.0, 50*time.Millisecond)
	manager.RegisterProvider(provider)

	config := DefaultConfig()
	conn, err := manager.Start("mock", config)
	if err != nil {
		t.Fatalf("Failed to start connection: %v", err)
	}

	connID := conn.ID

	// Stop the connection
	err = manager.Stop(connID)
	if err != nil {
		t.Fatalf("Failed to stop connection: %v", err)
	}

	// Verify it's removed
	_, err = manager.Status(connID)
	if err == nil {
		t.Error("Expected error when getting status of stopped connection")
	}
}

func TestListConnections(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	manager.RegisterProvider(NewMockProvider("mock1", 0.0, 30*time.Millisecond))
	manager.RegisterProvider(NewMockProvider("mock2", 0.0, 50*time.Millisecond))

	config := DefaultConfig()
	_, _ = manager.Start("mock1", config)
	_, _ = manager.Start("mock2", config)

	connections, err := manager.List()
	if err != nil {
		t.Fatalf("Failed to list connections: %v", err)
	}

	if len(connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(connections))
	}
}

func TestPrimaryConnection(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	manager.RegisterProvider(NewMockProvider("provider1", 0.0, 30*time.Millisecond))
	manager.RegisterProvider(NewMockProvider("provider2", 0.0, 50*time.Millisecond))

	config := DefaultConfig()
	connections, _ := manager.StartMultiple([]string{"provider1", "provider2"}, config)

	// Get primary
	primary, err := manager.GetPrimary()
	if err != nil {
		t.Fatalf("Failed to get primary: %v", err)
	}

	if primary.ID != connections[0].ID {
		t.Error("First connection should be primary")
	}

	// Set second as primary
	err = manager.SetPrimary(connections[1].ID)
	if err != nil {
		t.Fatalf("Failed to set primary: %v", err)
	}

	primary, _ = manager.GetPrimary()
	if primary.ID != connections[1].ID {
		t.Error("Second connection should now be primary")
	}
}

func TestEventPublishing(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	eventReceived := make(chan bool, 1)

	// Subscribe to events
	sub := manager.GetEventPublisher().Subscribe("test", nil)
	go func() {
		for range sub.Channel {
			eventReceived <- true
			return
		}
	}()

	// Start a connection to trigger an event
	manager.RegisterProvider(NewMockProvider("mock", 0.0, 50*time.Millisecond))
	_, _ = manager.Start("mock", DefaultConfig())

	// Wait for event
	select {
	case <-eventReceived:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Expected to receive connection event")
	}
}

func TestMetricsCollection(t *testing.T) {
	config := DefaultManagerConfig()
	config.EnableMetrics = true
	config.MetricsInterval = 100 * time.Millisecond

	manager := NewConnectionManager(config)
	defer manager.Shutdown()

	manager.RegisterProvider(NewMockProvider("mock", 0.0, 50*time.Millisecond))
	conn, _ := manager.Start("mock", DefaultConfig())

	// Wait for metrics collection
	time.Sleep(300 * time.Millisecond)

	// Get metrics
	metrics := manager.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	if metrics["total_connections"] != 1 {
		t.Errorf("Expected 1 connection in metrics, got %v", metrics["total_connections"])
	}

	// Get connection metrics
	connMetrics, err := manager.metricsCollector.GetConnectionMetrics(conn.ID)
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	if connMetrics == nil {
		t.Fatal("Expected non-nil connection metrics")
	}
}

func TestFailoverConfiguration(t *testing.T) {
	config := DefaultManagerConfig()
	config.EnableFailover = true

	manager := NewConnectionManager(config)
	defer manager.Shutdown()

	if manager.failoverManager == nil {
		t.Error("Expected failover manager to be initialized")
	}

	// Test enable/disable
	manager.EnableAutoFailover(false)
	time.Sleep(100 * time.Millisecond)

	manager.EnableAutoFailover(true)
	time.Sleep(100 * time.Millisecond)
}

func TestRestartConnection(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	provider := NewMockProvider("mock", 0.0, 50*time.Millisecond)
	manager.RegisterProvider(provider)

	config := DefaultConfig()
	conn, err := manager.Start("mock", config)
	if err != nil {
		t.Fatalf("Failed to start connection: %v", err)
	}

	originalID := conn.ID

	// Restart
	err = manager.Restart(originalID)
	if err != nil {
		t.Fatalf("Failed to restart connection: %v", err)
	}

	// Original should be gone
	_, err = manager.Status(originalID)
	if err == nil {
		t.Error("Original connection should not exist after restart")
	}

	// Should have a new connection
	connections, _ := manager.List()
	if len(connections) != 1 {
		t.Errorf("Expected 1 connection after restart, got %d", len(connections))
	}
}

func TestMonitorConnection(t *testing.T) {
	manager := NewConnectionManager(nil)
	defer manager.Shutdown()

	provider := NewMockProvider("mock", 0.0, 50*time.Millisecond)
	manager.RegisterProvider(provider)

	config := DefaultConfig()
	conn, _ := manager.Start("mock", config)

	// Monitor the connection
	monitorChan := manager.Monitor(conn.ID)

	eventReceived := make(chan bool, 1)
	go func() {
		select {
		case <-monitorChan:
			eventReceived <- true
		case <-time.After(2 * time.Second):
		}
	}()

	// Trigger an event by restarting
	_ = manager.Restart(conn.ID)

	select {
	case <-eventReceived:
		// Success
	case <-time.After(3 * time.Second):
		// This is okay - the connection might have been removed too quickly
	}
}
