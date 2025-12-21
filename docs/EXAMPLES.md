# TUNNEL Provider Examples

This document provides practical examples of using different providers in the TUNNEL application.

## Example 1: Using Tailscale for SSH Access

```go
package main

import (
    "fmt"
    "log"

    "github.com/jedarden/tunnel/internal/providers"
    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    // Get Tailscale provider
    provider, err := registry.GetProvider("tailscale")
    if err != nil {
        log.Fatal(err)
    }

    // Check if installed
    if !provider.IsInstalled() {
        fmt.Println("Please install Tailscale: https://tailscale.com/download")
        return
    }

    // Configure with auth key (optional)
    config := &providers.ProviderConfig{
        Name: "tailscale",
        AuthKey: "tskey-auth-xxxxxxxxxxxxx", // Get from Tailscale admin console
    }

    if err := provider.Configure(config); err != nil {
        log.Fatal(err)
    }

    // Connect
    fmt.Println("Connecting to Tailscale...")
    if err := provider.Connect(); err != nil {
        log.Fatal(err)
    }

    // Get connection info
    info, err := provider.GetConnectionInfo()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Connected! Your Tailscale IP: %s\n", info.LocalIP)
    fmt.Printf("SSH via: ssh user@%s\n", info.LocalIP)

    // Health check
    health, _ := provider.HealthCheck()
    fmt.Printf("Health: %s - %s\n", health.Status, health.Message)
}
```

## Example 2: Using ngrok for Quick SSH Tunnel

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/jedarden/tunnel/internal/providers"
    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    // Get ngrok provider
    provider, err := registry.GetProvider("ngrok")
    if err != nil {
        log.Fatal(err)
    }

    if !provider.IsInstalled() {
        fmt.Println("Installing ngrok: https://ngrok.com/download")
        return
    }

    // Configure
    config := &providers.ProviderConfig{
        Name: "ngrok",
        LocalPort: 22,
        AuthToken: "your-ngrok-auth-token", // Get from ngrok.com
    }

    if err := provider.Configure(config); err != nil {
        log.Fatal(err)
    }

    // Start tunnel
    fmt.Println("Starting ngrok tunnel...")
    if err := provider.Connect(); err != nil {
        log.Fatal(err)
    }

    // Wait for tunnel to establish
    time.Sleep(3 * time.Second)

    // Get the public tunnel URL
    info, err := provider.GetConnectionInfo()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Tunnel active at: %s\n", info.TunnelURL)
    fmt.Printf("Connect via: ssh -p PORT user@HOST\n")

    // Keep tunnel running
    fmt.Println("Press Ctrl+C to stop...")
    select {}
}
```

## Example 3: Multi-Provider Status Dashboard

```go
package main

import (
    "fmt"

    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    fmt.Println("TUNNEL Provider Status")
    fmt.Println("======================\n")

    // Get all provider info
    info := registry.GetProviderInfo()

    // Group by category
    vpnProviders := make([]registry.ProviderInfo, 0)
    tunnelProviders := make([]registry.ProviderInfo, 0)

    for _, p := range info {
        switch p.Category {
        case "vpn":
            vpnProviders = append(vpnProviders, p)
        case "tunnel":
            tunnelProviders = append(tunnelProviders, p)
        }
    }

    // Display VPN providers
    fmt.Println("VPN Providers:")
    for _, p := range vpnProviders {
        status := "❌ Not installed"
        if p.Installed {
            if p.Connected {
                status = "✅ Connected"
            } else {
                status = "⚠️  Installed, not connected"
            }
        }
        fmt.Printf("  %s: %s\n", p.Name, status)
    }

    // Display Tunnel providers
    fmt.Println("\nTunnel Providers:")
    for _, p := range tunnelProviders {
        status := "❌ Not installed"
        if p.Installed {
            if p.Connected {
                status = "✅ Connected"
            } else {
                status = "⚠️  Installed, not connected"
            }
        }
        fmt.Printf("  %s: %s\n", p.Name, status)
    }
}
```

## Example 4: WireGuard Configuration Manager

```go
package main

import (
    "fmt"
    "log"

    "github.com/jedarden/tunnel/internal/providers"
    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    provider, err := registry.GetProvider("wireguard")
    if err != nil {
        log.Fatal(err)
    }

    if !provider.IsInstalled() {
        fmt.Println("Please install WireGuard")
        return
    }

    // Configure with config file
    config := &providers.ProviderConfig{
        Name: "wireguard",
        ConfigFile: "/etc/wireguard/wg0.conf",
    }

    // Validate config
    if err := provider.ValidateConfig(config); err != nil {
        log.Fatal("Config validation failed:", err)
    }

    if err := provider.Configure(config); err != nil {
        log.Fatal(err)
    }

    // Bring up interface
    fmt.Println("Bringing up WireGuard interface...")
    if err := provider.Connect(); err != nil {
        log.Fatal(err)
    }

    // Get interface details
    info, err := provider.GetConnectionInfo()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Interface: %s\n", info.InterfaceName)
    fmt.Printf("Local IP: %s\n", info.LocalIP)
    fmt.Printf("Peers: %v\n", info.Peers)

    // Check health with stats
    health, _ := provider.HealthCheck()
    fmt.Printf("\nBytes sent: %d\n", health.BytesSent)
    fmt.Printf("Bytes received: %d\n", health.BytesReceived)
}
```

## Example 5: Automatic Provider Selection

```go
package main

import (
    "fmt"
    "log"

    "github.com/jedarden/tunnel/internal/providers"
    "github.com/jedarden/tunnel/internal/registry"
)

func findBestProvider() (providers.Provider, error) {
    // Try providers in order of preference
    preferences := []string{
        "tailscale",
        "wireguard",
        "ngrok",
        "bore",
    }

    for _, name := range preferences {
        provider, err := registry.GetProvider(name)
        if err != nil {
            continue
        }

        if provider.IsInstalled() {
            return provider, nil
        }
    }

    return nil, fmt.Errorf("no suitable provider found")
}

func main() {
    provider, err := findBestProvider()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Using provider: %s\n", provider.Name())

    // Auto-configure based on provider
    config := &providers.ProviderConfig{
        Name: provider.Name(),
    }

    // Add provider-specific defaults
    if provider.Name() == "ngrok" || provider.Name() == "bore" {
        config.LocalPort = 22
    }

    provider.Configure(config)

    if err := provider.Connect(); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Connected successfully!")
}
```

## Example 6: Provider Health Monitoring

```go
package main

import (
    "fmt"
    "time"

    "github.com/jedarden/tunnel/internal/registry"
)

func monitorProviders() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            connected := registry.GetConnectedProviders()

            fmt.Printf("\n[%s] Provider Health Check\n", time.Now().Format("15:04:05"))
            fmt.Println("----------------------------")

            if len(connected) == 0 {
                fmt.Println("No providers connected")
                continue
            }

            for _, provider := range connected {
                health, err := provider.HealthCheck()
                if err != nil {
                    fmt.Printf("%s: ERROR - %v\n", provider.Name(), err)
                    continue
                }

                status := "✅"
                if !health.Healthy {
                    status = "❌"
                }

                fmt.Printf("%s %s: %s\n", status, provider.Name(), health.Message)

                if health.Latency > 0 {
                    fmt.Printf("   Latency: %v\n", health.Latency)
                }
            }
        }
    }
}

func main() {
    fmt.Println("Starting provider health monitor...")
    monitorProviders()
}
```

## Example 7: ZeroTier Network Manager

```go
package main

import (
    "fmt"
    "log"

    "github.com/jedarden/tunnel/internal/providers"
    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    provider, err := registry.GetProvider("zerotier")
    if err != nil {
        log.Fatal(err)
    }

    if !provider.IsInstalled() {
        fmt.Println("Please install ZeroTier: https://www.zerotier.com/download")
        return
    }

    // Join network
    config := &providers.ProviderConfig{
        Name: "zerotier",
        NetworkID: "a0b1c2d3e4f5g6h7", // Your network ID
    }

    if err := provider.Configure(config); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Joining ZeroTier network...")
    if err := provider.Connect(); err != nil {
        log.Fatal(err)
    }

    // Wait for network to be ready
    fmt.Println("Waiting for network to be ready...")
    for i := 0; i < 30; i++ {
        if provider.IsConnected() {
            break
        }
        time.Sleep(1 * time.Second)
    }

    // Get assigned IP
    info, err := provider.GetConnectionInfo()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Connected to network!\n")
    fmt.Printf("Network ID: %v\n", info.Extra["network_id"])
    fmt.Printf("Assigned IP: %s\n", info.LocalIP)
}
```

## Example 8: Cloudflare Tunnel Setup

```go
package main

import (
    "fmt"
    "log"

    "github.com/jedarden/tunnel/internal/providers"
    "github.com/jedarden/tunnel/internal/providers/cloudflare"
    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    provider, err := registry.GetProvider("cloudflare")
    if err != nil {
        log.Fatal(err)
    }

    if !provider.IsInstalled() {
        fmt.Println("Please install cloudflared")
        return
    }

    // Cast to CloudflareProvider to access specific methods
    cfProvider := provider.(*cloudflare.CloudflareProvider)

    // List existing tunnels
    tunnels, err := cfProvider.ListTunnels()
    if err != nil {
        fmt.Println("Note: Could not list tunnels. Make sure you're authenticated.")
    } else {
        fmt.Println("Existing tunnels:")
        for _, t := range tunnels {
            fmt.Printf("  - %s (ID: %s)\n", t.Name, t.ID)
        }
    }

    // Configure and connect
    config := &providers.ProviderConfig{
        Name: "cloudflare",
        TunnelName: "my-ssh-tunnel",
        AuthToken: "your-tunnel-token",
    }

    if err := provider.Configure(config); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Starting Cloudflare Tunnel...")
    if err := provider.Connect(); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Tunnel started! Check Cloudflare dashboard for connection details.")
}
```

## Building and Running Examples

```bash
# Export Go path
export PATH=$PATH:/usr/local/go/bin

# Create an example file
cat > example.go << 'EOF'
package main

import (
    "fmt"
    "github.com/jedarden/tunnel/internal/registry"
)

func main() {
    providers := registry.ListProviders()
    fmt.Printf("Available providers: %d\n", len(providers))
    for _, p := range providers {
        fmt.Printf("- %s (%s)\n", p.Name(), p.Category())
    }
}
EOF

# Run the example
go run example.go
```
