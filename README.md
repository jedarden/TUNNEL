# TUNNEL

**T**erminal **U**nified **N**etwork **N**ode **E**ncrypted **L**ink

A CLI application for managing secure SSH access to devpods and containers through multiple connection methods with built-in redundancy. Configuration is managed via an embedded web interface that is forwarded to the user's device.

## Features

- **Web Interface**: Embedded web server for configuration management, forwarded to user's device
- **Multi-Method Redundancy**: Run multiple connection methods simultaneously for failover
- **Provider Support**: Tailscale, Cloudflare Tunnel, WireGuard, ngrok, ZeroTier, bore, and more
- **Connection Monitoring**: Real-time status, metrics, and health checks
- **Credential Management**: Secure storage via system keyring
- **Key Management**: SSH key import from GitHub, validation, and rotation
- **REST API**: Full-featured API for automation and integration

## Supported Connection Methods

| Category | Providers |
|----------|-----------|
| **VPN/Mesh** | Tailscale, WireGuard, ZeroTier, Nebula |
| **Tunnel Services** | Cloudflare Tunnel, ngrok, bore, VS Code Tunnels |
| **Direct** | Reverse SSH, Bastion/Jump Host |

## Installation

### From Source

```bash
git clone https://github.com/jedarden/tunnel.git
cd tunnel
make build
sudo make install
```

### Using Go

```bash
go install github.com/jedarden/tunnel/cmd/tunnel@latest
```

## Usage

### Launch Web Server

```bash
# Start the web interface (default)
tunnel

# Specify a custom port
tunnel --port 9000
```

The web interface will be available at `http://localhost:8080` (or specified port) and provides full configuration management capabilities.

### CLI Commands

```bash
# Start connection with default method
tunnel start

# Start specific method
tunnel start tailscale

# Start multiple methods for redundancy
tunnel start tailscale,wireguard

# Stop all connections
tunnel stop all

# Show connection status
tunnel status

# List available methods
tunnel list

# Configure a method
tunnel configure tailscale

# Run diagnostics
tunnel doctor
```

### Configuration

Configuration is stored in `~/.config/tunnel/config.yaml`:

```yaml
version: "1.0"

settings:
  default_method: tailscale
  auto_reconnect: true
  log_level: info

web:
  port: 8080
  host: 0.0.0.0

methods:
  tailscale:
    enabled: true
    priority: 1
    auth_key_ref: "keyring:tunnel/tailscale/auth_key"

  wireguard:
    enabled: true
    priority: 2
    config_path: /etc/wireguard/wg0.conf

  cloudflare:
    enabled: false
    tunnel_name: "devpod-tunnel"

monitoring:
  enabled: true
  health_check_interval: 30s
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      TUNNEL Web Interface                        │
├─────────────────────────────────────────────────────────────────┤
│                    REST API (Fiber + WebSocket)                  │
├─────────────────────────────────────────────────────────────────┤
│                    Core Application Logic                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ Connection   │  │ Credential   │  │ Key          │           │
│  │ Manager      │  │ Store        │  │ Manager      │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
├─────────────────────────────────────────────────────────────────┤
│                      Provider Adapters                           │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │Tailscale│ │Cloudflare│ │WireGuard│ │  ngrok │ │ZeroTier │   │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Multi-Method Redundancy

TUNNEL supports running multiple connection methods simultaneously for redundancy:

```bash
# Start with automatic failover
tunnel start --methods tailscale,wireguard,ngrok --auto-failover

# Set priority order
tunnel config set methods.tailscale.priority 1
tunnel config set methods.wireguard.priority 2
tunnel config set methods.ngrok.priority 3
```

When the primary connection fails, TUNNEL automatically fails over to the next available method.

## Key Management

```bash
# Import SSH keys from GitHub
tunnel keys import --github username

# Add key manually
tunnel keys add --user developer

# List keys
tunnel keys list

# Revoke a key
tunnel keys revoke <key-id>
```

## API Endpoints

The embedded web server provides the following REST API:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/providers` | List all providers |
| GET | `/api/providers/:name` | Get provider details |
| POST | `/api/providers/:name/connect` | Connect provider |
| POST | `/api/providers/:name/disconnect` | Disconnect provider |
| GET | `/api/providers/:name/status` | Get provider status |
| POST | `/api/providers/:name/health` | Health check |

## Development

### Prerequisites

- Go 1.22+
- Make

### Building

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make install    # Install to /usr/local/bin
```

### Project Structure

```
tunnel/
├── cmd/tunnel/          # CLI entry point
├── internal/
│   ├── core/            # Core logic (connection, credentials)
│   ├── providers/       # Provider adapters
│   ├── web/             # Web API and embedded frontend
│   └── registry/        # Provider registry
├── pkg/config/          # Configuration management
├── configs/             # Default configuration files
└── docs/                # Documentation
```

## License

MIT License

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.
