# TUNNEL Marathon Coding Mission

## Mission Brief
Complete the TUNNEL project by implementing all provider integrations, wiring components together, adding comprehensive tests, and setting up CI/CD for release.

## GitHub Issue
Track progress at: https://github.com/jedarden/tunnel/issues/1

**IMPORTANT**: Update the GitHub issue with progress after completing each phase using:
```bash
gh issue comment 1 --body "Phase X completed: [summary of what was done]"
```

## Phase 1: Provider Implementations (Priority: HIGH)

### 1.1 Cloudflare Tunnel (`internal/providers/cloudflare/cloudflare.go`)
- Implement `Connect()`: Execute `cloudflared tunnel run` with proper arguments
- Implement `Disconnect()`: Kill cloudflared process gracefully
- Implement `Status()`: Check if tunnel is running and get URL
- Handle authentication via `cloudflared tunnel login`
- Parse tunnel URL from cloudflared output

### 1.2 ngrok (`internal/providers/ngrok/ngrok.go`)
- Implement `Connect()`: Execute `ngrok tcp 22` or configured port
- Implement `Disconnect()`: Kill ngrok process
- Implement `Status()`: Query ngrok API (localhost:4040/api/tunnels)
- Support authtoken configuration
- Parse public URL from API response

### 1.3 bore (`internal/providers/bore/bore.go`)
- Implement `Connect()`: Execute `bore local <port> --to bore.pub`
- Implement `Disconnect()`: Kill bore process
- Implement `Status()`: Check process and parse output for URL
- Support custom bore server configuration

### 1.4 Tailscale (`internal/providers/tailscale/tailscale.go`)
- Implement `Connect()`: Ensure Tailscale is up with `tailscale up`
- Implement `Disconnect()`: Run `tailscale down`
- Implement `Status()`: Query `tailscale status --json`
- Get Tailscale IP and hostname

## Phase 2: Component Integration (Priority: HIGH)

### 2.1 CLI to Connection Manager
File: `cmd/tunnel/cli.go`
- Import and initialize ConnectionManager in root command
- Wire `start` command to `manager.Connect(providerName)`
- Wire `stop` command to `manager.Disconnect(providerName)`
- Wire `status` command to `manager.GetStatus()`
- Wire `list` command to registry.GetProviders()

### 2.2 TUI to Connection Manager
File: `internal/tui/app.go`
- Subscribe to ConnectionManager events
- Update dashboard with real connection status
- Show live metrics (latency, uptime, bytes)
- Handle connect/disconnect from TUI

### 2.3 Credential Integration
- Wire `auth login` to actually call provider login
- Wire `auth set-key` to store in credential manager
- Load credentials on provider initialization

### 2.4 Config Persistence
- Save config changes to file
- Load config on startup
- Support config file locations: ~/.config/tunnel/config.yaml

## Phase 3: Testing (Priority: MEDIUM)

### 3.1 Unit Tests
Create/update test files:
- `internal/providers/cloudflare/cloudflare_test.go`
- `internal/providers/ngrok/ngrok_test.go`
- `internal/providers/bore/bore_test.go`
- `internal/core/manager_test.go` (expand existing)
- `internal/core/failover_test.go`

### 3.2 Integration Tests
- Test provider registration and discovery
- Test connection lifecycle (connect -> status -> disconnect)
- Test failover scenarios
- Test credential storage and retrieval

### 3.3 Run Tests
```bash
go test ./... -v -cover
```
Target: >70% coverage

## Phase 4: CI/CD & Release (Priority: MEDIUM)

### 4.1 GitHub Actions - Testing
Create `.github/workflows/test.yml`:
```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -v -cover
      - run: go vet ./...
```

### 4.2 GitHub Actions - Release
Create `.github/workflows/release.yml`:
```yaml
name: Release
on:
  push:
    tags:
      - 'v*'
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: goreleaser/goreleaser-action@v5
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 4.3 Create Release
```bash
git tag v0.1.0
git push origin v0.1.0
```

## Completion Checklist

Before marking complete, verify:
- [ ] `make build` succeeds
- [ ] `make test` passes with >70% coverage
- [ ] `./bin/tunnel --help` works
- [ ] `./bin/tunnel doctor` shows all checks
- [ ] At least one provider can actually connect (if binary available)
- [ ] GitHub Actions workflows are committed
- [ ] Issue #1 is updated with final status

## Final Update

When complete, close the issue:
```bash
gh issue close 1 --comment "Mission complete! All phases implemented. v0.1.0 ready for release."
```
