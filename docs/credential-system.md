# Credential Store and Configuration System

This document describes the Credential Store and Configuration System implemented for the TUNNEL TUI application.

## Overview

The TUNNEL application provides a flexible credential management system with multiple storage backends and comprehensive configuration management with hot-reloading support.

## Components

### 1. Credential Store (`internal/core/credentials.go`)

The credential store provides secure storage for sensitive data with three backend implementations:

#### Backends

1. **Keyring Store** - Uses system keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
2. **File Store** - AES-256-GCM encrypted file storage with PBKDF2 key derivation
3. **Environment Store** - Environment variable-based storage (fallback/testing)

#### Usage Example

```go
import "github.com/jedarden/tunnel/internal/core"

// Create a file-based credential store
store, err := core.NewFileStore("~/.config/tunnel/credentials", "my-passphrase")
if err != nil {
    log.Fatal(err)
}

// Store a credential
err = store.Set("ssh", "password", []byte("my-secret-password"))

// Retrieve a credential
password, err := store.Get("ssh", "password")

// List all keys for a service
keys, err := store.List("ssh")

// Delete a credential
err = store.Delete("ssh", "password")
```

#### Factory Function

```go
// Automatically choose the best available store
store, err := core.NewCredentialStore(
    "keyring",              // preferred type
    "tunnel",               // service name
    "~/.config/tunnel/creds", // base dir for file store
    "passphrase",           // passphrase for file store
)
```

### 2. Configuration Management (`pkg/config/`)

The configuration system provides YAML-based configuration with validation and hot-reloading.

#### Configuration Structure

```yaml
version: "1.0.0"

settings:
  default_method: ssh-key
  auto_reconnect: true
  log_level: info
  theme: default

credentials:
  store: keyring
  base_dir: ~/.config/tunnel/credentials
  passphrase: ""

methods:
  ssh-key:
    enabled: true
    priority: 100
    auth_key_ref: ""
    extra_args: []
    settings: {}

ssh:
  port: 2222
  host_key_path: ~/.config/tunnel/ssh_host_key
  authorized_keys: ~/.ssh/authorized_keys
  max_sessions: 10
  idle_timeout: 300
  keep_alive: 60

monitoring:
  enabled: true
  audit_log: ~/.config/tunnel/audit.log
  syslog: false
  metrics_enabled: false
  metrics_port: 9090
```

#### Usage Example

```go
import "github.com/jedarden/tunnel/pkg/config"

// Load configuration
cfg, err := config.Load("~/.config/tunnel/config.yaml")
if err != nil {
    log.Fatal(err)
}

// Access configuration
port := cfg.SSH.Port
logLevel := cfg.Settings.LogLevel

// Get enabled authentication methods (sorted by priority)
methods := cfg.GetEnabledMethods()

// Watch for configuration changes
err = cfg.Watch()
cfg.OnChange(func(c *config.Config) {
    log.Println("Configuration reloaded!")
})

// Save configuration
cfg.Settings.LogLevel = "debug"
err = cfg.Save()
```

### 3. SSH Key Management (`internal/core/keymanager.go`)

Manages SSH public keys with support for importing from GitHub and other sources.

#### Features

- Parse and validate SSH public keys
- Generate SHA256 fingerprints
- Manage authorized_keys file
- Import keys from GitHub
- Import keys from URLs
- Audit logging for all operations

#### Usage Example

```go
import "github.com/jedarden/tunnel/internal/core"

// Create key manager
km, err := core.NewFileKeyManager(
    "~/.ssh/authorized_keys",
    auditLogger,
)

// Import keys from GitHub
keys, err := km.ImportFromGitHub("username")
fmt.Printf("Imported %d keys from GitHub\n", len(keys))

// Validate and add a key manually
keyStr := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... user@host"
key, err := km.ValidateKey(keyStr)
err = km.AddKey("username", *key)

// List all keys
keys, err := km.ListKeys("username")
for _, key := range keys {
    fmt.Printf("Key: %s (Type: %s, Fingerprint: %s)\n",
        key.Comment, key.Type, key.Fingerprint)
}

// Remove a key
err = km.RemoveKey("username", key.ID)
```

### 4. Audit Logging (`internal/core/audit.go`)

Comprehensive audit logging for security events.

#### Features

- JSON-formatted audit logs
- Syslog support (local and remote)
- Connection tracking
- Key operation logging
- Configuration change logging
- Log rotation

#### Usage Example

```go
import "github.com/jedarden/tunnel/internal/core"

// Create audit logger
logger, err := core.NewAuditLogger(
    "~/.config/tunnel/audit.log",
    true,  // use syslog
    "",    // local syslog (or "host:port" for remote)
)

// Log a connection attempt
err = logger.LogConnectionAttempt(
    "ssh-key",
    "username",
    "192.168.1.100",
    true,
    map[string]interface{}{
        "fingerprint": "SHA256:...",
    },
)

// Log a connection
err = logger.LogConnectionEstablished("ssh-key", "user", "192.168.1.100", nil)

// Log connection closed
err = logger.LogConnectionClosed("ssh-key", "user", "192.168.1.100",
    5*time.Minute, nil)

// Log key operations
err = logger.LogKeyOperation("key_added", "user", true, map[string]interface{}{
    "fingerprint": "SHA256:...",
    "type": "ssh-ed25519",
})

// Log configuration changes
err = logger.LogConfigChange("admin", map[string]interface{}{
    "field": "ssh.port",
    "old": 2222,
    "new": 2223,
})

// Rotate logs
err = logger.Rotate()

// Close logger
err = logger.Close()
```

## Security Considerations

### File Store Encryption

The file store uses industry-standard encryption:

- **Algorithm**: AES-256-GCM
- **Key Derivation**: PBKDF2 with SHA-256, 100,000 iterations
- **Salt**: 32-byte random salt per file
- **Nonce**: Random nonce per encryption operation

### File Permissions

All sensitive files are created with restrictive permissions:

- Credential files: `0600` (owner read/write only)
- SSH keys: `0600`
- Authorized keys: `0600`
- Config directory: `0700`

### Audit Trail

All security-relevant operations are logged:

- Authentication attempts (success and failure)
- Connection establishment and closure
- Key additions and removals
- Configuration changes
- Errors and anomalies

## Configuration Priority

Authentication methods are executed based on priority (highest first):

1. **ssh-key** (100) - Public key authentication
2. **fido2** (95) - Hardware security keys
3. **password** (90) - Password authentication
4. **totp** (80) - Time-based OTP
5. **oauth** (70) - OAuth 2.0 providers
6. **wireguard** (60) - WireGuard VPN
7. **tailscale** (50) - Tailscale mesh network

## Default Configuration

A default configuration is automatically created on first run at:
- Linux/macOS: `~/.config/tunnel/config.yaml`
- Windows: `%APPDATA%\tunnel\config.yaml`

## Migration Support

The configuration system includes migration support for upgrading between versions:

```go
// Validate and migrate if necessary
err := config.ValidateAndMigrate(cfg)
```

## Testing

Comprehensive test suites are included:

```bash
# Test credential store
go test ./internal/core/credentials_test.go

# Test configuration
go test ./pkg/config/config_test.go

# Run all tests
go test ./...
```

## Integration Example

Complete integration example:

```go
package main

import (
    "log"
    "github.com/jedarden/tunnel/pkg/config"
    "github.com/jedarden/tunnel/internal/core"
)

func main() {
    // Load configuration
    cfg, err := config.Load("")
    if err != nil {
        log.Fatal(err)
    }

    // Create audit logger
    auditLogger, err := core.NewAuditLogger(
        cfg.Monitoring.AuditLog,
        cfg.Monitoring.Syslog,
        cfg.Monitoring.SyslogServer,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer auditLogger.Close()

    // Create credential store
    credStore, err := core.NewCredentialStore(
        cfg.Credentials.Store,
        "tunnel",
        cfg.Credentials.BaseDir,
        cfg.Credentials.Passphrase,
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create key manager
    keyManager, err := core.NewFileKeyManager(
        cfg.SSH.AuthorizedKeys,
        auditLogger,
    )
    if err != nil {
        log.Fatal(err)
    }

    // Watch for config changes
    cfg.Watch()
    cfg.OnChange(func(c *config.Config) {
        log.Println("Configuration reloaded")
        // Update components as needed
    })

    // Application logic here...
}
```

## File Structure

```
/workspaces/ardenone-cluster/tunnel/
├── configs/
│   └── default.yaml              # Default configuration template
├── internal/core/
│   ├── credentials.go            # Credential store implementations
│   ├── credentials_test.go       # Credential store tests
│   ├── keymanager.go             # SSH key management
│   └── audit.go                  # Audit logging
└── pkg/config/
    ├── config.go                 # Configuration management
    ├── config_test.go            # Configuration tests
    └── defaults.go               # Default configuration
```

## Dependencies

- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/zalando/go-keyring` - System keyring access
- `github.com/fsnotify/fsnotify` - File system notifications
- `golang.org/x/crypto/ssh` - SSH key parsing
- `golang.org/x/crypto/pbkdf2` - Key derivation

## Next Steps

1. Integrate with authentication providers
2. Implement the TUI for credential management
3. Add support for additional authentication methods
4. Implement metrics collection
5. Add support for credential rotation
