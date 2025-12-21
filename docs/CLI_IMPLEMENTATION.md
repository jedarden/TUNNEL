# TUNNEL CLI Implementation

## Overview

The TUNNEL CLI interface and main entry point have been successfully implemented. This document provides an overview of the implementation, file structure, and usage.

## Project Structure

```
/workspaces/ardenone-cluster/tunnel/
├── cmd/tunnel/
│   ├── main.go          # Main entry point with signal handling
│   ├── cli.go           # Cobra CLI command structure
│   ├── doctor.go        # Diagnostic tool implementation
│   └── version.go       # Version command implementation
├── internal/system/
│   ├── process.go       # Process management utilities
│   ├── network.go       # Network connectivity utilities
│   └── ssh.go           # SSH server and key management
├── Makefile             # Build automation
├── .goreleaser.yaml     # Release automation configuration
└── bin/                 # Compiled binaries (generated)
```

## Implemented Files

### 1. cmd/tunnel/main.go
Main entry point with:
- Signal handling (SIGINT, SIGTERM) for graceful shutdown
- Configuration initialization using Viper
- Default configuration values
- Environment variable support (TUNNEL_* prefix)

### 2. cmd/tunnel/cli.go
Complete CLI structure using Cobra:

**Root Command:**
- `tunnel` - Launch TUI (default behavior)

**Connection Commands:**
- `tunnel start [method]` - Start tunnel connection
- `tunnel stop [method|all]` - Stop connection(s)
- `tunnel restart [method]` - Restart connection
- `tunnel status` - Show all connection status

**Method Management:**
- `tunnel list` - List available tunnel methods
- `tunnel configure <method>` - Configure method interactively

**Config Commands:**
- `tunnel config get [key]` - Show configuration
- `tunnel config set <key> <value>` - Set configuration
- `tunnel config edit` - Open config in $EDITOR

**Auth Commands:**
- `tunnel auth login <method>` - Interactive authentication
- `tunnel auth set-key <method>` - Set API key
- `tunnel auth status` - Show auth status

**Utility Commands:**
- `tunnel doctor` - Diagnose and fix issues
- `tunnel version` - Show version information
- `tunnel completions <shell>` - Generate shell completions (bash, zsh, fish)

**Global Flags:**
- `--config` - Custom config file path
- `--verbose/-v` - Verbose output
- `--json` - JSON output format

### 3. cmd/tunnel/doctor.go
Diagnostic tool that checks:
- ✓ Configuration file existence and validity
- ✓ Provider binary availability (cloudflared, ngrok, tailscale, bore)
- ✓ Internet connectivity
- ✓ SSH server status
- ✓ Port availability
- ✓ File permissions
- ✓ System requirements

Provides colored output with:
- Pass (✓) - Green
- Warning (⚠) - Yellow
- Fail (✗) - Red
- Suggested fixes for each issue

### 4. cmd/tunnel/version.go
Version command showing:
- Version number
- Build date
- Git commit hash
- Go version
- Compiler
- Platform (OS/Arch)

Supports `--json` flag for machine-readable output.

### 5. internal/system/process.go
Process management utilities:
- `ProcessManager` - Manages background processes
- Start, stop, kill processes
- Track process status (running, stopped, failed)
- Graceful shutdown with timeout
- Orphaned process detection
- Process monitoring

### 6. internal/system/network.go
Network utilities:
- Get local IP addresses
- Get public IP address
- Check port availability
- Test connectivity (TCP and HTTP)
- Resolve hostnames
- List network interfaces
- Validate IP addresses and ports

### 7. internal/system/ssh.go
SSH utilities:
- Check SSH server status
- Get SSH server port
- Manage authorized_keys file
- Add/remove public keys
- Generate SSH config snippets
- Generate SSH key pairs
- Validate SSH public keys
- Get SSH host key fingerprints

### 8. Makefile
Build automation with targets:
- `make build` - Build binary
- `make install` - Install to ~/.local/bin
- `make test` - Run tests
- `make clean` - Clean build artifacts
- `make fmt` - Format code
- `make vet` - Run go vet
- `make lint` - Run linter
- `make doctor` - Run diagnostics
- `make version` - Show version
- `make completions` - Generate shell completions
- `make release` - Build for all platforms
- `make help` - Show available targets

### 9. .goreleaser.yaml
Release automation supporting:
- Multi-platform builds (Linux, macOS, Windows)
- Multiple architectures (amd64, arm64, arm)
- Package formats (deb, rpm, apk)
- Homebrew tap
- Snapcraft
- Docker images
- GitHub releases
- Automatic changelog generation

## Dependencies

The following Go packages are used:

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/fatih/color` - Colored terminal output
- `github.com/charmbracelet/bubbletea` - TUI framework (for future integration)
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/charmbracelet/lipgloss` - TUI styling

## Build Instructions

### Prerequisites

- Go 1.22 or later
- Make (optional, for automation)

### Building

```bash
# Export Go path
export PATH=$PATH:/usr/local/go/bin

# Build using make
make build

# Or build directly with go
go build -o bin/tunnel ./cmd/tunnel
```

### Installing

```bash
# Install to ~/.local/bin
make install

# Ensure ~/.local/bin is in your PATH
export PATH=$HOME/.local/bin:$PATH
```

## Usage Examples

### Basic Usage

```bash
# Launch TUI (default)
tunnel

# Start a tunnel
tunnel start cloudflared

# Check status
tunnel status

# List available methods
tunnel list
```

### Configuration

```bash
# View all config
tunnel config get

# Get specific value
tunnel config get ssh.port

# Set configuration
tunnel config set ssh.port 2222

# Edit config file
tunnel config edit
```

### Authentication

```bash
# Login to provider
tunnel auth login ngrok

# Set API key
tunnel auth set-key cloudflared

# Check auth status
tunnel auth status
```

### Diagnostics

```bash
# Run diagnostics
tunnel doctor

# Show version
tunnel version

# Verbose output
tunnel --verbose status

# JSON output
tunnel --json list
```

### Shell Completions

```bash
# Bash
source <(tunnel completions bash)

# Zsh
source <(tunnel completions zsh)

# Fish
tunnel completions fish | source

# Or generate to files
make completions
```

## Configuration

Configuration is loaded from (in order):
1. `$HOME/.config/tunnel/config.yaml`
2. `$HOME/.tunnel/config.yaml`
3. Current directory `./config.yaml`
4. Environment variables (prefix: `TUNNEL_`)

### Default Configuration

```yaml
verbose: false
log_level: info
log_file: ""

ssh:
  port: 22
  listen_address: 0.0.0.0
  authorized_keys_file: $HOME/.ssh/authorized_keys

providers:
  cloudflared:
    enabled: true
    binary_path: cloudflared
  ngrok:
    enabled: true
    binary_path: ngrok
  tailscale:
    enabled: true
    binary_path: tailscale
  bore:
    enabled: true
    binary_path: bore
    server: bore.pub
  localhost:
    enabled: true

tui:
  show_help: true
  refresh_interval: 1
  theme: default

monitoring:
  enabled: true
  check_interval: 30
  auto_reconnect: true
```

## Testing

The CLI has been tested with:

1. **Help Command**: ✓ Working
   ```
   tunnel --help
   ```

2. **Version Command**: ✓ Working
   ```
   tunnel version
   ```

3. **List Command**: ✓ Working
   ```
   tunnel list
   ```

4. **Status Command**: ✓ Working
   ```
   tunnel status
   ```

5. **Doctor Command**: ✓ Working
   ```
   tunnel doctor
   ```

6. **Config Commands**: ✓ Working
   ```
   tunnel config get
   tunnel auth status
   ```

7. **JSON Output**: ✓ Working
   ```
   tunnel --json list
   ```

8. **Completions**: ✓ Working
   ```
   make completions
   ```

## Exit Codes

- `0` - Success
- `1` - General error
- Other codes may be used for specific error conditions

## Signal Handling

The application handles the following signals gracefully:
- `SIGINT` (Ctrl+C)
- `SIGTERM`

On receiving these signals, the application will:
1. Print a shutdown message
2. Stop all running tunnel connections
3. Clean up resources
4. Exit cleanly

## Future Integration Points

The CLI is designed to integrate with:

1. **TUI Package** - The `launchTUI()` function in cli.go is ready to launch the TUI when implemented
2. **Provider Packages** - Connection management functions are placeholders for actual provider implementations
3. **Core Package** - Will integrate with core connection management logic
4. **Monitoring** - Will integrate with connection monitoring and auto-reconnect features

## Color Output

The CLI uses colored output for better readability:
- **Green (✓)**: Success, enabled, active
- **Yellow (⚠)**: Warnings, not configured
- **Red (✗)**: Errors, failures, disabled
- **Cyan**: Headers, section titles
- **White**: General information

Colors are automatically disabled when output is not a TTY (e.g., piped to a file).

## Next Steps

1. Implement TUI components integration
2. Implement actual tunnel provider connections
3. Add state persistence for running connections
4. Add connection monitoring and health checks
5. Implement auto-reconnect logic
6. Add comprehensive unit tests
7. Add integration tests
8. Create user documentation
9. Set up CI/CD pipeline

## Files Summary

| File | Lines | Purpose |
|------|-------|---------|
| cmd/tunnel/main.go | 83 | Entry point, config initialization |
| cmd/tunnel/cli.go | 350 | CLI command structure |
| cmd/tunnel/doctor.go | 280 | Diagnostic tool |
| cmd/tunnel/version.go | 40 | Version command |
| internal/system/process.go | 230 | Process management |
| internal/system/network.go | 245 | Network utilities |
| internal/system/ssh.go | 380 | SSH utilities |
| Makefile | 150 | Build automation |
| .goreleaser.yaml | 200 | Release automation |

**Total**: ~1,958 lines of Go code + build configuration

## License

MIT License - See LICENSE file for details

## Author

Jed Arden <jed@ardenone.com>

## Repository

https://github.com/jedarden/tunnel
