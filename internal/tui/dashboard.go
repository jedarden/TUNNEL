package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/registry"
)

// Connection represents an active connection
type Connection struct {
	ID       string
	Method   string
	Status   string
	IP       string
	Upload   string
	Download string
	Icon     string
}

// SystemStatus represents system component status
type SystemStatus struct {
	Name   string
	Status string
	Info   string
}

// Dashboard is the main dashboard view model
type Dashboard struct {
	connections    []Connection
	quickActions   []string
	systemStatus   []SystemStatus
	selectedAction int
	width          int
	height         int

	// Dependencies
	registry        *registry.Registry
	manager         *core.DefaultConnectionManager
	instanceManager *registry.InstanceManager
}

// NewDashboard creates a new dashboard instance
func NewDashboard(reg *registry.Registry, mgr *core.DefaultConnectionManager, instanceMgr *registry.InstanceManager) *Dashboard {
	d := &Dashboard{
		connections: []Connection{},
		quickActions: []string{
			"Connect to new method",
			"View all connections",
			"Configure settings",
			"View logs",
			"System monitor",
		},
		systemStatus: []SystemStatus{
			{Name: "SSH Server", Status: "ready", Info: "Port 22"},
			{Name: "Firewall", Status: "ready", Info: "UFW enabled"},
			{Name: "Container", Status: "ready", Info: "Docker available"},
			{Name: "Network", Status: "connected", Info: "eth0 up"},
		},
		selectedAction:  0,
		width:           80,
		height:          24,
		registry:        reg,
		manager:         mgr,
		instanceManager: instanceMgr,
	}

	// Load initial connections
	d.RefreshConnections()

	return d
}

// Init initializes the dashboard
func (d *Dashboard) Init() tea.Cmd {
	return nil
}

// RefreshConnections populates connections from the registry and instance manager
func (d *Dashboard) RefreshConnections() {
	if d.registry == nil {
		return
	}

	connections := make([]Connection, 0)

	// First, get connections from instance manager (multi-instance support)
	if d.instanceManager != nil {
		instances := d.instanceManager.ListInstances()
		for _, instance := range instances {
			status := instance.GetStatus()
			icon := IconStopped
			displayStatus := "disconnected"

			switch status {
			case "connected":
				icon = IconConnected
				displayStatus = "connected"
			case "connecting":
				icon = IconReady
				displayStatus = "connecting"
			case "error":
				icon = IconCross
				displayStatus = "error"
			default:
				icon = IconReady
				displayStatus = "ready"
			}

			// Get connection info if available
			ip := "-"
			upload := "0 KB/s"
			download := "0 KB/s"

			if instance.IsConnected() {
				connInfo, err := instance.GetConnectionInfo()
				if err == nil && connInfo != nil {
					ip = connInfo.LocalIP
				}

				// Get health status for metrics
				health, err := instance.Provider.HealthCheck()
				if err == nil && health != nil {
					if health.BytesSent > 0 {
						upload = fmt.Sprintf("%.2f MB", float64(health.BytesSent)/(1024*1024))
					}
					if health.BytesReceived > 0 {
						download = fmt.Sprintf("%.2f MB", float64(health.BytesReceived)/(1024*1024))
					}
				}
			}

			conn := Connection{
				ID:       instance.ID,
				Method:   instance.DisplayName,
				Status:   displayStatus,
				IP:       ip,
				Upload:   upload,
				Download: download,
				Icon:     icon,
			}
			connections = append(connections, conn)
		}
	}

	// Then get connected providers from registry (singleton providers)
	connectedProviders := d.registry.GetConnectedProviders()

	for _, provider := range connectedProviders {
		// Check if this provider is already represented by an instance
		alreadyAdded := false
		for _, conn := range connections {
			if conn.Method == provider.Name() || conn.ID == provider.Name() {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}

		connInfo, err := provider.GetConnectionInfo()
		if err != nil {
			continue
		}

		// Determine upload/download metrics
		upload := "0 KB/s"
		download := "0 KB/s"

		// Get health status for metrics
		health, err := provider.HealthCheck()
		if err == nil && health != nil {
			if health.BytesSent > 0 {
				upload = fmt.Sprintf("%.2f MB", float64(health.BytesSent)/(1024*1024))
			}
			if health.BytesReceived > 0 {
				download = fmt.Sprintf("%.2f MB", float64(health.BytesReceived)/(1024*1024))
			}
		}

		conn := Connection{
			ID:       provider.Name(),
			Method:   provider.Name(),
			Status:   "connected",
			IP:       connInfo.LocalIP,
			Upload:   upload,
			Download: download,
			Icon:     IconConnected,
		}
		connections = append(connections, conn)
	}

	// Get installed but not connected providers
	installedProviders := d.registry.GetInstalledProviders()
	for _, provider := range installedProviders {
		if provider.IsConnected() {
			continue // Already added above
		}

		// Check if already represented by an instance
		alreadyAdded := false
		for _, conn := range connections {
			if conn.Method == provider.Name() || conn.ID == provider.Name() {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}

		conn := Connection{
			ID:       provider.Name(),
			Method:   provider.Name(),
			Status:   "ready",
			IP:       "-",
			Upload:   "0 KB/s",
			Download: "0 KB/s",
			Icon:     IconReady,
		}
		connections = append(connections, conn)
	}

	d.connections = connections
}

// SetSize updates the dashboard dimensions
func (d *Dashboard) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// Update handles messages for the dashboard
func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if d.selectedAction > 0 {
				d.selectedAction--
			}
		case "down", "j":
			if d.selectedAction < len(d.quickActions)-1 {
				d.selectedAction++
			}
		case "enter":
			// Handle action selection
			return d, d.executeAction(d.selectedAction)
		}
	}
	return d, nil
}

// View renders the dashboard
func (d *Dashboard) View() string {
	// Use compact single-column layout for small terminals
	if IsCompact(d.width, d.height) {
		return d.renderCompactView()
	}

	// Create three columns: connections, quick actions, system status
	connectionsPanel := d.renderConnections()
	actionsPanel := d.renderQuickActions()
	statusPanel := d.renderSystemStatus()

	// Calculate panel widths
	panelWidth := (d.width - 6) / 3 // 3 panels with spacing

	// Apply width to panels
	connectionsPanel = BoxStyle.Width(panelWidth).Render(connectionsPanel)
	actionsPanel = PanelStyle.Width(panelWidth).Render(actionsPanel)
	statusPanel = BoxStyle.Width(panelWidth).Render(statusPanel)

	// Join panels horizontally
	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		connectionsPanel,
		actionsPanel,
		statusPanel,
	)

	return row
}

// renderCompactView renders a compact single-column dashboard
func (d *Dashboard) renderCompactView() string {
	var b strings.Builder

	// Connections summary (one line per connection)
	b.WriteString(TitleStyle.Render("Connections"))
	b.WriteString("\n")

	if len(d.connections) == 0 {
		b.WriteString(HelpDescStyle.Render(" No active connections"))
		b.WriteString("\n")
	} else {
		for _, conn := range d.connections {
			icon := d.getStatusIndicator(conn.Status)
			// Compact: icon + name + IP
			line := fmt.Sprintf("%s %s", icon, conn.Method)
			if conn.IP != "" && conn.IP != "-" {
				line += " " + HelpDescStyle.Render(conn.IP)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Quick actions (compact)
	b.WriteString("\n")
	b.WriteString(TitleStyle.Render("Actions"))
	b.WriteString("\n")

	// Show only first 3 actions in compact mode
	maxActions := 3
	if d.height < 12 {
		maxActions = 2
	}

	for i := 0; i < len(d.quickActions) && i < maxActions; i++ {
		action := d.quickActions[i]
		// Truncate action text
		if len(action) > d.width-5 {
			action = action[:d.width-8] + "..."
		}
		if i == d.selectedAction {
			b.WriteString(SelectedItemStyle.Render(IconArrow + " " + action))
		} else {
			b.WriteString(ListItemStyle.Render(" " + action))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderConnections renders the active connections panel
func (d *Dashboard) renderConnections() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Active Connections"))
	b.WriteString("\n\n")

	if len(d.connections) == 0 {
		b.WriteString(InfoStyle.Render("No active connections"))
	} else {
		for _, conn := range d.connections {
			// Connection header
			statusIndicator := d.getStatusIndicator(conn.Status)
			header := lipgloss.JoinHorizontal(
				lipgloss.Left,
				statusIndicator,
				" ",
				lipgloss.NewStyle().Bold(true).Render(conn.Method),
			)
			b.WriteString(header)
			b.WriteString("\n")

			// Connection details
			b.WriteString(HelpDescStyle.Render(fmt.Sprintf("  IP: %s", conn.IP)))
			b.WriteString("\n")
			b.WriteString(HelpDescStyle.Render(fmt.Sprintf("  ↑ %s  ↓ %s", conn.Upload, conn.Download)))
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

// renderQuickActions renders the quick actions panel
func (d *Dashboard) renderQuickActions() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Quick Actions"))
	b.WriteString("\n\n")

	for i, action := range d.quickActions {
		actionText := fmt.Sprintf("%d. %s", i+1, action)
		if i == d.selectedAction {
			b.WriteString(SelectedItemStyle.Render(IconArrow + " " + actionText))
		} else {
			b.WriteString(ListItemStyle.Render("  " + actionText))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpDescStyle.Render("↑/↓ or j/k to navigate, Enter to select"))

	return b.String()
}

// renderSystemStatus renders the system status panel
func (d *Dashboard) renderSystemStatus() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("System Status"))
	b.WriteString("\n\n")

	for _, status := range d.systemStatus {
		statusIndicator := d.getStatusIndicator(status.Status)

		line := lipgloss.JoinHorizontal(
			lipgloss.Left,
			statusIndicator,
			" ",
			lipgloss.NewStyle().Bold(true).Render(status.Name),
		)
		b.WriteString(line)
		b.WriteString("\n")
		b.WriteString(HelpDescStyle.Render(fmt.Sprintf("  %s", status.Info)))
		b.WriteString("\n")
	}

	return b.String()
}

// getStatusIndicator returns a styled status indicator
func (d *Dashboard) getStatusIndicator(status string) string {
	switch status {
	case "connected":
		return StatusConnectedStyle.Render(IconConnected)
	case "ready":
		return StatusReadyStyle.Render(IconReady)
	case "stopped":
		return StatusStoppedStyle.Render(IconStopped)
	default:
		return StatusStyle.Foreground(StatusUnknown).Render(IconStopped)
	}
}

// executeAction executes the selected quick action
func (d *Dashboard) executeAction(action int) tea.Cmd {
	switch action {
	case 0: // Connect to new method
		return func() tea.Msg {
			return SwitchViewMsg{view: ViewBrowser}
		}
	case 1: // View all connections
		// Refresh connection data before switching
		d.RefreshConnections()
		return func() tea.Msg {
			return SwitchViewMsg{view: ViewMonitor}
		}
	case 2: // Configure settings
		return func() tea.Msg {
			return SwitchViewMsg{view: ViewConfig}
		}
	case 3: // View logs
		return func() tea.Msg {
			return SwitchViewMsg{view: ViewLogs}
		}
	case 4: // System monitor
		d.RefreshConnections()
		return func() tea.Msg {
			return SwitchViewMsg{view: ViewMonitor}
		}
	}
	return nil
}
