# Connection Manager - Multi-Method Redundancy

## Overview

The Connection Manager is the core component of TUNNEL TUI that provides robust SSH tunnel connection management with multi-method redundancy support. It enables simultaneous connections through multiple providers (Cloudflare, Tailscale, ngrok, etc.) with automatic failover for high availability.

## Architecture

### Core Components

```
┌─────────────────────────────────────────────┐
│        Connection Manager Interface         │
└─────────────────┬───────────────────────────┘
                  │
         ┌────────┴─────────┐
         │                  │
┌────────▼────────┐  ┌──────▼──────────┐
│  Failover Mgr   │  │  Metrics        │
│  - Health Check │  │  - Latency      │
│  - Auto Switch  │  │  - Throughput   │
│  - Priority     │  │  - Uptime       │
└────────┬────────┘  └──────┬──────────┘
         │                  │
         └────────┬─────────┘
                  │
         ┌────────▼─────────┐
         │  Event Publisher │
         │  - Subscribers   │
         │  - Channels      │
         └──────────────────┘
```

## Key Features

### 1. Multi-Connection Support

Start multiple tunnel connections simultaneously:

```go
manager := core.NewConnectionManager(core.DefaultManagerConfig())

// Register providers
manager.RegisterProvider(cloudflareProvider)
manager.RegisterProvider(tailscaleProvider)
manager.RegisterProvider(ngrokProvider)

// Start all connections in parallel
connections, err := manager.StartMultiple(
    []string{"cloudflare", "tailscale", "ngrok"},
    config,
)
```

### 2. Automatic Failover

Connections are monitored continuously with automatic failover:

```go
// Enable automatic failover
manager.EnableAutoFailover(true)

// Failover happens automatically when:
// - Primary connection fails health checks
// - Latency exceeds configured threshold
// - Connection drops unexpectedly
```

**Failover Priority:**
- Connections are assigned priority based on start order
- Lower priority number = higher priority
- Failover switches to next highest priority healthy connection

### 3. Health Monitoring

Continuous health monitoring with configurable thresholds:

```go
failoverConfig := &core.FailoverConfig{
    Enabled:             true,
    HealthCheckInterval: 10 * time.Second,
    FailureThreshold:    3,    // Failures before failover
    RecoveryThreshold:   5,    // Successes to mark recovered
    MaxLatency:          500 * time.Millisecond,
    AutoRecover:         true, // Auto-switch back to higher priority
}
```

**Health Check Process:**
1. Verify connection state is `StateConnected`
2. Check latency is below `MaxLatency`
3. Track consecutive successes/failures
4. Trigger failover when `FailureThreshold` reached
5. Recover when `RecoveryThreshold` reached

### 4. Metrics Collection

Real-time metrics for all connections:

```go
// Export all metrics
metrics := manager.GetMetrics()
// Returns:
// {
//   "timestamp": 1234567890,
//   "total_connections": 3,
//   "connections": [
//     {
//       "id": "cloudflare-123",
//       "method": "cloudflare",
//       "state": "Connected",
//       "bytes_sent": 1024,
//       "bytes_received": 2048,
//       "latency_ms": 45,
//       "uptime_seconds": 3600,
//       "is_primary": true,
//       "priority": 0
//     },
//     ...
//   ]
// }

// Get specific connection metrics
connMetrics, err := manager.GetConnectionMetrics(connID)
latency := connMetrics.GetLatency()
sent, received, _ := connMetrics.GetStats()
```

### 5. Event System

Subscribe to connection events for monitoring and alerting:

```go
// Subscribe to all events
sub := manager.GetEventPublisher().Subscribe("monitor", nil)

// Or filter specific events
filter := func(event *core.ConnectionEvent) bool {
    return event.Type == core.EventFailover ||
           event.Type == core.EventError
}
sub := manager.GetEventPublisher().Subscribe("alerts", filter)

// Process events
go func() {
    for event := range sub.Channel {
        log.Printf("[%s] %s: %s",
            event.Type, event.ConnID, event.Message)
    }
}()
```

**Event Types:**
- `EventConnected` - Connection established
- `EventDisconnected` - Connection terminated
- `EventReconnecting` - Connection restarting
- `EventFailover` - Primary changed due to failure
- `EventMetricsUpdate` - Metrics updated
- `EventError` - Error occurred
- `EventStateChange` - Connection state changed
- `EventPrimaryChange` - Primary changed manually

## API Reference

### ConnectionManager Interface

```go
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
```

### Connection States

```go
const (
    StateDisconnected  // Connection not active
    StateConnecting    // Establishing connection
    StateConnected     // Fully connected and operational
    StateReconnecting  // Attempting to reconnect
    StateFailed        // Connection failed
)
```

### Configuration

```go
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
    ProviderConfigs     map[string]interface{}
}
```

## Usage Examples

### Basic Single Connection

```go
manager := core.NewConnectionManager(nil)
defer manager.Shutdown()

// Register provider
manager.RegisterProvider(myProvider)

// Start connection
config := core.DefaultConfig()
config.RemoteHost = "example.com"
config.RemotePort = 22
config.LocalPort = 8080

conn, err := manager.Start("my-provider", config)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Connected: %s (State: %s)\n", conn.ID, conn.GetState())
```

### Multi-Connection with Redundancy

```go
manager := core.NewConnectionManager(core.DefaultManagerConfig())
defer manager.Shutdown()

// Register multiple providers
manager.RegisterProvider(core.NewMockProvider("cloudflare", 0.05, 50*time.Millisecond))
manager.RegisterProvider(core.NewMockProvider("tailscale", 0.03, 30*time.Millisecond))
manager.RegisterProvider(core.NewMockProvider("ngrok", 0.10, 70*time.Millisecond))

// Start all connections (concurrent)
config := core.DefaultConfig()
connections, err := manager.StartMultiple(
    []string{"cloudflare", "tailscale", "ngrok"},
    config,
)

if err != nil {
    log.Printf("Warning: %v", err)
}

// Enable auto-failover
manager.EnableAutoFailover(true)

// Get primary connection
primary, _ := manager.GetPrimary()
fmt.Printf("Primary: %s (%s)\n", primary.ID, primary.Method)

// Monitor all connections
for _, conn := range connections {
    fmt.Printf("  - %s [%s]: Priority %d, Primary: %v\n",
        conn.ID, conn.Method, conn.GetPriority(), conn.IsPrimaryConnection())
}
```

### Event Monitoring and Alerting

```go
// Subscribe to failover events
sub := manager.GetEventPublisher().Subscribe("failover-alerts", func(e *core.ConnectionEvent) bool {
    return e.Type == core.EventFailover || e.Type == core.EventError
})

go func() {
    for event := range sub.Channel {
        // Send alert
        sendAlert(fmt.Sprintf("ALERT: %s - %s", event.Type, event.Message))

        // Log to file
        logEvent(event)

        // Update monitoring dashboard
        updateDashboard(event)
    }
}()
```

### Manual Failover Control

```go
// List all connections
connections, _ := manager.List()

// Find a specific connection
var targetConn *core.Connection
for _, conn := range connections {
    if conn.Method == "tailscale" && conn.GetState() == core.StateConnected {
        targetConn = conn
        break
    }
}

// Manually set as primary
if targetConn != nil {
    err := manager.SetPrimary(targetConn.ID)
    if err != nil {
        log.Printf("Failed to set primary: %v", err)
    }
}
```

### Metrics Export

```go
// Periodically export metrics
ticker := time.NewTicker(30 * time.Second)
defer ticker.Stop()

for range ticker.C {
    metrics := manager.GetMetrics()

    // Export to monitoring system
    exportToPrometheus(metrics)

    // Or write to file
    writeMetricsToFile(metrics)

    // Or log
    log.Printf("Metrics: %d active connections", metrics["total_connections"])
}
```

## Provider Implementation

To add a new connection provider, implement the `ConnectionProvider` interface:

```go
type MyProvider struct {
    name string
}

func (p *MyProvider) Name() string {
    return p.name
}

func (p *MyProvider) Connect(ctx context.Context, config *core.Config) (*core.Connection, error) {
    // 1. Establish tunnel connection
    // 2. Create Connection object
    // 3. Set state to StateConnected
    // 4. Return connection

    conn := core.NewConnection(
        generateID(),
        p.name,
        config.LocalPort,
        config.RemoteHost,
        config.RemotePort,
    )

    // ... establish connection ...

    conn.SetState(core.StateConnected)
    conn.StartedAt = time.Now()

    return conn, nil
}

func (p *MyProvider) Disconnect(conn *core.Connection) error {
    // Tear down the tunnel
    conn.SetState(core.StateDisconnected)
    return nil
}

func (p *MyProvider) IsHealthy(conn *core.Connection) bool {
    // Check if connection is healthy
    return conn.GetState() == core.StateConnected
}
```

## Concurrency

All public methods are **thread-safe**:
- Use `sync.RWMutex` for read-heavy operations
- Goroutines for parallel operations
- Context for cancellation
- Channels for event distribution

**Concurrent Operations:**
- Starting multiple connections runs in parallel
- Health checks are concurrent across all connections
- Metrics collection is concurrent
- Event publishing is non-blocking

## Best Practices

1. **Always defer Shutdown()**
   ```go
   manager := core.NewConnectionManager(config)
   defer manager.Shutdown()
   ```

2. **Use contexts for timeouts**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ```

3. **Monitor events for production**
   ```go
   // Always subscribe to events in production
   sub := manager.GetEventPublisher().Subscribe("prod", nil)
   go handleEvents(sub.Channel)
   ```

4. **Configure appropriate thresholds**
   ```go
   // Tune for your network conditions
   failoverConfig.HealthCheckInterval = 5 * time.Second
   failoverConfig.MaxLatency = 200 * time.Millisecond
   ```

5. **Handle partial failures**
   ```go
   // StartMultiple may succeed with some connections
   connections, err := manager.StartMultiple(methods, config)
   if len(connections) > 0 {
       // Use what we got
       log.Printf("Started %d/%d connections", len(connections), len(methods))
   }
   ```

## Performance Characteristics

- **Connection Start**: O(n) where n = number of methods (parallel)
- **Health Check**: O(n) where n = number of connections (parallel)
- **Failover**: O(n log n) for finding best backup (sorting by priority)
- **Event Publishing**: O(m) where m = number of subscribers (non-blocking)
- **Metrics Collection**: O(n) concurrent collection

## Files

- `/internal/core/connection.go` - Connection data structures
- `/internal/core/manager.go` - Connection manager implementation
- `/internal/core/failover.go` - Failover logic
- `/internal/core/metrics.go` - Metrics collection
- `/internal/core/events.go` - Event system
- `/internal/core/example_provider.go` - Mock provider for testing
- `/internal/core/manager_test.go` - Comprehensive tests
- `/examples/connection_manager_demo.go` - Usage demonstration

## Testing

Run the test suite:

```bash
export PATH=$PATH:/usr/local/go/bin
go test -v ./internal/core
```

Run the demo:

```bash
go run examples/connection_manager_demo.go
```

## Future Enhancements

- [ ] Load balancing across multiple active connections
- [ ] Connection pooling
- [ ] Bandwidth aggregation
- [ ] Custom health check implementations
- [ ] Persistent connection state
- [ ] Historical metrics storage
- [ ] WebSocket support for real-time monitoring
- [ ] gRPC API for remote management
