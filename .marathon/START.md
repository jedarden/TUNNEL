# Marathon Coding - Startup Instructions

## Critical Flags

**IMPORTANT**: Marathon coding sessions MUST use these flags:
- `--dangerously-skip-permissions` - Skip all permission prompts for autonomous execution
- `-p` (headless mode) - Run without TUI for clean logging and background execution

## Quick Start

### 1. Create a new tmux session
```bash
tmux new-session -s tunnel-marathon
```

### 2. Navigate to the tunnel directory
```bash
cd /workspaces/ardenone-cluster/tunnel
```

### 3. Start Claude Code with the mission (HEADLESS + NO PERMISSIONS)
```bash
claude -p --dangerously-skip-permissions "Read .marathon/mission.md and execute the marathon coding mission. Update GitHub issue #1 after each phase. Work autonomously through all 4 phases."
```

Or use the resume flag if continuing:
```bash
claude -p --dangerously-skip-permissions --resume
```

## Recommended: Background Execution

Run in background with logging (HEADLESS MODE):
```bash
tmux new-session -d -s tunnel-marathon -c /workspaces/ardenone-cluster/tunnel \
  'claude -p --dangerously-skip-permissions "Read .marathon/mission.md and execute the marathon coding mission. Update GitHub issue #1 after each phase. Work autonomously through all 4 phases." 2>&1 | tee .marathon/session.log'
```

Attach to monitor:
```bash
tmux attach -t tunnel-marathon
```

## One-Liner Launch

```bash
tmux new-session -d -s tunnel-marathon -c /workspaces/ardenone-cluster/tunnel 'claude -p --dangerously-skip-permissions "Read .marathon/mission.md and execute the marathon coding mission. Update GitHub issue #1 after each phase." 2>&1 | tee .marathon/session.log'
```

## Monitoring Progress

### Check GitHub Issue
```bash
gh issue view 1 --comments
```

### Watch session log (if running in background)
```bash
tail -f /workspaces/ardenone-cluster/tunnel/.marathon/session.log
```

### Watch for file changes
```bash
watch -n 5 'git status --short'
```

### Check test status
```bash
watch -n 30 'go test ./... 2>&1 | tail -20'
```

## Session Management

### Detach from session
Press: `Ctrl+B` then `D`

### List sessions
```bash
tmux list-sessions
```

### Kill session
```bash
tmux kill-session -t tunnel-marathon
```

## Files

- **Mission**: `.marathon/mission.md` - The full mission brief
- **Log**: `.marathon/session.log` - Session output (headless mode logs here)
- **Issue**: https://github.com/jedarden/tunnel/issues/1

## Expected Duration

- Phase 1 (Providers): ~2-3 hours
- Phase 2 (Integration): ~1-2 hours
- Phase 3 (Testing): ~1-2 hours
- Phase 4 (CI/CD): ~30 mins

**Total estimated: 5-8 hours**

## Flag Reference

| Flag | Purpose |
|------|---------|
| `-p` | Headless/print mode - no TUI, outputs to stdout |
| `--dangerously-skip-permissions` | Skip all permission prompts for autonomous execution |
| `--resume` | Resume previous conversation |
| `-c` | Continue most recent conversation |
