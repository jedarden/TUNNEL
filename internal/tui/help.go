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
	width    int
	height   int
}

// NewHelp creates a new help instance
func NewHelp() *Help {
	return &Help{
		width:  80,
		height: 24,
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

// SetSize updates the help dimensions
func (h *Help) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// View renders the help overlay
func (h *Help) View() string {
	// Use compact view for small terminals
	if IsCompact(h.width, h.height) {
		return h.renderCompact()
	}

	// Use tiny view for very small terminals
	if IsTiny(h.width, h.height) {
		return h.renderTiny()
	}

	var content strings.Builder

	// Calculate available content width (leave room for padding and borders)
	contentWidth := h.width - 12
	if contentWidth > 90 {
		contentWidth = 90 // Cap max width for readability
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Title
	title := TitleStyle.Render("TUNNEL - Keyboard Shortcuts")
	content.WriteString(lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render(title))
	content.WriteString("\n\n")

	// Calculate how many sections we can fit
	availableHeight := h.height - 10 // Leave room for title, footer, padding
	linesUsed := 0

	// Render sections that fit
	for i, section := range h.sections {
		sectionContent := h.renderSection(section, contentWidth)
		sectionLines := strings.Count(sectionContent, "\n") + 1

		// Check if we have room for this section
		if linesUsed+sectionLines > availableHeight && linesUsed > 0 {
			content.WriteString(HelpDescStyle.Render("... (scroll with ↑/↓ or press ? to close)"))
			break
		}

		content.WriteString(sectionContent)
		linesUsed += sectionLines

		if i < len(h.sections)-1 && linesUsed < availableHeight {
			content.WriteString("\n")
			linesUsed++
		}
	}

	// Footer
	content.WriteString("\n\n")
	footer := HelpDescStyle.Render("Press ? or Esc to close")
	content.WriteString(lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render(footer))

	// Wrap in a box with dynamic width
	boxWidth := contentWidth + 8
	helpBox := PanelStyle.
		Width(boxWidth).
		Padding(1, 2).
		Render(content.String())

	// Center on screen using actual dimensions
	return lipgloss.Place(
		h.width,
		h.height,
		lipgloss.Center,
		lipgloss.Center,
		helpBox,
	)
}

// renderCompact renders a compact help view for small terminals
func (h *Help) renderCompact() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("Help"))
	content.WriteString("\n")

	// Show only essential shortcuts
	essentials := []HelpItem{
		{"1-5", "Switch view"},
		{"↑/↓", "Navigate"},
		{"Enter", "Select"},
		{"?", "Toggle help"},
		{"q", "Quit"},
	}

	for _, item := range essentials {
		key := HelpKeyStyle.Render(item.Key)
		desc := HelpDescStyle.Render(item.Description)
		content.WriteString(key + " " + desc + "\n")
	}

	content.WriteString(HelpDescStyle.Render("Press ? to close"))

	return content.String()
}

// renderTiny renders a minimal help view for very small terminals
func (h *Help) renderTiny() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("?:help"))
	content.WriteString(" ")
	content.WriteString(HelpDescStyle.Render("q:quit ↑↓:nav"))

	return content.String()
}

// renderSection renders a help section with given width constraint
func (h *Help) renderSection(section HelpSection, maxWidth int) string {
	var content strings.Builder

	// Section title
	content.WriteString(SubtitleStyle.Render(section.Title))
	content.WriteString("\n")

	// Calculate key column width (shorter for narrow terminals)
	keyWidth := 18
	if maxWidth < 60 {
		keyWidth = 12
	}

	// Section items
	for _, item := range section.Items {
		keyStyle := HelpKeyStyle.Render(item.Key)

		// Truncate description if needed
		desc := item.Description
		maxDescLen := maxWidth - keyWidth - 4
		if maxDescLen > 0 && len(desc) > maxDescLen {
			desc = desc[:maxDescLen-3] + "..."
		}
		descStyle := HelpDescStyle.Render(desc)

		// Pad key to align descriptions
		paddedKey := lipgloss.NewStyle().
			Width(keyWidth).
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
