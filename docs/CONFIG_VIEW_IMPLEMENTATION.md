# Config View Implementation

## Overview
The Config view has been implemented for the tunnel TUI application, allowing users to view and edit configuration settings through an interactive terminal interface.

## Files Modified

### 1. `/workspaces/ardenone-cluster/tunnel/internal/tui/config.go` (NEW)
**Purpose**: Main Config view implementation

**Key Components**:
- `Config` struct: Main model implementing tea.Model interface
- `ConfigSection` enum: Defines different configuration sections
- `configSection` struct: Represents a section with fields
- `configField` struct: Represents individual editable fields

**Configuration Sections**:
1. **General**: Default method, auto-reconnect, log level, theme
2. **Failover**: Enabled, health check interval, failure threshold, recovery threshold, max latency, auto-recover
3. **Metrics**: Enabled, collection interval
4. **SSH**: Port, max sessions, idle timeout, keep alive, TCP forwarding, agent forwarding
5. **Monitoring**: Enabled, metrics enabled, metrics port, syslog
6. **Providers**: Read-only list of configured providers

**Features**:
- Navigation between sections using Tab/Shift+Tab or arrow keys (←/→)
- Navigation between fields using arrow keys (↑/↓)
- Edit mode activated with Enter key
- Real-time field editing with validation
- Save confirmation dialog (press 's')
- Reload configuration (press 'r')
- Support for different field types: string, int, bool, duration
- Compact and tiny terminal mode support
- Visual feedback for selected/edited fields

**Keyboard Controls**:
- `↑/↓` or `j/k`: Navigate between fields
- `←/→` or `h/l`: Navigate between sections
- `Tab`: Next section
- `Shift+Tab`: Previous section
- `Enter`: Edit selected field
- `Esc`: Cancel editing
- `s`: Save configuration
- `r`: Reload configuration
- `1-5`: Switch to other views
- `q`: Quit application

**Display Modes**:
1. **Normal Mode**: Full layout with section tabs, field descriptions, and help text
2. **Compact Mode**: Single-column layout for terminals < 60x20
3. **Tiny Mode**: Minimal view for very small terminals < 40x12

### 2. `/workspaces/ardenone-cluster/tunnel/internal/tui/app.go` (MODIFIED)
**Changes**:
- Added `config.Config` import
- Added `appConfig *config.Config` field to App struct
- Added `config *Config` sub-model field
- Updated `NewApp()` to accept `cfg *config.Config` parameter
- Initialize Config view with `NewConfig(cfg, mgrConfig)`
- Added config view to window resize handling
- Added config view to Update() message routing
- Changed ViewConfig case in View() to render actual config view instead of placeholder

### 3. `/workspaces/ardenone-cluster/tunnel/cmd/tui-test/main.go` (MODIFIED)
**Changes**:
- Added `config` package import
- Load default config with `config.GetDefaultConfig()`
- Pass config to `tui.NewApp(reg, mgr, cfg)`

### 4. `/workspaces/ardenone-cluster/tunnel/cmd/tunnel/cli.go` (MODIFIED)
**Changes**:
- Updated `launchTUI()` to pass `appConfig` to `tui.NewApp(reg, manager, appConfig)`
- The `appConfig` variable was already available in the global scope

## Integration Points

### Configuration Reading
The Config view reads from two sources:
1. **pkg/config/config.go**: Application configuration (Settings, SSH, Monitoring, Methods)
2. **internal/core/manager.go**: Manager configuration (Failover, Metrics)

### Configuration Writing
- Changes are held in memory until saved
- Pressing 's' shows a save confirmation dialog
- Confirmation saves to the config file via `appConfig.Save()`
- Field changes are validated before applying

### Validation
Each field has an `OnChange` callback that:
- Validates the input value
- Converts to the appropriate type
- Updates the configuration struct
- Returns error if validation fails

## Configuration Structure

### General Settings
```go
Settings {
    DefaultMethod string
    AutoReconnect bool
    LogLevel      string
    Theme         string
}
```

### Failover Settings
```go
FailoverConfig {
    Enabled             bool
    HealthCheckInterval time.Duration
    FailureThreshold    int
    RecoveryThreshold   int
    MaxLatency          time.Duration
    AutoRecover         bool
}
```

### Metrics Settings
```go
ManagerConfig {
    EnableMetrics   bool
    MetricsInterval time.Duration
}
```

### SSH Settings
```go
SSHConfig {
    Port                 int
    MaxSessions          int
    IdleTimeout          int (seconds)
    KeepAlive            int (seconds)
    AllowTCPForwarding   bool
    AllowAgentForwarding bool
}
```

### Monitoring Settings
```go
MonitoringConfig {
    Enabled        bool
    MetricsEnabled bool
    MetricsPort    int
    Syslog         bool
}
```

## Helper Functions

### `formatBool(b bool) string`
Converts boolean to "true"/"false" string for display

### `parseBool(s string) (bool, error)`
Parses various boolean representations:
- true: "true", "yes", "y", "1", "on"
- false: "false", "no", "n", "0", "off"

## Future Enhancements

### Potential Improvements
1. **Provider Configuration**: Make provider settings editable
2. **Advanced Validation**: Add regex patterns, min/max values
3. **Config File Selection**: Allow choosing different config files
4. **Import/Export**: Support importing/exporting config presets
5. **Diff View**: Show changes before saving
6. **Undo/Redo**: Support reverting changes
7. **Search**: Filter fields by name
8. **Themes**: Apply theme changes in real-time
9. **Tooltips**: Show more detailed help for each field
10. **Keyboard Shortcuts**: Custom keybindings configuration

## Testing

### Manual Testing
1. Launch TUI: `./bin/tunnel tui`
2. Press `3` to switch to Config view
3. Navigate with arrow keys
4. Press Enter to edit a field
5. Type new value and press Enter to save
6. Press 's' to save configuration
7. Press 'y' to confirm save

### Test Cases
- ✓ Navigate between sections
- ✓ Navigate between fields
- ✓ Edit string fields
- ✓ Edit integer fields
- ✓ Edit boolean fields
- ✓ Edit duration fields
- ✓ Cancel editing with Esc
- ✓ Save configuration
- ✓ Reload configuration
- ✓ Compact mode rendering
- ✓ Tiny mode rendering
- ✓ Field validation
- ✓ Error handling

## Known Limitations

1. **File Persistence**: Currently saves to the config file immediately when confirmed. No staging area.
2. **Provider Settings**: Provider-specific settings are read-only
3. **No Validation Preview**: Field validation happens on Enter, not while typing
4. **Single File**: Only works with one config file at a time
5. **No Templates**: Cannot save/load configuration templates

## Dependencies

### Required Packages
- `github.com/charmbracelet/bubbletea`: TUI framework
- `github.com/charmbracelet/lipgloss`: Styling
- `github.com/jedarden/tunnel/internal/core`: Core types
- `github.com/jedarden/tunnel/pkg/config`: Configuration management

### Style Integration
Uses existing styles from `styles.go`:
- TitleStyle, SubtitleStyle
- SelectedItemStyle, ListItemStyle
- BoxStyle, PanelStyle, ActivePanelStyle
- HelpKeyStyle, HelpDescStyle
- ErrorStyle, SuccessStyle, InfoStyle
- TabStyle, ActiveTabStyle

## Code Statistics
- **Lines of Code**: ~938 lines
- **Functions**: 18 main functions + helpers
- **Structs**: 4 structs
- **Configuration Sections**: 6 sections
- **Editable Fields**: ~25 fields total

## Documentation
- Implementation details: This document
- User guide: See main TUI README
- API documentation: Inline code comments
