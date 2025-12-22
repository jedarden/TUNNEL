package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
)

// parsePort converts a string to a port number
func parsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range")
	}
	return port, nil
}

// WizardField represents a configuration field
type WizardField struct {
	Name        string
	Label       string
	Value       string
	Placeholder string
	Required    bool
	Secret      bool // Hide input for passwords/tokens
}

// Wizard is the connection configuration wizard
type Wizard struct {
	provider        providers.Provider
	providerName    string
	registry        *registry.Registry
	instanceManager *registry.InstanceManager
	fields          []WizardField
	selectedField   int
	editing         bool
	editBuffer      string
	errorMsg        string
	successMsg      string
	width           int
	height          int
	cancelled       bool
	completed       bool
	installing      bool
	installStatus   string
}

// WizardCompleteMsg is sent when the wizard completes
type WizardCompleteMsg struct {
	Success      bool
	ProviderName string
	InstanceID   string
	Error        error
}

// WizardCancelMsg is sent when the wizard is cancelled
type WizardCancelMsg struct{}

// WizardInstallMsg is sent to trigger installation
type WizardInstallMsg struct {
	ProviderName string
}

// WizardInstallCompleteMsg is sent when installation completes
type WizardInstallCompleteMsg struct {
	Success bool
	Error   error
}

// NewWizard creates a new connection wizard for a provider
func NewWizard(reg *registry.Registry, providerName string) *Wizard {
	w := &Wizard{
		providerName:  providerName,
		registry:      reg,
		fields:        []WizardField{},
		selectedField: 0,
		editing:       false,
		width:         80,
		height:        24,
	}

	// Get the provider
	if reg != nil {
		provider, err := reg.GetProvider(providerName)
		if err == nil {
			w.provider = provider
		}
	}

	// Set up fields based on provider type
	w.setupFields()

	return w
}

// NewWizardWithInstanceManager creates a wizard with instance manager support
func NewWizardWithInstanceManager(reg *registry.Registry, instanceMgr *registry.InstanceManager, providerName string) *Wizard {
	w := NewWizard(reg, providerName)
	w.instanceManager = instanceMgr
	return w
}

// setupFields configures the wizard fields based on provider type
func (w *Wizard) setupFields() {
	switch w.providerName {
	case "Tailscale":
		w.fields = []WizardField{
			{Name: "auth_key", Label: "Auth Key", Placeholder: "tskey-...", Required: false, Secret: true},
			{Name: "hostname", Label: "Hostname", Placeholder: "my-device", Required: false},
			{Name: "accept_routes", Label: "Accept Routes", Value: "yes", Placeholder: "yes/no", Required: false},
		}
	case "WireGuard":
		w.fields = []WizardField{
			{Name: "interface", Label: "Interface Name", Value: "wg0", Placeholder: "wg0", Required: true},
			{Name: "config_file", Label: "Config File Path", Placeholder: "/etc/wireguard/wg0.conf", Required: true},
		}
	case "Cloudflare Tunnel":
		w.fields = []WizardField{
			{Name: "token", Label: "Tunnel Token", Placeholder: "eyJ...", Required: true, Secret: true},
			{Name: "tunnel_name", Label: "Tunnel Name", Placeholder: "my-tunnel", Required: false},
		}
	case "ngrok":
		w.fields = []WizardField{
			{Name: "auth_token", Label: "Auth Token", Placeholder: "2abc123...", Required: true, Secret: true},
			{Name: "region", Label: "Region", Value: "us", Placeholder: "us/eu/ap/au", Required: false},
			{Name: "port", Label: "Local Port", Value: "22", Placeholder: "22", Required: true},
			{Name: "proto", Label: "Protocol", Value: "tcp", Placeholder: "tcp/http", Required: false},
		}
	case "ZeroTier":
		w.fields = []WizardField{
			{Name: "network_id", Label: "Network ID", Placeholder: "16-char hex", Required: true},
		}
	case "bore":
		w.fields = []WizardField{
			{Name: "server", Label: "Server Address", Value: "bore.pub", Placeholder: "bore.pub", Required: true},
			{Name: "local_port", Label: "Local Port", Value: "22", Placeholder: "22", Required: true},
			{Name: "remote_port", Label: "Remote Port", Placeholder: "auto", Required: false},
		}
	default:
		w.fields = []WizardField{
			{Name: "note", Label: "Note", Value: "No configuration needed", Required: false},
		}
	}
}

// SetSize updates the wizard dimensions
func (w *Wizard) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// Init initializes the wizard
func (w *Wizard) Init() tea.Cmd {
	return nil
}

// Update handles messages for the wizard
func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't process keys while installing
		if w.installing {
			return w, nil
		}

		// Clear error on any key press
		if w.errorMsg != "" && msg.String() != "enter" {
			w.errorMsg = ""
		}

		if w.editing {
			return w.handleEditMode(msg)
		}

		switch msg.String() {
		case "esc":
			w.cancelled = true
			return w, func() tea.Msg { return WizardCancelMsg{} }

		case "up", "k":
			if w.selectedField > 0 {
				w.selectedField--
			}

		case "down", "j":
			if w.selectedField < len(w.fields)-1 {
				w.selectedField++
			}

		case "enter":
			// If on a field, start editing
			if w.selectedField < len(w.fields) {
				w.editing = true
				w.editBuffer = w.fields[w.selectedField].Value
			}

		case "tab":
			// Move to next field
			if w.selectedField < len(w.fields)-1 {
				w.selectedField++
			}

		case "c":
			// Connect (or install first if needed)
			return w, w.connect()

		case "i":
			// Manual install trigger
			if w.provider != nil && !w.provider.IsInstalled() {
				return w, w.installProvider()
			}

		case "q":
			w.cancelled = true
			return w, func() tea.Msg { return WizardCancelMsg{} }
		}

	case WizardInstallCompleteMsg:
		w.installing = false
		if msg.Success {
			w.successMsg = w.providerName + " installed successfully!"
			w.installStatus = ""
		} else {
			w.errorMsg = fmt.Sprintf("Installation failed: %v", msg.Error)
			w.installStatus = ""
		}
		return w, nil
	}

	return w, nil
}

// handleEditMode handles keyboard input while editing a field
func (w *Wizard) handleEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel editing
		w.editing = false
		w.editBuffer = ""

	case "enter":
		// Save the edit
		w.fields[w.selectedField].Value = w.editBuffer
		w.editing = false
		w.editBuffer = ""
		// Move to next field
		if w.selectedField < len(w.fields)-1 {
			w.selectedField++
		}

	case "backspace":
		if len(w.editBuffer) > 0 {
			w.editBuffer = w.editBuffer[:len(w.editBuffer)-1]
		}

	default:
		// Add character to buffer
		if len(msg.String()) == 1 {
			w.editBuffer += msg.String()
		} else if msg.Type == tea.KeySpace {
			w.editBuffer += " "
		}
	}

	return w, nil
}

// installProvider installs the provider dependency
func (w *Wizard) installProvider() tea.Cmd {
	w.installing = true
	w.installStatus = "Installing " + w.providerName + "..."

	return func() tea.Msg {
		if w.provider == nil {
			return WizardInstallCompleteMsg{
				Success: false,
				Error:   fmt.Errorf("provider not found"),
			}
		}

		err := w.provider.Install()
		if err != nil {
			return WizardInstallCompleteMsg{
				Success: false,
				Error:   err,
			}
		}

		return WizardInstallCompleteMsg{
			Success: true,
			Error:   nil,
		}
	}
}

// connect attempts to connect using the configured settings
func (w *Wizard) connect() tea.Cmd {
	return func() tea.Msg {
		if w.provider == nil {
			return WizardCompleteMsg{
				Success:      false,
				ProviderName: w.providerName,
				Error:        fmt.Errorf("provider not found"),
			}
		}

		// Validate required fields
		for _, field := range w.fields {
			if field.Required && field.Value == "" {
				return WizardCompleteMsg{
					Success:      false,
					ProviderName: w.providerName,
					Error:        fmt.Errorf("required field '%s' is empty", field.Label),
				}
			}
		}

		// Check if installed - auto-install if not
		if !w.provider.IsInstalled() {
			// Try to install automatically
			if err := w.provider.Install(); err != nil {
				return WizardCompleteMsg{
					Success:      false,
					ProviderName: w.providerName,
					Error:        fmt.Errorf("auto-install failed: %w. Press 'i' to retry installation", err),
				}
			}
		}

		// Build ProviderConfig from fields
		config := &providers.ProviderConfig{
			Name:  w.providerName,
			Extra: make(map[string]string),
		}

		// Get display name from hostname field or generate one
		displayName := ""

		for _, field := range w.fields {
			if field.Value == "" {
				continue
			}
			switch field.Name {
			case "auth_key":
				config.AuthKey = field.Value
			case "auth_token":
				config.AuthToken = field.Value
			case "network_id":
				config.NetworkID = field.Value
			case "tunnel_name":
				config.TunnelName = field.Value
			case "token":
				// Cloudflare tunnel token goes to AuthToken
				config.AuthToken = field.Value
			case "config_file":
				config.ConfigFile = field.Value
			case "port", "local_port":
				if port, err := parsePort(field.Value); err == nil {
					config.LocalPort = port
				}
			case "remote_port":
				if port, err := parsePort(field.Value); err == nil {
					config.RemotePort = port
				}
			case "server":
				config.RemoteHost = field.Value
			case "hostname":
				displayName = field.Value
			default:
				// Store in Extra
				config.Extra[field.Name] = field.Value
			}
		}

		// If instance manager is available, create a new instance
		if w.instanceManager != nil {
			instance, err := w.instanceManager.CreateInstance(w.providerName, displayName, config)
			if err != nil {
				return WizardCompleteMsg{
					Success:      false,
					ProviderName: w.providerName,
					Error:        fmt.Errorf("failed to create instance: %w", err),
				}
			}

			// Connect the instance
			if err := instance.Connect(); err != nil {
				return WizardCompleteMsg{
					Success:      false,
					ProviderName: w.providerName,
					InstanceID:   instance.ID,
					Error:        fmt.Errorf("connection failed: %w", err),
				}
			}

			return WizardCompleteMsg{
				Success:      true,
				ProviderName: w.providerName,
				InstanceID:   instance.ID,
				Error:        nil,
			}
		}

		// Fallback: direct provider connection (singleton mode)
		if err := w.provider.Configure(config); err != nil {
			return WizardCompleteMsg{
				Success:      false,
				ProviderName: w.providerName,
				Error:        fmt.Errorf("failed to save configuration: %w", err),
			}
		}

		if err := w.provider.Connect(); err != nil {
			return WizardCompleteMsg{
				Success:      false,
				ProviderName: w.providerName,
				Error:        fmt.Errorf("connection failed: %w", err),
			}
		}

		return WizardCompleteMsg{
			Success:      true,
			ProviderName: w.providerName,
			Error:        nil,
		}
	}
}

// View renders the wizard
func (w *Wizard) View() string {
	if IsCompact(w.width, w.height) {
		return w.renderCompact()
	}

	var content strings.Builder

	// Title
	title := fmt.Sprintf("Configure %s Connection", w.providerName)
	content.WriteString(TitleStyle.Render(title))
	content.WriteString("\n\n")

	// Show installation status if installing
	if w.installing {
		content.WriteString(InfoStyle.Render("⏳ " + w.installStatus))
		content.WriteString("\n")
		content.WriteString(HelpDescStyle.Render("Please wait..."))
		content.WriteString("\n\n")
	}

	// Provider status
	if w.provider != nil {
		var status string
		if w.provider.IsConnected() {
			status = StatusConnectedStyle.Render(IconConnected + " Already Connected")
		} else if w.provider.IsInstalled() {
			status = StatusReadyStyle.Render(IconReady + " Installed & Ready")
		} else {
			status = StatusStoppedStyle.Render(IconStopped + " Not Installed") + " " + HelpDescStyle.Render("(press 'i' to install or 'c' to auto-install)")
		}
		content.WriteString(status)
		content.WriteString("\n\n")
	}

	// Fields
	content.WriteString(SubtitleStyle.Render("Configuration"))
	content.WriteString("\n\n")

	for i, field := range w.fields {
		// Field label
		label := field.Label
		if field.Required {
			label += " *"
		}

		// Field value display
		var valueDisplay string
		if w.editing && i == w.selectedField {
			// Show edit buffer with cursor
			if field.Secret {
				valueDisplay = strings.Repeat("*", len(w.editBuffer)) + "█"
			} else {
				valueDisplay = w.editBuffer + "█"
			}
			valueDisplay = FocusedInputStyle.Render(valueDisplay)
		} else {
			if field.Value == "" {
				valueDisplay = HelpDescStyle.Render(field.Placeholder)
			} else if field.Secret {
				valueDisplay = strings.Repeat("*", len(field.Value))
			} else {
				valueDisplay = field.Value
			}
		}

		// Render field
		if i == w.selectedField {
			content.WriteString(SelectedItemStyle.Render(IconArrow + " " + label + ": "))
		} else {
			content.WriteString(ListItemStyle.Render("  " + label + ": "))
		}
		content.WriteString(valueDisplay)
		content.WriteString("\n")
	}

	// Error message
	if w.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(ErrorStyle.Render(IconCross + " " + w.errorMsg))
		content.WriteString("\n")
	}

	// Success message
	if w.successMsg != "" {
		content.WriteString("\n")
		content.WriteString(SuccessStyle.Render(IconCheck + " " + w.successMsg))
		content.WriteString("\n")
	}

	// Help
	content.WriteString("\n\n")
	content.WriteString(w.renderHelp())

	// Wrap in box
	boxWidth := w.width - 4
	if boxWidth > 80 {
		boxWidth = 80
	}
	box := PanelStyle.Width(boxWidth).Render(content.String())

	// Center on screen
	return lipgloss.Place(
		w.width,
		w.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// renderCompact renders a compact wizard view
func (w *Wizard) renderCompact() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render(w.providerName))
	content.WriteString("\n")

	// Show only selected field and neighbors
	start := w.selectedField - 1
	if start < 0 {
		start = 0
	}
	end := start + 3
	if end > len(w.fields) {
		end = len(w.fields)
	}

	for i := start; i < end; i++ {
		field := w.fields[i]
		label := field.Label
		if len(label) > 12 {
			label = label[:12]
		}

		var value string
		if w.editing && i == w.selectedField {
			value = w.editBuffer + "█"
		} else if field.Value != "" {
			value = field.Value
			if field.Secret {
				value = "***"
			}
		} else {
			value = "-"
		}

		if i == w.selectedField {
			content.WriteString(SelectedItemStyle.Render(IconArrow + label + ": " + value))
		} else {
			content.WriteString(ListItemStyle.Render(" " + label + ": " + value))
		}
		content.WriteString("\n")
	}

	if w.errorMsg != "" {
		content.WriteString(ErrorStyle.Render(w.errorMsg[:minInt(len(w.errorMsg), w.width-2)]))
		content.WriteString("\n")
	}

	content.WriteString(HelpDescStyle.Render("c:connect esc:cancel"))

	return content.String()
}

// renderHelp renders help text
func (w *Wizard) renderHelp() string {
	var help []string

	if w.installing {
		help = []string{
			HelpDescStyle.Render("Installing... please wait"),
		}
	} else if w.editing {
		help = []string{
			HelpKeyStyle.Render("Enter") + HelpDescStyle.Render(" save"),
			HelpKeyStyle.Render("Esc") + HelpDescStyle.Render(" cancel"),
		}
	} else {
		help = []string{
			HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" navigate"),
			HelpKeyStyle.Render("Enter") + HelpDescStyle.Render(" edit"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" connect"),
		}
		// Show install option if not installed
		if w.provider != nil && !w.provider.IsInstalled() {
			help = append(help, HelpKeyStyle.Render("i")+HelpDescStyle.Render(" install"))
		}
		help = append(help, HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" cancel"))
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		strings.Join(help, HelpSeparatorStyle.Render(" • ")),
	)
}

// SetError sets an error message
func (w *Wizard) SetError(msg string) {
	w.errorMsg = msg
}

// SetSuccess sets a success message
func (w *Wizard) SetSuccess(msg string) {
	w.successMsg = msg
}

// minInt returns the smaller of two integers (using minInt to avoid conflict with logs.go)
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
