# TUI Logs View

The Logs view provides a unified interface for viewing and filtering logs from all connected network providers in the tunnel application.

## Features

### Log Aggregation
- Automatically fetches logs from all registered providers
- Aggregates logs from multiple sources into a single view
- Sorts logs by timestamp (newest first)
- Displays logs from the last hour by default

### Log Display
- **Timestamp**: Shows when the log entry was created (format: HH:MM:SS MM/DD)
- **Level**: Color-coded severity level
  - INFO (green): Informational messages
  - WARN (yellow): Warning messages
  - ERROR (red): Error and critical messages
- **Provider**: Name of the provider that generated the log
- **Message**: The log message content

### Terminal Modes
The view adapts to different terminal sizes:

#### Full Mode (>= 60x20)
- Table layout with full headers
- Full timestamp format
- Expanded help text
- Scroll indicator showing remaining entries

#### Compact Mode (< 60x20)
- Single-column layout
- Abbreviated display
- Essential information only

#### Tiny Mode (< 40x12)
- Minimal display
- Short format timestamps
- Abbreviated provider names (max 8 chars)

### Navigation

#### Scrolling
- `j` or `↓`: Scroll down one line
- `k` or `↑`: Scroll up one line
- `g`: Jump to top of logs
- `G`: Jump to bottom of logs

#### Filtering
- `f`: Enter filter mode
  - Filter by log level (info, warn, error)
  - Filter by provider name
  - Navigate with `↑`/`↓` or `j`/`k`
  - Press `Enter` to apply filter
  - Press `x` to clear current filter
  - Press `Esc` to cancel filter selection

#### Management
- `r`: Manually refresh logs (also scrolls to bottom)
- `c`: Clear all displayed logs
- Auto-refresh: Logs refresh automatically every 3 seconds

## Implementation Details

### Log Fetching
```go
// Logs are fetched from all providers using the Provider.GetLogs() method
logs, err := provider.GetLogs(since)
```

### Log Structure
```go
type AggregatedLogEntry struct {
    Timestamp    time.Time
    Level        LogLevel  // info, warn, error
    Provider     string    // Provider name
    Message      string    // Log message
    OriginalLog  providers.LogEntry
}
```

### Auto-Refresh
The view implements automatic refresh using the Bubble Tea tick mechanism:
```go
func (l *Logs) tickCmd() tea.Cmd {
    return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
        return TickMsg(t)
    })
}
```

### Filter System
Available filters are dynamically generated based on:
1. Log levels present in current logs (info, warn, error)
2. Providers that have generated logs

Filters are applied in real-time and affect scrolling behavior.

## Integration

### Registry Integration
The Logs view integrates with the provider registry to:
- Discover all registered providers
- Fetch logs from each provider
- Display provider names alongside log entries

### App Integration
The Logs view is integrated into the main TUI app:
```go
// In app.go
logs: NewLogs(reg)

// Pass messages to logs view
case ViewLogs:
    updatedLogs, cmd := a.logs.Update(msg)
    a.logs = updatedLogs.(*Logs)
    cmds = append(cmds, cmd)

// Render logs view
case ViewLogs:
    content = a.logs.View()
```

## Usage Examples

### Viewing All Logs
1. Press `4` to switch to Logs view
2. Use `j`/`k` to scroll through entries
3. Press `g` to go to top, `G` to go to bottom

### Filtering by Level
1. Press `f` to enter filter mode
2. Navigate to "error" using arrow keys
3. Press `Enter` to show only error logs
4. Press `f` then `x` to clear filter

### Filtering by Provider
1. Press `f` to enter filter mode
2. Navigate to a provider name (e.g., "Tailscale")
3. Press `Enter` to show only logs from that provider
4. Press `f` then `x` to clear filter

### Monitoring Live Logs
1. Press `4` to switch to Logs view
2. Press `G` to scroll to bottom
3. Logs will auto-refresh every 3 seconds
4. New entries appear at the top automatically

## Color Coding

- **INFO**: Green (`#10B981`)
- **WARN**: Yellow (`#F59E0B`)
- **ERROR**: Red (`#EF4444`)
- **Provider names**: Default text color
- **Timestamps**: Muted gray
- **Selected filter**: Primary purple with bold

## Performance Considerations

- Logs are fetched only from the last hour to prevent memory issues
- Maximum of 100 log entries per provider (configurable in provider implementation)
- Filters are applied in-memory for instant response
- Auto-refresh uses efficient goroutine-based ticking

## Future Enhancements

Potential improvements for the Logs view:
- [ ] Export logs to file
- [ ] Search within log messages
- [ ] Configurable time range
- [ ] Log streaming with tail -f behavior
- [ ] Syntax highlighting for structured logs (JSON)
- [ ] Copy log entry to clipboard
- [ ] Detailed log entry view with full metadata
