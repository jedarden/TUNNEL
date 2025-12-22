# TUNNEL Marathon Coding Mission - Phase 2

## Mission Brief
Complete the TUNNEL architecture implementation by addressing gaps identified in the architecture review. Each phase must be validated against the architecture documentation in `/workspaces/ardenone-cluster/research/remote-control/`.

## GitHub Issue
Track progress at: https://github.com/jedarden/tunnel/issues/1

**IMPORTANT**: Update the GitHub issue with progress after completing each phase:
```bash
gh issue comment 1 --body "Phase X completed: [summary]"
```

## Architecture Reference Documents
Before implementing each phase, review the relevant architecture docs:
- `research/remote-control/tui-architecture.md` - TUI views, components, CLI commands
- `research/remote-control/tui-key-management.md` - Key management, user access, security
- `research/remote-control/tunnel-relay-solutions.md` - Tunnel/relay providers
- `research/remote-control/vpn-mesh-solutions.md` - VPN/mesh providers
- `research/remote-control/direct-traditional-solutions.md` - SSH, bastion, autossh

---

## Phase 1: User Management TUI

### Architecture Reference
Read: `research/remote-control/tui-key-management.md` sections 2.1-2.5

### Tasks
1. Create `internal/tui/users.go` - User list view
   - Display all users with key counts
   - Search and filter functionality
   - Status indicators (active, pending, revoked)
   - Actions: Edit, Revoke, View Details

2. Create `internal/tui/keys.go` - Key management view
   - Per-user key list
   - Add/Remove/Rotate key actions
   - Fingerprint display
   - Expiration status

3. Create `internal/tui/approval.go` - Approval workflow
   - Pending requests list
   - Approve/Deny/Modify actions
   - Modification notes

4. Wire views to KeyManager and integrate with main TUI app

### Validation
Compare implementation against wireframes in tui-key-management.md sections 2.1-2.4

---

## Phase 2: Key Management Enhancements

### Architecture Reference
Read: `research/remote-control/tui-key-management.md` sections 1.1-1.5, 5.1-5.5

### Tasks
1. Add CLI commands in `cmd/tunnel/cli.go`:
   - `tunnel keys list [user]` - list keys
   - `tunnel keys add <user>` - add key interactively
   - `tunnel keys rotate <user> [key-id]` - rotate key
   - `tunnel keys revoke <user> <key-id>` - revoke key
   - `tunnel keys import-github <github-user>` - import from GitHub
   - `tunnel keys import-gitlab <gitlab-user>` - import from GitLab

2. Enhance `internal/core/keymanager.go`:
   - Add RotateKey() method
   - Add key expiration checking
   - Add bulk operations (BulkRevoke, BulkRotate)
   - Implement GitLab import (gitlab.com/<user>.keys)

3. Add key validation enhancements:
   - Weak key detection (RSA < 2048 bits)
   - Key age warnings
   - Duplicate fingerprint prevention

### Validation
Compare against security requirements in tui-key-management.md section 5

---

## Phase 3: Additional Providers

### Architecture Reference
Read: `research/remote-control/direct-traditional-solutions.md`

### Tasks
1. Create `internal/providers/autossh/autossh.go`:
   - Reverse SSH tunnel via autossh
   - Support for -M monitoring port
   - Systemd service generation

2. Create `internal/providers/bastion/bastion.go`:
   - SSH ProxyJump configuration
   - Multi-hop support
   - SSH agent forwarding

3. Create `internal/providers/frp/frp.go`:
   - frp client configuration
   - TCP/UDP tunnel support
   - Dashboard integration

4. Add provider health status to TUI dashboard

### Validation
Compare provider capabilities against direct-traditional-solutions.md

---

## Phase 4: Client Config Export

### Architecture Reference
Read: `research/remote-control/tui-key-management.md` section 3

### Tasks
1. Create `internal/export/export.go`:
   - Setup script generation (bash, powershell)
   - SSH config generation
   - Connection string generation

2. Add CLI commands:
   - `tunnel export <method> [--format=script|config|url]`
   - `tunnel export --qr <method>` (optional)

3. Add TUI export dialog in browser view

### Validation
Compare against config generation examples in tui-key-management.md section 3

---

## Phase 5: Security Enhancements

### Architecture Reference
Read: `research/remote-control/tui-key-management.md` sections 5.3-5.5

### Tasks
1. Emergency revocation:
   - Add `tunnel emergency-revoke <user> --reason <reason>`
   - Kill active sessions
   - Audit log entry
   - Notification support

2. Security monitoring:
   - Weak key detection on import
   - Connection anomaly detection
   - Failed auth tracking

3. Access control:
   - Per-method role restrictions
   - Time-based access windows (optional)
   - IP whitelist support (optional)

### Validation
Compare against security best practices in tui-key-management.md section 5

---

## Final Validation

Before marking complete:

1. **Architecture Compliance Check**
   ```bash
   # Read each architecture doc and verify implementation
   cat research/remote-control/tui-architecture.md | head -200
   cat research/remote-control/tui-key-management.md | head -200
   ```

2. **Feature Checklist**
   - [ ] User management TUI views working
   - [ ] Key rotation command working
   - [ ] At least 2 new providers implemented
   - [ ] Export command generates valid scripts
   - [ ] Emergency revoke command working
   - [ ] All tests passing

3. **Build & Test**
   ```bash
   make build
   make test
   ./bin/tunnel --help
   ./bin/tunnel keys --help
   ./bin/tunnel export --help
   ```

4. **Update GitHub Issue**
   ```bash
   gh issue comment 1 --body "Phase 2 marathon complete. All architecture gaps addressed."
   ```

---

## Completion

When all phases complete:
```bash
gh issue close 1 --comment "Mission complete! Architecture fully implemented."
git add -A
git commit -m "Complete architecture implementation - Phase 2 marathon"
git push
```
