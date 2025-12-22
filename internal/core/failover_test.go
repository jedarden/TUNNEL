package core

import (
	"testing"
	"time"
)

func TestNewFailoverManager(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	config := DefaultFailoverConfig()

	fm := NewFailoverManager(config, publisher, collector)

	if fm == nil {
		t.Fatal("Expected non-nil failover manager")
	}

	if fm.config == nil {
		t.Error("Expected config to be set")
	}

	if fm.connections == nil {
		t.Error("Expected connections map to be initialized")
	}

	if fm.healthStatus == nil {
		t.Error("Expected healthStatus map to be initialized")
	}

	if fm.eventPublisher == nil {
		t.Error("Expected eventPublisher to be set")
	}

	if fm.metricsCollector == nil {
		t.Error("Expected metricsCollector to be set")
	}

	if fm.done == nil {
		t.Error("Expected done channel to be initialized")
	}
}

func TestNewFailoverManagerNilConfig(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	fm := NewFailoverManager(nil, publisher, collector)

	if fm == nil {
		t.Fatal("Expected non-nil failover manager")
	}

	if fm.config == nil {
		t.Error("Expected default config to be created")
	}

	if !fm.config.Enabled {
		t.Error("Expected default config to have failover enabled")
	}
}

func TestRegisterConnection(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn.SetState(StateConnected)

	fm.RegisterConnection(conn)

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if _, exists := fm.connections[conn.ID]; !exists {
		t.Error("Expected connection to be registered")
	}

	if _, exists := fm.healthStatus[conn.ID]; !exists {
		t.Error("Expected health status to be initialized")
	}

	status := fm.healthStatus[conn.ID]
	if status.IsHealthy {
		t.Error("Expected initial health status to be false")
	}
}

func TestUnregisterConnection(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	fm.UnregisterConnection(conn1.ID)

	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if _, exists := fm.connections[conn1.ID]; exists {
		t.Error("Expected connection to be unregistered")
	}

	if _, exists := fm.healthStatus[conn1.ID]; exists {
		t.Error("Expected health status to be removed")
	}

	if _, exists := fm.connections[conn2.ID]; !exists {
		t.Error("Expected other connection to remain registered")
	}
}

func TestUnregisterPrimaryConnection(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn1.SetState(StateConnected)
	conn1.SetPriority(0)

	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)
	conn2.SetState(StateConnected)
	conn2.SetPriority(1)

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	// Make conn1 healthy so it can be selected as primary
	fm.healthStatus[conn1.ID].IsHealthy = true
	fm.healthStatus[conn2.ID].IsHealthy = true

	_ = fm.SetPrimary(conn1.ID)

	// Unregister primary connection
	fm.UnregisterConnection(conn1.ID)

	fm.mu.RLock()
	primaryID := fm.primaryConnID
	fm.mu.RUnlock()

	// Should select a new primary (conn2)
	if primaryID != conn2.ID {
		t.Errorf("Expected new primary to be %s, got %s", conn2.ID, primaryID)
	}
}

func TestSetPrimary(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	// Set conn1 as primary
	err := fm.SetPrimary(conn1.ID)
	if err != nil {
		t.Fatalf("Failed to set primary: %v", err)
	}

	if !conn1.IsPrimaryConnection() {
		t.Error("Expected conn1 to be marked as primary")
	}

	primaryID := fm.GetPrimary()
	if primaryID != conn1.ID {
		t.Errorf("Expected primary to be %s, got %s", conn1.ID, primaryID)
	}

	// Set conn2 as primary
	err = fm.SetPrimary(conn2.ID)
	if err != nil {
		t.Fatalf("Failed to set primary: %v", err)
	}

	if !conn2.IsPrimaryConnection() {
		t.Error("Expected conn2 to be marked as primary")
	}

	if conn1.IsPrimaryConnection() {
		t.Error("Expected conn1 to no longer be primary")
	}

	primaryID = fm.GetPrimary()
	if primaryID != conn2.ID {
		t.Errorf("Expected primary to be %s, got %s", conn2.ID, primaryID)
	}
}

func TestSetPrimaryInvalidConnection(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	err := fm.SetPrimary("non-existent")
	if err == nil {
		t.Error("Expected error when setting invalid connection as primary")
	}
}

func TestGetPrimary(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn := NewConnection("test-1", "mock", 8080, "localhost", 22)
	fm.RegisterConnection(conn)

	// Initially no primary
	primaryID := fm.GetPrimary()
	if primaryID != "" {
		t.Errorf("Expected no primary initially, got %s", primaryID)
	}

	// Set primary
	_ = fm.SetPrimary(conn.ID)

	primaryID = fm.GetPrimary()
	if primaryID != conn.ID {
		t.Errorf("Expected primary to be %s, got %s", conn.ID, primaryID)
	}
}

func TestFailoverOnPrimaryFailure(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	config := DefaultFailoverConfig()
	config.HealthCheckInterval = 100 * time.Millisecond
	config.FailureThreshold = 2

	fm := NewFailoverManager(config, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn1.SetState(StateConnected)
	conn1.StartedAt = time.Now()
	conn1.SetPriority(0)

	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)
	conn2.SetState(StateConnected)
	conn2.StartedAt = time.Now()
	conn2.SetPriority(1)

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	// Mark both as initially healthy
	fm.healthStatus[conn1.ID].IsHealthy = true
	fm.healthStatus[conn1.ID].ConsecutiveSuccesses = config.RecoveryThreshold
	fm.healthStatus[conn2.ID].IsHealthy = true
	fm.healthStatus[conn2.ID].ConsecutiveSuccesses = config.RecoveryThreshold

	// Set conn1 as primary
	_ = fm.SetPrimary(conn1.ID)

	// Simulate primary failure
	conn1.SetState(StateDisconnected)

	// Manually trigger health check to simulate failover
	fm.checkConnection(conn1)
	fm.checkConnection(conn1)
	fm.checkConnection(conn2)

	// Trigger failover evaluation
	fm.evaluateFailover(conn1.ID)

	// Verify failover occurred
	primaryID := fm.GetPrimary()
	if primaryID != conn2.ID {
		t.Errorf("Expected failover to conn2, got primary %s", primaryID)
	}

	if !conn2.IsPrimaryConnection() {
		t.Error("Expected conn2 to be marked as primary after failover")
	}
}

func TestHealthCheckMonitoring(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	config := DefaultFailoverConfig()
	config.HealthCheckInterval = 100 * time.Millisecond
	config.FailureThreshold = 2
	config.RecoveryThreshold = 3

	fm := NewFailoverManager(config, publisher, collector)

	conn := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn.SetState(StateConnected)
	conn.StartedAt = time.Now()

	fm.RegisterConnection(conn)

	// Initially unhealthy
	status := fm.healthStatus[conn.ID]
	if status.IsHealthy {
		t.Error("Expected initial status to be unhealthy")
	}

	// Perform successful health checks
	for i := 0; i < config.RecoveryThreshold; i++ {
		fm.checkConnection(conn)
	}

	status.mu.RLock()
	healthy := status.IsHealthy
	successCount := status.ConsecutiveSuccesses
	status.mu.RUnlock()

	if !healthy {
		t.Error("Expected connection to be healthy after recovery threshold")
	}

	if successCount < config.RecoveryThreshold {
		t.Errorf("Expected at least %d consecutive successes, got %d",
			config.RecoveryThreshold, successCount)
	}

	// Simulate failures
	conn.SetState(StateDisconnected)

	for i := 0; i < config.FailureThreshold; i++ {
		fm.checkConnection(conn)
	}

	status.mu.RLock()
	healthy = status.IsHealthy
	failCount := status.ConsecutiveFailures
	status.mu.RUnlock()

	if healthy {
		t.Error("Expected connection to be unhealthy after failure threshold")
	}

	if failCount < config.FailureThreshold {
		t.Errorf("Expected at least %d consecutive failures, got %d",
			config.FailureThreshold, failCount)
	}
}

func TestStartStopFailoverMonitoring(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	config := DefaultFailoverConfig()
	config.HealthCheckInterval = 100 * time.Millisecond

	fm := NewFailoverManager(config, publisher, collector)

	// Start monitoring
	fm.Start()

	time.Sleep(50 * time.Millisecond)

	fm.mu.RLock()
	running := fm.running
	fm.mu.RUnlock()

	if !running {
		t.Error("Expected failover manager to be running after Start()")
	}

	// Stop monitoring
	fm.Stop()

	time.Sleep(50 * time.Millisecond)

	fm.mu.RLock()
	running = fm.running
	fm.mu.RUnlock()

	if running {
		t.Error("Expected failover manager to be stopped after Stop()")
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	config := DefaultFailoverConfig()
	config.HealthCheckInterval = 100 * time.Millisecond

	fm := NewFailoverManager(config, publisher, collector)

	// Start monitoring
	fm.Start()

	// Try to start again - should not cause issues
	fm.Start()

	fm.mu.RLock()
	running := fm.running
	fm.mu.RUnlock()

	if !running {
		t.Error("Expected failover manager to remain running")
	}

	fm.Stop()
}

func TestAutoRecoveryToHigherPriority(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	config := DefaultFailoverConfig()
	config.AutoRecover = true
	config.RecoveryThreshold = 2

	fm := NewFailoverManager(config, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn1.SetState(StateConnected)
	conn1.SetPriority(0) // Higher priority

	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)
	conn2.SetState(StateConnected)
	conn2.SetPriority(1) // Lower priority

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	// Initially conn2 is primary (simulating failover scenario)
	fm.healthStatus[conn2.ID].IsHealthy = true
	fm.healthStatus[conn2.ID].ConsecutiveSuccesses = config.RecoveryThreshold
	_ = fm.SetPrimary(conn2.ID)

	// conn1 becomes healthy
	for i := 0; i < config.RecoveryThreshold; i++ {
		fm.checkConnection(conn1)
	}

	// Trigger auto-recovery evaluation
	fm.checkForBetterPrimary(conn2.ID)

	// Should switch back to higher priority conn1
	primaryID := fm.GetPrimary()
	if primaryID != conn1.ID {
		t.Errorf("Expected auto-recovery to conn1 (higher priority), got %s", primaryID)
	}
}

func TestFindBestBackup(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn1.SetState(StateConnected)
	conn1.SetPriority(2)

	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)
	conn2.SetState(StateConnected)
	conn2.SetPriority(0) // Highest priority

	conn3 := NewConnection("test-3", "mock", 8082, "localhost", 22)
	conn3.SetState(StateConnected)
	conn3.SetPriority(1)

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)
	fm.RegisterConnection(conn3)

	// Mark all as healthy
	fm.healthStatus[conn1.ID].IsHealthy = true
	fm.healthStatus[conn2.ID].IsHealthy = true
	fm.healthStatus[conn3.ID].IsHealthy = true

	// Find best backup
	backup := fm.findBestBackup("")

	if backup == nil {
		t.Fatal("Expected to find a backup connection")
	}

	// Should select conn2 (highest priority = lowest number)
	if backup.ID != conn2.ID {
		t.Errorf("Expected to select conn2 (priority 0), got %s (priority %d)",
			backup.ID, backup.GetPriority())
	}
}

func TestFindBestBackupNoHealthy(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn1.SetState(StateDisconnected) // Not connected

	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)
	conn2.SetState(StateConnected)

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	// Mark both as unhealthy
	fm.healthStatus[conn1.ID].IsHealthy = false
	fm.healthStatus[conn2.ID].IsHealthy = false

	// Find best backup
	backup := fm.findBestBackup("")

	if backup != nil {
		t.Error("Expected no backup when all connections are unhealthy")
	}
}

func TestGetHealthStatus(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	conn := NewConnection("test-1", "mock", 8080, "localhost", 22)
	fm.RegisterConnection(conn)

	status, err := fm.GetHealthStatus(conn.ID)
	if err != nil {
		t.Fatalf("Failed to get health status: %v", err)
	}

	if status == nil {
		t.Fatal("Expected non-nil health status")
	}

	if status.IsHealthy {
		t.Error("Expected initial health status to be unhealthy")
	}
}

func TestGetHealthStatusNotFound(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()
	fm := NewFailoverManager(nil, publisher, collector)

	_, err := fm.GetHealthStatus("non-existent")
	if err == nil {
		t.Error("Expected error when getting health status for non-existent connection")
	}
}

func TestPerformHealthChecks(t *testing.T) {
	publisher := NewEventPublisher(100)
	collector := NewMetricsCollector()

	config := DefaultFailoverConfig()
	config.RecoveryThreshold = 2

	fm := NewFailoverManager(config, publisher, collector)

	conn1 := NewConnection("test-1", "mock", 8080, "localhost", 22)
	conn1.SetState(StateConnected)
	conn1.StartedAt = time.Now()

	conn2 := NewConnection("test-2", "mock", 8081, "localhost", 22)
	conn2.SetState(StateConnected)
	conn2.StartedAt = time.Now()

	fm.RegisterConnection(conn1)
	fm.RegisterConnection(conn2)

	// Perform health checks
	fm.performHealthChecks()

	// Check that health status was updated
	status1, _ := fm.GetHealthStatus(conn1.ID)
	status2, _ := fm.GetHealthStatus(conn2.ID)

	if status1.LastCheck.IsZero() {
		t.Error("Expected LastCheck to be updated for conn1")
	}

	if status2.LastCheck.IsZero() {
		t.Error("Expected LastCheck to be updated for conn2")
	}
}
