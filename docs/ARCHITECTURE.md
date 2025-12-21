# TUNNEL Provider System Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      TUNNEL TUI Application                     │
│                   (Terminal User Interface)                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Provider Registry                           │
│  • Provider Management                                          │
│  • Category Filtering                                           │
│  • Status Tracking                                              │
└────────────────┬───┬───┬────────────┬────────────┬──────────────┘
                 │   │   │            │            │
        ┌────────┘   │   │            │            └────────┐
        │            │   │            │                     │
        ▼            ▼   ▼            ▼                     ▼
    ┌──────┐    ┌──────────────────────────┐         ┌──────────┐
    │ VPN  │    │  Tunnel Providers        │         │  Direct  │
    └──────┘    └──────────────────────────┘         └──────────┘
        │                    │                             │
   ┌────┼────┐          ┌────┼────┐                       │
   │    │    │          │    │    │                   (Future)
   ▼    ▼    ▼          ▼    ▼    ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tailscale  │  WireGuard  │  ZeroTier                          │
│             │             │                                     │
│  tailscale  │  wg/wg-quick│  zerotier-cli                      │
└─────────────┴─────────────┴─────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│  Cloudflare │  ngrok      │  bore                              │
│  Tunnel     │             │                                     │
│  cloudflared│  ngrok      │  bore                              │
└─────────────┴─────────────┴─────────────────────────────────────┘
```

## Component Architecture

### 1. Provider Interface Layer

```go
┌─────────────────────────────────────────────────────┐
│              Provider Interface                     │
├─────────────────────────────────────────────────────┤
│  Identity:                                          │
│    • Name() string                                  │
│    • Category() Category                            │
├─────────────────────────────────────────────────────┤
│  Lifecycle:                                         │
│    • Install() error                                │
│    • Uninstall() error                              │
│    • IsInstalled() bool                             │
├─────────────────────────────────────────────────────┤
│  Configuration:                                     │
│    • Configure(config) error                        │
│    • GetConfig() (*ProviderConfig, error)           │
│    • ValidateConfig(config) error                   │
├─────────────────────────────────────────────────────┤
│  Connection:                                        │
│    • Connect() error                                │
│    • Disconnect() error                             │
│    • IsConnected() bool                             │
│    • GetConnectionInfo() (*ConnectionInfo, error)   │
├─────────────────────────────────────────────────────┤
│  Health:                                            │
│    • HealthCheck() (*HealthStatus, error)           │
│    • GetLogs(since) ([]LogEntry, error)             │
└─────────────────────────────────────────────────────┘
```

### 2. Data Flow

```
User Request
     │
     ▼
┌─────────────────┐
│  TUI Layer      │ ──── User Input
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Registry       │ ──── Provider Selection
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Provider       │ ──── Configuration
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Validation     │ ──── Config Check
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Command Exec   │ ──── Binary Execution
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Status Parse   │ ──── JSON/Text Parsing
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Response       │ ──── Result to User
└─────────────────┘
```

### 3. Provider State Machine

```
                    ┌──────────────┐
                    │ Not Installed│
                    └──────┬───────┘
                           │ Install()
                           ▼
                    ┌──────────────┐
              ┌─────│  Installed   │◄────┐
              │     └──────┬───────┘     │
  Uninstall() │            │ Configure() │
              │            ▼             │
              │     ┌──────────────┐    │
              │     │  Configured  │    │
              │     └──────┬───────┘    │
              │            │ Connect()  │
              │            ▼            │
              │     ┌──────────────┐   │
              │     │  Connecting  │   │ Disconnect()
              │     └──────┬───────┘   │
              │            │            │
              │            ▼            │
              │     ┌──────────────┐   │
              └────►│  Connected   │───┘
                    └──────┬───────┘
                           │ HealthCheck()
                           ▼
                    ┌──────────────┐
                    │   Healthy?   │
                    └──────────────┘
```

### 4. Registry Architecture

```
┌──────────────────────────────────────────────────────┐
│             Provider Registry                        │
├──────────────────────────────────────────────────────┤
│  Thread-Safe Operations (Mutex Protected)           │
│                                                      │
│  ┌────────────────────────────────────────────┐    │
│  │  Provider Map                              │    │
│  │  map[string]Provider                       │    │
│  │  {                                         │    │
│  │    "tailscale":  TailscaleProvider        │    │
│  │    "wireguard":  WireGuardProvider        │    │
│  │    "zerotier":   ZeroTierProvider         │    │
│  │    "cloudflare": CloudflareProvider       │    │
│  │    "ngrok":      NgrokProvider            │    │
│  │    "bore":       BoreProvider             │    │
│  │  }                                         │    │
│  └────────────────────────────────────────────┘    │
│                                                      │
│  Operations:                                         │
│  • Register(provider)                                │
│  • Unregister(name)                                  │
│  • GetProvider(name)                                 │
│  • ListProviders()                                   │
│  • ListByCategory(category)                          │
│  • GetInstalledProviders()                           │
│  • GetConnectedProviders()                           │
│  • GetProviderInfo()                                 │
└──────────────────────────────────────────────────────┘
```

### 5. Provider Categories

```
┌─────────────────────────────────────────────────────┐
│                     Categories                      │
├─────────────────────────────────────────────────────┤
│                                                     │
│  VPN (Mesh/P2P Networks)                           │
│  ├── Tailscale    - WireGuard-based mesh VPN      │
│  ├── WireGuard    - Kernel-level VPN              │
│  └── ZeroTier     - Software-defined networking    │
│                                                     │
│  Tunnel (Port Forwarding)                          │
│  ├── Cloudflare   - Zero Trust tunnels            │
│  ├── ngrok        - TCP/HTTP tunnels              │
│  └── bore         - Simple TCP tunnels            │
│                                                     │
│  Direct (Future)                                    │
│  └── SSH          - Direct SSH connections         │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## Provider Implementation Details

### VPN Providers

#### Tailscale
```
┌─────────────────────────────────────────┐
│         Tailscale Provider              │
├─────────────────────────────────────────┤
│  Binary: tailscale                      │
│  Protocol: WireGuard                    │
│  Features:                              │
│    • Mesh networking                    │
│    • Built-in SSH                       │
│    • MagicDNS                           │
│    • ACLs                               │
│                                         │
│  Operations:                            │
│    Install:  Check 'tailscale version' │
│    Connect:  'tailscale up --ssh'      │
│    Status:   'tailscale status --json' │
│    Disconnect: 'tailscale down'        │
│                                         │
│  Config:                                │
│    • AuthKey (optional)                 │
│    • Accept routes                      │
│    • SSH enabled                        │
└─────────────────────────────────────────┘
```

#### WireGuard
```
┌─────────────────────────────────────────┐
│         WireGuard Provider              │
├─────────────────────────────────────────┤
│  Binary: wg, wg-quick                   │
│  Protocol: WireGuard (kernel)           │
│  Features:                              │
│    • High performance                   │
│    • Minimal attack surface             │
│    • Strong encryption                  │
│    • Config file based                  │
│                                         │
│  Operations:                            │
│    Install:  Check 'wg version'        │
│    Connect:  'wg-quick up <iface>'     │
│    Status:   'wg show <iface>'         │
│    Disconnect: 'wg-quick down <iface>' │
│                                         │
│  Config:                                │
│    • ConfigFile (/etc/wireguard/*.conf)│
│    • Interface name                     │
└─────────────────────────────────────────┘
```

#### ZeroTier
```
┌─────────────────────────────────────────┐
│         ZeroTier Provider               │
├─────────────────────────────────────────┤
│  Binary: zerotier-cli                   │
│  Protocol: Custom SDN                   │
│  Features:                              │
│    • Network virtualization             │
│    • Cloud/self-hosted controller       │
│    • Layer 2 networking                 │
│    • Easy management                    │
│                                         │
│  Operations:                            │
│    Install:  Check 'zerotier-cli info' │
│    Connect:  'zerotier-cli join <id>'  │
│    Status:   'zerotier-cli listnetworks'│
│    Disconnect: 'zerotier-cli leave <id>'│
│                                         │
│  Config:                                │
│    • NetworkID (16 chars)               │
└─────────────────────────────────────────┘
```

### Tunnel Providers

#### Cloudflare
```
┌─────────────────────────────────────────┐
│      Cloudflare Tunnel Provider         │
├─────────────────────────────────────────┤
│  Binary: cloudflared                    │
│  Features:                              │
│    • No open ports                      │
│    • DDoS protection                    │
│    • Access control                     │
│    • Global edge network                │
│                                         │
│  Operations:                            │
│    Install:  Check 'cloudflared version'│
│    Connect:  'cloudflared tunnel run'   │
│    Status:   Process check              │
│    Disconnect: Kill process             │
│                                         │
│  Config:                                │
│    • TunnelName                         │
│    • AuthToken                          │
└─────────────────────────────────────────┘
```

#### ngrok
```
┌─────────────────────────────────────────┐
│           ngrok Provider                │
├─────────────────────────────────────────┤
│  Binary: ngrok                          │
│  Features:                              │
│    • Quick tunnels                      │
│    • Public URLs                        │
│    • HTTP API                           │
│    • Free tier                          │
│                                         │
│  Operations:                            │
│    Install:  Check 'ngrok version'     │
│    Connect:  'ngrok tcp <port>'        │
│    Status:   HTTP API (localhost:4040) │
│    Disconnect: Kill process             │
│                                         │
│  Config:                                │
│    • LocalPort (default: 22)            │
│    • AuthToken (optional)               │
└─────────────────────────────────────────┘
```

#### bore
```
┌─────────────────────────────────────────┐
│           bore Provider                 │
├─────────────────────────────────────────┤
│  Binary: bore                           │
│  Features:                              │
│    • Simple TCP proxy                   │
│    • Minimal config                     │
│    • Rust-based (fast)                  │
│    • No auth required                   │
│                                         │
│  Operations:                            │
│    Install:  'cargo install bore-cli'  │
│    Connect:  'bore local <port>'       │
│    Status:   Process check              │
│    Disconnect: Kill process             │
│                                         │
│  Config:                                │
│    • LocalPort (default: 22)            │
│    • RemoteHost (default: bore.pub)     │
└─────────────────────────────────────────┘
```

## Error Handling Architecture

```
┌────────────────────────────────────────────┐
│          Error Hierarchy                   │
├────────────────────────────────────────────┤
│                                            │
│  Configuration Errors                      │
│  ├── ErrInvalidConfig                      │
│  ├── ErrNoConfig                           │
│  ├── ErrMissingName                        │
│  ├── ErrMissingToken                       │
│  └── ErrMissingKey                         │
│                                            │
│  Installation Errors                       │
│  ├── ErrNotInstalled                       │
│  ├── ErrAlreadyInstalled                   │
│  └── ErrInstallFailed                      │
│                                            │
│  Connection Errors                         │
│  ├── ErrNotConnected                       │
│  ├── ErrAlreadyConnected                   │
│  └── ErrConnectionFailed                   │
│                                            │
│  Provider Errors                           │
│  ├── ErrProviderNotFound                   │
│  ├── ErrCommandFailed                      │
│  └── ErrInvalidResponse                    │
│                                            │
└────────────────────────────────────────────┘
```

## Thread Safety

```
┌────────────────────────────────────────────┐
│      Registry Thread Safety                │
├────────────────────────────────────────────┤
│                                            │
│  sync.RWMutex                              │
│  ├── Read Lock:  Multiple simultaneous    │
│  │   • GetProvider()                       │
│  │   • ListProviders()                     │
│  │   • ListByCategory()                    │
│  │   • GetInstalledProviders()             │
│  │   • GetConnectedProviders()             │
│  │   • GetProviderInfo()                   │
│  │                                         │
│  └── Write Lock: Exclusive access          │
│      • Register()                           │
│      • Unregister()                         │
│                                            │
└────────────────────────────────────────────┘
```

## Integration Points

```
┌─────────────────────────────────────────────────┐
│         External System Integration             │
├─────────────────────────────────────────────────┤
│                                                 │
│  TUI Layer                                      │
│  └─► Provider selection and status display     │
│                                                 │
│  Configuration System                           │
│  └─► Persistent provider preferences            │
│                                                 │
│  Connection Manager                             │
│  └─► Automatic provider selection/failover      │
│                                                 │
│  Monitoring System                              │
│  └─► Health check aggregation and alerts        │
│                                                 │
│  Logging System                                 │
│  └─► Centralized log collection and viewing     │
│                                                 │
│  Credential Store                               │
│  └─► Secure storage of auth tokens/keys         │
│                                                 │
└─────────────────────────────────────────────────┘
```

## Performance Characteristics

```
Provider      | Init Time | Connect Time | Latency  | Overhead
--------------|-----------|--------------|----------|----------
Tailscale     | Fast      | 2-5s        | Very Low | Minimal
WireGuard     | Instant   | <1s         | Lowest   | Minimal
ZeroTier      | Fast      | 5-10s       | Low      | Low
Cloudflare    | Medium    | 3-5s        | Varies   | Low
ngrok         | Fast      | 2-3s        | Medium   | Low
bore          | Fast      | 1-2s        | Low      | Minimal
```

## Security Architecture

```
┌─────────────────────────────────────────────────┐
│            Security Layers                      │
├─────────────────────────────────────────────────┤
│                                                 │
│  1. Input Validation                            │
│     • Configuration validation                  │
│     • Parameter sanitization                    │
│     • Path traversal prevention                 │
│                                                 │
│  2. Credential Management                       │
│     • No hardcoded secrets                      │
│     • Environment variable support              │
│     • Secure credential storage                 │
│                                                 │
│  3. Command Execution                           │
│     • exec.Command (prevents injection)         │
│     • Output sanitization                       │
│     • Error message sanitization                │
│                                                 │
│  4. File Permissions                            │
│     • Config file permission checks             │
│     • Private key protection                    │
│     • Temporary file cleanup                    │
│                                                 │
│  5. Network Security                            │
│     • Encrypted connections                     │
│     • Certificate validation                    │
│     • Secure defaults                           │
│                                                 │
└─────────────────────────────────────────────────┘
```

## Future Architecture

```
Planned Enhancements:
│
├── Provider Plugins
│   └── Dynamic provider loading at runtime
│
├── Event System
│   └── Provider state change notifications
│
├── Metrics Collection
│   └── Performance and usage tracking
│
├── Configuration Profiles
│   └── Named configuration sets
│
├── Provider Chains
│   └── Multi-hop connections
│
└── Health History
    └── Time-series health data
```
