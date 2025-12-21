package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

// NewDashboard creates a new dashboard instance
func NewDashboard() *Dashboard {
	return &Dashboard{
		connections: []Connection{
			{
				ID:       "conn-001",
				Method:   "Tailscale",
				Status:   "connected",
				IP:       "100.64.0.1",
				Upload:   "1.2 MB/s",
				Download: "3.4 MB/s",
				Icon:     IconConnected,
			},
			{
				ID:       "conn-002",
				Method:   "WireGuard",
				Status:   "ready",
				IP:       "10.0.0.1",
				Upload:   "0 KB/s",
				Download: "0 KB/s",
				Icon:     IconReady,
			},
		},
		quickActions: []string{
			"Connect to new method",
			"View all connections",
			"Configure settings",
			"View logs",
			"System monitor",
		},
		systemStatus: []SystemStatus{
			{Name: "SSH Server", Status: "connected", Info: "Port 22"},
			{Name: "Firewall", Status: "ready", Info: "UFW enabled"},
			{Name: "Container", Status: "connected", Info: "Docker running"},
			{Name: "Network", Status: "connected", Info: "eth0 up"},
		},
		selectedAction: 0,
		width:          80,
		height:         24,
	}
}

// Init initializes the dashboard
func (d *Dashboard) Init() tea.Cmd {
	return nil
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
		return func() tea.Msg {
			return SwitchViewMsg{view: ViewMonitor}
		}
	}
	return nil
}
