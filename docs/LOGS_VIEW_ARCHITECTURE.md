# Logs View Architecture

## System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         TUI Application                          │
│                           (app.go)                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Views:                                                          │
│  ┌──────────┐  ┌─────────┐  ┌────────┐  ┌──────┐  ┌─────────┐ │
│  │Dashboard │  │ Browser │  │ Config │  │ LOGS │  │ Monitor │ │
│  └──────────┘  └─────────┘  └────────┘  └──────┘  └─────────┘ │
│                                             ▲                    │
└─────────────────────────────────────────────┼────────────────────┘
                                              │
                                              │ registry
                                              │
                    ┌─────────────────────────┴──────────────────┐
                    │        Provider Registry                   │
                    │         (registry.go)                      │
                    └─────────────────────────┬──────────────────┘
                                              │
                        ┌─────────────────────┼─────────────────────┐
                        │                     │                     │
                        ▼                     ▼                     ▼
                  ┌──────────┐          ┌──────────┐          ┌──────────┐
                  │Tailscale │          │WireGuard │          │Cloudflare│
                  │ Provider │          │ Provider │          │ Provider │
                  └────┬─────┘          └────┬─────┘          └────┬─────┘
                       │                     │                     │
                       │ GetLogs()           │ GetLogs()           │ GetLogs()
                       │                     │                     │
                       ▼                     ▼                     ▼
                  ┌──────────┐          ┌──────────┐          ┌──────────┐
                  │[]LogEntry│          │[]LogEntry│          │[]LogEntry│
                  └──────────┘          └──────────┘          └──────────┘
                       │                     │                     │
                       └─────────────────────┴─────────────────────┘
                                              │
                                              ▼
                                    ┌──────────────────┐
                                    │  Log Aggregator  │
                                    │   (logs.go)      │
                                    └──────────────────┘
                                              │
                                              ▼
                                   ┌────────────────────┐
                                   │ []AggregatedEntry  │
                                   │ - Timestamp        │
                                   │ - Level            │
                                   │ - Provider         │
                                   │ - Message          │
                                   └────────────────────┘
                                              │
                        ┌─────────────────────┼─────────────────────┐
                        │                     │                     │
                        ▼                     ▼                     ▼
                  ┌──────────┐          ┌──────────┐          ┌──────────┐
                  │  Filter  │          │  Scroll  │          │  Display │
                  │  Engine  │          │  Engine  │          │  Renderer│
                  └──────────┘          └──────────┘          └──────────┘
                        │                     │                     │
                        └─────────────────────┴─────────────────────┘
                                              │
                                              ▼
                                        ┌──────────┐
                                        │ Terminal │
                                        └──────────┘
```

## Component Interactions

### 1. Log Fetching Flow
```
User presses '4' or auto-refresh triggers
        │
        ▼
┌───────────────────┐
│  Logs.Update()    │ ◄── TickMsg (every 3s)
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ refreshLogs()     │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ registry.List     │
│   Providers()     │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ For each provider:│
│ provider.GetLogs()│
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Aggregate & Sort  │
│  by Timestamp     │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Update l.logs[]   │
└───────────────────┘
```

### 2. Filter Flow
```
User presses 'f'
        │
        ▼
┌───────────────────────┐
│ enterFilterMode()     │
│ - Build filter list   │
│ - Show filter UI      │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ User navigates with   │
│ j/k and selects Enter │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ applyFilters()        │
│ - Filter by level OR  │
│ - Filter by provider  │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ getVisibleLogs()      │
│ Returns filtered list │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ View() renders        │
│ filtered logs         │
└───────────────────────┘
```

### 3. Scrolling Flow
```
User presses 'j' or 'k'
        │
        ▼
┌───────────────────────┐
│ Update scrollOffset   │
│ Check bounds:         │
│ 0 <= offset <= max    │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ getVisibleLogs()      │
│ Returns logs[offset:] │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ View() renders only   │
│ visible portion       │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────┐
│ Show scroll indicator │
│ "... N more entries"  │
└───────────────────────┘
```

## State Management

### Logs View State
```go
type Logs struct {
    // Data
    logs []AggregatedLogEntry  // All aggregated logs

    // UI State
    scrollOffset     int        // Current scroll position
    width, height    int        // Terminal dimensions

    // Filter State
    filterMode       FilterMode // None/ByLevel/ByProvider
    selectedFilter   int        // Currently selected filter index
    availableFilters []string   // Dynamic filter list
    activeFilter     string     // Currently applied filter

    // Refresh State
    lastRefresh      time.Time  // Last refresh timestamp
    autoRefresh      bool       // Auto-refresh enabled

    // Dependencies
    registry         *Registry  // Provider registry
}
```

## Message Types

### Input Messages
```go
tea.KeyMsg          // Keyboard input
tea.WindowSizeMsg   // Terminal resize
TickMsg             // Auto-refresh timer (3s)
```

### Key Bindings
```go
// Navigation
"j", "down"   → Scroll down
"k", "up"     → Scroll up
"g"           → Jump to top
"G"           → Jump to bottom

// Actions
"f"           → Enter filter mode
"c"           → Clear logs
"r"           → Manual refresh

// Filter Mode
"up", "down"  → Navigate filters
"enter"       → Apply filter
"x"           → Clear filter
"esc"         → Exit filter mode
```

## Display Modes

### Full Mode (≥ 60x20)
```
┌────────────────────────────────────────────────────────┐
│ System Logs (auto-refresh: 2s ago) • 42 entries       │
├────────────────────────────────────────────────────────┤
│ Time                Level  Provider    Message         │
├────────────────────────────────────────────────────────┤
│ 15:04:05 01/02     INFO   Tailscale   Connected       │
│ 15:04:03 01/02     WARN   WireGuard   High latency    │
│ 15:04:01 01/02     ERROR  Cloudflare  Auth failed     │
│ ...                                                    │
├────────────────────────────────────────────────────────┤
│ j/k: scroll • g/G: top/bottom • f: filter • c: clear  │
└────────────────────────────────────────────────────────┘
```

### Compact Mode (< 60x20)
```
┌──────────────────────────────────────┐
│ Logs [error]                         │
├──────────────────────────────────────┤
│ 15:04 E Cloudfla: Auth failed        │
│ 15:03 W WireGuar: High latency       │
│ 15:01 I Tailscal: Connected          │
├──────────────────────────────────────┤
│ j/k:scroll f:filter r:refresh        │
└──────────────────────────────────────┘
```

### Tiny Mode (< 40x12)
```
┌──────────────────────────┐
│ Logs                     │
│ E: Auth failed           │
│ W: High latency          │
│ I: Connected             │
└──────────────────────────┘
```

## Auto-Refresh Mechanism

```
Init()
  │
  └─► tickCmd()
        │
        └─► tea.Tick(3s)
              │
              └─► TickMsg
                    │
                    ▼
              Update(TickMsg)
                    │
                    ├─► refreshLogs()
                    │     │
                    │     └─► Fetch from all providers
                    │
                    └─► tickCmd() (restart timer)
                          │
                          └─► [Loop continues]
```

## Filter System

### Filter Types
1. **By Level**: info, warn, error
2. **By Provider**: Dynamic list of provider names with logs

### Filter Application
```go
func (l *Logs) applyFilters() []AggregatedLogEntry {
    if l.activeFilter == "" {
        return l.logs  // No filter
    }

    filtered := []AggregatedLogEntry{}
    for _, entry := range l.logs {
        // Match by level
        if string(entry.Level) == strings.ToLower(l.activeFilter) {
            filtered = append(filtered, entry)
        }
        // Match by provider
        if entry.Provider == l.activeFilter {
            filtered = append(filtered, entry)
        }
    }
    return filtered
}
```

## Performance Characteristics

### Time Complexity
- Log aggregation: O(n × p) where n=logs per provider, p=provider count
- Sorting: O(n log n) where n=total log entries
- Filtering: O(n) single pass
- Scrolling: O(1) offset-based
- Rendering: O(h) where h=terminal height

### Space Complexity
- O(n) for storing aggregated logs
- O(n) for filtered view (worst case, same as logs)
- O(1) for UI state

### Optimizations
1. Fetch only last 1 hour (configurable)
2. Provider limit: 100 entries max
3. In-memory filtering (no re-fetch)
4. Offset-based scrolling (no copying)
5. Lazy rendering (only visible entries)

## Integration with Existing Components

### Uses From Other Components
- `TickMsg` type from `monitor.go`
- Styles from `styles.go` (ColorSuccess, ColorWarning, etc.)
- Layout helpers: `IsCompact()`, `IsTiny()`
- Pattern from `dashboard.go` and `browser.go`

### Provides To App
- `NewLogs(reg)` constructor
- `SetSize(w, h)` for terminal resize
- `Update(msg)` for message handling
- `View()` for rendering
- `Init()` for initialization

## Error Handling

```go
// Graceful degradation
logs, err := provider.GetLogs(since)
if err != nil {
    // Skip this provider, continue with others
    continue
}

// Empty logs are valid
if len(l.logs) == 0 {
    content.WriteString(InfoStyle.Render("No log entries"))
}
```

## Thread Safety

- Uses registry with RWMutex internally
- Update() called from single Bubble Tea event loop
- No concurrent access to Logs state
- Safe by design (Bubble Tea guarantees)
