# TUNNEL Provider System Implementation

This directory contains the provider adapter system for the TUNNEL TUI application.

## Directory Structure

```
internal/providers/
├── provider.go          # Base provider interface and types
├── errors.go            # Provider error definitions
├── provider_test.go     # Base provider tests
├── bore/
│   └── bore.go         # bore tunnel provider
├── cloudflare/
│   └── cloudflare.go   # Cloudflare Tunnel provider
├── ngrok/
│   └── ngrok.go        # ngrok tunnel provider
├── tailscale/
│   └── tailscale.go    # Tailscale VPN provider
├── wireguard/
│   └── wireguard.go    # WireGuard VPN provider
└── zerotier/
    └── zerotier.go     # ZeroTier VPN provider

internal/registry/
├── registry.go          # Provider registry and management
└── registry_test.go     # Registry tests
```

## Core Components

### Provider Interface

The `Provider` interface defines the contract that all providers must implement:

- **Identity**: `Name()`, `Category()`
- **Lifecycle**: `Install()`, `Uninstall()`, `IsInstalled()`
- **Configuration**: `Configure()`, `GetConfig()`, `ValidateConfig()`
- **Connection**: `Connect()`, `Disconnect()`, `IsConnected()`, `GetConnectionInfo()`
- **Health**: `HealthCheck()`, `GetLogs()`

### Categories

- `CategoryVPN`: Mesh/P2P VPN solutions
- `CategoryTunnel`: Port forwarding tunnels
- `CategoryDirect`: Direct connections

### Data Structures

**ProviderConfig**: Configuration for providers
```go
type ProviderConfig struct {
    Name       string
    AuthToken  string
    AuthKey    string
    NetworkID  string
    TunnelName string
    RemoteHost string
    RemotePort int
    LocalPort  int
    ConfigFile string
    Extra      map[string]string
}
```

**ConnectionInfo**: Current connection state
```go
type ConnectionInfo struct {
    Status        string
    ConnectedAt   time.Time
    LocalIP       string
    RemoteIP      string
    TunnelURL     string
    InterfaceName string
    Peers         []string
    Extra         map[string]interface{}
}
```

**HealthStatus**: Provider health information
```go
type HealthStatus struct {
    Healthy       bool
    Status        string
    Message       string
    LastCheck     time.Time
    Latency       time.Duration
    BytesSent     uint64
    BytesReceived uint64
    Metrics       map[string]interface{}
}
```

## Implemented Providers

### VPN Providers

1. **Tailscale** (`tailscale/`)
   - Mesh VPN with WireGuard protocol
   - Built-in SSH support
   - Command: `tailscale`
   - Status: Fully implemented

2. **WireGuard** (`wireguard/`)
   - Kernel-level VPN
   - Config file based
   - Command: `wg`, `wg-quick`
   - Status: Fully implemented

3. **ZeroTier** (`zerotier/`)
   - Software-defined networking
   - Network join/leave
   - Command: `zerotier-cli`
   - Status: Fully implemented

### Tunnel Providers

1. **Cloudflare Tunnel** (`cloudflare/`)
   - Cloudflare Zero Trust tunnels
   - No open ports required
   - Command: `cloudflared`
   - Status: Fully implemented

2. **ngrok** (`ngrok/`)
   - TCP tunnel for SSH
   - Public URL generation
   - Command: `ngrok`
   - Status: Fully implemented

3. **bore** (`bore/`)
   - Simple TCP tunnel
   - Rust-based
   - Command: `bore`
   - Status: Fully implemented with auto-install

## Provider Registry

The registry (`internal/registry/`) manages all available providers:

```go
// Get a provider
provider, err := registry.GetProvider("tailscale")

// List all providers
providers := registry.ListProviders()

// List by category
vpnProviders := registry.ListByCategory(providers.CategoryVPN)

// Get installed providers
installed := registry.GetInstalledProviders()

// Get connected providers
connected := registry.GetConnectedProviders()

// Get provider information
info := registry.GetProviderInfo()
```

## Usage Examples

### Basic Usage

```go
// Get provider
provider, err := registry.GetProvider("tailscale")

// Configure
config := &providers.ProviderConfig{
    Name: "tailscale",
    AuthKey: "tskey-auth-xxx",
}
provider.Configure(config)

// Connect
if err := provider.Connect(); err != nil {
    log.Fatal(err)
}

// Get info
info, _ := provider.GetConnectionInfo()
fmt.Printf("Connected! IP: %s\n", info.LocalIP)

// Disconnect
provider.Disconnect()
```

### Health Monitoring

```go
health, err := provider.HealthCheck()
if health.Healthy {
    fmt.Printf("✅ %s is healthy\n", provider.Name())
} else {
    fmt.Printf("❌ %s is unhealthy: %s\n", provider.Name(), health.Message)
}
```

## Testing

Run tests:

```bash
export PATH=$PATH:/usr/local/go/bin

# Test providers
go test ./internal/providers/... -v

# Test registry
go test ./internal/registry/... -v

# Test all
go test ./... -v
```

## Demo Application

A demo CLI is provided in `cmd/provider-demo/`:

```bash
# Build the demo
go build -o provider-demo ./cmd/provider-demo/

# List providers
./provider-demo list

# Show status
./provider-demo status

# Get provider info
./provider-demo info tailscale

# Check health
./provider-demo health tailscale
```

## Adding New Providers

To add a new provider:

1. Create a new directory under `internal/providers/yourprovider/`
2. Create `yourprovider.go` implementing the `Provider` interface
3. Embed `*providers.BaseProvider` for common functionality
4. Register in `internal/registry/registry.go`

Example:

```go
package yourprovider

import "github.com/jedarden/tunnel/internal/providers"

type YourProvider struct {
    *providers.BaseProvider
}

func New() *YourProvider {
    return &YourProvider{
        BaseProvider: providers.NewBaseProvider("yourprovider", providers.CategoryTunnel),
    }
}

// Implement all Provider interface methods...
```

Then register in `registry.go`:

```go
import "github.com/jedarden/tunnel/internal/providers/yourprovider"

func (r *Registry) registerDefaultProviders() {
    // ... existing providers
    r.Register(yourprovider.New())
}
```

## Error Handling

All providers use standard errors from `errors.go`:

- `ErrNotInstalled`: Provider not installed
- `ErrAlreadyInstalled`: Already installed
- `ErrInvalidConfig`: Invalid configuration
- `ErrConnectionFailed`: Connection failed
- `ErrNotConnected`: Not connected
- `ErrProviderNotFound`: Provider not registered

## Security Considerations

- **Credentials**: Never commit auth tokens/keys to version control
- **File Permissions**: Config files should have restricted permissions (e.g., 600)
- **Privileges**: Some providers require sudo (WireGuard, ZeroTier)
- **Network Security**: Providers make outbound connections

## Documentation

- **PROVIDERS.md**: Detailed provider documentation
- **EXAMPLES.md**: Usage examples and code samples
- **README.md**: This file

## Implementation Status

| Provider   | Category | Installed | Connect | Disconnect | Health | Status |
|------------|----------|-----------|---------|------------|--------|---------|
| Tailscale  | VPN      | ✅        | ✅      | ✅         | ✅     | Complete |
| WireGuard  | VPN      | ✅        | ✅      | ✅         | ✅     | Complete |
| ZeroTier   | VPN      | ✅        | ✅      | ✅         | ✅     | Complete |
| Cloudflare | Tunnel   | ✅        | ✅      | ✅         | ✅     | Complete |
| ngrok      | Tunnel   | ✅        | ✅      | ✅         | ✅     | Complete |
| bore       | Tunnel   | ✅        | ✅      | ✅         | ✅     | Complete |

## Performance Notes

- **Tailscale**: Excellent performance, uses WireGuard kernel module
- **WireGuard**: Best performance, kernel-level implementation
- **ZeroTier**: Good performance, user-space implementation
- **Cloudflare**: Depends on edge location, includes DDoS protection
- **ngrok**: Good for development, free tier has rate limits
- **bore**: Minimal overhead, direct TCP proxy

## Future Enhancements

Potential additions:

- [ ] SSH direct connection provider
- [ ] Twingate provider
- [ ] OpenVPN provider
- [ ] Nebula provider
- [ ] Headscale (open-source Tailscale) provider
- [ ] Provider metrics collection
- [ ] Provider event system
- [ ] Configuration file persistence
- [ ] Provider health history

## License

This implementation is part of the TUNNEL TUI application.
