# TUNNEL TUI Package

Terminal Unified Network Node Encrypted Link - TUI Framework

## Overview

This package implements a Terminal User Interface (TUI) for the TUNNEL application using [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss).

## Components

### 1. **app.go** - Main Application
The core TUI application model with:
- Multiple view modes: Dashboard, Browser, Config, Logs, Monitor
- Keyboard navigation and shortcuts
- Window resize handling
- View switching and state management

**Key Features:**
- Tab navigation (1-5 or Tab/Shift+Tab)
- Help overlay (press `?`)
- Graceful quit (press `q` or Ctrl+C)
- Responsive design

### 2. **dashboard.go** - Dashboard View
The main dashboard displaying:
- Active connections with status indicators
- Connection details (IP, upload/download speeds)
- Quick actions menu (navigable with arrow keys or j/k)
- System status panel (SSH server, firewall, container, network)

**Status Indicators:**
- `●` (green) - Connected
- `◐` (yellow) - Ready
- `○` (red) - Stopped

### 3. **browser.go** - Method Browser
Interactive browser for selecting connection methods:
- Categorized methods:
  - VPN/Mesh Networks (Tailscale, WireGuard, ZeroTier, Nebula)
  - Tunnel Services (Cloudflare Tunnel, ngrok, bore)
  - Direct/Traditional (VS Code Tunnels, SSH Port Forward)
- Search functionality (press `/`)
- Recommended methods marked with `★`
- Category navigation (left/right arrows or h/l)
- Method selection (up/down arrows or j/k)

### 4. **styles.go** - Styling System
Lipgloss-based styling with:
- Dark theme color palette
- Consistent component styles (boxes, panels, lists, badges)
- Status colors and indicators
- Reusable style functions

**Color Scheme:**
- Primary: Purple (#7D56F4)
- Success: Green (#10B981)
- Warning: Yellow (#F59E0B)
- Danger: Red (#EF4444)
- Info: Blue (#3B82F6)

### 5. **help.go** - Help System
Context-aware help overlay with:
- Keyboard shortcuts reference
- View-specific help sections
- Categorized help items

## Usage

### Basic Integration

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/jedarden/tunnel/internal/tui"
)

func main() {
    app := tui.NewApp()
    p := tea.NewProgram(app, tea.WithAltScreen())

    if _, err := p.Run(); err != nil {
        // Handle error
    }
}
```

### Keyboard Shortcuts

#### Global
- `1-5` - Switch to specific view
- `Tab` - Next view
- `Shift+Tab` - Previous view
- `?` - Toggle help overlay
- `q` or `Ctrl+C` - Quit application
- `Esc` - Cancel/Go back

#### Dashboard View
- `↑/↓` or `k/j` - Navigate quick actions
- `Enter` - Execute selected action

#### Browser View
- `←/→` or `h/l` - Switch categories
- `↑/↓` or `k/j` - Select method
- `/` - Search methods
- `Enter` - Connect with selected method
- `Esc` - Exit search mode

## Architecture

```
App (Main TUI Model)
├── Dashboard (View)
│   ├── Active Connections
│   ├── Quick Actions
│   └── System Status
├── Browser (View)
│   ├── Category List
│   ├── Method List
│   └── Search
├── Config (Placeholder)
├── Logs (Placeholder)
├── Monitor (Placeholder)
└── Help (Overlay)
```

## Extension Points

### Adding a New View

1. Define the view model in a new file (e.g., `myview.go`)
2. Implement `tea.Model` interface:
   - `Init() tea.Cmd`
   - `Update(tea.Msg) (tea.Model, tea.Cmd)`
   - `View() string`
3. Add view mode to `ViewMode` enum in `app.go`
4. Update `App.View()` to render your view
5. Add keyboard shortcut in `App.Update()`

### Customizing Styles

Edit `styles.go` to:
- Change color palette
- Modify component styles
- Add new reusable styles

## Testing

Build and run the test application:

```bash
# Build test app
go build -o tui-test ./cmd/tui-test/

# Run it
./tui-test
```

## Dependencies

- [bubbletea](https://github.com/charmbracelet/bubbletea) v1.3.10 - TUI framework
- [lipgloss](https://github.com/charmbracelet/lipgloss) v1.1.0 - Style definitions

## File Statistics

- **app.go**: 296 lines - Main application logic
- **browser.go**: 395 lines - Method browser implementation
- **dashboard.go**: 259 lines - Dashboard view
- **help.go**: 175 lines - Help system
- **styles.go**: 237 lines - Styling system
- **Total**: 1,362 lines of code

## Next Steps

1. Integrate with actual connection providers
2. Implement Config, Logs, and Monitor views
3. Add real-time status updates
4. Implement connection management actions
5. Add configuration persistence
6. Enhance search with fuzzy matching
