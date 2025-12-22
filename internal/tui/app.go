package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/registry"
)

// ViewMode represents the current active view
type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewBrowser
	ViewConfig
	ViewLogs
	ViewMonitor
)

// App is the main TUI application model
type App struct {
	currentView ViewMode
	width       int
	height      int
	showHelp    bool

	// Dependencies
	registry *registry.Registry
	manager  *core.DefaultConnectionManager

	// Sub-models
	dashboard *Dashboard
	browser   *Browser
	help      *Help
}

// Message types for view switching
type SwitchViewMsg struct {
	view ViewMode
}

type ToggleHelpMsg struct{}

type RefreshConnectionsMsg struct{}

// NewApp creates a new TUI application instance
func NewApp(reg *registry.Registry, mgr *core.DefaultConnectionManager) *App {
	return &App{
		currentView: ViewDashboard,
		registry:    reg,
		manager:     mgr,
		dashboard:   NewDashboard(reg, mgr),
		browser:     NewBrowser(reg),
		help:        NewHelp(),
		showHelp:    false,
		width:       80,
		height:      24,
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return a.refreshConnections()
}

// refreshConnections returns a tea.Cmd to fetch real connection data
func (a *App) refreshConnections() tea.Cmd {
	return func() tea.Msg {
		return RefreshConnectionsMsg{}
	}
}

// Update handles messages and updates the model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key bindings
		switch msg.String() {
		case "ctrl+c", "q":
			if !a.showHelp {
				return a, tea.Quit
			}
			a.showHelp = false
			return a, nil

		case "?":
			a.showHelp = !a.showHelp
			return a, nil

		case "esc":
			if a.showHelp {
				a.showHelp = false
				return a, nil
			}

		// Tab switching (1-5)
		case "1":
			a.currentView = ViewDashboard
			return a, nil
		case "2":
			a.currentView = ViewBrowser
			return a, nil
		case "3":
			a.currentView = ViewConfig
			return a, nil
		case "4":
			a.currentView = ViewLogs
			return a, nil
		case "5":
			a.currentView = ViewMonitor
			return a, nil

		// Tab navigation with Tab/Shift+Tab
		case "tab":
			a.currentView = (a.currentView + 1) % 5
			return a, nil
		case "shift+tab":
			if a.currentView == 0 {
				a.currentView = 4
			} else {
				a.currentView--
			}
			return a, nil
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Update sub-models with new dimensions
		a.dashboard.SetSize(msg.Width, msg.Height)
		a.browser.SetSize(msg.Width, msg.Height)
		return a, nil

	case SwitchViewMsg:
		a.currentView = msg.view
		return a, nil

	case ToggleHelpMsg:
		a.showHelp = !a.showHelp
		return a, nil

	case RefreshConnectionsMsg:
		// Refresh connections in the dashboard
		if a.dashboard != nil {
			a.dashboard.RefreshConnections()
		}
		return a, nil
	}

	// Pass messages to active view
	if !a.showHelp {
		switch a.currentView {
		case ViewDashboard:
			updatedDashboard, cmd := a.dashboard.Update(msg)
			a.dashboard = updatedDashboard.(*Dashboard)
			cmds = append(cmds, cmd)

		case ViewBrowser:
			updatedBrowser, cmd := a.browser.Update(msg)
			a.browser = updatedBrowser.(*Browser)
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// View renders the application UI
func (a *App) View() string {
	if a.showHelp {
		return a.renderHelp()
	}

	// Check for tiny terminal
	if IsTiny(a.width, a.height) {
		return a.renderTinyView()
	}

	var content string

	// Render current view
	switch a.currentView {
	case ViewDashboard:
		content = a.dashboard.View()
	case ViewBrowser:
		content = a.browser.View()
	case ViewConfig:
		content = a.renderPlaceholder("Config", "Coming soon...")
	case ViewLogs:
		content = a.renderPlaceholder("Logs", "Coming soon...")
	case ViewMonitor:
		content = a.renderPlaceholder("Monitor", "Coming soon...")
	}

	// Build the full UI based on terminal size
	compact := IsCompact(a.width, a.height)

	var header, tabs, footer string
	if compact {
		header = a.renderCompactHeader()
		tabs = a.renderCompactTabs()
		footer = a.renderCompactFooter()
	} else {
		header = a.renderHeader()
		tabs = a.renderTabs()
		footer = a.renderFooter()
	}

	// Calculate content height
	contentHeight := a.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer)

	// Ensure content fits within available height
	if contentHeight > 0 {
		contentLines := strings.Split(content, "\n")
		if len(contentLines) > contentHeight {
			content = strings.Join(contentLines[:contentHeight], "\n")
		}
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		content,
		footer,
	)
}

// renderTinyView renders a minimal view for very small terminals
func (a *App) renderTinyView() string {
	var b strings.Builder

	// Just show essential info
	b.WriteString(TitleStyle.Render("TUNNEL"))
	b.WriteString(" ")

	// Show current view indicator
	views := []string{"D", "B", "C", "L", "M"}
	for i, v := range views {
		if ViewMode(i) == a.currentView {
			b.WriteString(ActiveTabStyle.Render("[" + v + "]"))
		} else {
			b.WriteString(TabStyle.Render(v))
		}
	}
	b.WriteString("\n")

	// Show connection status in compact form
	if a.registry != nil {
		connected := a.registry.GetConnectedProviders()
		if len(connected) > 0 {
			b.WriteString(StatusConnectedStyle.Render(IconConnected))
			b.WriteString(" ")
			for i, p := range connected {
				if i > 0 {
					b.WriteString(",")
				}
				// Truncate name to first 3 chars
				name := p.Name()
				if len(name) > 3 {
					name = name[:3]
				}
				b.WriteString(name)
			}
		} else {
			b.WriteString(StatusStoppedStyle.Render(IconStopped + " No conn"))
		}
	}
	b.WriteString("\n")

	// Minimal controls
	b.WriteString(HelpDescStyle.Render("1-5:view ?:help q:quit"))

	return b.String()
}

// renderCompactHeader renders a minimal header
func (a *App) renderCompactHeader() string {
	return TitleStyle.Render("TUNNEL") + " " + HelpDescStyle.Render("SSH Tunnel Manager")
}

// renderCompactTabs renders compact tab navigation
func (a *App) renderCompactTabs() string {
	tabs := []string{"1:Dash", "2:Browse", "3:Cfg", "4:Log", "5:Mon"}
	var result []string

	for i, t := range tabs {
		if ViewMode(i) == a.currentView {
			result = append(result, ActiveTabStyle.Render(t))
		} else {
			result = append(result, TabStyle.Render(t))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, result...)
}

// renderCompactFooter renders a minimal footer
func (a *App) renderCompactFooter() string {
	return HelpDescStyle.Render("?:help q:quit ↑↓:nav")
}

// renderHeader renders the application header
func (a *App) renderHeader() string {
	title := TitleStyle.Render("TUNNEL")
	subtitle := SubtitleStyle.Render("Terminal Unified Network Node Encrypted Link")

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
		subtitle,
	)

	return HeaderStyle.Width(a.width).Render(header)
}

// renderTabs renders the tab navigation
func (a *App) renderTabs() string {
	tabs := []string{}

	views := []struct {
		name  string
		index ViewMode
	}{
		{"1. Dashboard", ViewDashboard},
		{"2. Browser", ViewBrowser},
		{"3. Config", ViewConfig},
		{"4. Logs", ViewLogs},
		{"5. Monitor", ViewMonitor},
	}

	for _, v := range views {
		if a.currentView == v.index {
			tabs = append(tabs, ActiveTabStyle.Render(v.name))
		} else {
			tabs = append(tabs, TabStyle.Render(v.name))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	separator := strings.Repeat("─", a.width)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		lipgloss.NewStyle().Foreground(ColorBorder).Render(separator),
	)
}

// renderFooter renders the application footer with help hint
func (a *App) renderFooter() string {
	helpHint := HelpKeyStyle.Render("?") + HelpDescStyle.Render(" help")
	quitHint := HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	tabHint := HelpKeyStyle.Render("1-5") + HelpDescStyle.Render(" switch view")

	hints := lipgloss.JoinHorizontal(
		lipgloss.Left,
		helpHint,
		HelpSeparatorStyle.Render(" • "),
		tabHint,
		HelpSeparatorStyle.Render(" • "),
		quitHint,
	)

	return FooterStyle.Width(a.width).Render(hints)
}

// renderHelp renders the help overlay
func (a *App) renderHelp() string {
	return a.help.View()
}

// renderPlaceholder renders a placeholder view for unimplemented features
func (a *App) renderPlaceholder(title, message string) string {
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		TitleStyle.Render(title),
		"",
		InfoStyle.Render(message),
		"",
		HelpDescStyle.Render("Press any number (1-5) to switch views"),
	)

	// Center the content
	verticalPadding := (a.height - lipgloss.Height(content)) / 2
	if verticalPadding > 0 {
		padding := strings.Repeat("\n", verticalPadding)
		content = padding + content
	}

	return lipgloss.Place(
		a.width,
		a.height-6, // Account for header, tabs, footer
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
