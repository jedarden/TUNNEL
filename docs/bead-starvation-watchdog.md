# Bead Starvation Detection and Auto-Repair Watchdog

## Overview

The **Bead Starvation Watchdog** is an automated system that detects and repairs bead starvation conditions - situations where open beads exist but no candidates are available for workers to claim (i.e., `bead list --status open` returns results but `bead list --ready` returns empty).

This prevents the common failure mode where NEEDLE workers spin indefinitely with no work to do, even though open beads exist in the workspace.

## What It Does

The watchdog performs the following automated steps when starvation is detected:

1. **Detect Starvation** - Compares open bead count with ready bead count
2. **Run Diagnostics** - Executes `bead doctor` to identify corruption issues
3. **Attempt Auto-Repair** - Runs `bead doctor --repair` to fix common issues automatically
4. **Check Stuck States** - Identifies beads in invalid state transitions:
   - Open beads with assignees (shouldn't happen)
   - In-progress beads without assignees
   - Closed beads with assignees
5. **Execute State Corrections**:
   - Clears stale assignees on open beads
   - Releases stuck in-progress beads
   - Removes assignees from closed beads
6. **Verify Dependency Graph** - Checks for broken dependencies or circular references
7. **Log All Actions** - Creates detailed diagnostic logs in `.beads/diagnostics/watchdog/`
8. **Escalate if Needed** - Only raises an alert if auto-repair fails to resolve starvation

## Installation

### Quick Install

```bash
cd /home/coding/TUNNEL
./scripts/install-starvation-watchdog.sh
```

The installation script will prompt you to select between:

- **Timer mode (recommended)**: Runs every 5 minutes via systemd timer
- **Daemon mode**: Runs continuously as a background service
- **Both**: Install both timer and daemon

### Manual Install

```bash
# Copy systemd files to user directory
mkdir -p ~/.config/systemd/user
cp configs/systemd/tunnel-bead-starvation-watchdog@.service ~/.config/systemd/user/
cp configs/systemd/tunnel-bead-starvation-watchdog.timer ~/.config/systemd/user/

# Reload systemd and enable timer
systemctl --user daemon-reload
systemctl --user enable --now tunnel-bead-starvation-watchdog.timer
```

## Usage

### Manual Execution

```bash
# Run single check (with auto-repair enabled by default)
./scripts/bead-starvation-watchdog.sh

# Run in dry-run mode (no changes applied)
./scripts/bead-starvation-watchdog.sh --dry-run

# Run with auto-repair disabled
./scripts/bead-starvation-watchdog.sh --auto-repair=false
```

### Systemd Timer Mode

```bash
# Check timer status
systemctl --user status tunnel-bead-starvation-watchdog.timer

# View watchdog logs
journalctl --user -u tunnel-bead-starvation-watchdog@* -f

# Manually trigger the timer
systemctl --user start tunnel-bead-starvation-watchdog@$(date +%s)
```

### Systemd Daemon Mode

```bash
# Check service status
systemctl --user status tunnel-bead-starvation-watchdog.service

# View service logs
journalctl --user -u tunnel-bead-starvation-watchdog.service -f

# Restart service
systemctl --user restart tunnel-bead-starvation-watchdog.service
```

## What Gets Repaired

The watchdog automatically fixes these common bead state issues:

### 1. Open Beads with Stale Assignees

**Problem**: A bead is `open` but has an assignee (prevents it from appearing in ready frontier)

**Fix**: `bead update <id> --clear-assignee`

**Impact**: Bead becomes immediately claimable by workers

### 2. In-Progress Beads Without Assignees

**Problem**: A bead is `in_progress` but has no assignee (orphaned state)

**Fix**: `bead release <id>` (returns to open/unassigned)

**Impact**: Bead returns to normal workflow

### 3. Closed Beads with Assignees

**Problem**: A bead is `closed` but still has an assignee (cleanup issue)

**Fix**: `bead update <id> --clear-assignee`

**Impact**: Assignee is freed for other work

### 4. Dependency Graph Issues

**Problem**: Beads reference dependencies that don't exist

**Detection**: Logged but not auto-repaired (requires manual review)

**Impact**: Identified for manual intervention

## Diagnostic Logs

All watchdog runs create detailed logs in `.beads/diagnostics/watchdog/`:

```
.beads/diagnostics/watchdog/
├── watchdog-2026-08-31T10:57:53Z.log         # Main execution log
├── all-beads-2026-08-31T10:57:53Z.json       # Complete bead inventory
├── doctor-output-2026-08-31T10:57:53Z.txt     # Bead doctor results
├── stuck-states-2026-08-31T10:57:53Z.json     # Detected stuck states
└── escalation-2026-08-31T10:57:53Z.json      # Escalation details (if needed)
```

### Log Format

```
[2026-08-31T10:57:53Z] [INFO] === Bead Starvation Watchdog Starting ===
[2026-08-31T10:57:53Z] [INFO] Workspace: /home/coding/TUNNEL
[2026-08-31T10:57:53Z] [INFO] Dry run: false
[2026-08-31T10:57:53Z] [INFO] Auto-repair: true
[2026-08-31T10:57:53Z] [INFO] === Step 1: Detecting starvation condition ===
[2026-08-31T10:57:53Z] [INFO] Open beads: 4
[2026-08-31T10:57:53Z] [INFO] Ready beads: 0
[2026-08-31T10:57:53Z] [WARN] ⚠️  STARVATION DETECTED: 4 open beads but 0 ready candidates
...
[2026-08-31T10:57:53Z] [REPAIR] ✓ Cleared assignee on open bead: tunnel-abc123
[2026-08-31T10:57:53Z] [REPAIR] ✓ Released in-progress bead: tunnel-def456
[2026-08-31T10:57:53Z] [INFO] ✓ Repair successful - ready beads now available
```

## Exit Codes

- `0` - No starvation detected or successful repair
- `1` - Starvation detected and auto-repair failed (escalation required)
- `2` - Runtime error (script execution failed)

## When to Use Manual Execution

Run the watchdog manually when:

1. **Workers are starving** - You notice NEEDLE workers spinning with no work
2. **After git operations** - You've done a git rebase, merge, or cherry-pick that may have affected bead state
3. **After manual intervention** - You've manually edited bead state and want to verify consistency
4. **Before deploying workers** - You want to ensure the workspace is healthy before starting workers

## Troubleshooting

### Starvation persists after watchdog runs

1. **Check the logs**: Look for the latest watchdog log in `.beads/diagnostics/watchdog/`
2. **Review what was repaired**: Check if auto-repair made corrections
3. **Verify dependency graph**: Check for broken dependencies in the stuck-states JSON
4. **Run bead doctor manually**: `bead doctor --starvation-recovery`

### Watchdog keeps triggering but no visible issues

1. **Check for transient conditions**: Some issues may be timing-related
2. **Review diagnostic JSONs**: Look for subtle state inconsistencies in the JSON outputs
3. **Increase timer interval**: If running every 5 minutes is too frequent, edit the timer unit

### Systemd service won't start

1. **Check user systemd**: Ensure user systemd is running: `loginctl list-users`
2. **Verify permissions**: The script must be executable: `chmod +x scripts/bead-starvation-watchdog.sh`
3. **Check workspace path**: Ensure the systemd service file has the correct WorkingDirectory path

## Integration with Other Tools

The watchdog complements other bead maintenance tools:

- **`bead doctor --starvation-recovery`** - More aggressive starvation-specific recovery
- **`bead-state-diagnostics.sh`** - Comprehensive bead state inventory
- **NEEDLE workers** - Benefit from the watchdog keeping the workspace healthy

## Comparison: Starvation Detector vs. Self-Healer vs. Watchdog

| Tool | Purpose | Frequency | Scope |
|------|---------|-----------|-------|
| **Starvation Detector** | Alert on starvation conditions | Every 5 min | Detection only (no repair) |
| **Self-Healer** | Proactive maintenance to prevent starvation | Every 5 min | Preventive (stale assignments, long-running) |
| **Starvation Watchdog** | Detect AND repair active starvation | Every 5 min | Reactive (fixes stuck states) |

The watchdog is the most aggressive of the three - it only runs when starvation is detected and actively repairs the state.

## Uninstall

```bash
# Stop and disable services
systemctl --user disable --now tunnel-bead-starvation-watchdog.timer
systemctl --user disable --now tunnel-bead-starvation-watchdog.service

# Remove unit files
rm ~/.config/systemd/user/tunnel-bead-starvation-watchdog@.service
rm ~/.config/systemd/user/tunnel-bead-starvation-watchdog.timer
rm ~/.config/systemd/user/tunnel-bead-starvation-watchdog.service

# Reload systemd
systemctl --user daemon-reload
```

## License

MIT - Part of the TUNNEL project

---

**Note**: This watchdog is designed to be non-destructive. It only clears assignees or releases beads that are in provably invalid states. If you encounter issues, review the detailed logs before taking manual action.
