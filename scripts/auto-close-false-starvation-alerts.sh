#!/usr/bin/env bash
# Auto-close false positive starvation alerts
# This watchdog detects starvation alerts that are false positives (workspace legitimately idle)
# and closes them automatically, restoring signal-to-noise ratio.

set -euo pipefail

# Configuration
AUDIT_LOG="$HOME/.local/log/starvation-watchdog.log"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Ensure audit log directory exists
ensure_audit_log() {
    local log_dir="$(dirname "$AUDIT_LOG")"
    if [[ ! -d "$log_dir" ]]; then
        mkdir -p "$log_dir"
    fi
}

# Log to audit trail with timestamp
audit_log() {
    local message="$1"
    local timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    echo "[$timestamp] $message" >> "$AUDIT_LOG"
}

# Check if workspace has 0 open beads (legitimate idle state)
is_workspace_legitimately_idle() {
    local open_beads_count
    open_beads_count=$(bead list --status open --json 2>/dev/null | jq 'length' 2>/dev/null || echo "-1")

    if [[ "$open_beads_count" == "0" ]]; then
        return 0  # True - workspace is idle
    else
        return 1  # False - workspace has work
    fi
}

# Main logic
main() {
    cd "$WORKSPACE_ROOT"
    ensure_audit_log

    audit_log "Starvation watchdog starting run"

    # Get all open beads and filter for starvation-alert label
    # bead list --json returns JSONL format (one JSON object per line)
    local open_alerts
    open_alerts=$(bead list --status open --json 2>/dev/null | jq -s '[.[] | select(.labels != null and (.labels | index("starvation-alert")) != null)]' 2>/dev/null || echo '[]')

    # Get all in_progress beads with starvation-alert label
    local in_progress_alerts
    in_progress_alerts=$(bead list --status in_progress --json 2>/dev/null | jq -s '[.[] | select(.labels != null and (.labels | index("starvation-alert")) != null)]' 2>/dev/null || echo '[]')

    # Combine both lists
    local all_alerts
    all_alerts=$(echo "$open_alerts" "$in_progress_alerts" | jq -s 'add' 2>/dev/null || echo '[]')

    local alert_count
    alert_count=$(echo "$all_alerts" | jq 'length')

    audit_log "Found $alert_count starvation alert beads to evaluate"

    if [[ "$alert_count" == "0" ]]; then
        audit_log "No starvation alerts found - nothing to do"
        exit 0
    fi

    # Check if workspace is legitimately idle (0 open beads)
    if is_workspace_legitimately_idle; then
        audit_log "Workspace is legitimately idle (0 open beads) - evaluating alerts"

        # Process each alert bead
        while IFS= read -r bead_info; do
            local bead_id
            local bead_title
            bead_id=$(echo "$bead_info" | jq -r '.id')
            bead_title=$(echo "$bead_info" | jq -r '.title')

            audit_log "Processing starvation alert: $bead_id - $bead_title"

            # Auto-close the bead with standard reason
            if bead close "$bead_id" --reason "False positive - workspace has 0 open beads (legitimate idle state). Auto-closed by starvation watchdog." 2>&1; then
                audit_log "✓ Closed false positive starvation alert: $bead_id"
            else
                audit_log "✗ Failed to close bead $bead_id"
            fi
        done < <(echo "$all_alerts" | jq -c '.[]')
    else
        audit_log "Workspace has open beads - starvation alerts may be valid, skipping auto-closure"
    fi

    audit_log "Starvation watchdog run complete"
}

main "$@"
