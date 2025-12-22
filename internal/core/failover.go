package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// FailoverConfig holds configuration for failover behavior
type FailoverConfig struct {
	Enabled             bool
	HealthCheckInterval time.Duration
	FailureThreshold    int           // Number of failures before triggering failover
	RecoveryThreshold   int           // Number of successes before marking as recovered
	MaxLatency          time.Duration // Maximum acceptable latency
	AutoRecover         bool          // Automatically switch back to higher priority on recovery
}

// DefaultFailoverConfig returns a failover config with sensible defaults
func DefaultFailoverConfig() *FailoverConfig {
	return &FailoverConfig{
		Enabled:             true,
		HealthCheckInterval: 10 * time.Second,
		FailureThreshold:    3,
		RecoveryThreshold:   5,
		MaxLatency:          500 * time.Millisecond,
		AutoRecover:         true,
	}
}

// FailoverManager manages automatic failover between connections
type FailoverManager struct {
	mu               sync.RWMutex
	config           *FailoverConfig
	connections      map[string]*Connection
	healthStatus     map[string]*HealthStatus
	primaryConnID    string
	eventPublisher   *EventPublisher
	metricsCollector MetricsCollector
	ticker           *time.Ticker
	running          bool
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// HealthStatus tracks the health of a connection
type HealthStatus struct {
	mu                   sync.RWMutex
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastCheck            time.Time
	LastError            error
	IsHealthy            bool
}

// NewFailoverManager creates a new failover manager
func NewFailoverManager(config *FailoverConfig, publisher *EventPublisher, collector MetricsCollector) *FailoverManager {
	if config == nil {
		config = DefaultFailoverConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FailoverManager{
		config:           config,
		connections:      make(map[string]*Connection),
		healthStatus:     make(map[string]*HealthStatus),
		eventPublisher:   publisher,
		metricsCollector: collector,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// RegisterConnection adds a connection to the failover pool
func (fm *FailoverManager) RegisterConnection(conn *Connection) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.connections[conn.ID] = conn
	fm.healthStatus[conn.ID] = &HealthStatus{
		IsHealthy: false,
		LastCheck: time.Now(),
	}
}

// UnregisterConnection removes a connection from the failover pool
func (fm *FailoverManager) UnregisterConnection(connID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	delete(fm.connections, connID)
	delete(fm.healthStatus, connID)

	// If this was the primary, select a new one
	if fm.primaryConnID == connID {
		fm.primaryConnID = ""
		fm.selectNewPrimary()
	}
}

// Start begins the failover monitoring loop
func (fm *FailoverManager) Start() {
	fm.mu.Lock()
	if fm.running || !fm.config.Enabled {
		fm.mu.Unlock()
		return
	}
	fm.running = true
	fm.ticker = time.NewTicker(fm.config.HealthCheckInterval)
	// Recreate context for this run (in case of restart after Stop)
	fm.ctx, fm.cancel = context.WithCancel(context.Background())
	// Copy context to local var to avoid race with Stop() modifying fm.ctx
	ctx := fm.ctx
	fm.wg.Add(1)
	fm.mu.Unlock()

	go fm.monitorLoop(ctx)
}

// Stop halts the failover monitoring
func (fm *FailoverManager) Stop() {
	fm.mu.Lock()
	if !fm.running {
		fm.mu.Unlock()
		return
	}

	fm.running = false
	if fm.ticker != nil {
		fm.ticker.Stop()
	}
	// Cancel context to signal goroutines to stop
	fm.cancel()
	fm.mu.Unlock()

	// Wait for goroutine to exit
	fm.wg.Wait()
}

// monitorLoop continuously monitors connection health
func (fm *FailoverManager) monitorLoop(ctx context.Context) {
	defer fm.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-fm.ticker.C:
			fm.performHealthChecks()
		}
	}
}

// performHealthChecks checks all connections and triggers failover if needed
func (fm *FailoverManager) performHealthChecks() {
	fm.mu.RLock()
	connections := make([]*Connection, 0, len(fm.connections))
	for _, conn := range fm.connections {
		connections = append(connections, conn)
	}
	primaryID := fm.primaryConnID
	fm.mu.RUnlock()

	// Check all connections concurrently
	var wg sync.WaitGroup
	for _, conn := range connections {
		wg.Add(1)
		go func(c *Connection) {
			defer wg.Done()
			fm.checkConnection(c)
		}(conn)
	}
	wg.Wait()

	// After health checks, evaluate if failover is needed
	fm.evaluateFailover(primaryID)
}

// checkConnection performs a health check on a single connection
func (fm *FailoverManager) checkConnection(conn *Connection) {
	fm.mu.RLock()
	status, exists := fm.healthStatus[conn.ID]
	fm.mu.RUnlock()

	if !exists {
		return
	}

	// Perform the health check
	healthy := fm.isConnectionHealthy(conn)

	status.mu.Lock()
	status.LastCheck = time.Now()

	if healthy {
		status.ConsecutiveSuccesses++
		status.ConsecutiveFailures = 0

		// Mark as healthy if we've reached recovery threshold
		if status.ConsecutiveSuccesses >= fm.config.RecoveryThreshold {
			status.IsHealthy = true
			status.LastError = nil
		}
	} else {
		status.ConsecutiveFailures++
		status.ConsecutiveSuccesses = 0

		// Mark as unhealthy if we've reached failure threshold
		if status.ConsecutiveFailures >= fm.config.FailureThreshold {
			status.IsHealthy = false

			// Publish error event
			if fm.eventPublisher != nil {
				event := NewEvent(EventError, conn.ID, status.LastError,
					fmt.Sprintf("Connection %s marked unhealthy after %d failures",
						conn.ID, status.ConsecutiveFailures))
				fm.eventPublisher.Publish(event)
			}
		}
	}
	status.mu.Unlock()
}

// isConnectionHealthy checks if a connection is healthy
func (fm *FailoverManager) isConnectionHealthy(conn *Connection) bool {
	// Check connection state
	if conn.GetState() != StateConnected {
		return false
	}

	// Check latency if metrics collector is available
	if fm.metricsCollector != nil {
		metrics, err := fm.metricsCollector.GetConnectionMetrics(conn.ID)
		if err == nil {
			latency := metrics.GetLatency()
			if latency > fm.config.MaxLatency {
				return false
			}
		}
	}

	// Additional health checks can be added here
	// For example: checking if the process is still running, port is open, etc.

	return true
}

// evaluateFailover determines if failover should be triggered
func (fm *FailoverManager) evaluateFailover(currentPrimaryID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// If no primary is set, select one
	if currentPrimaryID == "" {
		fm.selectNewPrimary()
		return
	}

	// Check if current primary is healthy
	primaryStatus, exists := fm.healthStatus[currentPrimaryID]
	if !exists {
		fm.selectNewPrimary()
		return
	}

	primaryStatus.mu.RLock()
	primaryHealthy := primaryStatus.IsHealthy
	primaryStatus.mu.RUnlock()

	// If primary is unhealthy, trigger failover
	if !primaryHealthy {
		fm.triggerFailover(currentPrimaryID)
		return
	}

	// If auto-recovery is enabled, check if a higher priority connection is available
	if fm.config.AutoRecover {
		fm.checkForBetterPrimary(currentPrimaryID)
	}
}

// triggerFailover switches to a backup connection
func (fm *FailoverManager) triggerFailover(failedPrimaryID string) {
	// Find the best available backup
	backup := fm.findBestBackup(failedPrimaryID)

	if backup == nil {
		// No healthy backup available
		if fm.eventPublisher != nil {
			event := NewEvent(EventError, failedPrimaryID, nil,
				"Primary connection failed and no healthy backup available")
			fm.eventPublisher.Publish(event)
		}
		return
	}

	// Switch primary
	oldPrimary := fm.connections[failedPrimaryID]
	if oldPrimary != nil {
		oldPrimary.SetPrimaryConnection(false)
	}

	backup.SetPrimaryConnection(true)
	fm.primaryConnID = backup.ID

	// Publish failover event
	if fm.eventPublisher != nil {
		event := NewEvent(EventFailover, backup.ID,
			map[string]string{
				"old_primary": failedPrimaryID,
				"new_primary": backup.ID,
			},
			fmt.Sprintf("Failed over from %s to %s", failedPrimaryID, backup.ID))
		fm.eventPublisher.Publish(event)
	}
}

// checkForBetterPrimary checks if a higher priority connection is available
func (fm *FailoverManager) checkForBetterPrimary(currentPrimaryID string) {
	currentPrimary, exists := fm.connections[currentPrimaryID]
	if !exists {
		return
	}

	currentPriority := currentPrimary.GetPriority()

	// Find a healthy connection with higher priority (lower number)
	for _, conn := range fm.connections {
		if conn.ID == currentPrimaryID {
			continue
		}

		status, exists := fm.healthStatus[conn.ID]
		if !exists {
			continue
		}

		status.mu.RLock()
		healthy := status.IsHealthy
		status.mu.RUnlock()

		if healthy && conn.GetPriority() < currentPriority {
			// Found a better connection, switch to it
			currentPrimary.SetPrimaryConnection(false)
			conn.SetPrimaryConnection(true)
			fm.primaryConnID = conn.ID

			if fm.eventPublisher != nil {
				event := NewEvent(EventPrimaryChange, conn.ID,
					map[string]string{
						"old_primary": currentPrimaryID,
						"new_primary": conn.ID,
					},
					fmt.Sprintf("Recovered to higher priority connection: %s", conn.ID))
				fm.eventPublisher.Publish(event)
			}
			return
		}
	}
}

// findBestBackup finds the best available backup connection
func (fm *FailoverManager) findBestBackup(excludeID string) *Connection {
	candidates := make([]*Connection, 0)

	// Collect healthy connections
	for id, conn := range fm.connections {
		if id == excludeID {
			continue
		}

		status, exists := fm.healthStatus[id]
		if !exists {
			continue
		}

		status.mu.RLock()
		healthy := status.IsHealthy
		status.mu.RUnlock()

		if healthy && conn.GetState() == StateConnected {
			candidates = append(candidates, conn)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by priority (lower number = higher priority)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].GetPriority() < candidates[j].GetPriority()
	})

	return candidates[0]
}

// selectNewPrimary selects a new primary connection
func (fm *FailoverManager) selectNewPrimary() {
	backup := fm.findBestBackup("")
	if backup != nil {
		backup.SetPrimaryConnection(true)
		fm.primaryConnID = backup.ID

		if fm.eventPublisher != nil {
			event := NewEvent(EventPrimaryChange, backup.ID, nil,
				fmt.Sprintf("Selected new primary connection: %s", backup.ID))
			fm.eventPublisher.Publish(event)
		}
	}
}

// SetPrimary manually sets the primary connection
func (fm *FailoverManager) SetPrimary(connID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	conn, exists := fm.connections[connID]
	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}

	// Unset old primary
	if fm.primaryConnID != "" {
		if oldPrimary, exists := fm.connections[fm.primaryConnID]; exists {
			oldPrimary.SetPrimaryConnection(false)
		}
	}

	// Set new primary
	conn.SetPrimaryConnection(true)
	fm.primaryConnID = connID

	if fm.eventPublisher != nil {
		event := NewEvent(EventPrimaryChange, connID, nil,
			fmt.Sprintf("Manually set primary connection: %s", connID))
		fm.eventPublisher.Publish(event)
	}

	return nil
}

// GetPrimary returns the current primary connection ID
func (fm *FailoverManager) GetPrimary() string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.primaryConnID
}

// GetHealthStatus returns the health status of a connection
func (fm *FailoverManager) GetHealthStatus(connID string) (*HealthStatus, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	status, exists := fm.healthStatus[connID]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", connID)
	}

	return status, nil
}
