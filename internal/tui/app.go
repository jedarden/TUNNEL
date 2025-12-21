package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type WindowSizeMsg struct {
	width  int
	height int
}

// NewApp creates a new TUI application instance
func NewApp() *App {
	return &App{
		currentView: ViewDashboard,
		dashboard:   NewDashboard(),
		browser:     NewBrowser(),
		help:        NewHelp(),
		showHelp:    false,
		width:       80,
		height:      24,
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return nil
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

	var content string

	// Render current view
	switch a.currentView {
	case ViewDashboard:
		content = a.dashboard.View()
	case ViewBrowser:
		content = a.browser.View()
	case ViewConfig:
		content = a.renderPlaceholder("Configuration", "Configuration view coming soon...")
	case ViewLogs:
		content = a.renderPlaceholder("Logs", "Log viewer coming soon...")
	case ViewMonitor:
		content = a.renderPlaceholder("Monitor", "Connection monitor coming soon...")
	}

	// Build the full UI
	header := a.renderHeader()
	tabs := a.renderTabs()
	footer := a.renderFooter()

	// Calculate content height
	contentHeight := a.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer) - 2

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
