package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/registry"
)

// MonitorConnection represents a connection in the monitor view
type MonitorConnection struct {
	ID            string
	Method        string
	Status        string
	Latency       time.Duration
	BytesSent     int64
	BytesReceived int64
	Uptime        time.Duration
	LocalIP       string
	RemoteIP      string
	Health        string // "healthy", "degraded", "failed"
}

// Monitor is the connection monitor view model
type Monitor struct {
	connections       []MonitorConnection
	selectedIndex     int
	paused            bool
	lastRefresh       time.Time
	refreshInterval   time.Duration
	width             int
	height            int
	autoRefreshTicker *time.Ticker

	// Dependencies
	registry        *registry.Registry
	manager         *core.DefaultConnectionManager
	instanceManager *registry.InstanceManager
}

// TickMsg is sent periodically to trigger auto-refresh
type TickMsg time.Time

// NewMonitor creates a new monitor instance
func NewMonitor(reg *registry.Registry, mgr *core.DefaultConnectionManager, instanceMgr *registry.InstanceManager) *Monitor {
	return &Monitor{
		connections:     []MonitorConnection{},
		selectedIndex:   0,
		paused:          false,
		lastRefresh:     time.Now(),
		refreshInterval: 2 * time.Second,
		width:           80,
		height:          24,
		registry:        reg,
		manager:         mgr,
		instanceManager: instanceMgr,
	}
}

// Init initializes the monitor
func (m *Monitor) Init() tea.Cmd {
	return tea.Batch(
		m.refresh(),
		m.tickCmd(),
	)
}

// tickCmd returns a command that sends a TickMsg after the refresh interval
func (m *Monitor) tickCmd() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// SetSize updates the monitor dimensions
func (m *Monitor) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the monitor
func (m *Monitor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Manual refresh
			return m, m.refresh()

		case "p":
			// Toggle pause
			m.paused = !m.paused
			return m, nil

		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil

		case "down", "j":
			if m.selectedIndex < len(m.connections)-1 {
				m.selectedIndex++
			}
			return m, nil

		case "enter":
			// Show details for selected connection
			// For now, just refresh
			return m, m.refresh()
		}

	case TickMsg:
		// Auto-refresh if not paused
		if !m.paused {
			return m, tea.Batch(m.refresh(), m.tickCmd())
		}
		return m, m.tickCmd()

	case RefreshConnectionsMsg:
		m.RefreshConnections()
		return m, nil
	}

	return m, nil
}

// refresh returns a command to refresh connection data
func (m *Monitor) refresh() tea.Cmd {
	return func() tea.Msg {
		return RefreshConnectionsMsg{}
	}
}

// RefreshConnections updates the connection list from the manager, registry, and instance manager
func (m *Monitor) RefreshConnections() {
	m.lastRefresh = time.Now()

	if m.registry == nil {
		return
	}

	connections := make([]MonitorConnection, 0)
	addedIDs := make(map[string]bool)

	// First, add connections from instance manager (multi-instance support)
	if m.instanceManager != nil {
		instances := m.instanceManager.ListInstances()
		for _, instance := range instances {
			status := instance.GetStatus()
			health := "healthy"

			switch status {
			case "connected":
				health = "healthy"
			case "connecting":
				health = "degraded"
			case "error":
				health = "failed"
			default:
				health = "unknown"
			}

			var latency time.Duration
			var bytesSent, bytesReceived int64
			var uptime time.Duration
			localIP := "-"
			remoteIP := "-"

			if instance.IsConnected() {
				connInfo, err := instance.GetConnectionInfo()
				if err == nil && connInfo != nil {
					localIP = connInfo.LocalIP
					remoteIP = connInfo.RemoteIP
					if !connInfo.ConnectedAt.IsZero() {
						uptime = time.Since(connInfo.ConnectedAt)
					}
				}

				// Get health status for metrics
				healthStatus, err := instance.Provider.HealthCheck()
				if err == nil && healthStatus != nil {
					if !healthStatus.Healthy {
						health = "failed"
					}
					latency = healthStatus.Latency
					bytesSent = int64(healthStatus.BytesSent)
					bytesReceived = int64(healthStatus.BytesReceived)
				}
			}

			monConn := MonitorConnection{
				ID:            instance.ID,
				Method:        instance.DisplayName,
				Status:        status,
				Latency:       latency,
				BytesSent:     bytesSent,
				BytesReceived: bytesReceived,
				Uptime:        uptime,
				LocalIP:       localIP,
				RemoteIP:      remoteIP,
				Health:        health,
			}

			connections = append(connections, monConn)
			addedIDs[instance.ID] = true
			addedIDs[instance.ProviderName] = true
		}
	}

	// Get connections from manager if available
	var managerConns []*core.Connection
	if m.manager != nil {
		conns, err := m.manager.List()
		if err == nil {
			managerConns = conns
		}
	}

	// Add connections from manager (these have detailed metrics)
	managerConnMap := make(map[string]*core.Connection)
	for _, conn := range managerConns {
		if addedIDs[conn.ID] || addedIDs[conn.Method] {
			continue
		}
		managerConnMap[conn.Method] = conn

		health := m.determineHealth(conn)

		monConn := MonitorConnection{
			ID:            conn.ID,
			Method:        conn.Method,
			Status:        conn.GetState().String(),
			Latency:       conn.Metrics.GetLatency(),
			BytesSent:     0,
			BytesReceived: 0,
			Uptime:        conn.GetUptime(),
			LocalIP:       "-",
			RemoteIP:      fmt.Sprintf("%s:%d", conn.RemoteHost, conn.RemotePort),
			Health:        health,
		}

		// Get bytes sent/received
		sent, received, _ := conn.Metrics.GetStats()
		monConn.BytesSent = sent
		monConn.BytesReceived = received

		connections = append(connections, monConn)
		addedIDs[conn.ID] = true
		addedIDs[conn.Method] = true
	}

	// Get connected providers from registry
	connectedProviders := m.registry.GetConnectedProviders()

	// Add connected providers not already added
	for _, provider := range connectedProviders {
		// Skip if already added
		if addedIDs[provider.Name()] {
			continue
		}

		connInfo, err := provider.GetConnectionInfo()
		if err != nil {
			continue
		}

		// Get health status from provider
		health := "healthy"
		var latency time.Duration
		var bytesSent, bytesReceived int64

		healthStatus, err := provider.HealthCheck()
		if err == nil && healthStatus != nil {
			if !healthStatus.Healthy {
				health = "failed"
			}
			latency = healthStatus.Latency
			bytesSent = int64(healthStatus.BytesSent)
			bytesReceived = int64(healthStatus.BytesReceived)
		}

		uptime := time.Duration(0)
		if !connInfo.ConnectedAt.IsZero() {
			uptime = time.Since(connInfo.ConnectedAt)
		}

		monConn := MonitorConnection{
			ID:            provider.Name(),
			Method:        provider.Name(),
			Status:        connInfo.Status,
			Latency:       latency,
			BytesSent:     bytesSent,
			BytesReceived: bytesReceived,
			Uptime:        uptime,
			LocalIP:       connInfo.LocalIP,
			RemoteIP:      connInfo.RemoteIP,
			Health:        health,
		}

		connections = append(connections, monConn)
	}

	m.connections = connections

	// Adjust selected index if needed
	if m.selectedIndex >= len(m.connections) && len(m.connections) > 0 {
		m.selectedIndex = len(m.connections) - 1
	}
}

// determineHealth determines the health status of a connection
func (m *Monitor) determineHealth(conn *core.Connection) string {
	state := conn.GetState()

	switch state {
	case core.StateConnected:
		// Check latency
		latency := conn.Metrics.GetLatency()
		if latency > 500*time.Millisecond {
			return "degraded"
		}
		return "healthy"
	case core.StateConnecting, core.StateReconnecting:
		return "degraded"
	case core.StateFailed, core.StateDisconnected:
		return "failed"
	default:
		return "unknown"
	}
}

// View renders the monitor
func (m *Monitor) View() string {
	// Use compact layout for small terminals
	if IsCompact(m.width, m.height) {
		return m.renderCompactView()
	}

	// Use tiny layout for very small terminals
	if IsTiny(m.width, m.height) {
		return m.renderTinyView()
	}

	var b strings.Builder

	// Title and status
	title := TitleStyle.Render("Connection Monitor")
	status := m.renderStatusLine()

	b.WriteString(title)
	b.WriteString("  ")
	b.WriteString(status)
	b.WriteString("\n\n")

	// No connections message
	if len(m.connections) == 0 {
		b.WriteString(InfoStyle.Render("No active connections"))
		b.WriteString("\n\n")
		b.WriteString(HelpDescStyle.Render("Press 'r' to refresh, or navigate to Browser to connect"))
		return b.String()
	}

	// Connection list with metrics
	b.WriteString(m.renderConnectionTable())

	// Help text
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderTinyView renders a minimal view for very small terminals
func (m *Monitor) renderTinyView() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Monitor"))

	if m.paused {
		b.WriteString(" " + WarningBadgeStyle.Render("[PAUSED]"))
	}
	b.WriteString("\n")

	if len(m.connections) == 0 {
		b.WriteString(HelpDescStyle.Render("No connections"))
		return b.String()
	}

	// Show only selected connection in tiny mode
	if m.selectedIndex < len(m.connections) {
		conn := m.connections[m.selectedIndex]
		healthIcon := m.getHealthIcon(conn.Health)

		b.WriteString(healthIcon)
		b.WriteString(" ")
		b.WriteString(conn.Method)
		b.WriteString("\n")
		b.WriteString(HelpDescStyle.Render(fmt.Sprintf("↑%s ↓%s",
			formatBytes(conn.BytesSent),
			formatBytes(conn.BytesReceived))))
	}

	return b.String()
}

// renderCompactView renders a compact view for small terminals
func (m *Monitor) renderCompactView() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("Monitor"))
	if m.paused {
		b.WriteString(" " + WarningBadgeStyle.Render("[PAUSED]"))
	}
	b.WriteString("\n")

	if len(m.connections) == 0 {
		b.WriteString(HelpDescStyle.Render("No active connections"))
		b.WriteString("\n")
		b.WriteString(HelpDescStyle.Render("r:refresh p:pause"))
		return b.String()
	}

	// Show connections in compact list
	maxHeight := m.height - 4
	if maxHeight < 2 {
		maxHeight = 2
	}

	for i := 0; i < len(m.connections) && i < maxHeight; i++ {
		conn := m.connections[i]

		// Health icon
		healthIcon := m.getHealthIcon(conn.Health)

		// Connection name
		name := conn.Method
		if len(name) > 15 {
			name = name[:12] + "..."
		}

		// Build line
		line := fmt.Sprintf("%s %s", healthIcon, name)

		// Add latency if available
		if conn.Latency > 0 {
			line += fmt.Sprintf(" %dms", conn.Latency.Milliseconds())
		}

		if i == m.selectedIndex {
			b.WriteString(SelectedItemStyle.Render(IconArrow + " " + line))
		} else {
			b.WriteString(ListItemStyle.Render(" " + line))
		}
		b.WriteString("\n")
	}

	// Help
	b.WriteString(HelpDescStyle.Render("↑↓:nav r:refresh p:pause"))

	return b.String()
}

// renderStatusLine renders the status line with refresh time and pause state
func (m *Monitor) renderStatusLine() string {
	timeSinceRefresh := time.Since(m.lastRefresh)
	refreshText := fmt.Sprintf("Updated %ds ago", int(timeSinceRefresh.Seconds()))

	if m.paused {
		return HelpDescStyle.Render(refreshText) + " " + WarningBadgeStyle.Render("[PAUSED]")
	}

	return HelpDescStyle.Render(refreshText) + " " + SuccessBadgeStyle.Render("[LIVE]")
}

// renderConnectionTable renders a detailed table of connections
func (m *Monitor) renderConnectionTable() string {
	var b strings.Builder

	// Table header
	header := fmt.Sprintf("%-3s %-20s %-12s %-10s %-12s %-12s %-10s",
		"", "METHOD", "STATUS", "LATENCY", "SENT", "RECEIVED", "UPTIME")
	b.WriteString(SubtitleStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n")

	// Table rows
	for i, conn := range m.connections {
		// Health indicator
		healthIcon := m.getHealthIcon(conn.Health)

		// Format values
		status := conn.Status
		if len(status) > 12 {
			status = status[:9] + "..."
		}

		latency := "-"
		if conn.Latency > 0 {
			latency = fmt.Sprintf("%dms", conn.Latency.Milliseconds())
		}

		sent := formatBytes(conn.BytesSent)
		received := formatBytes(conn.BytesReceived)
		uptime := formatDuration(conn.Uptime)

		// Build row
		row := fmt.Sprintf("%-3s %-20s %-12s %-10s %-12s %-12s %-10s",
			healthIcon,
			truncate(conn.Method, 20),
			status,
			latency,
			sent,
			received,
			uptime,
		)

		// Style based on selection and health
		if i == m.selectedIndex {
			b.WriteString(SelectedItemStyle.Render(row))
		} else {
			switch conn.Health {
			case "failed":
				b.WriteString(ErrorStyle.Render(row))
			case "degraded":
				b.WriteString(WarningBadgeStyle.Render(row))
			default:
				b.WriteString(ListItemStyle.Render(row))
			}
		}
		b.WriteString("\n")

		// Show additional details for selected connection
		if i == m.selectedIndex {
			details := m.renderConnectionDetails(conn)
			b.WriteString(HelpDescStyle.Render(details))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderConnectionDetails renders detailed information for a connection
func (m *Monitor) renderConnectionDetails(conn MonitorConnection) string {
	var parts []string

	if conn.ID != "" && conn.ID != conn.Method {
		parts = append(parts, fmt.Sprintf("ID: %s", conn.ID))
	}

	if conn.LocalIP != "" && conn.LocalIP != "-" {
		parts = append(parts, fmt.Sprintf("Local: %s", conn.LocalIP))
	}

	if conn.RemoteIP != "" && conn.RemoteIP != "-" {
		parts = append(parts, fmt.Sprintf("Remote: %s", conn.RemoteIP))
	}

	if len(parts) == 0 {
		return ""
	}

	return "  " + strings.Join(parts, " | ")
}

// renderHelp renders help text
func (m *Monitor) renderHelp() string {
	help := []string{
		HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" select"),
		HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
		HelpKeyStyle.Render("p") + HelpDescStyle.Render(" pause/resume"),
		HelpKeyStyle.Render("Enter") + HelpDescStyle.Render(" details"),
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		help[0],
		HelpSeparatorStyle.Render(" • "),
		help[1],
		HelpSeparatorStyle.Render(" • "),
		help[2],
		HelpSeparatorStyle.Render(" • "),
		help[3],
	)
}

// getHealthIcon returns a styled health indicator
func (m *Monitor) getHealthIcon(health string) string {
	switch health {
	case "healthy":
		return StatusConnectedStyle.Render(IconConnected)
	case "degraded":
		return StatusReadyStyle.Render(IconWarning)
	case "failed":
		return StatusStoppedStyle.Render(IconCross)
	default:
		return StatusStyle.Foreground(StatusUnknown).Render(IconStopped)
	}
}

// formatBytes formats bytes into a human-readable string
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

// truncate truncates a string to a maximum length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
