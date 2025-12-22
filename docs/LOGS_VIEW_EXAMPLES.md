# Logs View Visual Examples

## Full Mode Example (80x24 terminal)

```
╭────────────────────────────────────────────────────────────────────────────╮
│ TUNNEL                   Terminal Unified Network Node Encrypted Link     │
╰────────────────────────────────────────────────────────────────────────────╯
┌──────────┬──────────┬────────┬────────┬─────────┐
│1.Dashboard│2.Browser│3.Config│4.Logs*│5.Monitor│
└──────────┴──────────┴────────┴────────┴─────────┘
────────────────────────────────────────────────────────────────────────────

System Logs (auto-refresh: 2s ago) • 42 entries

Time                 Level  Provider        Message
────────────────────────────────────────────────────────────────────────────
15:04:05 01/02      INFO   Tailscale       Connection established to 100.64.0.1
15:04:03 01/02      WARN   WireGuard       High latency detected: 250ms
15:04:01 01/02      ERROR  Cloudflare      Tunnel authentication failed
15:03:58 01/02      INFO   Tailscale       Peer added: device-abc123
15:03:55 01/02      INFO   ngrok           Tunnel started on port 8080
15:03:52 01/02      WARN   ZeroTier        Network join pending
15:03:50 01/02      INFO   WireGuard       Interface wg0 configured
15:03:48 01/02      INFO   bore            TCP tunnel established
15:03:45 01/02      ERROR  Cloudflare      Connection timeout
15:03:42 01/02      INFO   Tailscale       Login successful
... 32 more entries (scroll with j/k)

j/k: scroll • g/G: top/bottom • f: filter • c: clear • r: refresh
╰────────────────────────────────────────────────────────────────────────────╯
?: help • 1-5: switch view • q: quit
```

## Compact Mode Example (50x18 terminal)

```
╭────────────────────────────────────────────╮
│ TUNNEL  SSH Tunnel Manager                │
╰────────────────────────────────────────────╯
1:Dash│2:Browse│3:Cfg│4:Log*│5:Mon
────────────────────────────────────────────

Logs
15:04:05 I Tailscal: Connection established
15:04:03 W WireGuar: High latency detected
15:04:01 E Cloudfla: Auth failed
15:03:58 I Tailscal: Peer added
15:03:55 I ngrok: Tunnel started
15:03:52 W ZeroTier: Network join pending
15:03:50 I WireGuar: Interface configured
15:03:48 I bore: TCP tunnel established

j/k:scroll f:filter r:refresh c:clear
╰────────────────────────────────────────────╯
?:help q:quit ↑↓:nav
```

## Tiny Mode Example (35x10 terminal)

```
TUNNEL [D][B][C][L*][M]
● Tailscal,WireGuar,Cloudfla
─────────────────────────────────
Logs
E: Auth failed
W: High latency
I: Connected
─────────────────────────────────
1-5:view ?:help q:quit
```

## Filter Mode Example

### Selecting Filter (Full Mode)

```
╭────────────────────────────────────────────────────────────────────────────╮
│ TUNNEL                   Terminal Unified Network Node Encrypted Link     │
╰────────────────────────────────────────────────────────────────────────────╯
┌──────────┬──────────┬────────┬────────┬─────────┐
│1.Dashboard│2.Browser│3.Config│4.Logs*│5.Monitor│
└──────────┴──────────┴────────┴────────┴─────────┘
────────────────────────────────────────────────────────────────────────────

Select Filter

  error (level)
→ info (level)
  warn (level)
  bore (provider)
  Cloudflare (provider)
  ngrok (provider)
  Tailscale (provider)
  WireGuard (provider)
  ZeroTier (provider)

↑/↓: navigate, Enter: apply, Esc: cancel, x: clear filter

╰────────────────────────────────────────────────────────────────────────────╯
?: help • 1-5: switch view • q: quit
```

### Active Filter Display (Error Logs Only)

```
╭────────────────────────────────────────────────────────────────────────────╮
│ TUNNEL                   Terminal Unified Network Node Encrypted Link     │
╰────────────────────────────────────────────────────────────────────────────╯
┌──────────┬──────────┬────────┬────────┬─────────┐
│1.Dashboard│2.Browser│3.Config│4.Logs*│5.Monitor│
└──────────┴──────────┴────────┴────────┴─────────┘
────────────────────────────────────────────────────────────────────────────

System Logs (auto-refresh: 1s ago) • 8 entries
Filter: error

Time                 Level  Provider        Message
────────────────────────────────────────────────────────────────────────────
15:04:01 01/02      ERROR  Cloudflare      Tunnel authentication failed
15:03:45 01/02      ERROR  Cloudflare      Connection timeout
15:03:12 01/02      ERROR  ngrok           Rate limit exceeded
15:02:58 01/02      ERROR  WireGuard       Handshake failed
15:02:33 01/02      ERROR  Tailscale       Network unreachable
15:01:45 01/02      ERROR  Cloudflare      TLS verification failed
15:01:20 01/02      ERROR  bore            Port already in use
15:00:55 01/02      ERROR  ZeroTier        Authorization denied

j/k: scroll • g/G: top/bottom • f: filter • c: clear • r: refresh
╰────────────────────────────────────────────────────────────────────────────╯
?: help • 1-5: switch view • q: quit
```

### Active Filter Display (Provider: Tailscale)

```
╭────────────────────────────────────────────────────────────────────────────╮
│ TUNNEL                   Terminal Unified Network Node Encrypted Link     │
╰────────────────────────────────────────────────────────────────────────────╯
┌──────────┬──────────┬────────┬────────┬─────────┐
│1.Dashboard│2.Browser│3.Config│4.Logs*│5.Monitor│
└──────────┴──────────┴────────┴────────┴─────────┘
────────────────────────────────────────────────────────────────────────────

System Logs (auto-refresh: 0s ago) • 15 entries
Filter: Tailscale

Time                 Level  Provider        Message
────────────────────────────────────────────────────────────────────────────
15:04:05 01/02      INFO   Tailscale       Connection established to 100.64.0.1
15:03:58 01/02      INFO   Tailscale       Peer added: device-abc123
15:03:42 01/02      INFO   Tailscale       Login successful
15:03:15 01/02      INFO   Tailscale       Starting tailscaled
15:02:58 01/02      INFO   Tailscale       Interface tailscale0 configured
15:02:45 01/02      WARN   Tailscale       DERP connection latency high
15:02:33 01/02      ERROR  Tailscale       Network unreachable
15:02:20 01/02      INFO   Tailscale       Received ping from peer
15:02:05 01/02      INFO   Tailscale       Route update: 100.64.0.0/10
15:01:50 01/02      INFO   Tailscale       Exit node enabled
15:01:35 01/02      WARN   Tailscale       MagicDNS resolution slow
15:01:20 01/02      INFO   Tailscale       Accept routes enabled
15:01:05 01/02      INFO   Tailscale       Connected to control server
15:00:50 01/02      INFO   Tailscale       Checking for updates
15:00:35 01/02      INFO   Tailscale       Starting connection

j/k: scroll • g/G: top/bottom • f: filter • c: clear • r: refresh
╰────────────────────────────────────────────────────────────────────────────╯
?: help • 1-5: switch view • q: quit
```

## Empty State Example

```
╭────────────────────────────────────────────────────────────────────────────╮
│ TUNNEL                   Terminal Unified Network Node Encrypted Link     │
╰────────────────────────────────────────────────────────────────────────────╯
┌──────────┬──────────┬────────┬────────┬─────────┐
│1.Dashboard│2.Browser│3.Config│4.Logs*│5.Monitor│
└──────────┴──────────┴────────┴────────┴─────────┘
────────────────────────────────────────────────────────────────────────────

System Logs (auto-refresh: 3s ago) • 0 entries

Time                 Level  Provider        Message
────────────────────────────────────────────────────────────────────────────
No log entries





j/k: scroll • g/G: top/bottom • f: filter • c: clear • r: refresh
╰────────────────────────────────────────────────────────────────────────────╯
?: help • 1-5: switch view • q: quit
```

## Color Scheme Reference

### Log Levels (Actual Colors)

```
INFO   → Green  (#10B981) ● Healthy, normal operations
WARN   → Yellow (#F59E0B) ● Warnings, non-critical issues
ERROR  → Red    (#EF4444) ● Errors, critical failures
```

### UI Elements

```
Title          → Purple (#7D56F4) TUNNEL
Selected Tab   → Purple (#7D56F4) 4.Logs*
Inactive Tab   → Gray   (#6B7280) 1.Dashboard
Timestamp      → Gray   (#6B7280) 15:04:05 01/02
Provider       → White  (#E5E7EB) Tailscale
Selected Item  → Purple (#7D56F4) → info (level)
Border         → Gray   (#4B5563) ────────
Help Text      → Gray   (#6B7280) j/k: scroll
```

## Terminal Adaptation

### Size Detection

```go
// Full Mode: width >= 60, height >= 20
┌─────────────────────────────────────────────┐
│ Full table with headers, complete help text │
│ Maximum information density                 │
└─────────────────────────────────────────────┘

// Compact Mode: width < 60 OR height < 20
┌──────────────────────────────┐
│ Single column layout         │
│ Abbreviated text             │
│ Essential info only          │
└──────────────────────────────┘

// Tiny Mode: width < 40 OR height < 12
┌─────────────────┐
│ Minimal display │
│ Status only     │
└─────────────────┘
```

## Interaction Examples

### Scrolling Down Through Logs

```
Initial View (top):
────────────────────────────────────────────────────────────────────────────
15:04:05 01/02      INFO   Tailscale       Connection established
15:04:03 01/02      WARN   WireGuard       High latency detected
15:04:01 01/02      ERROR  Cloudflare      Auth failed
↓ Press 'j' or down arrow

After Scrolling:
────────────────────────────────────────────────────────────────────────────
15:04:03 01/02      WARN   WireGuard       High latency detected
15:04:01 01/02      ERROR  Cloudflare      Auth failed
15:03:58 01/02      INFO   Tailscale       Peer added
↓ Press 'j' or down arrow

Continue Scrolling:
────────────────────────────────────────────────────────────────────────────
15:04:01 01/02      ERROR  Cloudflare      Auth failed
15:03:58 01/02      INFO   Tailscale       Peer added
15:03:55 01/02      INFO   ngrok           Tunnel started
```

### Jump to Top/Bottom

```
Current Position (middle):
15:03:15 01/02      INFO   Tailscale       Starting tailscaled
15:03:12 01/02      ERROR  ngrok           Rate limit exceeded

Press 'G' (jump to bottom):
15:00:35 01/02      INFO   Tailscale       Starting connection
15:00:20 01/02      INFO   bore            Service initialized

Press 'g' (jump to top):
15:04:05 01/02      INFO   Tailscale       Connection established
15:04:03 01/02      WARN   WireGuard       High latency detected
```

### Auto-Refresh Indicator

```
Time 0s:  (auto-refresh: 0s ago) • 42 entries
Time 1s:  (auto-refresh: 1s ago) • 42 entries
Time 2s:  (auto-refresh: 2s ago) • 42 entries
Time 3s:  (auto-refresh: 0s ago) • 45 entries  ← Refreshed, new entries!
```

## Real-World Usage Scenarios

### Scenario 1: Debugging Connection Issues

1. Switch to Logs view (press `4`)
2. Filter by ERROR level (press `f`, select "error", press Enter)
3. Identify failing provider (e.g., "Cloudflare: Auth failed")
4. Switch to Browser view (press `2`)
5. Reconfigure Cloudflare provider

### Scenario 2: Monitoring System Health

1. Open Logs view (press `4`)
2. Watch auto-refresh for new entries
3. Look for WARNING or ERROR indicators
4. Jump to bottom (press `G`) to see latest
5. Filter by specific provider if needed

### Scenario 3: Tracking Specific Provider

1. Navigate to Logs (press `4`)
2. Open filter (press `f`)
3. Select provider name (e.g., "Tailscale")
4. Press Enter to apply
5. View all Tailscale-specific logs
6. Clear filter (press `f`, then `x`) when done

### Scenario 4: Quick Status Check

1. Switch to Logs (press `4`)
2. Glance at recent entries
3. Look for color indicators:
   - Green (INFO) = Normal
   - Yellow (WARN) = Investigate
   - Red (ERROR) = Action needed
4. Return to Dashboard (press `1`)
