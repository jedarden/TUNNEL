package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/registry"
	"github.com/jedarden/tunnel/pkg/config"
	"github.com/jedarden/tunnel/pkg/version"
)

// InstanceCreatedMsg is sent when a new instance is created
type InstanceCreatedMsg struct {
	InstanceID   string
	ProviderName string
}

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
	showWizard  bool
	errorMsg    string
	errorTimer  int

	// Dependencies
	registry        *registry.Registry
	manager         *core.DefaultConnectionManager
	appConfig       *config.Config
	instanceManager *registry.InstanceManager

	// Sub-models
	dashboard *Dashboard
	browser   *Browser
	config    *Config
	monitor   *Monitor
	logs      *Logs
	help      *Help
	wizard    *Wizard
}

// Message types for view switching
type SwitchViewMsg struct {
	view ViewMode
}

type ToggleHelpMsg struct{}

type RefreshConnectionsMsg struct{}

// OpenWizardMsg requests opening the connection wizard
type OpenWizardMsg struct {
	ProviderName string
}

// ShowErrorMsg displays an error message
type ShowErrorMsg struct {
	Message string
}

// NewApp creates a new TUI application instance
func NewApp(reg *registry.Registry, mgr *core.DefaultConnectionManager, cfg *config.Config) *App {
	// Get manager config from manager
	mgrConfig := core.DefaultManagerConfig()

	// Create instance manager for multi-instance support
	instanceMgr := registry.NewInstanceManager(reg)

	return &App{
		currentView:     ViewDashboard,
		registry:        reg,
		manager:         mgr,
		appConfig:       cfg,
		instanceManager: instanceMgr,
		dashboard:       NewDashboard(reg, mgr, instanceMgr),
		browser:         NewBrowser(reg),
		config:          NewConfig(cfg, mgrConfig),
		monitor:         NewMonitor(reg, mgr, instanceMgr),
		logs:            NewLogs(reg),
		help:            NewHelp(),
		showHelp:        false,
		width:           80,
		height:          24,
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

	// Handle wizard mode first
	if a.showWizard && a.wizard != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			// Pass to wizard
			updatedWizard, cmd := a.wizard.Update(msg)
			a.wizard = updatedWizard.(*Wizard)
			return a, cmd

		case WizardCompleteMsg:
			a.showWizard = false
			if msg.Success {
				// Refresh connections and show success
				a.errorMsg = ""
				if a.dashboard != nil {
					a.dashboard.RefreshConnections()
				}
				if a.monitor != nil {
					a.monitor.RefreshConnections()
				}
				// Switch to monitor to see the connection
				a.currentView = ViewMonitor
			} else if msg.Error != nil {
				a.errorMsg = msg.Error.Error()
			}
			return a, nil

		case WizardCancelMsg:
			a.showWizard = false
			a.wizard = nil
			return a, nil

		case tea.WindowSizeMsg:
			a.width = msg.Width
			a.height = msg.Height
			a.wizard.SetSize(msg.Width, msg.Height)
			return a, nil
		}
		return a, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear error message on any key
		if a.errorMsg != "" {
			a.errorMsg = ""
		}

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
		a.config.SetSize(msg.Width, msg.Height)
		a.monitor.SetSize(msg.Width, msg.Height)
		a.logs.SetSize(msg.Width, msg.Height)
		a.help.SetSize(msg.Width, msg.Height)
		return a, nil

	case SwitchViewMsg:
		a.currentView = msg.view
		return a, nil

	case ToggleHelpMsg:
		a.showHelp = !a.showHelp
		return a, nil

	case OpenWizardMsg:
		// Create and show the wizard for the selected provider with instance manager support
		a.wizard = NewWizardWithInstanceManager(a.registry, a.instanceManager, msg.ProviderName)
		a.wizard.SetSize(a.width, a.height)
		a.showWizard = true
		return a, nil

	case ShowErrorMsg:
		a.errorMsg = msg.Message
		return a, nil

	case RefreshConnectionsMsg:
		// Refresh connections in the dashboard and monitor
		if a.dashboard != nil {
			a.dashboard.RefreshConnections()
		}
		if a.monitor != nil {
			a.monitor.RefreshConnections()
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

		case ViewConfig:
			updatedConfig, cmd := a.config.Update(msg)
			a.config = updatedConfig.(*Config)
			cmds = append(cmds, cmd)

		case ViewMonitor:
			updatedMonitor, cmd := a.monitor.Update(msg)
			a.monitor = updatedMonitor.(*Monitor)
			cmds = append(cmds, cmd)

		case ViewLogs:
			updatedLogs, cmd := a.logs.Update(msg)
			a.logs = updatedLogs.(*Logs)
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// View renders the application UI
func (a *App) View() string {
	// Show wizard overlay if active
	if a.showWizard && a.wizard != nil {
		return a.wizard.View()
	}

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
		content = a.config.View()
	case ViewMonitor:
		content = a.monitor.View()
	case ViewLogs:
		content = a.logs.View()
	}

	// Add error message if present
	if a.errorMsg != "" {
		errorBox := ErrorStyle.Render(IconCross + " " + a.errorMsg)
		content = errorBox + "\n\n" + content
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

	// Just show essential info with version and dimensions
	b.WriteString(TitleStyle.Render("TUNNEL"))
	b.WriteString(" ")
	b.WriteString(HelpDescStyle.Render(version.Version))
	b.WriteString(" ")
	b.WriteString(HelpDescStyle.Render(fmt.Sprintf("[%dx%d]", a.width, a.height)))
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
	dims := HelpDescStyle.Render(fmt.Sprintf("[%dx%d]", a.width, a.height))
	return TitleStyle.Render("TUNNEL") + " " + HelpDescStyle.Render(version.Version) + " " + dims
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
	versionStr := HelpDescStyle.Render(version.Version)
	dims := HelpDescStyle.Render(fmt.Sprintf("[%dx%d]", a.width, a.height))

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
		subtitle,
		"  ",
		versionStr,
		" ",
		dims,
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
