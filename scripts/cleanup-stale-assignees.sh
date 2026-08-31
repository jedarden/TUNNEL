#!/usr/bin/env bash
#
# Automated bead state cleanup script
# Identifies and remediate problematic bead states:
# - Assigned but not in_progress (should be in_progress or unassigned)
# - Assigned to workers that are no longer active
# - Closed beads with assignees (bead-rs invariant violation)
#

set -euo pipefail

WORKSPACE_ROOT="$(pwd)"
REPORT_FILE="/tmp/bead-cleanup-report-$(date +%s).json"
LOG_FILE="/tmp/bead-cleanup-log-$(date +%s).txt"

echo "=== Bead State Cleanup ===" | tee "$LOG_FILE"
echo "Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Initialize report counters
FIXED_CLOSED_ASSIGNEES=0
FIXED_OPEN_ASSIGNED=0
FIXED_IN_PROGRESS_UNASSIGNED=0
TOTAL_PROBLEMATIC=0

# Function to check if a worker is still active (has recent heartbeats)
worker_is_active() {
    local worker_name="$1"
    
    # Check if heartbeats.jsonl exists and has recent activity from this worker
    if [[ -f ".beads/heartbeats.jsonl" ]]; then
        # Look for heartbeat from this worker in last 24 hours
        local cutoff=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-24H +%Y-%m-%dT%H:%M:%SZ)
        if grep -q "\"worker\":\"${worker_name}\"" .beads/heartbeats.jsonl; then
            # Check if any heartbeat is recent
            local latest=$(grep "\"worker\":\"${worker_name}\"" .beads/heartbeats.jsonl | tail -1 | jq -r '.timestamp // .time // empty' | head -1)
            if [[ -n "$latest" ]]; then
                if [[ "$latest" > "$cutoff" ]]; then
                    return 0  # Worker is active
                fi
            fi
        fi
    fi
    
    return 1  # Worker is inactive or no heartbeat data
}

# Get all beads and analyze
echo "Analyzing workspace bead state..." | tee -a "$LOG_FILE"

# Use bead list with JSON output and parse it
bead list --json 2>/dev/null | while IFS= read -r line; do
    if [[ -z "$line" ]]; then
        continue
    fi
    
    BEAD_ID=$(echo "$line" | jq -r '.id')
    BEAD_STATUS=$(echo "$line" | jq -r '.status')
    ASSIGNEE=$(echo "$line" | jq -r '.assignee // ""')
    TITLE=$(echo "$line" | jq -r '.title')
    UPDATED=$(echo "$line" | jq -r '.updated_at')
    
    # Skip if no assignee
    if [[ -z "$ASSIGNEE" ]] || [[ "$ASSIGNEE" == "null" ]]; then
        continue
    fi
    
    # Check for problematic states
    ISSUE=""
    ACTION=""
    
    # Issue 1: Closed beads should NOT have assignees (bead-rs invariant)
    if [[ "$BEAD_STATUS" == "closed" ]]; then
        ISSUE="closed_with_assignee"
        ACTION="clear_assignee"
        TOTAL_PROBLEMATIC=$((TOTAL_PROBLEMATIC + 1))
        
        echo "Problematic: $BEAD_ID - closed but has assignee: $ASSIGNEE" | tee -a "$LOG_FILE"
        
        # For closed beads, we need direct database update (bead update fails on closed)
        # Check if we can access the database
        if [[ -f ".beads/beads.db" ]]; then
            echo "  → Clearing assignee via database update..." | tee -a "$LOG_FILE"
            sqlite3 .beads/beads.db "UPDATE issues SET assignee = NULL WHERE id = '$BEAD_ID' AND base_status = 'closed';" 2>/dev/null && \
                FIXED_CLOSED_ASSIGNEES=$((FIXED_CLOSED_ASSIGNEES + 1)) && \
                echo "  ✓ Cleared assignee for $BEAD_ID" | tee -a "$LOG_FILE"
        else
            echo "  ✗ No database access, skipping" | tee -a "$LOG_FILE"
        fi
    
    # Issue 2: Open beads with assignees should be in_progress
    elif [[ "$BEAD_STATUS" == "open" ]] && [[ -n "$ASSIGNEE" ]]; then
        ISSUE="open_with_assignee"
        TOTAL_PROBLEMATIC=$((TOTAL_PROBLEMATIC + 1))
        
        echo "Problematic: $BEAD_ID - open but has assignee: $ASSIGNEE" | tee -a "$LOG_FILE"
        
        # Check if assignee is still active
        if worker_is_active "$ASSIGNEE"; then
            echo "  → Assignee is active, updating status to in_progress..." | tee -a "$LOG_FILE"
            if bead update "$BEAD_ID" --status in_progress 2>/dev/null; then
                FIXED_OPEN_ASSIGNED=$((FIXED_OPEN_ASSIGNED + 1))
                echo "  ✓ Updated $BEAD_ID to in_progress" | tee -a "$LOG_FILE"
            else
                echo "  ✗ Failed to update $BEAD_ID" | tee -a "$LOG_FILE"
            fi
        else
            echo "  → Assignee is inactive, clearing assignee..." | tee -a "$LOG_FILE"
            if bead update "$BEAD_ID" --clear-assignee 2>/dev/null; then
                FIXED_OPEN_ASSIGNED=$((FIXED_OPEN_ASSIGNED + 1))
                echo "  ✓ Cleared assignee for $BEAD_ID" | tee -a "$LOG_FILE"
            else
                echo "  ✗ Failed to clear assignee for $BEAD_ID" | tee -a "$LOG_FILE"
            fi
        fi
    
    # Issue 3: In-progress beads without assignees
    elif [[ "$BEAD_STATUS" == "in_progress" ]] && [[ -z "$ASSIGNEE" || "$ASSIGNEE" == "null" ]]; then
        ISSUE="in_progress_no_assignee"
        TOTAL_PROBLEMATIC=$((TOTAL_PROBLEMATIC + 1))
        
        echo "Problematic: $BEAD_ID - in_progress but has no assignee" | tee -a "$LOG_FILE"
        echo "  → Releasing to open status..." | tee -a "$LOG_FILE"
        
        if bead update "$BEAD_ID" --status open 2>/dev/null; then
            FIXED_IN_PROGRESS_UNASSIGNED=$((FIXED_IN_PROGRESS_UNASSIGNED + 1))
            echo "  ✓ Released $BEAD_ID to open" | tee -a "$LOG_FILE"
        else
            echo "  ✗ Failed to release $BEAD_ID" | tee -a "$LOG_FILE"
        fi
    fi
done

echo "" | tee -a "$LOG_FILE"
echo "=== Cleanup Summary ===" | tee -a "$LOG_FILE"
echo "Total problematic beads found: $TOTAL_PROBLEMATIC" | tee -a "$LOG_FILE"
echo "Fixed closed beads with assignees: $FIXED_CLOSED_ASSIGNEES" | tee -a "$LOG_FILE"
echo "Fixed open beads with assignees: $FIXED_OPEN_ASSIGNED" | tee -a "$LOG_FILE"
echo "Fixed in_progress beads without assignees: $FIXED_IN_PROGRESS_UNASSIGNED" | tee -a "$LOG_FILE"
echo "Completed: $(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$LOG_FILE"

# Generate JSON report
cat > "$REPORT_FILE" << EOFJSON
{
  "cleanup_run": {
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "workspace": "$WORKSPACE_ROOT"
  },
  "summary": {
    "total_problematic": $TOTAL_PROBLEMATIC,
    "fixed_closed_with_assignees": $FIXED_CLOSED_ASSIGNEES,
    "fixed_open_with_assignees": $FIXED_OPEN_ASSIGNED,
    "fixed_in_progress_no_assignees": $FIXED_IN_PROGRESS_UNASSIGNED
  },
  "log_file": "$LOG_FILE"
}
EOFJSON

echo "" | tee -a "$LOG_FILE"
echo "Report saved to: $REPORT_FILE" | tee -a "$LOG_FILE"
echo "Log saved to: $LOG_FILE" | tee -a "$LOG_FILE"

# Exit with error count as status
exit $TOTAL_PROBLEMATIC
