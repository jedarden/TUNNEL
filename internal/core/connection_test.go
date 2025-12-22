package core

import (
	"testing"
	"time"
)

func TestNewConnection(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}

	if conn.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", conn.ID)
	}

	if conn.Method != "mock" {
		t.Errorf("Expected Method 'mock', got '%s'", conn.Method)
	}

	if conn.LocalPort != 8080 {
		t.Errorf("Expected LocalPort 8080, got %d", conn.LocalPort)
	}

	if conn.RemoteHost != "localhost" {
		t.Errorf("Expected RemoteHost 'localhost', got '%s'", conn.RemoteHost)
	}

	if conn.RemotePort != 22 {
		t.Errorf("Expected RemotePort 22, got %d", conn.RemotePort)
	}

	if conn.State != StateDisconnected {
		t.Errorf("Expected initial state Disconnected, got %s", conn.State)
	}

	if conn.Metrics == nil {
		t.Error("Expected Metrics to be initialized")
	}

	if conn.cancel == nil {
		t.Error("Expected cancel channel to be initialized")
	}
}

func TestGetState(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	// Test initial state
	state := conn.GetState()
	if state != StateDisconnected {
		t.Errorf("Expected Disconnected, got %s", state)
	}

	// Test after changing state
	conn.State = StateConnected
	state = conn.GetState()
	if state != StateConnected {
		t.Errorf("Expected Connected, got %s", state)
	}
}

func TestSetState(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	conn.SetState(StateConnecting)
	if conn.GetState() != StateConnecting {
		t.Errorf("Expected Connecting, got %s", conn.GetState())
	}

	conn.SetState(StateConnected)
	if conn.GetState() != StateConnected {
		t.Errorf("Expected Connected, got %s", conn.GetState())
	}

	conn.SetState(StateReconnecting)
	if conn.GetState() != StateReconnecting {
		t.Errorf("Expected Reconnecting, got %s", conn.GetState())
	}

	conn.SetState(StateFailed)
	if conn.GetState() != StateFailed {
		t.Errorf("Expected Failed, got %s", conn.GetState())
	}

	conn.SetState(StateDisconnected)
	if conn.GetState() != StateDisconnected {
		t.Errorf("Expected Disconnected, got %s", conn.GetState())
	}
}

func TestGetPriority(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	// Test initial priority (should be 0)
	priority := conn.GetPriority()
	if priority != 0 {
		t.Errorf("Expected initial priority 0, got %d", priority)
	}

	// Test after setting priority
	conn.Priority = 5
	priority = conn.GetPriority()
	if priority != 5 {
		t.Errorf("Expected priority 5, got %d", priority)
	}
}

func TestSetPriority(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	conn.SetPriority(10)
	if conn.GetPriority() != 10 {
		t.Errorf("Expected priority 10, got %d", conn.GetPriority())
	}

	conn.SetPriority(0)
	if conn.GetPriority() != 0 {
		t.Errorf("Expected priority 0, got %d", conn.GetPriority())
	}

	conn.SetPriority(100)
	if conn.GetPriority() != 100 {
		t.Errorf("Expected priority 100, got %d", conn.GetPriority())
	}
}

func TestIsPrimaryConnection(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	// Test initial value (should be false)
	if conn.IsPrimaryConnection() {
		t.Error("Expected IsPrimaryConnection to be false initially")
	}

	// Test after setting to true
	conn.IsPrimary = true
	if !conn.IsPrimaryConnection() {
		t.Error("Expected IsPrimaryConnection to be true")
	}
}

func TestSetPrimaryConnection(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	conn.SetPrimaryConnection(true)
	if !conn.IsPrimaryConnection() {
		t.Error("Expected IsPrimaryConnection to be true")
	}

	conn.SetPrimaryConnection(false)
	if conn.IsPrimaryConnection() {
		t.Error("Expected IsPrimaryConnection to be false")
	}
}

func TestGetUptime(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	// Test when not connected
	uptime := conn.GetUptime()
	if uptime != 0 {
		t.Errorf("Expected uptime 0 for disconnected connection, got %v", uptime)
	}

	// Test when connected
	conn.State = StateConnected
	conn.StartedAt = time.Now().Add(-5 * time.Second)

	uptime = conn.GetUptime()
	if uptime < 4*time.Second || uptime > 6*time.Second {
		t.Errorf("Expected uptime around 5 seconds, got %v", uptime)
	}
}

func TestGetUptimeNotStarted(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)
	conn.State = StateConnected
	// StartedAt is zero

	uptime := conn.GetUptime()
	if uptime != 0 {
		t.Errorf("Expected uptime 0 when StartedAt is zero, got %v", uptime)
	}
}

func TestGetUptimeDisconnected(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)
	conn.State = StateDisconnected
	conn.StartedAt = time.Now().Add(-10 * time.Second)

	uptime := conn.GetUptime()
	if uptime != 0 {
		t.Errorf("Expected uptime 0 for disconnected connection, got %v", uptime)
	}
}

func TestClone(t *testing.T) {
	original := NewConnection("test-id", "mock", 8080, "localhost", 22)
	original.State = StateConnected
	original.StartedAt = time.Now()
	original.PID = 12345
	original.Priority = 5
	original.IsPrimary = true

	// Set some metrics
	original.Metrics.Update(1000, 2000, 100*time.Millisecond)

	clone := original.Clone()

	if clone == nil {
		t.Fatal("Expected non-nil clone")
	}

	// Verify all fields are copied
	if clone.ID != original.ID {
		t.Errorf("Expected ID '%s', got '%s'", original.ID, clone.ID)
	}

	if clone.Method != original.Method {
		t.Errorf("Expected Method '%s', got '%s'", original.Method, clone.Method)
	}

	if clone.State != original.State {
		t.Errorf("Expected State %s, got %s", original.State, clone.State)
	}

	if clone.LocalPort != original.LocalPort {
		t.Errorf("Expected LocalPort %d, got %d", original.LocalPort, clone.LocalPort)
	}

	if clone.RemoteHost != original.RemoteHost {
		t.Errorf("Expected RemoteHost '%s', got '%s'", original.RemoteHost, clone.RemoteHost)
	}

	if clone.RemotePort != original.RemotePort {
		t.Errorf("Expected RemotePort %d, got %d", original.RemotePort, clone.RemotePort)
	}

	if clone.PID != original.PID {
		t.Errorf("Expected PID %d, got %d", original.PID, clone.PID)
	}

	if clone.Priority != original.Priority {
		t.Errorf("Expected Priority %d, got %d", original.Priority, clone.Priority)
	}

	if clone.IsPrimary != original.IsPrimary {
		t.Errorf("Expected IsPrimary %v, got %v", original.IsPrimary, clone.IsPrimary)
	}

	// Verify metrics are copied
	sent, received, latency := clone.Metrics.GetStats()
	if sent != 1000 {
		t.Errorf("Expected BytesSent 1000, got %d", sent)
	}
	if received != 2000 {
		t.Errorf("Expected BytesReceived 2000, got %d", received)
	}
	if latency != 100*time.Millisecond {
		t.Errorf("Expected Latency 100ms, got %v", latency)
	}
}

func TestCloneIndependentMetrics(t *testing.T) {
	original := NewConnection("test-id", "mock", 8080, "localhost", 22)
	original.Metrics.Update(1000, 2000, 100*time.Millisecond)

	clone := original.Clone()

	// Clone should have a snapshot of original's metrics at time of cloning
	cloneSent, cloneReceived, cloneLatency := clone.Metrics.GetStats()
	if cloneSent != 1000 {
		t.Errorf("Expected clone BytesSent 1000 (snapshot), got %d", cloneSent)
	}
	if cloneReceived != 2000 {
		t.Errorf("Expected clone BytesReceived 2000 (snapshot), got %d", cloneReceived)
	}
	if cloneLatency != 100*time.Millisecond {
		t.Errorf("Expected clone Latency 100ms (snapshot), got %v", cloneLatency)
	}

	// Modifying clone's metrics should not affect original
	clone.Metrics.Update(500, 500, 50*time.Millisecond)

	// Original should still have its original values
	origSent, origReceived, origLatency := original.Metrics.GetStats()
	if origSent != 1000 {
		t.Errorf("Expected original BytesSent 1000 (unchanged), got %d", origSent)
	}
	if origReceived != 2000 {
		t.Errorf("Expected original BytesReceived 2000 (unchanged), got %d", origReceived)
	}
	if origLatency != 100*time.Millisecond {
		t.Errorf("Expected original Latency 100ms (unchanged), got %v", origLatency)
	}

	// Clone should have updated values
	cloneSent, cloneReceived, cloneLatency = clone.Metrics.GetStats()
	if cloneSent != 1500 {
		t.Errorf("Expected clone BytesSent 1500, got %d", cloneSent)
	}
	if cloneReceived != 2500 {
		t.Errorf("Expected clone BytesReceived 2500, got %d", cloneReceived)
	}
	if cloneLatency != 50*time.Millisecond {
		t.Errorf("Expected clone Latency 50ms, got %v", cloneLatency)
	}
}

func TestConnectionStateString(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{StateDisconnected, "Disconnected"},
		{StateConnecting, "Connecting"},
		{StateConnected, "Connected"},
		{StateReconnecting, "Reconnecting"},
		{StateFailed, "Failed"},
	}

	for _, test := range tests {
		str := test.state.String()
		if str != test.expected {
			t.Errorf("Expected %s.String() to be '%s', got '%s'",
				test.state, test.expected, str)
		}
	}
}

func TestConnectionMetricsUpdate(t *testing.T) {
	metrics := &ConnectionMetrics{}

	beforeUpdate := time.Now()
	metrics.Update(100, 200, 50*time.Millisecond)
	afterUpdate := time.Now()

	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	if metrics.BytesSent != 100 {
		t.Errorf("Expected BytesSent 100, got %d", metrics.BytesSent)
	}

	if metrics.BytesReceived != 200 {
		t.Errorf("Expected BytesReceived 200, got %d", metrics.BytesReceived)
	}

	if metrics.Latency != 50*time.Millisecond {
		t.Errorf("Expected Latency 50ms, got %v", metrics.Latency)
	}

	if metrics.LastActive.Before(beforeUpdate) || metrics.LastActive.After(afterUpdate) {
		t.Error("Expected LastActive to be updated to current time")
	}
}

func TestConnectionMetricsUpdateIncremental(t *testing.T) {
	metrics := &ConnectionMetrics{}

	metrics.Update(100, 200, 50*time.Millisecond)
	metrics.Update(50, 100, 60*time.Millisecond)

	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	// Should be cumulative
	if metrics.BytesSent != 150 {
		t.Errorf("Expected BytesSent 150, got %d", metrics.BytesSent)
	}

	if metrics.BytesReceived != 300 {
		t.Errorf("Expected BytesReceived 300, got %d", metrics.BytesReceived)
	}

	// Latency should be overwritten (not cumulative)
	if metrics.Latency != 60*time.Millisecond {
		t.Errorf("Expected Latency 60ms, got %v", metrics.Latency)
	}
}

func TestConnectionMetricsGetStats(t *testing.T) {
	metrics := &ConnectionMetrics{}
	metrics.Update(1000, 2000, 75*time.Millisecond)

	sent, received, latency := metrics.GetStats()

	if sent != 1000 {
		t.Errorf("Expected sent 1000, got %d", sent)
	}

	if received != 2000 {
		t.Errorf("Expected received 2000, got %d", received)
	}

	if latency != 75*time.Millisecond {
		t.Errorf("Expected latency 75ms, got %v", latency)
	}
}

func TestConnectionMetricsGetLatency(t *testing.T) {
	metrics := &ConnectionMetrics{}
	metrics.Update(100, 200, 123*time.Millisecond)

	latency := metrics.GetLatency()

	if latency != 123*time.Millisecond {
		t.Errorf("Expected latency 123ms, got %v", latency)
	}
}

func TestConnectionMetricsRecordFailure(t *testing.T) {
	metrics := &ConnectionMetrics{}

	err1 := &testError{"first error"}
	metrics.RecordFailure(err1)

	metrics.mu.RLock()
	if metrics.FailureCount != 1 {
		t.Errorf("Expected FailureCount 1, got %d", metrics.FailureCount)
	}
	if metrics.LastError != err1 {
		t.Error("Expected LastError to be set to err1")
	}
	metrics.mu.RUnlock()

	err2 := &testError{"second error"}
	metrics.RecordFailure(err2)

	metrics.mu.RLock()
	if metrics.FailureCount != 2 {
		t.Errorf("Expected FailureCount 2, got %d", metrics.FailureCount)
	}
	if metrics.LastError != err2 {
		t.Error("Expected LastError to be updated to err2")
	}
	metrics.mu.RUnlock()
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.RemoteHost != "localhost" {
		t.Errorf("Expected RemoteHost 'localhost', got '%s'", config.RemoteHost)
	}

	if config.RemotePort != 22 {
		t.Errorf("Expected RemotePort 22, got %d", config.RemotePort)
	}

	if config.LocalPort != 8080 {
		t.Errorf("Expected LocalPort 8080, got %d", config.LocalPort)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout 30s, got %v", config.Timeout)
	}

	if config.RetryAttempts != 3 {
		t.Errorf("Expected RetryAttempts 3, got %d", config.RetryAttempts)
	}

	if config.RetryDelay != 5*time.Second {
		t.Errorf("Expected RetryDelay 5s, got %v", config.RetryDelay)
	}

	if config.HealthCheckInterval != 10*time.Second {
		t.Errorf("Expected HealthCheckInterval 10s, got %v", config.HealthCheckInterval)
	}

	if config.ProviderConfigs == nil {
		t.Error("Expected ProviderConfigs to be initialized")
	}
}

func TestConnectionCancelAndDone(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	// Test Done() channel
	done := conn.Done()
	if done == nil {
		t.Fatal("Expected non-nil done channel")
	}

	// Channel should not be closed initially
	select {
	case <-done:
		t.Error("Expected done channel to be open initially")
	default:
		// Expected
	}

	// Cancel the connection
	conn.Cancel()

	// Channel should now be closed
	select {
	case <-done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected done channel to be closed after Cancel()")
	}
}

func TestConnectionConcurrentAccess(t *testing.T) {
	conn := NewConnection("test-id", "mock", 8080, "localhost", 22)

	// Test concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			conn.SetState(StateConnected)
			conn.SetPriority(i)
			conn.SetPrimaryConnection(i%2 == 0)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = conn.GetState()
			_ = conn.GetPriority()
			_ = conn.IsPrimaryConnection()
			_ = conn.GetUptime()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without race conditions, test passes
}

// Helper type for testing errors
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
