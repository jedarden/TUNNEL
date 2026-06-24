# TUNNEL

**T**erminal **U**nified **N**etwork **N**ode **E**ncrypted **L**ink

TUNNEL is a Go application that unifies multiple SSH tunnel and VPN providers behind a single tool. Run `tunnel` to start a local web UI and a small terminal status display; from there you can configure providers, start connections, monitor health, and set up automatic failover — all without touching config files by hand.

## What it does

- **Single entry point** for Tailscale, WireGuard, Cloudflare Tunnel, ngrok, ZeroTier, bore, reverse SSH, VS Code Tunnels, and bastion/jump hosts
- **Web UI** (React, embedded in the binary) served on `localhost:8080` — configure providers, view connection state, manage credentials
- **Automatic failover** — run multiple providers at the same priority and TUNNEL switches to the next healthy one when the primary drops
- **Encrypted credential storage** — system keyring by default, or AES-GCM encrypted files
- **SSH key management** — import from GitHub, add manually, revoke
- **Hot-swap binary** — file watcher detects a new binary and performs a graceful restart with no dropped connections
- **Prometheus metrics** endpoint (optional, configurable port)

## Supported providers

| Category | Provider | Notes |
|----------|----------|-------|
| VPN/Mesh | Tailscale | `tailscale` binary required |
| VPN/Mesh | WireGuard | `wg-quick` required |
| VPN/Mesh | ZeroTier | `zerotier-cli` required |
| Tunnel | Cloudflare Tunnel | `cloudflared` required |
| Tunnel | ngrok | `ngrok` binary required; local API on port 4040 |
| Tunnel | bore | installed via `cargo install bore-cli` |
| SSH | VS Code Tunnels | `code` CLI required |
| SSH | SSH Port Forward | uses system `ssh` |
| SSH | Reverse SSH | uses system `ssh` |
| SSH | Bastion/Jump Host | uses system `ssh` |

TUNNEL detects which provider binaries are already installed and marks the others as unavailable until you install them.

## Quick start

```bash
git clone https://github.com/jedarden/tunnel.git
cd tunnel
make build          # builds frontend (requires Node) then compiles binary
make install        # installs to ~/.local/bin/tunnel
tunnel              # launch
```

The terminal shows a status pane. Press `o` to open the web UI in your browser, or navigate to `http://localhost:8080` directly. Press `q` or `Ctrl+C` to quit.

## Installation

### Prerequisites

- Go 1.24+
- Node.js 18+ and npm (for the embedded React frontend)
- Make

### Build options

```bash
make build          # full build: React frontend + Go binary → bin/tunnel
make build-dev      # Go binary only (no frontend, faster; shows placeholder page)
make install        # build + copy to ~/.local/bin/tunnel
make test           # run Go tests
make lint           # run golangci-lint (if installed)
make release        # cross-compile for linux/darwin/windows (amd64 + arm64)
```

### Development mode

```bash
# Terminal 1: run Vite dev server with hot reload
make frontend-dev

# Terminal 2: run Go binary pointing at the dev server
make dev
```

## Configuration

Config file: `~/.config/tunnel/config.yaml` (created automatically on first run).

```yaml
version: "1.0.0"

settings:
  default_method: ssh-key   # method used by `tunnel start` with no arguments
  auto_reconnect: true
  log_level: info           # debug | info | warn | error
  theme: default            # default | dark | light | nord | dracula

credentials:
  store: keyring            # keyring | file | env
  # For file store only:
  base_dir: ~/.config/tunnel/credentials
  passphrase: ""            # leave empty to be prompted

methods:
  tailscale:
    enabled: true
    priority: 1             # lower number = higher priority; used for failover ordering
    auth_key_ref: "keyring:tunnel/tailscale/auth_key"
    settings:
      control_url: https://controlplane.tailscale.com

  wireguard:
    enabled: false
    priority: 2
    settings:
      interface: wg0
      config_path: /etc/wireguard/wg0.conf

  cloudflare:
    enabled: false
    priority: 3
    auth_key_ref: "keyring:tunnel/cloudflare/token"
    settings:
      tunnel_name: my-tunnel

  ngrok:
    enabled: false
    priority: 4
    auth_key_ref: "keyring:tunnel/ngrok/token"
    settings:
      region: us

ssh:
  port: 2222
  host_key_path: ~/.config/tunnel/ssh_host_key
  authorized_keys: ~/.ssh/authorized_keys
  max_sessions: 10
  idle_timeout: 300         # seconds; 0 = no timeout
  allow_tcp_forwarding: true
  allow_agent_forwarding: true

monitoring:
  enabled: true
  audit_log: ~/.config/tunnel/audit.log
  metrics_enabled: false
  metrics_port: 9090
```

## CLI reference

Running `tunnel` with no arguments launches the TUI + web server. All subcommands can also be run non-interactively.

```bash
tunnel [--port 8080]       # launch (web UI on given port, default 8080)

tunnel start               # connect using default_method
tunnel start tailscale     # connect a specific provider
tunnel start tailscale,wireguard,ngrok   # connect multiple (redundancy mode)
tunnel start --auto-failover             # enable automatic failover

tunnel stop <provider>     # disconnect a provider
tunnel stop all            # disconnect everything

tunnel status              # show connection state + latency for all providers
tunnel status -v           # verbose (includes metrics)

tunnel restart <provider>  # graceful stop + reconnect with state preservation

tunnel configure <provider>  # interactive prompt for provider credentials
tunnel auth set-key <provider>  # store an API key/token securely

tunnel list                # list all providers and install status

tunnel keys import --github <username>   # import SSH public keys from GitHub
tunnel keys add --user <username>        # add a key manually
tunnel keys list                         # show authorised keys
tunnel keys revoke <key-id>              # remove a key

tunnel doctor              # run connectivity and environment diagnostics

tunnel version             # show build version, commit, Go version
tunnel completions bash    # generate shell completions (bash | zsh | fish)
```

## Automatic failover

TUNNEL runs multiple providers simultaneously. Each has a `priority` (lower = preferred). A background health checker (default interval: 10 s) measures latency and marks providers healthy or not. When the current primary fails `FailureThreshold` consecutive checks, TUNNEL promotes the next healthy provider. When the original comes back and passes `RecoveryThreshold` checks, it hands control back automatically if `auto_recover` is set.

```bash
# Start three providers; failover is automatic
tunnel start tailscale,wireguard,ngrok --auto-failover

# Inspect current state
tunnel status
```

Failover events appear in the audit log and are emitted as Prometheus metrics when `metrics_enabled: true`.

## Credential storage

Credentials are **never stored in `config.yaml`** in plaintext. The `auth_key_ref` field holds a reference to the credential store, not the value itself.

- **`keyring`** (default): delegates to the OS keyring (libsecret on Linux, Keychain on macOS, Credential Manager on Windows)
- **`file`**: AES-GCM encryption with PBKDF2 key derivation (100,000 iterations); files stored at `~/.config/tunnel/credentials/<provider>.cred` with mode 0600
- **`env`**: reads from environment variables (useful in CI)

```bash
# Store a credential interactively
tunnel auth set-key cloudflare
# or
tunnel configure tailscale
```

## Architecture

```
tunnel (binary)
├── Web server (Fiber)           ← serves embedded React UI + REST/WebSocket API
│   ├── /api/providers           ← list, connect, disconnect, health
│   ├── /api/connections         ← active connections + metrics
│   └── /api/keys                ← SSH key management
├── Terminal UI (Bubbletea)      ← minimal status pane: server URL + controls
├── Connection Manager           ← tracks state for all active connections
├── Failover Manager             ← health polling + automatic promotion
├── Metrics Collector            ← latency measurement (real TCP probe), byte counters
├── Credential Store             ← keyring / encrypted file / env
├── Key Manager                  ← authorized_keys CRUD
├── Upgrade Watcher              ← hot-swap: detects new binary, graceful restart
└── Provider Adapters
    ├── tailscale   (tailscale up/status/down)
    ├── wireguard   (wg-quick up/down, wg show)
    ├── zerotier    (zerotier-cli join/leave/listnetworks)
    ├── cloudflare  (cloudflared tunnel run)
    ├── ngrok       (ngrok tcp + local API on :4040)
    ├── bore        (bore local)
    ├── sshforward  (ssh -L / -R)
    ├── reversessh  (ssh -R)
    ├── vscodetunnel (code tunnel)
    └── bastion     (ssh -J)
```

## Project structure

```
tunnel/
├── cmd/
│   ├── tunnel/              # main entry point, CLI (Cobra), web server (Fiber)
│   ├── provider-demo/       # standalone provider status demo
│   └── tui-test/            # TUI development harness
├── internal/
│   ├── core/                # connection state, failover, metrics, events, audit
│   ├── providers/           # provider interface + per-provider implementations
│   ├── registry/            # thread-safe provider registry
│   ├── tui/                 # Bubbletea application (status pane)
│   ├── upgrade/             # binary hot-swap watcher
│   └── web/                 # Fiber API handlers + embed shim for React dist
├── pkg/
│   ├── config/              # YAML config loader with fsnotify live reload
│   ├── tunnel/              # higher-level tunnel manager / registry
│   └── version/             # version string injection
├── web/                     # React + Vite + Tailwind frontend
│   └── src/
│       ├── pages/           # Dashboard, Providers, Connections, Settings
│       └── components/      # layout, modals, status indicators
├── configs/
│   └── default.yaml         # template config (copied on first run)
├── docs/                    # provider details, examples, connection manager
└── examples/                # connection manager demo
```

## Diagnostics

```bash
tunnel doctor
```

Checks that required binaries are present, that the SSH server config is valid, that credential files have correct permissions, and that monitored providers can reach their upstream endpoints.

## License

MIT
