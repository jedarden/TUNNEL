# TUNNEL Provider System

The TUNNEL TUI application uses a provider adapter system to support multiple VPN and tunnel solutions. This document describes the provider architecture and available implementations.

## Architecture

### Provider Interface

All providers implement the `Provider` interface defined in `/workspaces/ardenone-cluster/tunnel/internal/providers/provider.go`:

```go
type Provider interface {
    Name() string
    Category() Category // VPN, Tunnel, Direct

    // Lifecycle
    Install() error
    Uninstall() error
    IsInstalled() bool

    // Configuration
    Configure(config *ProviderConfig) error
    GetConfig() (*ProviderConfig, error)
    ValidateConfig(config *ProviderConfig) error

    // Connection
    Connect() error
    Disconnect() error
    IsConnected() bool
    GetConnectionInfo() (*ConnectionInfo, error)

    // Health
    HealthCheck() (*HealthStatus, error)
    GetLogs(since time.Time) ([]LogEntry, error)
}
```

### Categories

Providers are categorized into three types:

- **VPN**: Full mesh or peer-to-peer VPN solutions (Tailscale, WireGuard, ZeroTier)
- **Tunnel**: Port forwarding and tunnel services (Cloudflare Tunnel, ngrok, bore)
- **Direct**: Direct SSH connections (future implementation)

## Available Providers

### VPN Providers

#### Tailscale

**Location**: `/workspaces/ardenone-cluster/tunnel/internal/providers/tailscale/tailscale.go`

**Features**:
- Easy mesh VPN setup
- Built-in SSH support
- Automatic NAT traversal
- DNS names for all devices

**Configuration**:
```go
config := &ProviderConfig{
    Name: "tailscale",
    AuthKey: "tskey-auth-xxxx", // Optional for interactive auth
}
```

**Usage**:
```bash
# Check installation
tailscale --version

# Connect (requires auth key or browser login)
provider.Connect()

# Get status
provider.GetConnectionInfo()
```

#### WireGuard

**Location**: `/workspaces/ardenone-cluster/tunnel/internal/providers/wireguard/wireguard.go`

**Features**:
- High-performance VPN
- Strong encryption
- Minimal attack surface
- Configurable via config files

**Configuration**:
```go
config := &ProviderConfig{
    Name: "wireguard",
    ConfigFile: "/etc/wireguard/wg0.conf",
}
```

**Usage**:
```bash
# Check installation
wg version

# Connect (uses wg-quick)
provider.Connect()

# View interface
wg show wg0
```

#### ZeroTier

**Location**: `/workspaces/ardenone-cluster/tunnel/internal/providers/zerotier/zerotier.go`

**Features**:
- Software-defined networking
- Easy network management
- Multi-platform support
- Cloud or self-hosted controller

**Configuration**:
```go
config := &ProviderConfig{
    Name: "zerotier",
    NetworkID: "a0b1c2d3e4f5g6h7", // 16-character network ID
}
```

**Usage**:
```bash
# Check installation
zerotier-cli info

# Join network
provider.Connect()

# List networks
zerotier-cli listnetworks -j
```

### Tunnel Providers

#### Cloudflare Tunnel

**Location**: `/workspaces/ardenone-cluster/tunnel/internal/providers/cloudflare/cloudflare.go`

**Features**:
- No open ports required
- DDoS protection
- Access control policies
- HTTP/TCP/SSH support

**Configuration**:
```go
config := &ProviderConfig{
    Name: "cloudflare",
    TunnelName: "my-tunnel",
    AuthToken: "cloudflare-token", // Optional if using config file
}
```

**Usage**:
```bash
# Check installation
cloudflared --version

# Create tunnel (one-time setup)
cloudflared tunnel create my-tunnel

# Connect
provider.Connect()
```

#### ngrok

**Location**: `/workspaces/ardenone-cluster/tunnel/internal/providers/ngrok/ngrok.go`

**Features**:
- Quick TCP tunnel setup
- Public URL generation
- Local API for tunnel info
- Free tier available

**Configuration**:
```go
config := &ProviderConfig{
    Name: "ngrok",
    AuthToken: "ngrok-auth-token", // Optional for free tier
    LocalPort: 22, // SSH port
}
```

**Usage**:
```bash
# Check installation
ngrok version

# Start tunnel
provider.Connect()

# Get tunnel URL
info, _ := provider.GetConnectionInfo()
fmt.Println(info.TunnelURL) // tcp://0.tcp.ngrok.io:12345
```

#### bore

**Location**: `/workspaces/ardenone-cluster/tunnel/internal/providers/bore/bore.go`

**Features**:
- Simple TCP tunnel
- Minimal setup
- Rust-based (fast and efficient)
- No authentication required

**Configuration**:
```go
config := &ProviderConfig{
    Name: "bore",
    LocalPort: 22,
    RemoteHost: "bore.pub", // Default public server
}
```

**Usage**:
```bash
# Install via cargo
cargo install bore-cli

# Start tunnel
provider.Connect()
```

## Provider Registry

The provider registry manages all available providers and is located at `/workspaces/ardenone-cluster/tunnel/internal/registry/registry.go`.

### Usage Examples

```go
import "github.com/jedarden/tunnel/internal/registry"

// Get a specific provider
provider, err := registry.GetProvider("tailscale")

// List all providers
allProviders := registry.ListProviders()

// List by category
vpnProviders := registry.ListByCategory(providers.CategoryVPN)
tunnelProviders := registry.ListByCategory(providers.CategoryTunnel)

// Get installed providers
installed := registry.GetInstalledProviders()

// Get connected providers
connected := registry.GetConnectedProviders()

// Get provider information
info := registry.GetProviderInfo()
for _, p := range info {
    fmt.Printf("%s (%s): installed=%v, connected=%v\n",
        p.Name, p.Category, p.Installed, p.Connected)
}
```

## Common Provider Operations

### Check Installation

```go
provider, _ := registry.GetProvider("tailscale")
if provider.IsInstalled() {
    fmt.Println("Tailscale is installed")
}
```

### Configure and Connect

```go
provider, _ := registry.GetProvider("ngrok")

config := &providers.ProviderConfig{
    Name: "ngrok",
    LocalPort: 22,
    AuthToken: "your-token",
}

if err := provider.Configure(config); err != nil {
    log.Fatal(err)
}

if err := provider.Connect(); err != nil {
    log.Fatal(err)
}
```

### Get Connection Info

```go
info, err := provider.GetConnectionInfo()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", info.Status)
fmt.Printf("Local IP: %s\n", info.LocalIP)
fmt.Printf("Tunnel URL: %s\n", info.TunnelURL)
```

### Health Check

```go
health, err := provider.HealthCheck()
if err != nil {
    log.Fatal(err)
}

if health.Healthy {
    fmt.Printf("Provider is healthy: %s\n", health.Message)
} else {
    fmt.Printf("Provider is unhealthy: %s\n", health.Message)
}
```

### Disconnect

```go
if err := provider.Disconnect(); err != nil {
    log.Fatal(err)
}
```

## Adding New Providers

To add a new provider:

1. Create a new package under `/workspaces/ardenone-cluster/tunnel/internal/providers/yourprovider/`
2. Implement the `Provider` interface
3. Embed `*providers.BaseProvider` for common functionality
4. Register the provider in `/workspaces/ardenone-cluster/tunnel/internal/registry/registry.go`

Example skeleton:

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

func (y *YourProvider) Install() error {
    // Implementation
}

func (y *YourProvider) IsInstalled() bool {
    // Implementation
}

func (y *YourProvider) Connect() error {
    // Implementation
}

// ... implement other interface methods
```

## Error Handling

All providers return standard errors defined in `/workspaces/ardenone-cluster/tunnel/internal/providers/errors.go`:

- `ErrNotInstalled`: Provider binary not found
- `ErrAlreadyInstalled`: Provider already installed
- `ErrInvalidConfig`: Configuration validation failed
- `ErrConnectionFailed`: Connection attempt failed
- `ErrNotConnected`: Operation requires active connection
- `ErrProviderNotFound`: Provider not registered

## Testing

Run provider tests:

```bash
export PATH=$PATH:/usr/local/go/bin

# Test base provider
go test ./internal/providers/... -v

# Test registry
go test ./internal/registry/... -v

# Test all
go test ./... -v
```

## Security Considerations

- **Authentication Tokens**: Store tokens securely, never commit to version control
- **Config Files**: Ensure proper file permissions (600 for WireGuard configs)
- **Root Privileges**: Some operations may require sudo (WireGuard, ZeroTier)
- **Network Access**: Providers make outbound connections to their respective services

## Performance

Performance characteristics by provider:

- **Tailscale**: Low overhead, uses WireGuard protocol
- **WireGuard**: Fastest, kernel-level implementation
- **ZeroTier**: Moderate overhead, user-space implementation
- **Cloudflare**: Depends on edge location proximity
- **ngrok**: Good for development, free tier has limitations
- **bore**: Minimal overhead, direct TCP tunneling

## Troubleshooting

### Provider Not Found

```
Error: provider not found: xyz
```
Check that the provider is registered in the registry.

### Not Installed

```
Error: provider not installed
```
Install the provider using the appropriate package manager or download from the provider's website.

### Connection Failed

```
Error: connection failed
```
Check:
- Network connectivity
- Authentication credentials
- Firewall settings
- Provider service status

### Permission Denied

```
Error: permission denied
```
Some providers require elevated privileges. Try with sudo or configure appropriate permissions.
