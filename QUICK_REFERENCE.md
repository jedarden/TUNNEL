# Quick Reference: Metrics & CLI Enhancements

## New Features

### 1. Real Latency Measurement
- **Location**: `internal/core/metrics.go`
- **What Changed**: Replaced 10ms simulated delay with actual TCP connection tests
- **How It Works**: Measures connection time to provider-specific targets
- **Benefits**: Real-time latency data, historical averaging, provider-aware targets

### 2. Interactive Configuration
- **Command**: `tunnel configure <provider>`
- **Supported Providers**: tailscale, wireguard, cloudflare, ngrok, zerotier, bore
- **Storage**: Encrypted credentials in `~/.config/tunnel/credentials/`

### 3. Secure API Key Setting
- **Command**: `tunnel auth set-key <provider>`
- **Features**: Input validation, encrypted storage, helpful feedback
- **Validation**: Provider-specific rules (e.g., ngrok: 20+ chars, tailscale: starts with "tskey-")

### 4. Enhanced Connection Restart
- **Command**: `tunnel restart <provider>`
- **Features**: State preservation, graceful shutdown, detailed status reporting

## Usage Examples

### Configure a Provider
```bash
tunnel configure ngrok
# Prompts for: Auth Token, Region
```

### Set API Key
```bash
tunnel auth set-key cloudflare
# Prompts for: Tunnel Token
```

### Check Latency
```bash
tunnel start tailscale
tunnel status  # Shows real latency measurements
```

### Restart Connection
```bash
tunnel restart ngrok
# Gracefully stops and restarts with state preservation
```

## Provider-Specific Latency Targets

| Provider   | Target                           |
|------------|----------------------------------|
| Cloudflare | 1.1.1.1:443                     |
| Tailscale  | controlplane.tailscale.com:443  |
| ngrok      | tunnel.us.ngrok.com:443         |
| WireGuard  | 127.0.0.1:51820                 |
| ZeroTier   | my.zerotier.com:443             |
| bore       | 127.0.0.1:2200                  |
| Default    | 8.8.8.8:443                     |

## Latency Metrics

- **History Size**: 10 samples
- **Calculation**: Average of valid samples
- **Update Interval**: Configured in metrics collector start
- **Display**: Via `tunnel status` command

## Configuration Storage

### File Locations
- **Credentials**: `~/.config/tunnel/credentials/<provider>.cred`
- **Encryption**: AES-GCM with PBKDF2 (100,000 iterations)
- **Permissions**: 0600 (owner read/write only)

### Stored Data by Provider

**Tailscale**:
- auth_key
- hostname

**WireGuard**:
- interface
- config_path

**Cloudflare**:
- tunnel_token

**ngrok**:
- auth_token
- region

**ZeroTier**:
- network_id

**bore**:
- server
- port

## API Key Validation Rules

| Provider   | Rule                                      |
|------------|-------------------------------------------|
| ngrok      | Min 20 chars, alphanumeric               |
| Cloudflare | Min 32 chars, base64-encoded             |
| Tailscale  | Starts with "tskey-", min 10 chars       |
| ZeroTier   | 16-char hex string for network IDs       |
| Generic    | No spaces, min 8 chars, non-empty        |

## Code Changes Summary

### Files Modified
1. `/workspaces/ardenone-cluster/tunnel/internal/core/metrics.go` (+110 lines)
2. `/workspaces/ardenone-cluster/tunnel/cmd/tunnel/cli.go` (+430 lines)

### New Functions
1. `measureLatency()` - TCP connection test
2. `getLatencyTarget()` - Provider-specific targets
3. `calculateAverageLatency()` - Historical averaging
4. `validateAPIKey()` - Input validation
5. `NewCredentialStore()` - Credential store helper

### Enhanced Functions
1. `Collect()` - Real latency measurement
2. `configureMethod()` - Interactive prompts
3. `setAPIKey()` - Secure storage
4. `restartConnection()` - State preservation

## Troubleshooting

### Latency Measurement Not Working
- Check network connectivity to target hosts
- Verify firewall rules allow outbound connections
- Use `tunnel status -v` for detailed output

### Configuration Not Saving
- Check directory permissions: `~/.config/tunnel/credentials/`
- Ensure disk space available
- Verify write permissions

### API Key Validation Failing
- Check key format matches provider requirements
- Use `--force` flag to bypass validation (if available)
- Verify key is not truncated or modified

### Restart Hanging
- Kill stuck processes: `pkill -f <provider>`
- Check logs: `tunnel logs <provider>`
- Try manual stop then start: `tunnel stop <provider> && tunnel start <provider>`

## Security Notes

⚠️ **Production Deployment**:
- Change default passphrase from "tunnel-credentials"
- Use environment variable for passphrase
- Enable keyring storage on supported platforms
- Rotate credentials regularly

🔒 **Best Practices**:
- Never commit credential files to git
- Use `.gitignore` for `~/.config/tunnel/credentials/`
- Restrict file permissions to 0600
- Use unique API keys per environment

## Next Steps

After implementation:
1. Test latency measurement: `tunnel status`
2. Configure providers: `tunnel configure <provider>`
3. Set API keys: `tunnel auth set-key <provider>`
4. Monitor connections: `watch -n 5 'tunnel status'`
5. Check logs for errors: `tunnel logs`

## Documentation

- Full implementation details: `METRICS_CLI_ENHANCEMENTS.md`
- Provider information: `docs/PROVIDERS.md`
- Usage examples: `docs/EXAMPLES.md`
