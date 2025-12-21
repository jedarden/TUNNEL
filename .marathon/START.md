# Marathon Coding - Startup Instructions

## Quick Start

### 1. Create a new tmux session
```bash
tmux new-session -s tunnel-marathon
```

### 2. Navigate to the tunnel directory
```bash
cd /workspaces/ardenone-cluster/tunnel
```

### 3. Start Claude Code with the mission
```bash
claude --print ".marathon/mission.md" "Execute this marathon coding mission. Update GitHub issue #1 with progress after each phase. Work through all phases systematically."
```

Or use the resume flag if continuing:
```bash
claude --resume
```

## Alternative: Background Execution

Run in background with logging:
```bash
tmux new-session -d -s tunnel-marathon "claude --print .marathon/mission.md 'Execute this marathon coding mission. Update GitHub issue #1 with progress.' 2>&1 | tee .marathon/session.log"
```

Attach to monitor:
```bash
tmux attach -t tunnel-marathon
```

## Monitoring Progress

### Check GitHub Issue
```bash
gh issue view 1 --comments
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
- **Log**: `.marathon/session.log` - Session output (if using background mode)
- **Issue**: https://github.com/jedarden/tunnel/issues/1

## Expected Duration

- Phase 1 (Providers): ~2-3 hours
- Phase 2 (Integration): ~1-2 hours  
- Phase 3 (Testing): ~1-2 hours
- Phase 4 (CI/CD): ~30 mins

**Total estimated: 5-8 hours**
