#!/usr/bin/env bash
#
# Bead Starvation Detection and Auto-Repair Watchdog
#
# This script detects starvation conditions (no candidates but open beads exist)
# and automatically executes repair steps. Only escalates to human if auto-repair fails.
#
# Usage:
#   bead-starvation-watchdog.sh [--dry-run] [--auto-repair=true|false]
#
# Exit codes:
#   0 - No starvation detected or successful repair
#   1 - Starvation detected and auto-repair failed (escalation required)
#   2 - Runtime error
#

set -euo pipefail

# Configuration
WORKSPACE_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
DIAGNOSTICS_DIR="${WORKSPACE_ROOT}/.beads/diagnostics"
LOG_DIR="${DIAGNOSTICS_DIR}/watchdog"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LOG_FILE="${LOG_DIR}/watchdog-${TIMESTAMP}.log"

# Parse arguments
DRY_RUN=false
AUTO_REPAIR=true
ESCALATION_BEAD_ID=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --auto-repair=*)
            AUTO_REPAIR="${1#*=}"
            shift
            ;;
        --escalation-bead=*)
            ESCALATION_BEAD_ID="${1#*=}"
            shift
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 2
            ;;
    esac
done

# Ensure directories exist
mkdir -p "${DIAGNOSTICS_DIR}"
mkdir -p "${LOG_DIR}"

# Logging functions
log() {
    local level="$1"
    shift
    local message="$*"
    local log_timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$log_timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

log_info() { log "INFO" "$@"; }
log_warn() { log "WARN" "$@"; }
log_error() { log "ERROR" "$@"; }
log_repair() { log "REPAIR" "$@"; }

# Bead query helpers
get_open_beads() {
    bead list --status open --json --limit 10000 2>/dev/null | jq -s '.' || echo '[]'
}

get_ready_beads() {
    bead list --ready --json --limit 10000 2>/dev/null | jq -s '.' || echo '[]'
}

get_inprogress_beads() {
    bead list --status in_progress --json --limit 10000 2>/dev/null | jq -s '.' || echo '[]'
}

get_all_beads() {
    bead list --json --limit 10000 2>/dev/null | jq -s '.' || echo '[]'
}

# Step 1: Detect starvation condition
detect_starvation() {
    log_info "=== Step 1: Detecting starvation condition ==="

    local open_count
    local ready_count
    local inprogress_count

    open_count=$(get_open_beads | jq '. | length')
    ready_count=$(get_ready_beads | jq '. | length')
    inprogress_count=$(get_inprogress_beads | jq '. | length')

    log_info "Open beads: $open_count"
    log_info "Ready beads: $ready_count"
    log_info "In-progress beads: $inprogress_count"

    # Starvation = open beads exist but no ready candidates
    if [ "$open_count" -gt 0 ] && [ "$ready_count" -eq 0 ]; then
        log_warn "⚠️  STARVATION DETECTED: $open_count open beads but 0 ready candidates"
        return 0
    else
        log_info "✓ No starvation detected"
        return 1
    fi
}

# Step 2: Run bead doctor to diagnose issues
run_bead_doctor() {
    log_info "=== Step 2: Running bead doctor diagnostics ==="

    local doctor_output_file="${LOG_DIR}/doctor-output-${TIMESTAMP}.txt"

    if bead doctor > "$doctor_output_file" 2>&1; then
        log_info "✓ Bead doctor completed successfully"
        cat "$doctor_output_file" >> "$LOG_FILE"
        return 0
    else
        log_error "✗ Bead doctor detected issues"
        cat "$doctor_output_file" >> "$LOG_FILE"
        return 1
    fi
}

# Step 3: Attempt auto-repair with bead doctor
attempt_doctor_repair() {
    log_info "=== Step 3: Attempting bead doctor auto-repair ==="

    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] Would run: bead doctor --repair"
        log_info "[DRY RUN] Would run: bead sync flush-only"
        return 0
    fi

    if [ "$AUTO_REPAIR" = false ]; then
        log_info "Auto-repair disabled, skipping doctor repair"
        return 1
    fi

    local repair_output_file="${LOG_DIR}/doctor-repair-${TIMESTAMP}.txt"

    if bead doctor --repair > "$repair_output_file" 2>&1; then
        log_repair "✓ Bead doctor auto-repair completed"
        cat "$repair_output_file" >> "$LOG_FILE"
    else
        log_warn "✗ Bead doctor auto-repair had issues (continuing to manual checks)"
        cat "$repair_output_file" >> "$LOG_FILE"
    fi

    # Flush checkpoint to ensure it's current after repair
    log_info "Flushing checkpoint to ensure consistency"
    local sync_output_file="${LOG_DIR}/sync-flush-${TIMESTAMP}.txt"
    if bead sync flush-only > "$sync_output_file" 2>&1; then
        log_repair "✓ Checkpoint flushed successfully"
        cat "$sync_output_file" >> "$LOG_FILE"
        return 0
    else
        log_warn "✗ Checkpoint flush had issues (continuing to manual checks)"
        cat "$sync_output_file" >> "$LOG_FILE"
        return 1
    fi
}

# Step 4: Check for stuck state transitions
check_stuck_states() {
    log_info "=== Step 4: Checking for stuck state transitions ==="

    local all_beads="${LOG_DIR}/all-beads-${TIMESTAMP}.json"
    get_all_beads > "$all_beads"

    # Check for stuck states
    local stuck_issues="${LOG_DIR}/stuck-states-${TIMESTAMP}.json"

    jq -n '{
        open_but_assigned: [],
        in_progress_but_unassigned: [],
        closed_but_assigned: [],
        has_dependencies_but_ready: []
    }' > "$stuck_issues"

    # Open but assigned (should not happen)
    jq '[.[] | select(.status == "open" and (.assignee != null and .assignee != ""))]' "$all_beads" > "${LOG_DIR}/open-assigned.json"
    local open_assigned_count=$(jq 'length' "${LOG_DIR}/open-assigned.json")
    if [ "$open_assigned_count" -gt 0 ]; then
        log_warn "Found $open_assigned_count open beads with assignees (stuck state)"
        jq --rawfile results "${LOG_DIR}/open-assigned.json" '.open_but_assigned = ($results | fromjson)' "$stuck_issues" > "${stuck_issues}.tmp"
        mv "${stuck_issues}.tmp" "$stuck_issues"
    fi

    # In-progress but unassigned
    jq '[.[] | select(.status == "in_progress" and (.assignee == null or .assignee == ""))]' "$all_beads" > "${LOG_DIR}/inprogress-unassigned.json"
    local inprogress_unassigned_count=$(jq 'length' "${LOG_DIR}/inprogress-unassigned.json")
    if [ "$inprogress_unassigned_count" -gt 0 ]; then
        log_warn "Found $inprogress_unassigned_count in-progress beads without assignees"
        jq --rawfile results "${LOG_DIR}/inprogress-unassigned.json" '.in_progress_but_unassigned = ($results | fromjson)' "$stuck_issues" > "${stuck_issues}.tmp"
        mv "${stuck_issues}.tmp" "$stuck_issues"
    fi

    # Closed but assigned
    jq '[.[] | select(.status == "closed" and (.assignee != null and .assignee != ""))]' "$all_beads" > "${LOG_DIR}/closed-assigned.json"
    local closed_assigned_count=$(jq 'length' "${LOG_DIR}/closed-assigned.json")
    if [ "$closed_assigned_count" -gt 0 ]; then
        log_warn "Found $closed_assigned_count closed beads with assignees"
        jq --rawfile results "${LOG_DIR}/closed-assigned.json" '.closed_but_assigned = ($results | fromjson)' "$stuck_issues" > "${stuck_issues}.tmp"
        mv "${stuck_issues}.tmp" "$stuck_issues"
    fi

    # Output stuck states summary
    if [ "$open_assigned_count" -gt 0 ] || [ "$inprogress_unassigned_count" -gt 0 ] || [ "$closed_assigned_count" -gt 0 ]; then
        log_warn "Total stuck states: $((open_assigned_count + inprogress_unassigned_count + closed_assigned_count))"
        return 0
    else
        log_info "✓ No stuck state transitions detected"
        return 1
    fi
}

# Step 5: Execute state corrections
execute_state_corrections() {
    log_info "=== Step 5: Executing state corrections ==="

    local all_beads="${LOG_DIR}/all-beads-${TIMESTAMP}.json"
    local corrections_made=0
    local corrections_failed=0

    # Fix open but assigned beads
    jq -r '.[] | select(.status == "open" and (.assignee != null and .assignee != "")) | .id' "$all_beads" | while read -r bead_id; do
        if [ -z "$bead_id" ] || [ "$bead_id" = "null" ]; then
            continue
        fi

        local bead_title=$(jq -r ".[] | select(.id == \"$bead_id\") | .title" "$all_beads")
        local assignee=$(jq -r ".[] | select(.id == \"$bead_id\") | .assignee" "$all_beads")

        log_info "Fixing open-but-assigned bead: $bead_id ($bead_title) [assigned to: $assignee]"

        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY RUN] Would run: bead update $bead_id --clear-assignee"
        elif [ "$AUTO_REPAIR" = true ]; then
            if bead update "$bead_id" --clear-assignee >> "$LOG_FILE" 2>&1; then
                log_repair "✓ Cleared assignee on open bead: $bead_id"
                corrections_made=$((corrections_made + 1))
            else
                log_error "✗ Failed to clear assignee on: $bead_id"
                corrections_failed=$((corrections_failed + 1))
            fi
        fi
    done

    # Fix in-progress but unassigned beads
    jq -r '.[] | select(.status == "in_progress" and (.assignee == null or .assignee == "")) | .id' "$all_beads" | while read -r bead_id; do
        if [ -z "$bead_id" ] || [ "$bead_id" = "null" ]; then
            continue
        fi

        local bead_title=$(jq -r ".[] | select(.id == \"$bead_id\") | .title" "$all_beads")

        log_info "Fixing in-progress-but-unassigned bead: $bead_id ($bead_title)"

        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY RUN] Would run: bead release $bead_id"
        elif [ "$AUTO_REPAIR" = true ]; then
            if bead release "$bead_id" >> "$LOG_FILE" 2>&1; then
                log_repair "✓ Released in-progress bead: $bead_id"
                corrections_made=$((corrections_made + 1))
            else
                log_error "✗ Failed to release: $bead_id"
                corrections_failed=$((corrections_failed + 1))
            fi
        fi
    done

    # Fix closed but assigned beads
    jq -r '.[] | select(.status == "closed" and (.assignee != null and .assignee != "")) | .id' "$all_beads" | while read -r bead_id; do
        if [ -z "$bead_id" ] || [ "$bead_id" = "null" ]; then
            continue
        fi

        local bead_title=$(jq -r ".[] | select(.id == \"$bead_id\") | .title" "$all_beads")
        local assignee=$(jq -r ".[] | select(.id == \"$bead_id\") | .assignee" "$all_beads")

        log_info "Fixing closed-but-assigned bead: $bead_id ($bead_title) [assigned to: $assignee]"

        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY RUN] Would run: bead update $bead_id --clear-assignee"
        elif [ "$AUTO_REPAIR" = true ]; then
            if bead update "$bead_id" --clear-assignee >> "$LOG_FILE" 2>&1; then
                log_repair "✓ Cleared assignee on closed bead: $bead_id"
                corrections_made=$((corrections_made + 1))
            else
                log_error "✗ Failed to clear assignee on: $bead_id"
                corrections_failed=$((corrections_failed + 1))
            fi
        fi
    done

    log_info "Corrections made: $corrections_made, Failed: $corrections_failed"

    if [ "$corrections_made" -gt 0 ]; then
        return 0
    else
        return 1
    fi
}

# Step 6: Verify dependency graph integrity
verify_dependency_integrity() {
    log_info "=== Step 6: Verifying dependency graph integrity ==="

    local all_beads="${LOG_DIR}/all-beads-${TIMESTAMP}.json"

    # Check for circular dependencies or other issues
    # This is a basic check - full graph analysis would be more complex

    local total_beads
    total_beads=$(jq 'length' "$all_beads")
    local beads_with_deps
    beads_with_deps=$(jq '[.[] | select(.dependencies != null and .dependencies != [] and (.dependencies | length) > 0)] | length' "$all_beads")

    log_info "Total beads: $total_beads"
    log_info "Beads with dependencies: $beads_with_deps"

    # Check for broken dependencies (referencing non-existent beads)
    local broken_deps="${LOG_DIR}/broken-deps-${TIMESTAMP}.json"
    jq -n '[]' > "$broken_deps"

    local has_broken_deps=false

    jq -r '.[] | select(.dependencies != null and .dependencies != []) | .id as $id | .dependencies[]? | "\($id)|\(.)"' "$all_beads" | while IFS='|' read -r bead_id dep_id; do
        if [ -z "$dep_id" ] || [ "$dep_id" = "null" ]; then
            continue
        fi

        # Check if dependency exists
        if ! jq -e ".[] | select(.id == \"$dep_id\")" "$all_beads" > /dev/null 2>&1; then
            log_warn "Broken dependency: $bead_id depends on non-existent $dep_id"
            echo "{\"bead\": \"$bead_id\", \"missing_dependency\": \"$dep_id\"}" >> "${LOG_DIR}/broken-deps-list.json"
            has_broken_deps=true
        fi
    done

    if [ "$has_broken_deps" = true ]; then
        log_warn "⚠️  Found broken dependencies in graph"
        jq -s '.' "${LOG_DIR}/broken-deps-list.json" > "$broken_deps"
        return 1
    else
        log_info "✓ Dependency graph appears intact"
        return 0
    fi
}

# Step 7: Re-check for starvation after repairs
verify_repair_success() {
    log_info "=== Step 7: Verifying repair success ==="

    local ready_count_after
    ready_count_after=$(get_ready_beads | jq '. | length')

    log_info "Ready beads after repair: $ready_count_after"

    if [ "$ready_count_after" -gt 0 ]; then
        log_info "✓ Repair successful - ready beads now available"
        return 0
    else
        log_warn "✗ Repair failed - still no ready beads"
        return 1
    fi
}

# Step 8: Escalate to human if needed
escalate_to_human() {
    log_info "=== Step 8: Escalating to human intervention ==="

    local escalation_file="${LOG_DIR}/escalation-${TIMESTAMP}.json"

    jq -n \
        --arg timestamp "$TIMESTAMP" \
        --arg log_file "$LOG_FILE" \
        '{
            timestamp: $timestamp,
            log_file: $log_file,
            reason: "Auto-repair failed to resolve starvation condition",
            action_required: "Manual intervention required to fix bead state"
        }' > "$escalation_file"

    log_error "⚠️  AUTO-REPAIR FAILED - ESCALATION REQUIRED"
    log_error "Escalation details saved to: $escalation_file"

    # If this is running from a bead context, update the bead
    if [ -n "$ESCALATION_BEAD_ID" ]; then
        log_info "Updating escalation bead: $ESCALATION_BEAD_ID"
        bead update "$ESCALATION_BEAD_ID" \
            --status "in_progress" \
            --notes "Auto-repair failed. Log: $LOG_FILE. Escalation: $escalation_file" \
            >> "$LOG_FILE" 2>&1 || true
    fi

    return 1
}

# Main execution flow
main() {
    log_info "=== Bead Starvation Watchdog Starting ==="
    log_info "Workspace: $WORKSPACE_ROOT"
    log_info "Dry run: $DRY_RUN"
    log_info "Auto-repair: $AUTO_REPAIR"

    local starvation_detected=false
    local repairs_attempted=false
    local repairs_successful=false

    # Step 1: Detect starvation
    if detect_starvation; then
        starvation_detected=true
    else
        log_info "No starvation detected - exiting successfully"
        return 0
    fi

    # Step 2: Run diagnostics
    run_bead_doctor

    # Step 3: Attempt doctor repair
    if attempt_doctor_repair; then
        repairs_attempted=true
    fi

    # Step 4: Check for stuck states
    if check_stuck_states; then
        # Step 5: Execute corrections
        if execute_state_corrections; then
            repairs_attempted=true
        fi
    fi

    # Step 6: Verify dependency integrity
    verify_dependency_integrity

    # Step 7: Verify repair success
    if [ "$starvation_detected" = true ] && [ "$repairs_attempted" = true ]; then
        if verify_repair_success; then
            repairs_successful=true
        fi
    fi

    # Step 8: Escalate if needed
    if [ "$starvation_detected" = true ] && [ "$repairs_successful" = false ]; then
        escalate_to_human
        return 1
    fi

    log_info "=== Watchdog completed successfully ==="
    return 0
}

# Run main
main
exit $?
