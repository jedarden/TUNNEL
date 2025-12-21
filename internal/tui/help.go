package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpSection represents a section in the help screen
type HelpSection struct {
	Title string
	Items []HelpItem
}

// HelpItem represents a single help item
type HelpItem struct {
	Key         string
	Description string
}

// Help is the help overlay model
type Help struct {
	sections []HelpSection
}

// NewHelp creates a new help instance
func NewHelp() *Help {
	return &Help{
		sections: []HelpSection{
			{
				Title: "Navigation",
				Items: []HelpItem{
					{"1-5", "Switch to specific view (Dashboard, Browser, Config, Logs, Monitor)"},
					{"Tab", "Next view"},
					{"Shift+Tab", "Previous view"},
					{"↑/↓ or k/j", "Navigate up/down in lists"},
					{"←/→ or h/l", "Navigate left/right (where applicable)"},
					{"Enter", "Select/Activate item"},
					{"Esc", "Go back/Cancel"},
				},
			},
			{
				Title: "Dashboard View",
				Items: []HelpItem{
					{"↑/↓ or k/j", "Navigate quick actions"},
					{"Enter", "Execute selected action"},
					{"1", "Connect to new method (opens Browser)"},
					{"2", "View all connections (opens Monitor)"},
					{"3", "Configure settings (opens Config)"},
				},
			},
			{
				Title: "Browser View",
				Items: []HelpItem{
					{"←/→ or h/l", "Switch between categories"},
					{"↑/↓ or k/j", "Select connection method"},
					{"/", "Search for methods"},
					{"Enter", "Connect using selected method"},
					{"Esc", "Exit search mode"},
				},
			},
			{
				Title: "Search Mode",
				Items: []HelpItem{
					{"Type", "Enter search query"},
					{"Backspace", "Delete character"},
					{"Enter", "Accept search"},
					{"Esc", "Cancel search"},
				},
			},
			{
				Title: "General",
				Items: []HelpItem{
					{"?", "Toggle this help screen"},
					{"q or Ctrl+C", "Quit application"},
				},
			},
			{
				Title: "Connection Methods",
				Items: []HelpItem{
					{"★", "Indicates recommended method"},
					{"●", "Connected status"},
					{"◐", "Ready status"},
					{"○", "Stopped status"},
				},
			},
		},
	}
}

// View renders the help overlay
func (h *Help) View() string {
	var content strings.Builder

	// Title
	title := TitleStyle.Render("TUNNEL - Keyboard Shortcuts")
	content.WriteString(lipgloss.NewStyle().
		Width(100).
		Align(lipgloss.Center).
		Render(title))
	content.WriteString("\n\n")

	// Render each section
	for i, section := range h.sections {
		content.WriteString(h.renderSection(section))
		if i < len(h.sections)-1 {
			content.WriteString("\n")
		}
	}

	// Footer
	content.WriteString("\n\n")
	footer := HelpDescStyle.Render("Press ? or Esc to close this help screen")
	content.WriteString(lipgloss.NewStyle().
		Width(100).
		Align(lipgloss.Center).
		Render(footer))

	// Wrap in a box
	helpBox := PanelStyle.
		Width(100).
		Padding(2, 4).
		Render(content.String())

	// Center on screen
	return lipgloss.Place(
		120,
		40,
		lipgloss.Center,
		lipgloss.Center,
		helpBox,
	)
}

// renderSection renders a help section
func (h *Help) renderSection(section HelpSection) string {
	var content strings.Builder

	// Section title
	content.WriteString(SubtitleStyle.Render(section.Title))
	content.WriteString("\n")

	// Section items
	for _, item := range section.Items {
		keyStyle := HelpKeyStyle.Render(item.Key)
		descStyle := HelpDescStyle.Render(item.Description)

		// Pad key to align descriptions
		paddedKey := lipgloss.NewStyle().
			Width(20).
			Render(keyStyle)

		line := lipgloss.JoinHorizontal(
			lipgloss.Left,
			paddedKey,
			descStyle,
		)
		content.WriteString("  " + line)
		content.WriteString("\n")
	}

	return content.String()
}

// GetContextHelp returns context-sensitive help for the current view
func (h *Help) GetContextHelp(viewMode ViewMode) []HelpItem {
	switch viewMode {
	case ViewDashboard:
		return h.sections[1].Items // Dashboard help
	case ViewBrowser:
		return h.sections[2].Items // Browser help
	default:
		return h.sections[0].Items // Navigation help
	}
}
