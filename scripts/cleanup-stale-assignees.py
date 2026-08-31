#!/usr/bin/env python3
"""
Automated bead state cleanup script
Identifies and remediates problematic bead states:
- Assigned but not in_progress (should be in_progress or unassigned)
- Assigned to workers that are no longer active
- Closed beads with assignees (bead-rs invariant violation)
"""

import sqlite3
import json
import subprocess
import sys
from datetime import datetime, timezone, timedelta
from pathlib import Path
import os

WORKSPACE_ROOT = Path.cwd()
BEADS_DB = WORKSPACE_ROOT / ".beads" / "beads.db"
HEARTBEATS_FILE = WORKSPACE_ROOT / ".beads" / "heartbeats.jsonl"

# Initialize counters
fixed_closed_assignees = 0
fixed_open_assigned = 0
fixed_in_progress_unassigned = 0
total_problematic = 0

log_entries = []

def log(msg):
    """Add to log and print"""
    print(msg)
    log_entries.append(msg)

def worker_is_active(worker_name, cutoff_hours=24):
    """Check if a worker has recent heartbeats"""
    if not HEARTBEATS_FILE.exists():
        return False
    
    cutoff = datetime.now(timezone.utc) - timedelta(hours=cutoff_hours)
    
    try:
        with open(HEARTBEATS_FILE, 'r') as f:
            for line in f:
                try:
                    heartbeat = json.loads(line)
                    if heartbeat.get('worker') == worker_name:
                        timestamp_str = heartbeat.get('timestamp') or heartbeat.get('time')
                        if timestamp_str:
                            timestamp = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                            if timestamp > cutoff:
                                return True
                except (json.JSONDecodeError, ValueError):
                    continue
    except Exception as e:
        log(f"Warning: Error reading heartbeats: {e}")
    
    return False

def clear_closed_assignee(conn, bead_id, assignee):
    """Clear assignee from a closed bead via direct database update"""
    global fixed_closed_assignees, total_problematic
    
    log(f"Problematic: {bead_id} - closed but has assignee: {assignee}")
    log(f"  → Clearing assignee via database update...")
    
    try:
        cursor = conn.cursor()
        cursor.execute(
            "UPDATE issues SET assignee = NULL WHERE id = ? AND base_status = 'closed'",
            (bead_id,)
        )
        conn.commit()
        
        if cursor.rowcount > 0:
            fixed_closed_assignees += 1
            total_problematic += 1
            log(f"  ✓ Cleared assignee for {bead_id}")
            return True
        else:
            log(f"  ✗ No rows updated for {bead_id}")
            return False
    except Exception as e:
        log(f"  ✗ Database error: {e}")
        return False

def update_open_bead(bead_id, assignee):
    """Update an open bead with assignee - either set to in_progress or clear assignee"""
    global fixed_open_assigned, total_problematic
    
    log(f"Problematic: {bead_id} - open but has assignee: {assignee}")
    total_problematic += 1
    
    # Check if assignee is still active
    if worker_is_active(assignee):
        log(f"  → Assignee is active, updating status to in_progress...")
        try:
            result = subprocess.run(
                ['bead', 'update', bead_id, '--status', 'in_progress'],
                capture_output=True, text=True
            )
            if result.returncode == 0:
                fixed_open_assigned += 1
                log(f"  ✓ Updated {bead_id} to in_progress")
                return True
            else:
                log(f"  ✗ Failed to update {bead_id}: {result.stderr}")
                return False
        except Exception as e:
            log(f"  ✗ Error updating {bead_id}: {e}")
            return False
    else:
        log(f"  → Assignee is inactive, clearing assignee...")
        try:
            result = subprocess.run(
                ['bead', 'update', bead_id, '--clear-assignee'],
                capture_output=True, text=True
            )
            if result.returncode == 0:
                fixed_open_assigned += 1
                log(f"  ✓ Cleared assignee for {bead_id}")
                return True
            else:
                log(f"  ✗ Failed to clear assignee for {bead_id}: {result.stderr}")
                return False
        except Exception as e:
            log(f"  ✗ Error clearing assignee for {bead_id}: {e}")
            return False

def release_in_progress_bead(bead_id):
    """Release an in_progress bead without assignee to open"""
    global fixed_in_progress_unassigned, total_problematic
    
    log(f"Problematic: {bead_id} - in_progress but has no assignee")
    log(f"  → Releasing to open status...")
    total_problematic += 1
    
    try:
        result = subprocess.run(
            ['bead', 'update', bead_id, '--status', 'open'],
            capture_output=True, text=True
        )
        if result.returncode == 0:
            fixed_in_progress_unassigned += 1
            log(f"  ✓ Released {bead_id} to open")
            return True
        else:
            log(f"  ✗ Failed to release {bead_id}: {result.stderr}")
            return False
    except Exception as e:
        log(f"  ✗ Error releasing {bead_id}: {e}")
        return False

def main():
    global total_problematic
    
    timestamp = datetime.now(timezone.utc).isoformat()
    
    log("=" * 80)
    log("BEAD STATE CLEANUP")
    log("=" * 80)
    log(f"Started: {timestamp}")
    log(f"Workspace: {WORKSPACE_ROOT}")
    log("")
    
    # Get all beads
    log("Analyzing workspace bead state...")
    
    try:
        result = subprocess.run(
            ['bead', 'list', '--json'],
            capture_output=True, text=True, check=True
        )
    except subprocess.CalledProcessError as e:
        log(f"Error running bead list: {e}")
        sys.exit(1)
    
    beads = []
    for line in result.stdout.strip().split('\n'):
        if line:
            try:
                beads.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    
    log(f"Total beads analyzed: {len(beads)}")
    
    # Connect to database for closed bead updates
    conn = None
    if BEADS_DB.exists():
        try:
            conn = sqlite3.connect(str(BEADS_DB))
            conn.execute("PRAGMA foreign_keys = OFF")
        except Exception as e:
            log(f"Warning: Could not connect to database: {e}")
            conn = None
    
    # Process each bead
    for bead in beads:
        bead_id = bead.get('id')
        status = bead.get('status')
        assignee = bead.get('assignee')
        
        if not bead_id or not status:
            continue
        
        # Skip beads without assignees
        if not assignee or assignee == "null" or assignee == "":
            continue
        
        # Issue 1: Closed beads should NOT have assignees
        if status == 'closed' and conn:
            clear_closed_assignee(conn, bead_id, assignee)
        
        # Issue 2: Open beads with assignees should be in_progress
        elif status == 'open':
            update_open_bead(bead_id, assignee)
        
        # Issue 3: In-progress beads without assignees (caught in next pass)
        # This will be handled in a second pass
    
    # Second pass: check for in_progress beads without assignees
    if conn:
        try:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT id FROM issues WHERE base_status = 'in_progress' AND (assignee IS NULL OR assignee = '')"
            )
            for row in cursor.fetchall():
                release_in_progress_bead(row[0])
        except Exception as e:
            log(f"Warning: Error querying in_progress beads: {e}")
    
    # Close database connection
    if conn:
        conn.close()
    
    # Print summary
    log("")
    log("=" * 80)
    log("CLEANUP SUMMARY")
    log("=" * 80)
    log(f"Total problematic beads found: {total_problematic}")
    log(f"Fixed closed beads with assignees: {fixed_closed_assignees}")
    log(f"Fixed open beads with assignees: {fixed_open_assigned}")
    log(f"Fixed in_progress beads without assignees: {fixed_in_progress_unassigned}")
    log(f"Completed: {datetime.now(timezone.utc).isoformat()}")
    
    # Generate report file
    report = {
        "cleanup_run": {
            "timestamp": timestamp,
            "workspace": str(WORKSPACE_ROOT)
        },
        "summary": {
            "total_problematic": total_problematic,
            "fixed_closed_with_assignees": fixed_closed_assignees,
            "fixed_open_with_assignees": fixed_open_assigned,
            "fixed_in_progress_no_assignees": fixed_in_progress_unassigned
        },
        "log": log_entries
    }
    
    report_file = Path(f"/tmp/bead-cleanup-report-{int(datetime.now().timestamp())}.json")
    with open(report_file, 'w') as f:
        json.dump(report, f, indent=2)
    
    log("")
    log(f"Report saved to: {report_file}")
    
    sys.exit(min(total_problematic, 1))  # Exit 0 if no issues, 1 if issues found

if __name__ == '__main__':
    main()
