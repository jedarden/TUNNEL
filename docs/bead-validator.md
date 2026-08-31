# Bead State Validator

The TUNNEL bead state validator automatically detects and repairs bead state machine inconsistencies that can cause bead starvation (open beads that workers cannot claim).

## Overview

Bead starvation occurs when open beads are invisible to workers due to invalid state transitions or configuration issues. The validator detects these conditions and automatically repairs them, ensuring all open beads remain claimable.

## Problems Detected

### 1. Assigned-but-Open Beads
**Problem:** A bead has an assignee but its status is `open` instead of `in_progress`. Workers ignore assigned beads, so these effectively disappear from the ready frontier.

**Detection:** Finds beads where `assignee != null` AND `status == "open"`

**Auto-fix:** Clears the assignee (`bead update --clear-assignee`), making the bead immediately visible to workers

**Safety:** This is the exact fix applied by `bead reopen` (per CLAUDE.md), so it's safe and idempotent

### 2. Closed Beads with Assignees
**Problem:** A bead is `closed` but still has an assignee assigned. This wastes worker slots.

**Detection:** Finds beads where `status == "closed"` AND `assignee != null`

**Auto-fix:** Clears the assignee from closed beads

**Safety:** Closed beads are not active work, so clearing assignees has no operational impact

### 3. Circular Self-Dependencies
**Problem:** A bead lists itself in `blocks` or `blocked_by`, creating an unresolvable dependency cycle.

**Detection:** Finds beads where `bead.id in bead.blocks` OR `bead.id in bead.blocked_by`

**Auto-fix:** Clears all dependencies (`bead dep --clear`), breaking the cycle

**Safety:** Circular dependencies are always bugs—clearing them restores correct behavior

### 4. Database-Checkpoint Mismatch
**Problem:** The SQLite database has diverged from the git-tracked checkpoint, indicating corruption or an incomplete flush.

**Detection:** Compares bead counts between `bead list` and `.beads/checkpoint/current.json`

**Auto-fix:** Rebuilds database from forensic checkpoint (`bead sync import-only --restore-into-empty`)

**Safety:** Lossless as long as checkpoint was flushed recently (bead-rs R026 auto-flushes)

### 5. Workspace Backend Mismatch
**Problem:** `.needle.yaml` specifies `bead-rs` but the workspace still uses the old `bead-forge` backend (detected by missing `.beads/config.json`).

**Detection:** Checks for `bead_cli.backend: bead-rs` in `.needle.yaml` and presence of `.beads/config.json`

**Auto-fix:** Manual intervention required—see CLAUDE.md "Beads (bead-rs CLI)" for migration instructions

**Safety:** No auto-fix to prevent accidental corruption during backend transition

## Usage

### Manual Validation

```bash
# Check for issues without fixing (dry run)
tunnel bead-validator --dry-run

# Check and auto-fix all issues
tunnel bead-validator --fix

# Output results as JSON
tunnel bead-validator --json
```

### Scheduled Execution

The validator runs hourly via systemd user timer:

```bash
# Install the systemd timer
./scripts/install-bead-validator.sh

# Check timer status
systemctl --user status tunnel-bead-validator.timer

# View recent validation logs
journalctl --user -u tunnel-bead-validator@$(whoami) --since "1 hour ago"

# Manually trigger a scheduled run
systemctl --user start tunnel-bead-validator@$(whoami)
```

The scheduled mode automatically applies fixes and generates a summary bead if issues remain unresolved.

### Interactive Mode

```bash
# Run interactively with colored output
tunnel bead-validator

# Specify a custom workspace directory
tunnel bead-validator --workspace /path/to/project
```

## Output Format

### Console Output

```
=== Bead State Validation Results ===

✓ PASS: Verify workspace bead backend matches configuration
  Check: workspace_backend

✗ FAIL: Check for assigned beads with open status
  Check: assigned_but_open
  - Bead tunnel-abc123 (Fix connection leak) is assigned to worker-alpha but has status 'open'
  Status: Auto-fixable

✓ PASS: Check for closed beads with assignees
  Check: closed_with_assignee

Summary: 2 passed, 1 failed
Fixes applied: 1
```

### JSON Output

```json
{
  "results": [
    {
      "name": "assigned_but_open",
      "passed": false,
      "description": "Check for assigned beads with open status",
      "issues": [
        "Bead tunnel-abc123 is assigned to worker-alpha but has status 'open'"
      ],
      "fixable": true
    }
  ],
  "total": 5,
  "passed": 4,
  "failed": 1
}
```

## Integration with NEEDLE Workers

The validator complements NEEDLE fleet workers:

- **Preventive:** Runs hourly to catch issues before they impact worker dispatch
- **Safe:** All auto-fixes are reversible and documented in CLAUDE.md
- **Non-disruptive:** Dry-run mode shows what would be fixed without touching data
- **Auditable:** All fixes are logged to the audit log if audit logging is enabled

## Configuration

The validator reads from the current workspace's `.beads` directory. It requires:

1. `bead` CLI in PATH (bead-rs backend)
2. Workspace with `.beads/` directory
3. Read/write permissions to `.beads/beads.db`
4. (Optional) Audit logger for tracking fixes

For systemd scheduled execution, configure:

```bash
# Edit the service file to adjust workspace path
systemctl --user edit tunnel-bead-validator@$(whoami)

# Change validation frequency (default: hourly)
# Edit OnUnitActiveSec in tunnel-bead-validator.timer
```

## Troubleshooting

### "No .beads directory found"

The validator must run from within a workspace. Either:
- Run from the workspace root: `cd /path/to/workspace && tunnel bead-validator`
- Specify workspace explicitly: `tunnel bead-validator --workspace /path/to/workspace`

### "bead command failed"

Ensure the `bead` CLI (bead-rs) is installed and in PATH:

```bash
which bead  # Should show path to bead binary
bead --version  # Verify it works
```

### "Failed to read checkpoint"

Run `bead sync flush-only` to create an up-to-date checkpoint before validation.

### "Systemd user services not available"

Some systems require a re-login after first enabling user systemd:

```bash
# Login again, then:
systemctl --user daemon-reload
systemctl --user enable --now tunnel-bead-validator.timer
```

## Implementation Details

### File Structure

```
internal/core/bead_validator.go       # Core validation logic
cmd/tunnel/bead-validator.go          # CLI command
configs/systemd/
  tunnel-bead-validator.service       # Systemd service template
  tunnel-bead-validator.timer         # Hourly schedule
scripts/install-bead-validator.sh     # Installation script
```

### Validation Flow

1. **Discovery:** List all beads via `bead list --json`
2. **Validation:** Run 5 state machine checks in parallel
3. **Auto-fix:** Apply fixes for all fixable issues
4. **Reporting:** Generate summary bead if issues remain

### Safety Guarantees

- **Dry-run mode:** All operations previewed before execution
- **Optimistic locking:** Bead updates use `--if-revision` to prevent race conditions
- **Idempotent:** Safe to run multiple times; fixes only applied once per issue
- **Auditable:** All fixes logged with success/failure status

## Related Documentation

- `CLAUDE.md` - Bead backend requirements and state machine rules
- `NEEDLE/AGENTS.md` - NEEDLE worker dispatch and bead claiming
- `ADR-015` - Why per-worker worktrees were rejected (dependency serialization)

## License

Same as TUNNEL project.
