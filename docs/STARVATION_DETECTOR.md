# Starvation Detector - Automated Bead Starvation Detection and Repair

## Overview

The Starvation Detector is an automated background service that monitors for bead starvation conditions (when Pluck finds no candidates but open beads exist) and automatically diagnoses and repairs common issues.

## Features

### 1. Detection
- Monitors workspace for starvation conditions automatically
- Detects when `bead list --ready` returns no candidates but open beads exist
- Identifies exclusion reasons for each excluded bead

### 2. Auto-Diagnostics
When starvation is detected, the system automatically:
- Lists all open beads with full details (status, assignee, dependencies, revision)
- Checks each bead against ready criteria to identify why it's excluded
- Detects ALL fixable issues:
  - Stale assignees (assigned-but-open beads)
  - Invalid status transitions (closed beads with assignees)
  - Dependency cycles or stale dependencies (blocked by closed beads)
  - Missing or incorrect labels (based on bead state)
  - Database inconsistencies (out of sync with checkpoint)
  - Workspace configuration issues
  - Bead backend health problems

### 3. Auto-Repair
For ALL fixable issues, the system automatically:
- Clears stale assignees on assigned-but-open beads
- Clears assignees from closed beads
- Unblocks beads that are blocked by closed beads
- Adds missing labels (priority:, in-progress) based on bead state
- Syncs database with checkpoint if corruption detected
- Removes circular self-dependencies
- Flushes stale checkpoints (older than 24 hours)
- Attempts bead backend recovery

**All fixes are idempotent and safe to run automatically.** The system will only create diagnostic beads for issues that genuinely require human intervention (e.g., ambiguous workspace backend configuration).

### 4. Manual Intervention
For issues requiring manual review, the system:
- Creates diagnostic beads with detailed findings
- Provides actionable recommendations
- Includes context and exclusion reasons

## Installation

### Prerequisites
- TUNNEL binary installed (`make install`)
- systemd user services enabled
- Workspace with bead-rs backend

### Installation Methods

#### Method 1: Timer Mode (Recommended)
Runs periodically via systemd timer:

```bash
cd /home/coding/TUNNEL
./scripts/install-starvation-detector.sh
# Choose option 1 for timer mode
```

This configures a systemd timer that runs every 5 minutes.

#### Method 2: Daemon Mode
Runs continuously as a background service:

```bash
cd /home/coding/TUNNEL
./scripts/install-starvation-detector.sh
# Choose option 2 for daemon mode
```

#### Method 3: Manual Execution
Run on-demand without systemd:

```bash
# Single detection check
tunnel starvation-detector

# Dry-run mode (no changes)
tunnel starvation-detector --dry-run

# Scheduled mode (for cron)
tunnel starvation-detector --scheduled
```

## Usage

### Interactive Mode
```bash
tunnel starvation-detector
```
Runs a single detection check with detailed output.

### Daemon Mode
```bash
tunnel starvation-detector --daemon
```
Runs continuously, checking every 5 minutes (configurable).

### Scheduled Mode
```bash
tunnel starvation-detector --scheduled
```
Single-shot execution for cron/systemd timers with minimal output.

### Configuration Options

```bash
--check-interval 5m        # How often to check for starvation
--alert-cooldown 15m       # Minimum time between alerts
--auto-repair=true         # Enable automatic repairs
--dry-run=false           # Preview changes without applying
--workspace=/path/to/ws    # Specify workspace directory
--json                     # Output results in JSON format
```

## Architecture

### Components

1. **StarvationDetector** (`internal/core/starvation_detector.go`)
   - Core detection logic
   - Diagnostics engine
   - Auto-repair orchestration

2. **BeadValidator** (existing)
   - Validates bead state consistency
   - Applies fixes for common issues
   - Creates summary reports

3. **CLI Command** (`cmd/tunnel/starvation_detector.go`)
   - User-facing interface
   - Daemon mode support
   - Scheduled execution mode

### Detection Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Starvation Detector                      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
                ┌───────────────────────┐
                │   Detect Starvation   │
                │   - List ready beads  │
                │   - List open beads   │
                │   - Compare counts    │
                └───────────────────────┘
                            │
                ┌───────────▼────────────┐
                │ Starvation Detected?   │
                └────────────────────────┘
                     │              │
                    No             Yes
                     │              │
                     │          ┌───▼──────────────┐
                     │          │  Run Diagnostics │
                     │          │  - Validate beads │
                     │          │  - Check deps    │
                     │          │  - Analyze state │
                     │          └──────────────────┘
                     │                  │
                     │          ┌───────▼──────────┐
                     │          │ Auto-Repair Safe │
                     │          │ Issues           │
                     │          └──────────────────┘
                     │                  │
                     │          ┌───────▼──────────┐
                     │          │ Create Diagnostic│
                     │          │ Bead for Manual  │
                     │          │ Issues           │
                     │          └──────────────────┘
                     │                  │
                     └──────────────────┴─────────┘
                                        │
                                        ▼
                                ┌───────────────┐
                                │   Report &    │
                                │   Alert       │
                                └───────────────┘
```

## Monitoring

### Check Status

```bash
# Timer mode status
systemctl --user status tunnel-starvation-detector.timer

# Daemon mode status
systemctl --user status tunnel-starvation-detector.service

# View logs
journalctl --user -u tunnel-starvation-detector@* -f

# View detection history
tunnel starvation-detector --json
```

### Alert Management

The detector implements alert cooldown to prevent spam:
- Default cooldown: 15 minutes
- Configurable via `--alert-cooldown`
- Per-workspace independent tracking

## Troubleshooting

### Not detecting starvation
1. Check workspace path: `--workspace=/correct/path`
2. Verify bead backend is bead-rs: `cat .needle.yaml`
3. Test manually: `bead list --ready`

### Auto-repair not working
1. Check if enabled: `--auto-repair=true`
2. Try dry-run first: `--dry-run`
3. Check logs: `journalctl --user -u tunnel-starvation-detector@*`

### Systemd service fails
1. Check binary path: `which tunnel`
2. Verify permissions: `ls -la ~/.config/systemd/user/`
3. Check logs: `journalctl --user -u tunnel-starvation-detector*`

## Uninstallation

```bash
# Stop and disable services
systemctl --user disable --now tunnel-starvation-detector.timer
systemctl --user disable --now tunnel-starvation-detector.service

# Remove service files
rm ~/.config/systemd/user/tunnel-starvation-detector@.service
rm ~/.config/systemd/user/tunnel-starvation-detector.service
rm ~/.config/systemd/user/tunnel-starvation-detector.timer

# Reload systemd
systemctl --user daemon-reload
```

## Integration with Pluck

The starvation detector complements Pluck by:
- Automatically fixing issues that Pluck detects
- Providing detailed diagnostics for manual review
- Creating actionable tickets for complex issues
- Running continuously without human intervention

When Pluck reports "no candidates but open beads exist," the detector:
1. Identifies why each bead is excluded
2. Fixes common issues automatically
3. Creates diagnostic beads for remaining issues
4. Prevents fleet starvation without manual intervention

## Development

### Code Structure
- `internal/core/starvation_detector.go` - Core detection logic
- `cmd/tunnel/starvation_detector.go` - CLI interface
- `configs/systemd/` - Systemd service files
- `scripts/install-starvation-detector.sh` - Installation script

### Testing
```bash
# Dry-run test
tunnel starvation-detector --dry-run --workspace=/path/to/test

# Manual trigger
tunnel starvation-detector --auto-repair=false

# JSON output for automation
tunnel starvation-detector --json
```

## License

MIT - Part of TUNNEL project