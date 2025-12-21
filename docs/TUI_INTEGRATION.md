# TUI Integration Guide

This guide shows how to integrate the TUNNEL TUI into the main CLI application.

## Quick Integration

Update `/workspaces/ardenone-cluster/tunnel/cmd/tunnel/cli.go` to launch the TUI:

```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/jedarden/tunnel/internal/tui"
)

func launchTUI(ctx context.Context) error {
    if verbose {
        fmt.Println("Launching TUI...")
    }

    // Create and configure TUI app
    app := tui.NewApp()

    // Create Bubbletea program with alternate screen
    p := tea.NewProgram(
        app,
        tea.WithAltScreen(),           // Use alternate screen buffer
        tea.WithMouseCellMotion(),      // Enable mouse support (optional)
    )

    // Run the TUI
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("TUI error: %w", err)
    }

    return nil
}
```

## Testing the TUI

### Option 1: Standalone Test Application

```bash
cd /workspaces/ardenone-cluster/tunnel
go run ./cmd/tui-test/
```

### Option 2: Main CLI (after integration)

```bash
cd /workspaces/ardenone-cluster/tunnel
go run ./cmd/tunnel/
```

## Building the Application

```bash
# Build the main tunnel binary
go build -o tunnel ./cmd/tunnel/

# Run it
./tunnel
```

## Advanced Configuration

### Custom Initialization

```go
func launchTUI(ctx context.Context) error {
    app := tui.NewApp()

    // Access and configure sub-models if needed
    // app.SetConnectionData(connections)
    // app.SetSystemStatus(status)

    p := tea.NewProgram(
        app,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    // Handle context cancellation
    go func() {
        <-ctx.Done()
        p.Send(tea.Quit())
    }()

    _, err := p.Run()
    return err
}
```

### Passing Data to TUI

To pass real connection data to the dashboard:

1. Add methods to update dashboard data:

```go
// In internal/tui/app.go
func (a *App) UpdateConnections(connections []Connection) {
    a.dashboard.connections = connections
}

func (a *App) UpdateSystemStatus(status []SystemStatus) {
    a.dashboard.systemStatus = status
}
```

2. Call from CLI before launching:

```go
func launchTUI(ctx context.Context) error {
    app := tui.NewApp()

    // Fetch and set real data
    connections := fetchActiveConnections()
    app.UpdateConnections(connections)

    systemStatus := getSystemStatus()
    app.UpdateSystemStatus(systemStatus)

    p := tea.NewProgram(app, tea.WithAltScreen())
    _, err := p.Run()
    return err
}
```

### Real-time Updates

For live data updates, use Bubbletea's message system:

```go
// Define update message
type ConnectionUpdateMsg struct {
    Connections []Connection
}

// In dashboard.go Update method
func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case ConnectionUpdateMsg:
        d.connections = msg.Connections
        return d, nil
    // ... other cases
    }
    return d, nil
}

// Create ticker command
func tickEvery(duration time.Duration) tea.Cmd {
    return tea.Tick(duration, func(t time.Time) tea.Msg {
        // Fetch latest connection data
        connections := fetchActiveConnections()
        return ConnectionUpdateMsg{Connections: connections}
    })
}
```

## Dependencies

Make sure these are in your `go.mod`:

```go
require (
    github.com/charmbracelet/bubbletea v1.3.10
    github.com/charmbracelet/lipgloss v1.1.0
    // ... other dependencies
)
```

Install with:

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
```

## Troubleshooting

### Terminal Issues

If the TUI doesn't render correctly:

```go
// Add more terminal options
p := tea.NewProgram(
    app,
    tea.WithAltScreen(),
    tea.WithMouseCellMotion(),
    tea.WithoutSignalHandler(), // If needed
)
```

### Debugging

For development, you can log to a file since stdout is used by the TUI:

```go
import "log"

// At program start
logFile, _ := os.Create("/tmp/tunnel-tui.log")
log.SetOutput(logFile)
defer logFile.Close()

// Then use log.Println() for debugging
log.Println("Debug message")
```

### Graceful Shutdown

Handle signals properly:

```go
func launchTUI(ctx context.Context) error {
    app := tui.NewApp()
    p := tea.NewProgram(app, tea.WithAltScreen())

    // Handle context cancellation
    go func() {
        <-ctx.Done()
        p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
    }()

    _, err := p.Run()
    return err
}
```

## Production Checklist

- [ ] Import TUI package in CLI
- [ ] Update `launchTUI()` function
- [ ] Test keyboard navigation
- [ ] Test window resizing
- [ ] Verify help overlay works
- [ ] Test all view switches
- [ ] Add real connection data
- [ ] Implement live updates
- [ ] Handle errors gracefully
- [ ] Test on target terminals
- [ ] Add configuration options
- [ ] Document user-facing shortcuts

## Example: Complete Integration

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/jedarden/tunnel/internal/tui"
)

func launchTUI(ctx context.Context) error {
    if verbose {
        fmt.Fprintln(os.Stderr, "Launching TUNNEL TUI...")
    }

    // Create TUI application
    app := tui.NewApp()

    // TODO: Load real connection data
    // connections := loadConnections()
    // app.UpdateConnections(connections)

    // Create Bubbletea program
    p := tea.NewProgram(
        app,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    // Handle graceful shutdown
    go func() {
        <-ctx.Done()
        p.Send(tea.Quit())
    }()

    // Start auto-refresh (optional)
    // go func() {
    //     ticker := time.NewTicker(time.Second * 2)
    //     defer ticker.Stop()
    //     for {
    //         select {
    //         case <-ticker.C:
    //             connections := loadConnections()
    //             p.Send(tui.ConnectionUpdateMsg{Connections: connections})
    //         case <-ctx.Done():
    //             return
    //         }
    //     }
    // }()

    // Run TUI
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("TUI error: %w", err)
    }

    return nil
}
```

## Next Steps

1. Copy the integration code to `cmd/tunnel/cli.go`
2. Test the basic TUI launch
3. Integrate with real data sources
4. Add message handlers for live updates
5. Implement connection actions (start/stop)
6. Add configuration persistence
