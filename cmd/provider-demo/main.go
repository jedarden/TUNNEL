package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "list":
		listProviders()
	case "status":
		showStatus()
	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Usage: provider-demo info <provider-name>")
			os.Exit(1)
		}
		showProviderInfo(os.Args[2])
	case "health":
		if len(os.Args) < 3 {
			fmt.Println("Usage: provider-demo health <provider-name>")
			os.Exit(1)
		}
		checkHealth(os.Args[2])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("TUNNEL Provider Demo")
	fmt.Println("\nUsage:")
	fmt.Println("  provider-demo list                 - List all available providers")
	fmt.Println("  provider-demo status               - Show provider status")
	fmt.Println("  provider-demo info <provider>      - Show detailed provider info")
	fmt.Println("  provider-demo health <provider>    - Check provider health")
	fmt.Println("\nAvailable providers:")
	fmt.Println("  VPN: tailscale, wireguard, zerotier")
	fmt.Println("  Tunnel: cloudflare, ngrok, bore")
}

func listProviders() {
	fmt.Println("Available Providers")
	fmt.Println("===================\n")

	// Group by category
	vpnProviders := registry.ListByCategory(providers.CategoryVPN)
	tunnelProviders := registry.ListByCategory(providers.CategoryTunnel)

	fmt.Println("VPN Providers:")
	for _, p := range vpnProviders {
		fmt.Printf("  - %s\n", p.Name())
	}

	fmt.Println("\nTunnel Providers:")
	for _, p := range tunnelProviders {
		fmt.Printf("  - %s\n", p.Name())
	}

	fmt.Printf("\nTotal: %d providers\n", len(vpnProviders)+len(tunnelProviders))
}

func showStatus() {
	fmt.Println("Provider Status")
	fmt.Println("===============\n")

	info := registry.GetProviderInfo()

	// Create a tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tCATEGORY\tINSTALLED\tCONNECTED")
	fmt.Fprintln(w, "----\t--------\t---------\t---------")

	for _, p := range info {
		installed := "No"
		if p.Installed {
			installed = "Yes"
		}

		connected := "No"
		if p.Connected {
			connected = "Yes"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Category, installed, connected)
	}

	w.Flush()

	// Summary
	installedCount := 0
	connectedCount := 0
	for _, p := range info {
		if p.Installed {
			installedCount++
		}
		if p.Connected {
			connectedCount++
		}
	}

	fmt.Printf("\nSummary: %d installed, %d connected\n", installedCount, connectedCount)
}

func showProviderInfo(name string) {
	provider, err := registry.GetProvider(name)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Provider: %s\n", provider.Name())
	fmt.Printf("Category: %s\n", provider.Category())
	fmt.Printf("Installed: %v\n", provider.IsInstalled())
	fmt.Printf("Connected: %v\n", provider.IsConnected())

	if !provider.IsInstalled() {
		fmt.Println("\nThis provider is not installed on your system.")
		return
	}

	// Get connection info if connected
	if provider.IsConnected() {
		fmt.Println("\nConnection Information:")
		info, err := provider.GetConnectionInfo()
		if err != nil {
			fmt.Printf("  Error getting connection info: %v\n", err)
			return
		}

		fmt.Printf("  Status: %s\n", info.Status)
		if info.LocalIP != "" {
			fmt.Printf("  Local IP: %s\n", info.LocalIP)
		}
		if info.RemoteIP != "" {
			fmt.Printf("  Remote IP: %s\n", info.RemoteIP)
		}
		if info.TunnelURL != "" {
			fmt.Printf("  Tunnel URL: %s\n", info.TunnelURL)
		}
		if info.InterfaceName != "" {
			fmt.Printf("  Interface: %s\n", info.InterfaceName)
		}
		if len(info.Peers) > 0 {
			fmt.Printf("  Peers: %d\n", len(info.Peers))
		}
	} else {
		fmt.Println("\nThis provider is not currently connected.")
	}
}

func checkHealth(name string) {
	provider, err := registry.GetProvider(name)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Checking health of %s...\n\n", name)

	health, err := provider.HealthCheck()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Display health status
	status := "HEALTHY"
	icon := "✅"
	if !health.Healthy {
		status = "UNHEALTHY"
		icon = "❌"
	}

	fmt.Printf("%s Status: %s\n", icon, status)
	fmt.Printf("State: %s\n", health.Status)
	fmt.Printf("Message: %s\n", health.Message)
	fmt.Printf("Last Check: %s\n", health.LastCheck.Format("2006-01-02 15:04:05"))

	if health.Latency > 0 {
		fmt.Printf("Latency: %v\n", health.Latency)
	}

	if health.BytesSent > 0 || health.BytesReceived > 0 {
		fmt.Printf("\nTransfer Statistics:\n")
		fmt.Printf("  Sent: %d bytes\n", health.BytesSent)
		fmt.Printf("  Received: %d bytes\n", health.BytesReceived)
	}

	if len(health.Metrics) > 0 {
		fmt.Printf("\nMetrics:\n")
		for key, value := range health.Metrics {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// Exit with non-zero code if unhealthy
	if !health.Healthy {
		os.Exit(1)
	}
}
