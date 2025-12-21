package main

import (
	"fmt"
	"time"

	"github.com/jedarden/tunnel/internal/core"
)

func main() {
	fmt.Println("=== TUNNEL Connection Manager Demo ===\n")

	// Create connection manager with default config
	managerConfig := core.DefaultManagerConfig()
	manager := core.NewConnectionManager(managerConfig)

	// Register multiple providers (using mock providers for demo)
	fmt.Println("Registering connection providers...")
	manager.RegisterProvider(core.NewMockProvider("cloudflare", 0.1, 50*time.Millisecond))
	manager.RegisterProvider(core.NewMockProvider("tailscale", 0.05, 30*time.Millisecond))
	manager.RegisterProvider(core.NewMockProvider("ngrok", 0.15, 70*time.Millisecond))
	fmt.Println("  ✓ Registered: cloudflare, tailscale, ngrok\n")

	// Subscribe to events
	fmt.Println("Setting up event monitoring...")
	eventSub := manager.GetEventPublisher().Subscribe("demo", nil)
	go func() {
		for event := range eventSub.Channel {
			timestamp := event.Timestamp.Format("15:04:05")
			fmt.Printf("  [%s] EVENT: %s - %s (ConnID: %s)\n",
				timestamp, event.Type, event.Message, event.ConnID)
		}
	}()
	fmt.Println("  ✓ Event monitoring active\n")

	// Create connection configuration
	connConfig := core.DefaultConfig()
	connConfig.RemoteHost = "example.com"
	connConfig.RemotePort = 22
	connConfig.LocalPort = 8080

	// Start multiple connections for redundancy
	fmt.Println("Starting multiple connections for redundancy...")
	connections, err := manager.StartMultiple(
		[]string{"cloudflare", "tailscale", "ngrok"},
		connConfig,
	)

	if err != nil {
		fmt.Printf("  ✗ Error starting connections: %v\n", err)
	} else {
		fmt.Printf("  ✓ Successfully started %d connections\n\n", len(connections))
	}

	// Display connection details
	fmt.Println("Connection Details:")
	for _, conn := range connections {
		fmt.Printf("  - ID: %s\n", conn.ID)
		fmt.Printf("    Method: %s\n", conn.Method)
		fmt.Printf("    State: %s\n", conn.GetState())
		fmt.Printf("    Priority: %d\n", conn.GetPriority())
		fmt.Printf("    Primary: %v\n", conn.IsPrimaryConnection())
		fmt.Println()
	}

	// Enable automatic failover
	fmt.Println("Enabling automatic failover...")
	manager.EnableAutoFailover(true)
	fmt.Println("  ✓ Auto-failover enabled\n")

	// Get primary connection
	primary, err := manager.GetPrimary()
	if err != nil {
		fmt.Printf("  ✗ Error getting primary: %v\n", err)
	} else {
		fmt.Printf("Primary Connection: %s (Method: %s)\n\n", primary.ID, primary.Method)
	}

	// Simulate some activity - wait for health checks
	fmt.Println("Monitoring connections for 5 seconds...")
	time.Sleep(5 * time.Second)

	// List all active connections
	allConns, _ := manager.List()
	fmt.Printf("\nActive Connections: %d\n", len(allConns))
	for _, conn := range allConns {
		uptime := conn.GetUptime()
		sent, received, latency := conn.Metrics.GetStats()
		fmt.Printf("  - %s [%s]: Uptime=%v, Latency=%v, Sent=%d, Received=%d\n",
			conn.ID, conn.Method, uptime.Round(time.Second), latency, sent, received)
	}

	// Export metrics
	fmt.Println("\nExporting metrics...")
	metrics := manager.GetMetrics()
	fmt.Printf("Metrics Snapshot:\n")
	fmt.Printf("  Total Connections: %v\n", metrics["total_connections"])
	fmt.Printf("  Timestamp: %v\n\n", metrics["timestamp"])

	// Test manual failover
	if len(connections) > 1 {
		fmt.Println("Testing manual failover to second connection...")
		secondConn := connections[1]
		err := manager.SetPrimary(secondConn.ID)
		if err != nil {
			fmt.Printf("  ✗ Error setting primary: %v\n", err)
		} else {
			fmt.Printf("  ✓ Primary switched to: %s\n\n", secondConn.ID)
		}
	}

	// Monitor a specific connection
	if len(connections) > 0 {
		fmt.Printf("Monitoring specific connection: %s\n", connections[0].ID)
		monitorChan := manager.Monitor(connections[0].ID)
		go func() {
			timeout := time.After(3 * time.Second)
			for {
				select {
				case event, ok := <-monitorChan:
					if !ok {
						return
					}
					fmt.Printf("  [Monitor] %s: %s\n", event.Type, event.Message)
				case <-timeout:
					return
				}
			}
		}()
		time.Sleep(3 * time.Second)
		fmt.Println()
	}

	// Restart a connection
	if len(connections) > 0 {
		fmt.Printf("Testing connection restart: %s\n", connections[0].ID)
		err := manager.Restart(connections[0].ID)
		if err != nil {
			fmt.Printf("  ✗ Error restarting: %v\n", err)
		} else {
			fmt.Println("  ✓ Connection restarted\n")
		}
		time.Sleep(1 * time.Second)
	}

	// Graceful shutdown
	fmt.Println("Shutting down connection manager...")
	err = manager.Shutdown()
	if err != nil {
		fmt.Printf("  ✗ Error during shutdown: %v\n", err)
	} else {
		fmt.Println("  ✓ Shutdown complete\n")
	}

	fmt.Println("=== Demo Complete ===")
}
