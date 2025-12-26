package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/pkg/version"
)

// WebServerStatus represents the state of the web server
type WebServerStatus int

const (
	ServerStarting WebServerStatus = iota
	ServerRunning
	ServerError
	ServerStopped
)

// App is the minimal TUI application model
type App struct {
	width  int
	height int

	// Web server state
	serverStatus  WebServerStatus
	serverPort    int
	serverURL     string
	serverError   error
	connections   int
	browserOpened bool
}

// ServerStatusMsg updates the server status
type ServerStatusMsg struct {
	Status      WebServerStatus
	Port        int
	URL         string
	Error       error
	Connections int
}

// NewApp creates a new minimal TUI application instance
func NewApp(port int) *App {
	return &App{
		width:        80,
		height:       24,
		serverStatus: ServerStarting,
		serverPort:   port,
		serverURL:    fmt.Sprintf("http://localhost:%d", port),
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit

		case "o":
			// Open browser
			if a.serverStatus == ServerRunning {
				a.openBrowser()
			}
			return a, nil

		case "r":
			// Refresh - could trigger a status update
			return a, nil
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case ServerStatusMsg:
		a.serverStatus = msg.Status
		if msg.Port > 0 {
			a.serverPort = msg.Port
			a.serverURL = fmt.Sprintf("http://localhost:%d", msg.Port)
		}
		if msg.URL != "" {
			a.serverURL = msg.URL
		}
		a.serverError = msg.Error
		a.connections = msg.Connections
		return a, nil
	}

	return a, nil
}

// View renders the application UI
func (a *App) View() string {
	var b strings.Builder

	// Header
	header := a.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Server status box
	statusBox := a.renderStatusBox()
	b.WriteString(statusBox)
	b.WriteString("\n\n")

	// Footer with controls
	footer := a.renderFooter()
	b.WriteString(footer)

	// Center content vertically
	content := b.String()
	contentHeight := lipgloss.Height(content)
	topPadding := (a.height - contentHeight) / 3
	if topPadding > 0 {
		content = strings.Repeat("\n", topPadding) + content
	}

	// Center horizontally
	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Top,
		content,
	)
}

// renderHeader renders the application header
func (a *App) renderHeader() string {
	title := TitleStyle.Render("TUNNEL")
	ver := HelpDescStyle.Render(version.Version)
	return lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", ver)
}

// renderStatusBox renders the server status
func (a *App) renderStatusBox() string {
	var statusLine, urlLine, connectionsLine string

	switch a.serverStatus {
	case ServerStarting:
		statusLine = StatusReadyStyle.Render(IconReady + " Starting web server...")

	case ServerRunning:
		statusLine = StatusConnectedStyle.Render(IconConnected + " Web server running")
		urlLine = "\n\n" + InfoStyle.Render("Open in browser:") + "\n" +
			TitleStyle.Render(a.serverURL)
		connectionsLine = "\n\n" + HelpDescStyle.Render(fmt.Sprintf("Active connections: %d", a.connections))

	case ServerError:
		statusLine = StatusStoppedStyle.Render(IconCross + " Server error")
		if a.serverError != nil {
			urlLine = "\n\n" + ErrorStyle.Render(a.serverError.Error())
		}

	case ServerStopped:
		statusLine = StatusStoppedStyle.Render(IconStopped + " Server stopped")
	}

	content := statusLine + urlLine + connectionsLine

	// Create a centered box
	boxWidth := 50
	if a.width < 60 {
		boxWidth = a.width - 4
	}

	return BoxStyle.
		Width(boxWidth).
		Align(lipgloss.Center).
		Render(content)
}

// renderFooter renders the control hints
func (a *App) renderFooter() string {
	var hints []string

	if a.serverStatus == ServerRunning {
		hints = append(hints, HelpKeyStyle.Render("o")+HelpDescStyle.Render(" open browser"))
	}
	hints = append(hints, HelpKeyStyle.Render("q")+HelpDescStyle.Render(" quit"))

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		strings.Join(hints, HelpSeparatorStyle.Render("  •  ")),
	)
}

// openBrowser opens the server URL in the default browser
func (a *App) openBrowser() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", a.serverURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", a.serverURL)
	default: // Linux and others
		cmd = exec.Command("xdg-open", a.serverURL)
	}

	a.browserOpened = true
	return cmd.Start()
}

// SetServerStatus updates the server status (called from main)
func (a *App) SetServerStatus(status WebServerStatus, err error, connections int) tea.Cmd {
	return func() tea.Msg {
		return ServerStatusMsg{
			Status:      status,
			Port:        a.serverPort,
			Connections: connections,
			Error:       err,
		}
	}
}
