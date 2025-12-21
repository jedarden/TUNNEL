# TUNNEL - Quick Reference Card

## Installation

```bash
# Build
export PATH=$PATH:/usr/local/go/bin
make build

# Install
make install

# Verify installation
tunnel version
```

## Common Commands

### Launch TUI
```bash
tunnel                    # Default - launches interactive TUI
```

### Connection Management
```bash
tunnel start cloudflared  # Start Cloudflare Tunnel
tunnel start ngrok        # Start ngrok
tunnel start tailscale    # Start Tailscale
tunnel start bore         # Start bore

tunnel stop cloudflared   # Stop specific tunnel
tunnel stop all           # Stop all tunnels

tunnel restart ngrok      # Restart tunnel

tunnel status             # Show all connection status
```

### Method Management
```bash
tunnel list               # List available methods
tunnel configure ngrok    # Interactive configuration
```

### Configuration
```bash
tunnel config get                    # Show all config
tunnel config get ssh.port           # Get specific value
tunnel config set ssh.port 2222      # Set value
tunnel config edit                   # Open in $EDITOR
```

### Authentication
```bash
tunnel auth login cloudflared        # Interactive login
tunnel auth set-key ngrok            # Set API key
tunnel auth status                   # Show auth status
```

### Diagnostics
```bash
tunnel doctor             # Run full diagnostics
tunnel version            # Show version info
```

### Output Options
```bash
tunnel --verbose status   # Verbose output
tunnel --json list        # JSON format
```

### Shell Completions
```bash
# Bash
tunnel completions bash > /etc/bash_completion.d/tunnel

# Zsh
tunnel completions zsh > "${fpath[1]}/_tunnel"

# Fish
tunnel completions fish > ~/.config/fish/completions/tunnel.fish
```

## Configuration File Locations

1. `$HOME/.config/tunnel/config.yaml`
2. `$HOME/.tunnel/config.yaml`
3. `./config.yaml`

## Environment Variables

Prefix all config keys with `TUNNEL_`:
```bash
export TUNNEL_SSH_PORT=2222
export TUNNEL_VERBOSE=true
```

## Exit Codes

- `0` - Success
- `1` - Error

## Keyboard Shortcuts (TUI Mode)

```
q, Ctrl+C    Quit
?            Help
↑/↓          Navigate
Enter        Select/Connect
Space        Toggle selection
d            Disconnect
r            Reconnect
```

## Available Tunnel Methods

1. **cloudflared** - Cloudflare Tunnel
2. **ngrok** - ngrok tunnel
3. **tailscale** - Tailscale VPN
4. **bore** - bore tunnel
5. **localhost.run** - localhost.run tunnel

## Common Configuration

### SSH Settings
```yaml
ssh:
  port: 22
  listen_address: 0.0.0.0
  authorized_keys_file: $HOME/.ssh/authorized_keys
```

### Provider Settings
```yaml
providers:
  cloudflared:
    enabled: true
    binary_path: cloudflared
  ngrok:
    enabled: true
    binary_path: ngrok
    api_key: "your-api-key"
```

### TUI Settings
```yaml
tui:
  show_help: true
  refresh_interval: 1
  theme: default
```

## Make Targets

```bash
make build        # Build binary
make install      # Install to ~/.local/bin
make test         # Run tests
make clean        # Clean build artifacts
make fmt          # Format code
make vet          # Run go vet
make lint         # Run linter
make doctor       # Run diagnostics
make version      # Show version
make completions  # Generate completions
make release      # Build for all platforms
make help         # Show all targets
```

## Troubleshooting

### Binary not found
```bash
# Ensure ~/.local/bin is in PATH
export PATH=$HOME/.local/bin:$PATH
```

### Provider not working
```bash
# Run diagnostics
tunnel doctor

# Check if provider binary is installed
which cloudflared
which ngrok
```

### SSH server not running
```bash
# Install and start SSH server
sudo apt-get install openssh-server
sudo systemctl start ssh
```

### Config file not found
```bash
# Create config directory
mkdir -p ~/.config/tunnel

# Create config file
tunnel config edit
```

## Resources

- Documentation: `/workspaces/ardenone-cluster/tunnel/docs/`
- Repository: https://github.com/jedarden/tunnel
- Issues: https://github.com/jedarden/tunnel/issues

## Support

For help:
```bash
tunnel --help
tunnel [command] --help
tunnel doctor
```

## Version Info

Check current version:
```bash
tunnel version
```

Output includes:
- Version number
- Build date
- Git commit
- Go version
- Platform (OS/Arch)
