#!/usr/bin/env bash
#
# Pluck Query Diagnostic Tool
# Investigates why beads are invisible in Pluck (--ready) queries
# Provides detailed logging of filter conditions and bead eligibility
#

set -euo pipefail

# Configuration
WORKSPACE_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
DIAGNOSTICS_DIR="${WORKSPACE_ROOT}/.beads/diagnostics"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
REPORT_FILE="${DIAGNOSTICS_DIR}/pluck-query-diagnostic-${TIMESTAMP}.json"
TEMP_DIR=$(mktemp -d)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

cleanup() {
    rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

# Ensure diagnostics directory exists
mkdir -p "${DIAGNOSTICS_DIR}"

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Initialize report structure
initialize_report() {
    cat > "${REPORT_FILE}" <<EOF
{
  "timestamp": "${TIMESTAMP}",
  "workspace": "${WORKSPACE_ROOT}",
  "diagnostic_type": "pluck_query_eligibility",
  "version": "1.0",
  "summary": {},
  "filter_analysis": [],
  "configuration_check": {},
  "bead_doctor_output": {},
  "issues_found": [],
  "recommendations": []
}
EOF
}

update_report_field() {
    local field="$1"
    local value="$2"
    local temp_report="${TEMP_DIR}/report.tmp.json"

    jq --arg field "$field" --argjson value "$value" '.[$field] = $value' "$REPORT_FILE" > "$temp_report"
    mv "$temp_report" "$REPORT_FILE"
}

# Step 1: Run bead doctor and check for basic issues
run_bead_doctor() {
    log_step "Running bead doctor to check workspace health..."

    local doctor_output="${TEMP_DIR}/doctor_output.txt"
    local doctor_json="${TEMP_DIR}/doctor_output.json"

    if bead doctor > "$doctor_output" 2>&1; then
        log_info "✓ Bead doctor completed successfully"
    else
        log_warn "⚠ Bead doctor reported issues"
    fi

    # Parse doctor output into structured format
    jq -n \
        --rawfile output "$doctor_output" \
        '{
            status: (if $output | contains("OK") then "healthy" else "issues_found" end),
            raw_output: $output,
            checks: ($output | split("\n") | [.[], "OK ", "WARN ", "ERROR "] | map(select(. != "")))
        }' > "$doctor_json"

    # Add to report
    update_report_field "bead_doctor_output" "$(cat "$doctor_json")"

    # Check for specific issues
    if grep -q "database inconsistent" "$doctor_output" 2>/dev/null; then
        echo "database_inconsistent" >> "${TEMP_DIR}/issues.txt"
    fi

    if grep -q "checkpoint" "$doctor_output" 2>/dev/null; then
        echo "checkpoint_issue" >> "${TEMP_DIR}/issues.txt"
    fi
}

# Step 2: Verify configuration files
check_configuration() {
    log_step "Verifying workspace configuration..."

    local config_json="${TEMP_DIR}/config_check.json"

    jq -n '{
        needle_yaml: {exists: false, backend: null, issues: []},
        beads_backend: {exists: false, type: null, issues: []},
        checkpoint_fresh: null,
        issues: []
    }' > "$config_json"

    # Check .needle.yaml
    if [ -f "${WORKSPACE_ROOT}/.needle.yaml" ]; then
        jq '.needle_yaml.exists = true' "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"

        # Extract backend
        if grep -q "backend: bead-rs" .needle.yaml 2>/dev/null; then
            jq '.needle_yaml.backend = "bead-rs"' "$config_json" > "${TEMP_DIR}/config.tmp.json"
            mv "${TEMP_DIR}/config.tmp.json" "$config_json"
        elif grep -q "backend: bf" .needle.yaml 2>/dev/null; then
            jq '.needle_yaml.backend = "bf"' "$config_json" > "${TEMP_DIR}/config.tmp.json"
            mv "${TEMP_DIR}/config.tmp.json" "$config_json"
        else
            jq '.needle_yaml.issues += ["No backend declared in .needle.yaml"]' "$config_json" > "${TEMP_DIR}/config.tmp.json"
            mv "${TEMP_DIR}/config.tmp.json" "$config_json"
        fi
    else
        jq '.needle_yaml.issues += [".needle.yaml not found"]' "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"
    fi

    # Check .beads backend state
    if [ -f "${WORKSPACE_ROOT}/.beads/config.json" ]; then
        jq '.beads_backend.exists = true | .beads_backend.type = "bead-rs"' "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"
    elif [ -f "${WORKSPACE_ROOT}/.beads/config.yaml" ]; then
        jq '.beads_backend.exists = true | .beads_backend.type = "bf"' "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"
    fi

    # Check backend mismatch
    local needle_backend=$(jq -r '.needle_yaml.backend' "$config_json")
    local beads_backend=$(jq -r '.beads_backend.type' "$config_json")

    if [ "$needle_backend" != "null" ] && [ "$beads_backend" != "null" ] && [ "$needle_backend" != "$beads_backend" ]; then
        jq ".issues += [\"Backend mismatch: .needle.yaml says '$needle_backend' but .beads has '$beads_backend'\"]" "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"
    fi

    # Check checkpoint freshness
    if [ -f "${WORKSPACE_ROOT}/.beads/checkpoint/current.json" ]; then
        jq '.checkpoint_fresh = true' "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"
    else
        jq '.checkpoint_fresh = false | .issues += ["Checkpoint not found or stale"]' "$config_json" > "${TEMP_DIR}/config.tmp.json"
        mv "${TEMP_DIR}/config.tmp.json" "$config_json"
    fi

    # Add to report
    update_report_field "configuration_check" "$(cat "$config_json")"
}

# Step 3: Analyze Pluck query filters in detail
analyze_pluck_filters() {
    log_step "Analyzing Pluck (--ready) query filters..."

    local all_beads="${TEMP_DIR}/all_beads.json"
    local open_beads="${TEMP_DIR}/open_beads.json"
    local ready_beads="${TEMP_DIR}/ready_beads.json"
    local analysis="${TEMP_DIR}/filter_analysis.json"

    # Get all beads
    bead list --json --limit 10000 | jq -s '.' > "$all_beads"

    # Get open beads (should include ready candidates)
    bead list --status open --json | jq -s '.' > "$open_beads"

    # Get ready beads (Pluck result)
    bead list --ready --json | jq -s '.' > "$ready_beads"

    local total_beads=$(jq 'length' "$all_beads")
    local open_count=$(jq 'length' "$open_beads")
    local ready_count=$(jq 'length' "$ready_beads")

    log_info "Total beads: $total_beads"
    log_info "Open beads: $open_count"
    log_info "Ready candidates: $ready_count"

    # Build filter analysis
    jq -n \
        --argjson total "$total_beads" \
        --argjson open "$open_count" \
        --argjson ready "$ready_count" \
        '{
            total_beads: $total,
            open_beads: $open,
            ready_candidates: $ready,
            filters: []
        }' > "$analysis"

    # Filter 1: Status check
    log_info "Filter 1: Status must be 'open'"
    local status_excluded=$(jq '[.[] | select(.status != "open")] | length' "$all_beads")
    jq ".filters += [{name: \"status_check\", description: \"Status must be 'open'\", excluded: $status_excluded}]" "$analysis" > "${TEMP_DIR}/analysis.tmp.json"
    mv "${TEMP_DIR}/analysis.tmp.json" "$analysis"

    # Filter 2: Manual block check
    log_info "Filter 2: Must not be manually blocked"
    local blocked_count=$(jq '[.[] | select(.manual_blocked == true)] | length' "$open_beads")
    jq ".filters += [{name: \"manual_block\", description: \"Must not be manually blocked\", excluded: $blocked_count}]" "$analysis" > "${TEMP_DIR}/analysis.tmp.json"
    mv "${TEMP_DIR}/analysis.tmp.json" "$analysis"

    # Filter 3: Assignee check
    log_info "Filter 3: Must not have assignee (ready frontier)"
    local assigned_count=$(jq '[.[] | select(.assignee != null and .assignee != "") and select(.status == "open")] | length' "$all_beads")
    jq ".filters += [{name: \"assignee_check\", description: \"Must not have assignee\", excluded: $assigned_count}]" "$analysis" > "${TEMP_DIR}/analysis.tmp.json"
    mv "${TEMP_DIR}/analysis.tmp.json" "$analysis"

    # Filter 4: Dependency check
    log_info "Filter 4: Must have no unresolved dependencies"
    local blocked_count=$(jq '[.[] | select(.blocked_by != null and (.blocked_by | length > 0))] | length' "$open_beads")
    jq ".filters += [{name: \"dependency_check\", description: \"Must have no unresolved dependencies\", excluded: $blocked_count}]" "$analysis" > "${TEMP_DIR}/analysis.tmp.json"
    mv "${TEMP_DIR}/analysis.tmp.json" "$analysis"

    # Filter 5: Label exclusions
    log_info "Filter 5: Must not have exclusion labels"
    local label_exclusions=$(jq '[.[] | select(.labels != null) | select(.labels[] | test("human|blocked|exclude"))] | length' "$open_beads")
    jq ".filters += [{name: \"label_filter\", description: \"Must not have exclusion labels\", excluded: $label_exclusions}]" "$analysis" > "${TEMP_DIR}/analysis.tmp.json"
    mv "${TEMP_DIR}/analysis.tmp.json" "$analysis"

    # Find beads that SHOULD be candidates but aren't
    log_step "Identifying beads that should be ready but aren't..."

    local should_be_ready="${TEMP_DIR}/should_be_ready.json"
    local actually_ready="${TEMP_DIR}/actually_ready.json"

    # Beads that are open, unblocked, unassigned, but not in ready list
    jq '[.[] | select(.status == "open" and .manual_blocked == false and (.assignee == null or .assignee == "") and (.blocked_by == null or (.blocked_by | length) == 0))]' "$open_beads" > "$should_be_ready"

    local should_count=$(jq 'length' "$should_be_ready")
    local ready_ids=$(jq -r '.[].id' "$ready_beads")
    local should_ready_ids=$(jq -r '.[].id' "$should_be_ready")

    # Find missing beads
    local missing_beads="${TEMP_DIR}/missing_beads.json"

    # Check if files exist and have content
    if [ ! -s "$should_be_ready" ] || [ ! -s "$ready_beads" ]; then
        echo "[]" > "$missing_beads"
        local missing_count=0
    else
        # Extract ready IDs into a JSON array
        local ready_ids_list=$(jq -r '.[].id' "$ready_beads" 2>/dev/null | jq -R . | jq -s '.' 2>/dev/null || echo "[]")

        # Find beads in should_be_ready that are not in ready_beads
        if jq --slurpfile should_ready "$should_be_ready" \
           --argjson ready_ids "$ready_ids_list" \
           '[$should_ready[0][] | select(.id as $id | $ready_ids | index($id) | not)]' \
           > "$missing_beads" 2>/dev/null; then
            # Success
            :
        else
            echo "[]" > "$missing_beads"
        fi

        local missing_count=$(jq 'length' "$missing_beads" 2>/dev/null || echo "0")
    fi

    # Ensure missing_count is a valid number
    if [ -z "$missing_count" ] || ! [[ "$missing_count" =~ ^[0-9]+$ ]]; then
        missing_count=0
    fi

    log_info "Beads that should be ready: $should_count"
    log_info "Actually ready: $ready_count"
    log_info "Missing from ready list: $missing_count"

    if [ "$missing_count" -gt 0 ]; then
        log_warn "Found $missing_count beads missing from ready query!"

        jq ".missing_beads = $(cat "$missing_beads")" "$analysis" > "${TEMP_DIR}/analysis.tmp.json"
        mv "${TEMP_DIR}/analysis.tmp.json" "$analysis"

        # Add missing beads to issues
        jq -r '.[].id' "$missing_beads" > "${TEMP_DIR}/missing_ids.txt"
    fi

    # Add analysis to report
    update_report_field "filter_analysis" "$(cat "$analysis")"

    # Update summary
    jq -n \
        --arg total "$total_beads" \
        --arg open "$open_count" \
        --arg ready "$ready_count" \
        --arg missing "$missing_count" \
        '{
            total_beads: ($total | tonumber),
            open_beads: ($open | tonumber),
            ready_candidates: ($ready | tonumber),
            missing_from_ready: ($missing | tonumber),
            starvation_detected: (($open | tonumber) > 0 and ($ready | tonumber) == 0)
        }' > "${TEMP_DIR}/summary.json"

    update_report_field "summary" "$(cat "${TEMP_DIR}/summary.json")"
}

# Step 4: Identify specific exclusion reasons for missing beads
analyze_missing_beads() {
    local missing_beads="$1"

    if [ ! -s "$missing_beads" ] || [ "$(jq 'length' "$missing_beads")" -eq 0 ]; then
        return
    fi

    log_step "Analyzing why specific beads are excluded..."

    local exclusion_details="${TEMP_DIR}/exclusion_details.json"
    jq -n '[]' > "$exclusion_details"

    jq -c '.[]' "$missing_beads" | while read -r bead; do
        local bead_id=$(echo "$bead" | jq -r '.id')
        local bead_title=$(echo "$bead" | jq -r '.title')
        local bead_labels=$(echo "$bead" | jq -r '.labels')
        local bead_blocked=$(echo "$bead" | jq -r '.blocked_by')

        log_info "Analyzing bead: $bead_id - $bead_title"

        local reasons=()

        # Check for problematic labels
        if echo "$bead_labels" | jq -e '.[] | test("human|blocked|exclude|unravel)' > /dev/null 2>&1; then
            reasons+=("Has exclusion label: $(echo "$bead_labels" | jq -r '.[] | select(test("human|blocked|exclude|unravel"))' | tr '\n' ', ')")
        fi

        # Check for dependencies
        if [ "$bead_blocked" != "null" ] && [ "$(echo "$bead_blocked" | jq 'length')" -gt 0 ]; then
            reasons+=("Blocked by dependencies: $(echo "$bead_blocked" | jq -r 'join(", ")')")
        fi

        # Check assignee
        local assignee=$(echo "$bead" | jq -r '.assignee')
        if [ "$assignee" != "null" ] && [ -n "$assignee" ]; then
            reasons+=("Has assignee: $assignee")
        fi

        # Create detail entry
        local detail_json=$(jq -n \
            --arg id "$bead_id" \
            --arg title "$bead_title" \
            --argjson reasons "$(printf '%s\n' "${reasons[@]}" | jq -R . | jq -s '.')" \
            '{id: $id, title: $title, exclusion_reasons: $reasons}')

        jq --argjson detail "$detail_json" '. += [$detail]' "$exclusion_details" > "${TEMP_DIR}/exclusion.tmp.json"
        mv "${TEMP_DIR}/exclusion.tmp.json" "$exclusion_details"
    done

    update_report_field "missing_beads_analysis" "$(cat "$exclusion_details")"
}

# Step 5: Generate recommendations
generate_recommendations() {
    log_step "Generating remediation recommendations..."

    local recommendations="${TEMP_DIR}/recommendations.json"
    jq -n '[]' > "$recommendations"

    local summary=$(jq -r '.summary' "$REPORT_FILE")
    local open_count=$(echo "$summary" | jq -r '.open_beads')
    local ready_count=$(echo "$summary" | jq -r '.ready_candidates')
    local missing_count=$(echo "$summary" | jq -r '.missing_from_ready')

    # Check for starvation condition
    if [ "$open_count" -gt 0 ] && [ "$ready_count" -eq 0 ]; then
        jq '. += ["CRITICAL: Starvation detected - open beads exist but zero ready candidates"]' "$recommendations" > "${TEMP_DIR}/rec.tmp.json"
        mv "${TEMP_DIR}/rec.tmp.json" "$recommendations"
    fi

    # Check for missing beads
    if [ "$missing_count" -gt 0 ]; then
        jq '. += ["Found beads missing from ready list - check labels and dependencies"]' "$recommendations" > "${TEMP_DIR}/rec.tmp.json"
        mv "${TEMP_DIR}/rec.tmp.json" "$recommendations"
    fi

    # Check configuration issues
    local config_issues=$(jq -r '.configuration_check.issues // []' "$REPORT_FILE")
    if [ "$(echo "$config_issues" | jq 'length')" -gt 0 ]; then
        jq '. += ["Configuration issues detected - review .needle.yaml and .beads backend"]' "$recommendations" > "${TEMP_DIR}/rec.tmp.json"
        mv "${TEMP_DIR}/rec.tmp.json" "$recommendations"
    fi

    # Add to report
    update_report_field "recommendations" "$(cat "$recommendations")"
}

# Step 6: Attempt auto-repair for fixable issues
attempt_auto_repair() {
    log_step "Attempting automatic repairs..."

    local repairs_made=0
    local repairs_log="${TEMP_DIR}/repairs.json"
    jq -n '[]' > "$repairs_log"

    # Run bead doctor --repair if safe
    local doctor_status=$(jq -r '.bead_doctor_output.status' "$REPORT_FILE")
    if [ "$doctor_status" = "healthy" ]; then
        log_info "Workspace appears healthy, skipping auto-repair"

        jq '. += ["Skipped auto-repair: workspace healthy"]' "$repairs_log" > "${TEMP_DIR}/repairs.tmp.json"
        mv "${TEMP_DIR}/repairs.tmp.json" "$repairs_log"
    else
        log_info "Attempting bead doctor --repair..."

        if bead doctor --repair > "${TEMP_DIR}/repair_output.txt" 2>&1; then
            log_info "✓ Auto-repair completed"
            repairs_made=1

            jq '. += ["Ran bead doctor --repair"]' "$repairs_log" > "${TEMP_DIR}/repairs.tmp.json"
            mv "${TEMP_DIR}/repairs.tmp.json" "$repairs_log"
        else
            log_warn "⚠ Auto-repair reported issues"

            jq '. += ["bead doctor --repair had issues - see output"]' "$repairs_log" > "${TEMP_DIR}/repairs.tmp.json"
            mv "${TEMP_DIR}/repairs.tmp.json" "$repairs_log"
        fi

        # Include repair output in report
        update_report_field "auto_repair_output" "$(cat "${TEMP_DIR}/repair_output.txt" | jq -Rs .)"
    fi

    update_report_field "auto_repair_log" "$(cat "$repairs_log")"
}

# Step 7: Create diagnostic bead if issues found
create_diagnostic_bead_if_needed() {
    local summary=$(jq -r '.summary' "$REPORT_FILE")
    local starvation_detected=$(echo "$summary" | jq -r '.starvation_detected')

    if [ "$starvation_detected" = "false" ]; then
        log_info "No starvation detected, skipping diagnostic bead creation"
        return
    fi

    log_step "Creating diagnostic bead..."

    local missing=$(jq -r '.missing_beads_analysis // []' "$REPORT_FILE")
    local missing_count=$(echo "$missing" | jq 'length')

    local bead_description="## Pluck Query Diagnostic Results

**Timestamp:** ${TIMESTAMP}
**Diagnostic Report:** \`${REPORT_FILE##*/}\`

### Summary
- Total beads: $(jq -r '.summary.total_beads' "$REPORT_FILE")
- Open beads: $(jq -r '.summary.open_beads' "$REPORT_FILE")
- Ready candidates: $(jq -r '.summary.ready_candidates' "$REPORT_FILE")
- Missing from ready: $(jq -r '.summary.missing_from_ready' "$REPORT_FILE")

### Issues Found
$(jq -r '.recommendations[]? | "- " + .' "$REPORT_FILE")

### Configuration Check
$(jq -r '.configuration_check | to_entries[] | select(.key != "issues") | "**\(.key):** \(.value | if type == "object" then . else . end)"' "$REPORT_FILE")

### Auto-Repair Status
$(jq -r '.auto_repair_log[]? | "- " + .' "$REPORT_FILE")

### Next Steps
1. Review the full diagnostic report at: \`${REPORT_FILE##*/}\`
2. Check specific bead exclusion reasons in the report
3. Run \`bead doctor\` for additional diagnostics
4. Consider manual intervention if auto-repair did not resolve issues

---

This is an automated diagnostic bead created by \`scripts/pluck-query-diagnostic.sh\`
"

    local bead_title="Pluck query diagnostic - $(jq -r '.summary.open_beads' "$REPORT_FILE") open, $(jq -r '.summary.ready_candidates' "$REPORT_FILE") ready"

    local remediation_bead_id=$(bead create \
        --title "$bead_title" \
        --priority 2 \
        --issue-type task \
        --label automated,diagnostic,pluck-query \
        --description "$bead_description" 2>&1 | head -1 | tr -d ' \t\n\r')

    if [ -n "$remediation_bead_id" ]; then
        log_info "✓ Created diagnostic bead: $remediation_bead_id"

        bead update "$remediation_bead_id" --notes "Diagnostic report: ${REPORT_FILE}"

        jq --arg bead_id "$remediation_bead_id" '.diagnostic_bead = $bead_id' "$REPORT_FILE" > "${TEMP_DIR}/report_with_bead.json"
        mv "${TEMP_DIR}/report_with_bead.json" "$REPORT_FILE"
    else
        log_warn "Failed to create diagnostic bead"
    fi
}

# Main execution
main() {
    echo "=== Pluck Query Diagnostic Tool ==="
    echo "Workspace: ${WORKSPACE_ROOT}"
    echo "Timestamp: ${TIMESTAMP}"
    echo ""

    initialize_report
    run_bead_doctor
    check_configuration
    analyze_pluck_filters

    local missing_beads="${TEMP_DIR}/missing_beads.json"
    analyze_missing_beads "$missing_beads"

    generate_recommendations
    attempt_auto_repair
    create_diagnostic_bead_if_needed

    echo ""
    echo "=== Diagnostic Complete ==="
    echo "Report saved to: $REPORT_FILE"

    # Display summary
    echo ""
    echo "Summary:"
    jq -r '.summary | to_entries[] | "  \(.key): \(.value)"' "$REPORT_FILE"

    local recommendations_count=$(jq '.recommendations | length' "$REPORT_FILE")
    if [ "$recommendations_count" -gt 0 ]; then
        echo ""
        echo "Recommendations:"
        jq -r '.recommendations[]? | "  - "' "$REPORT_FILE"
    fi

    # Create symlink to latest
    LATEST_LINK="${DIAGNOSTICS_DIR}/pluck-query-diagnostic-latest.json"
    ln -sf "$(basename "$REPORT_FILE")" "$LATEST_LINK"
    echo ""
    echo "Latest report: $LATEST_LINK"

    # Exit with error if starvation detected
    local starvation_detected=$(jq -r '.summary.starvation_detected' "$REPORT_FILE")
    if [ "$starvation_detected" = "true" ]; then
        exit 1
    fi

    exit 0
}

main "$@"
