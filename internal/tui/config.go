package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/pkg/config"
)

// ConfigSection represents different configuration sections
type ConfigSection int

const (
	SectionGeneral ConfigSection = iota
	SectionFailover
	SectionMetrics
	SectionProviders
	SectionSSH
	SectionMonitoring
)

// Config is the configuration view model
type Config struct {
	width  int
	height int

	// Current state
	selectedSection ConfigSection
	selectedField   int
	editMode        bool
	editValue       string
	showSaveDialog  bool
	saveMessage     string

	// Configuration data
	appConfig     *config.Config
	managerConfig *core.ManagerConfig

	// Field definitions for each section
	sections []configSection
}

// configSection represents a configuration section with fields
type configSection struct {
	Name   string
	Fields []configField
}

// configField represents a single configuration field
type configField struct {
	Label       string
	Value       string
	Description string
	Editable    bool
	FieldType   string // "string", "int", "bool", "duration"
	OnChange    func(value string) error
}

// NewConfig creates a new configuration view
func NewConfig(appCfg *config.Config, mgrCfg *core.ManagerConfig) *Config {
	c := &Config{
		width:           80,
		height:          24,
		selectedSection: SectionGeneral,
		selectedField:   0,
		editMode:        false,
		editValue:       "",
		showSaveDialog:  false,
		saveMessage:     "",
		appConfig:       appCfg,
		managerConfig:   mgrCfg,
	}

	// Initialize sections
	c.buildSections()

	return c
}

// buildSections constructs the configuration sections based on current config
func (c *Config) buildSections() {
	c.sections = []configSection{
		// General Settings
		{
			Name: "General",
			Fields: []configField{
				{
					Label:       "Default Method",
					Value:       c.appConfig.Settings.DefaultMethod,
					Description: "Default connection method to use",
					Editable:    true,
					FieldType:   "string",
					OnChange: func(v string) error {
						c.appConfig.Settings.DefaultMethod = v
						return nil
					},
				},
				{
					Label:       "Auto Reconnect",
					Value:       formatBool(c.appConfig.Settings.AutoReconnect),
					Description: "Automatically reconnect on disconnect",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.appConfig.Settings.AutoReconnect = val
						return nil
					},
				},
				{
					Label:       "Log Level",
					Value:       c.appConfig.Settings.LogLevel,
					Description: "Logging level (debug, info, warn, error)",
					Editable:    true,
					FieldType:   "string",
					OnChange: func(v string) error {
						c.appConfig.Settings.LogLevel = v
						return nil
					},
				},
				{
					Label:       "Theme",
					Value:       c.appConfig.Settings.Theme,
					Description: "UI theme (default, dark, light)",
					Editable:    true,
					FieldType:   "string",
					OnChange: func(v string) error {
						c.appConfig.Settings.Theme = v
						return nil
					},
				},
			},
		},

		// Failover Settings
		{
			Name: "Failover",
			Fields: []configField{
				{
					Label:       "Enabled",
					Value:       formatBool(c.managerConfig.FailoverConfig.Enabled),
					Description: "Enable automatic failover",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.managerConfig.FailoverConfig.Enabled = val
						return nil
					},
				},
				{
					Label:       "Health Check Interval",
					Value:       c.managerConfig.FailoverConfig.HealthCheckInterval.String(),
					Description: "How often to check connection health",
					Editable:    true,
					FieldType:   "duration",
					OnChange: func(v string) error {
						dur, err := time.ParseDuration(v)
						if err != nil {
							return err
						}
						c.managerConfig.FailoverConfig.HealthCheckInterval = dur
						return nil
					},
				},
				{
					Label:       "Failure Threshold",
					Value:       strconv.Itoa(c.managerConfig.FailoverConfig.FailureThreshold),
					Description: "Failures before triggering failover",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(v)
						if err != nil {
							return err
						}
						c.managerConfig.FailoverConfig.FailureThreshold = val
						return nil
					},
				},
				{
					Label:       "Recovery Threshold",
					Value:       strconv.Itoa(c.managerConfig.FailoverConfig.RecoveryThreshold),
					Description: "Successes before marking as recovered",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(v)
						if err != nil {
							return err
						}
						c.managerConfig.FailoverConfig.RecoveryThreshold = val
						return nil
					},
				},
				{
					Label:       "Max Latency",
					Value:       c.managerConfig.FailoverConfig.MaxLatency.String(),
					Description: "Maximum acceptable latency",
					Editable:    true,
					FieldType:   "duration",
					OnChange: func(v string) error {
						dur, err := time.ParseDuration(v)
						if err != nil {
							return err
						}
						c.managerConfig.FailoverConfig.MaxLatency = dur
						return nil
					},
				},
				{
					Label:       "Auto Recover",
					Value:       formatBool(c.managerConfig.FailoverConfig.AutoRecover),
					Description: "Auto switch back on recovery",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.managerConfig.FailoverConfig.AutoRecover = val
						return nil
					},
				},
			},
		},

		// Metrics Settings
		{
			Name: "Metrics",
			Fields: []configField{
				{
					Label:       "Enabled",
					Value:       formatBool(c.managerConfig.EnableMetrics),
					Description: "Enable metrics collection",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.managerConfig.EnableMetrics = val
						return nil
					},
				},
				{
					Label:       "Collection Interval",
					Value:       c.managerConfig.MetricsInterval.String(),
					Description: "How often to collect metrics",
					Editable:    true,
					FieldType:   "duration",
					OnChange: func(v string) error {
						dur, err := time.ParseDuration(v)
						if err != nil {
							return err
						}
						c.managerConfig.MetricsInterval = dur
						return nil
					},
				},
			},
		},

		// SSH Settings
		{
			Name: "SSH",
			Fields: []configField{
				{
					Label:       "Port",
					Value:       strconv.Itoa(c.appConfig.SSH.Port),
					Description: "SSH server port",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(v)
						if err != nil {
							return err
						}
						c.appConfig.SSH.Port = val
						return nil
					},
				},
				{
					Label:       "Max Sessions",
					Value:       strconv.Itoa(c.appConfig.SSH.MaxSessions),
					Description: "Maximum concurrent SSH sessions",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(v)
						if err != nil {
							return err
						}
						c.appConfig.SSH.MaxSessions = val
						return nil
					},
				},
				{
					Label:       "Idle Timeout",
					Value:       fmt.Sprintf("%ds", c.appConfig.SSH.IdleTimeout),
					Description: "Idle timeout in seconds",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(strings.TrimSuffix(v, "s"))
						if err != nil {
							return err
						}
						c.appConfig.SSH.IdleTimeout = val
						return nil
					},
				},
				{
					Label:       "Keep Alive",
					Value:       fmt.Sprintf("%ds", c.appConfig.SSH.KeepAlive),
					Description: "Keep alive interval in seconds",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(strings.TrimSuffix(v, "s"))
						if err != nil {
							return err
						}
						c.appConfig.SSH.KeepAlive = val
						return nil
					},
				},
				{
					Label:       "TCP Forwarding",
					Value:       formatBool(c.appConfig.SSH.AllowTCPForwarding),
					Description: "Allow TCP forwarding",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.appConfig.SSH.AllowTCPForwarding = val
						return nil
					},
				},
				{
					Label:       "Agent Forwarding",
					Value:       formatBool(c.appConfig.SSH.AllowAgentForwarding),
					Description: "Allow agent forwarding",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.appConfig.SSH.AllowAgentForwarding = val
						return nil
					},
				},
			},
		},

		// Monitoring Settings
		{
			Name: "Monitoring",
			Fields: []configField{
				{
					Label:       "Enabled",
					Value:       formatBool(c.appConfig.Monitoring.Enabled),
					Description: "Enable monitoring",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.appConfig.Monitoring.Enabled = val
						return nil
					},
				},
				{
					Label:       "Metrics Enabled",
					Value:       formatBool(c.appConfig.Monitoring.MetricsEnabled),
					Description: "Enable metrics export",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.appConfig.Monitoring.MetricsEnabled = val
						return nil
					},
				},
				{
					Label:       "Metrics Port",
					Value:       strconv.Itoa(c.appConfig.Monitoring.MetricsPort),
					Description: "Port for metrics server",
					Editable:    true,
					FieldType:   "int",
					OnChange: func(v string) error {
						val, err := strconv.Atoi(v)
						if err != nil {
							return err
						}
						c.appConfig.Monitoring.MetricsPort = val
						return nil
					},
				},
				{
					Label:       "Syslog",
					Value:       formatBool(c.appConfig.Monitoring.Syslog),
					Description: "Enable syslog",
					Editable:    true,
					FieldType:   "bool",
					OnChange: func(v string) error {
						val, err := parseBool(v)
						if err != nil {
							return err
						}
						c.appConfig.Monitoring.Syslog = val
						return nil
					},
				},
			},
		},

		// Providers (read-only for now)
		{
			Name: "Providers",
			Fields: c.buildProviderFields(),
		},
	}
}

// buildProviderFields creates fields for provider configuration
func (c *Config) buildProviderFields() []configField {
	var fields []configField

	// Get enabled methods
	enabledMethods := c.appConfig.GetEnabledMethods()

	if len(enabledMethods) == 0 {
		fields = append(fields, configField{
			Label:       "No providers",
			Value:       "No enabled providers",
			Description: "Configure providers in the config file",
			Editable:    false,
		})
		return fields
	}

	for _, method := range enabledMethods {
		cfg, ok := c.appConfig.GetMethod(method)
		if !ok {
			continue
		}

		fields = append(fields, configField{
			Label:       method,
			Value:       fmt.Sprintf("Priority: %d, Enabled: %v", cfg.Priority, cfg.Enabled),
			Description: "Provider configuration",
			Editable:    false,
		})
	}

	return fields
}

// Init initializes the config view
func (c *Config) Init() tea.Cmd {
	return nil
}

// SetSize updates the config view dimensions
func (c *Config) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// Update handles messages for the config view
func (c *Config) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle edit mode separately
		if c.editMode {
			return c.handleEditInput(msg)
		}

		// Handle save dialog
		if c.showSaveDialog {
			switch msg.String() {
			case "y", "Y":
				c.saveConfig()
				c.showSaveDialog = false
				return c, nil
			case "n", "N", "esc":
				c.showSaveDialog = false
				return c, nil
			}
			return c, nil
		}

		// Normal navigation
		switch msg.String() {
		case "up", "k":
			if c.selectedField > 0 {
				c.selectedField--
			}

		case "down", "j":
			section := c.sections[c.selectedSection]
			if c.selectedField < len(section.Fields)-1 {
				c.selectedField++
			}

		case "left", "h":
			if c.selectedSection > 0 {
				c.selectedSection--
				c.selectedField = 0
			}

		case "right", "l":
			if int(c.selectedSection) < len(c.sections)-1 {
				c.selectedSection++
				c.selectedField = 0
			}

		case "tab":
			// Move to next section
			if int(c.selectedSection) < len(c.sections)-1 {
				c.selectedSection++
				c.selectedField = 0
			}

		case "shift+tab":
			// Move to previous section
			if c.selectedSection > 0 {
				c.selectedSection--
				c.selectedField = 0
			}

		case "enter":
			// Start editing the selected field
			section := c.sections[c.selectedSection]
			if c.selectedField < len(section.Fields) {
				field := section.Fields[c.selectedField]
				if field.Editable {
					c.editMode = true
					c.editValue = field.Value
				}
			}

		case "s", "S":
			// Show save confirmation
			c.showSaveDialog = true

		case "r", "R":
			// Reload/refresh config from file
			c.buildSections()
		}
	}

	return c, nil
}

// handleEditInput handles keyboard input in edit mode
func (c *Config) handleEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel editing
		c.editMode = false
		c.editValue = ""
		return c, nil

	case "enter":
		// Save the edited value
		section := &c.sections[c.selectedSection]
		if c.selectedField < len(section.Fields) {
			field := &section.Fields[c.selectedField]
			if err := field.OnChange(c.editValue); err != nil {
				c.saveMessage = fmt.Sprintf("Error: %v", err)
			} else {
				field.Value = c.editValue
				c.saveMessage = "Value updated (press 's' to save)"
			}
		}
		c.editMode = false
		c.editValue = ""
		return c, nil

	case "backspace":
		if len(c.editValue) > 0 {
			c.editValue = c.editValue[:len(c.editValue)-1]
		}

	default:
		// Add character to edit value
		if len(msg.String()) == 1 {
			c.editValue += msg.String()
		}
	}

	return c, nil
}

// saveConfig saves the configuration
func (c *Config) saveConfig() {
	if err := c.appConfig.Save(); err != nil {
		c.saveMessage = fmt.Sprintf("Error saving config: %v", err)
	} else {
		c.saveMessage = "Configuration saved successfully!"
	}
}

// View renders the config view
func (c *Config) View() string {
	// Show save dialog if active
	if c.showSaveDialog {
		return c.renderSaveDialog()
	}

	// Use compact layout for small terminals
	if IsCompact(c.width, c.height) {
		return c.renderCompactView()
	}

	// Use tiny layout for very small terminals
	if IsTiny(c.width, c.height) {
		return c.renderTinyView()
	}

	var content strings.Builder

	// Header
	content.WriteString(TitleStyle.Render("Configuration Settings"))
	content.WriteString("\n\n")

	// Section tabs
	content.WriteString(c.renderSectionTabs())
	content.WriteString("\n\n")

	// Current section content
	content.WriteString(c.renderSection())

	// Help text
	content.WriteString("\n\n")
	content.WriteString(c.renderHelp())

	// Save message if any
	if c.saveMessage != "" {
		content.WriteString("\n")
		if strings.HasPrefix(c.saveMessage, "Error") {
			content.WriteString(ErrorStyle.Render(c.saveMessage))
		} else {
			content.WriteString(SuccessStyle.Render(c.saveMessage))
		}
	}

	return content.String()
}

// renderTinyView renders a minimal view for very small terminals
func (c *Config) renderTinyView() string {
	var b strings.Builder

	section := c.sections[c.selectedSection]

	// Section name with navigation
	b.WriteString(TitleStyle.Render(section.Name))
	b.WriteString(" ")
	b.WriteString(HelpDescStyle.Render(fmt.Sprintf("(%d/%d)", int(c.selectedSection)+1, len(c.sections))))
	b.WriteString("\n")

	// Current field
	if c.selectedField < len(section.Fields) {
		field := section.Fields[c.selectedField]

		if c.editMode {
			b.WriteString(SelectedItemStyle.Render(field.Label + ": "))
			b.WriteString(FocusedInputStyle.Render(c.editValue + "█"))
		} else {
			b.WriteString(field.Label + ": ")
			b.WriteString(InfoStyle.Render(field.Value))
		}
		b.WriteString("\n")

		// Show field counter
		b.WriteString(HelpDescStyle.Render(fmt.Sprintf("(%d/%d)", c.selectedField+1, len(section.Fields))))
	}

	b.WriteString("\n")
	b.WriteString(HelpDescStyle.Render("←→:sec ↑↓:fld ⏎:edit s:save"))

	return b.String()
}

// renderCompactView renders a compact view for small terminals
func (c *Config) renderCompactView() string {
	var b strings.Builder

	section := c.sections[c.selectedSection]

	// Section name
	b.WriteString(TitleStyle.Render(section.Name + " Settings"))
	b.WriteString("\n\n")

	// Show fields (limited to fit screen)
	maxFields := c.height - 6
	if maxFields < 3 {
		maxFields = 3
	}

	startIdx := 0
	endIdx := len(section.Fields)

	// Scroll to show selected field
	if len(section.Fields) > maxFields {
		if c.selectedField >= maxFields {
			startIdx = c.selectedField - maxFields + 1
		}
		endIdx = startIdx + maxFields
		if endIdx > len(section.Fields) {
			endIdx = len(section.Fields)
		}
	}

	for i := startIdx; i < endIdx; i++ {
		field := section.Fields[i]

		// Truncate label and value to fit
		label := field.Label
		value := field.Value
		maxLen := c.width - 4

		if len(label)+len(value) > maxLen {
			if len(label) > maxLen/2 {
				label = label[:maxLen/2-2] + ".."
			}
			if len(value) > maxLen/2 {
				value = value[:maxLen/2-2] + ".."
			}
		}

		if i == c.selectedField {
			if c.editMode {
				b.WriteString(SelectedItemStyle.Render(IconArrow + " " + label + ": "))
				b.WriteString(FocusedInputStyle.Render(c.editValue + "█"))
			} else {
				b.WriteString(SelectedItemStyle.Render(IconArrow + " " + label + ": " + value))
			}
		} else {
			b.WriteString(ListItemStyle.Render("  " + label + ": " + HelpDescStyle.Render(value)))
		}
		b.WriteString("\n")
	}

	// Compact help
	b.WriteString("\n")
	b.WriteString(HelpDescStyle.Render("Tab:section ↑↓:navigate Enter:edit Esc:cancel s:save"))

	return b.String()
}

// renderSectionTabs renders the section navigation tabs
func (c *Config) renderSectionTabs() string {
	var tabs []string

	for i, section := range c.sections {
		tabText := section.Name
		if ConfigSection(i) == c.selectedSection {
			tabs = append(tabs, ActiveTabStyle.Render(tabText))
		} else {
			tabs = append(tabs, TabStyle.Render(tabText))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
}

// renderSection renders the current section content
func (c *Config) renderSection() string {
	var b strings.Builder

	section := c.sections[c.selectedSection]

	// Section description
	sectionDesc := c.getSectionDescription(c.selectedSection)
	if sectionDesc != "" {
		b.WriteString(HelpDescStyle.Render(sectionDesc))
		b.WriteString("\n\n")
	}

	// Fields
	for i, field := range section.Fields {
		// Field label and value
		isSelected := i == c.selectedField

		if isSelected {
			b.WriteString(SelectedItemStyle.Render(IconArrow + " " + field.Label))
		} else {
			b.WriteString(ListItemStyle.Render("  " + field.Label))
		}
		b.WriteString("\n")

		// Value (with edit indicator if in edit mode)
		valueStr := "  "
		if isSelected && c.editMode {
			valueStr += FocusedInputStyle.Render(c.editValue + "█")
		} else {
			if field.Editable {
				valueStr += InfoStyle.Render(field.Value)
			} else {
				valueStr += HelpDescStyle.Render(field.Value)
			}
		}
		b.WriteString(valueStr)
		b.WriteString("\n")

		// Description
		if isSelected && field.Description != "" {
			b.WriteString("  ")
			b.WriteString(HelpDescStyle.Render(field.Description))
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	return b.String()
}

// getSectionDescription returns a description for a section
func (c *Config) getSectionDescription(section ConfigSection) string {
	descriptions := map[ConfigSection]string{
		SectionGeneral:    "General application settings",
		SectionFailover:   "Automatic failover configuration",
		SectionMetrics:    "Metrics collection settings",
		SectionProviders:  "Connection provider configuration",
		SectionSSH:        "SSH server settings",
		SectionMonitoring: "Monitoring and logging settings",
	}
	return descriptions[section]
}

// renderHelp renders help text
func (c *Config) renderHelp() string {
	if c.editMode {
		return HelpDescStyle.Render("Type to edit, Enter to save, Esc to cancel")
	}

	help := []string{
		HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" navigate"),
		HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" section"),
		HelpKeyStyle.Render("Tab") + HelpDescStyle.Render(" next section"),
		HelpKeyStyle.Render("Enter") + HelpDescStyle.Render(" edit"),
		HelpKeyStyle.Render("s") + HelpDescStyle.Render(" save"),
		HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reload"),
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
		HelpSeparatorStyle.Render(" • "),
		help[4],
		HelpSeparatorStyle.Render(" • "),
		help[5],
	)
}

// renderSaveDialog renders the save confirmation dialog
func (c *Config) renderSaveDialog() string {
	dialogContent := lipgloss.JoinVertical(
		lipgloss.Center,
		TitleStyle.Render("Save Configuration?"),
		"",
		InfoStyle.Render("This will save all changes to the config file."),
		"",
		HelpDescStyle.Render("Press 'y' to save, 'n' or 'Esc' to cancel"),
	)

	// Center the dialog
	width := 50
	height := 8

	dialog := BoxStyle.
		Width(width).
		Height(height).
		Render(dialogContent)

	return lipgloss.Place(
		c.width,
		c.height-6,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)
}

// Helper functions

func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func parseBool(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "yes", "y", "1", "on":
		return true, nil
	case "false", "no", "n", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}
