package core

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// MetricsCollector defines the interface for collecting connection metrics
type MetricsCollector interface {
	// Collect metrics for a specific connection
	Collect(ctx context.Context, conn *Connection) error

	// Start continuous metric collection
	Start(ctx context.Context, interval time.Duration)

	// Stop metric collection
	Stop()

	// Export metrics in a standard format
	Export() map[string]interface{}

	// GetConnectionMetrics returns metrics for a specific connection
	GetConnectionMetrics(connID string) (*ConnectionMetrics, error)
}

// DefaultMetricsCollector implements MetricsCollector
type DefaultMetricsCollector struct {
	mu              sync.RWMutex
	connections     map[string]*Connection
	latencyHistory  map[string][]time.Duration // Historical latency data for averaging
	historySize     int                        // Number of historical samples to keep
	ticker          *time.Ticker
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *DefaultMetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &DefaultMetricsCollector{
		connections:    make(map[string]*Connection),
		latencyHistory: make(map[string][]time.Duration),
		historySize:    10, // Keep last 10 samples for averaging
		ctx:            ctx,
		cancel:         cancel,
	}
}

// RegisterConnection adds a connection to be monitored
func (mc *DefaultMetricsCollector) RegisterConnection(conn *Connection) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.connections[conn.ID] = conn
}

// UnregisterConnection removes a connection from monitoring
func (mc *DefaultMetricsCollector) UnregisterConnection(connID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.connections, connID)
}

// Collect gathers metrics for a specific connection
func (mc *DefaultMetricsCollector) Collect(ctx context.Context, conn *Connection) error {
	// Measure actual latency
	latency, err := mc.measureLatency(ctx, conn)
	if err != nil {
		// If measurement fails, record the error but don't fail
		conn.Metrics.RecordFailure(err)
		latency = 0 // Use 0 to indicate measurement failure
	}

	// Store in history and calculate average
	mc.mu.Lock()
	history := mc.latencyHistory[conn.ID]
	history = append(history, latency)

	// Keep only the most recent samples
	if len(history) > mc.historySize {
		history = history[len(history)-mc.historySize:]
	}
	mc.latencyHistory[conn.ID] = history

	// Calculate average latency
	avgLatency := mc.calculateAverageLatency(history)
	mc.mu.Unlock()

	// Update connection metrics
	conn.Metrics.mu.Lock()
	conn.Metrics.Latency = avgLatency
	conn.Metrics.LastActive = time.Now()
	if conn.GetState() == StateConnected && !conn.StartedAt.IsZero() {
		conn.Metrics.Uptime = time.Since(conn.StartedAt)
	}
	conn.Metrics.mu.Unlock()

	return nil
}

// measureLatency performs actual latency measurement using TCP connection test
func (mc *DefaultMetricsCollector) measureLatency(ctx context.Context, conn *Connection) (time.Duration, error) {
	// Determine the target address for latency measurement
	target := mc.getLatencyTarget(conn)
	if target == "" {
		return 0, fmt.Errorf("no target available for latency measurement")
	}

	// Measure latency using TCP dial
	start := time.Now()

	// Use a timeout for the dial operation
	timeout := 5 * time.Second
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	tcpConn, err := dialer.DialContext(dialCtx, "tcp", target)
	if err != nil {
		// TCP connection failed, could be firewall or network issue
		return 0, fmt.Errorf("tcp dial failed: %w", err)
	}
	defer tcpConn.Close()

	latency := time.Since(start)
	return latency, nil
}

// getLatencyTarget determines the appropriate target for latency measurement
func (mc *DefaultMetricsCollector) getLatencyTarget(conn *Connection) string {
	// Try to use the connection's remote host and port if available
	if conn.RemoteHost != "" && conn.RemotePort > 0 {
		return fmt.Sprintf("%s:%d", conn.RemoteHost, conn.RemotePort)
	}

	// Fallback targets based on provider type
	switch conn.Method {
	case "cloudflare", "cloudflared":
		// Cloudflare's DNS service for latency check
		return "1.1.1.1:443"
	case "tailscale":
		// Tailscale coordination server
		return "controlplane.tailscale.com:443"
	case "ngrok":
		// ngrok's main server
		return "tunnel.us.ngrok.com:443"
	case "wireguard":
		// Try to connect to the local WireGuard interface
		return "127.0.0.1:51820"
	case "zerotier":
		// ZeroTier's main service
		return "my.zerotier.com:443"
	case "bore":
		// Bore typically uses a custom server, default to localhost
		return "127.0.0.1:2200"
	default:
		// Default fallback: use Google's DNS
		return "8.8.8.8:443"
	}
}

// calculateAverageLatency computes the average from historical samples
func (mc *DefaultMetricsCollector) calculateAverageLatency(history []time.Duration) time.Duration {
	if len(history) == 0 {
		return 0
	}

	var total time.Duration
	validSamples := 0

	for _, latency := range history {
		if latency > 0 { // Only count valid samples
			total += latency
			validSamples++
		}
	}

	if validSamples == 0 {
		return 0
	}

	return total / time.Duration(validSamples)
}

// Start begins continuous metric collection
func (mc *DefaultMetricsCollector) Start(ctx context.Context, interval time.Duration) {
	mc.mu.Lock()
	if mc.running {
		mc.mu.Unlock()
		return
	}
	mc.running = true
	mc.ticker = time.NewTicker(interval)
	// Recreate internal context for this run (in case of restart after Stop)
	mc.ctx, mc.cancel = context.WithCancel(ctx)
	// Copy context to local var to avoid race with Stop() modifying mc.ctx
	localCtx := mc.ctx
	mc.wg.Add(1)
	mc.mu.Unlock()

	go mc.collectLoop(localCtx)
}

// collectLoop runs the continuous collection loop
func (mc *DefaultMetricsCollector) collectLoop(ctx context.Context) {
	defer mc.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.ticker.C:
			mc.collectAll(ctx)
		}
	}
}

// collectAll collects metrics for all registered connections
func (mc *DefaultMetricsCollector) collectAll(ctx context.Context) {
	mc.mu.RLock()
	connections := make([]*Connection, 0, len(mc.connections))
	for _, conn := range mc.connections {
		connections = append(connections, conn)
	}
	mc.mu.RUnlock()

	// Collect metrics for each connection concurrently
	var wg sync.WaitGroup
	for _, conn := range connections {
		if conn.GetState() != StateConnected {
			continue
		}

		wg.Add(1)
		go func(c *Connection) {
			defer wg.Done()
			_ = mc.Collect(ctx, c)
		}(conn)
	}
	wg.Wait()
}

// Stop halts metric collection
func (mc *DefaultMetricsCollector) Stop() {
	mc.mu.Lock()
	if !mc.running {
		mc.mu.Unlock()
		return
	}

	mc.running = false
	if mc.ticker != nil {
		mc.ticker.Stop()
	}
	// Cancel context to signal goroutines to stop
	mc.cancel()
	mc.mu.Unlock()

	// Wait for goroutine to exit
	mc.wg.Wait()
}

// Export returns metrics in a standard format
func (mc *DefaultMetricsCollector) Export() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]interface{})
	result["timestamp"] = time.Now().Unix()
	result["total_connections"] = len(mc.connections)

	connections := make([]map[string]interface{}, 0)

	for _, conn := range mc.connections {
		sent, received, latency := conn.Metrics.GetStats()

		connData := map[string]interface{}{
			"id":             conn.ID,
			"method":         conn.Method,
			"state":          conn.GetState().String(),
			"bytes_sent":     sent,
			"bytes_received": received,
			"latency_ms":     latency.Milliseconds(),
			"uptime_seconds": conn.GetUptime().Seconds(),
			"is_primary":     conn.IsPrimaryConnection(),
			"priority":       conn.GetPriority(),
		}

		connections = append(connections, connData)
	}

	result["connections"] = connections
	return result
}

// GetConnectionMetrics returns metrics for a specific connection
func (mc *DefaultMetricsCollector) GetConnectionMetrics(connID string) (*ConnectionMetrics, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	conn, exists := mc.connections[connID]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", connID)
	}

	return conn.Metrics, nil
}

// LatencyMonitor monitors connection latency and reports issues
type LatencyMonitor struct {
	mu               sync.RWMutex
	thresholds       map[string]time.Duration // ConnID -> max acceptable latency
	violations       map[string]int           // ConnID -> violation count
	callback         func(connID string, latency time.Duration)
	defaultThreshold time.Duration
}

// NewLatencyMonitor creates a new latency monitor
func NewLatencyMonitor(defaultThreshold time.Duration, callback func(string, time.Duration)) *LatencyMonitor {
	return &LatencyMonitor{
		thresholds:       make(map[string]time.Duration),
		violations:       make(map[string]int),
		callback:         callback,
		defaultThreshold: defaultThreshold,
	}
}

// SetThreshold sets the latency threshold for a connection
func (lm *LatencyMonitor) SetThreshold(connID string, threshold time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.thresholds[connID] = threshold
}

// Check checks if latency exceeds threshold
func (lm *LatencyMonitor) Check(connID string, latency time.Duration) bool {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	threshold, exists := lm.thresholds[connID]
	if !exists {
		threshold = lm.defaultThreshold
	}

	if latency > threshold {
		lm.violations[connID]++
		if lm.callback != nil {
			go lm.callback(connID, latency)
		}
		return false
	}

	// Reset violations on success
	lm.violations[connID] = 0
	return true
}

// GetViolations returns the number of violations for a connection
func (lm *LatencyMonitor) GetViolations(connID string) int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.violations[connID]
}

// Reset clears violation counts
func (lm *LatencyMonitor) Reset(connID string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	delete(lm.violations, connID)
}
