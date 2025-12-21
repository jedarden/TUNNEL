# TUNNEL

**T**erminal **U**nified **N**etwork **N**ode **E**ncrypted **L**ink

A TUI (Terminal User Interface) application for managing secure SSH access to devpods and containers through multiple connection methods with built-in redundancy.

## Features

- **Unified Interface**: Single TUI for all remote access methods
- **Multi-Method Redundancy**: Run multiple connection methods simultaneously for failover
- **Provider Support**: Tailscale, Cloudflare Tunnel, WireGuard, ngrok, ZeroTier, bore, and more
- **Connection Monitoring**: Real-time status, metrics, and health checks
- **Credential Management**: Secure storage via system keyring
- **Key Management**: SSH key import from GitHub, validation, and rotation
- **Guided Setup**: Step-by-step wizards for each provider

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

### Launch TUI

```bash
tunnel
```

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
  theme: dark

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
│                           TUNNEL TUI                             │
├─────────────────────────────────────────────────────────────────┤
│  Dashboard  │  Methods  │  Config  │  Logs  │  Monitor          │
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
│   ├── tui/             # TUI components (Bubbletea)
│   ├── core/            # Core logic (connection, credentials)
│   ├── providers/       # Provider adapters
│   └── system/          # System utilities
├── pkg/config/          # Configuration management
├── configs/             # Default configuration files
└── docs/                # Documentation
```

## License

MIT License

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.
