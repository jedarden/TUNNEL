package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	ColorPrimary = lipgloss.Color("#7D56F4")
	ColorSuccess = lipgloss.Color("#10B981")
	ColorWarning = lipgloss.Color("#F59E0B")
	ColorDanger  = lipgloss.Color("#EF4444")
	ColorInfo    = lipgloss.Color("#3B82F6")
	ColorMuted   = lipgloss.Color("#6B7280")
	ColorText    = lipgloss.Color("#E5E7EB")
	ColorBorder  = lipgloss.Color("#4B5563")
)

// Styles used by minimal TUI
var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	StatusConnectedStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StatusReadyStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	StatusStoppedStyle = lipgloss.NewStyle().
				Foreground(ColorDanger).
				Bold(true)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)
)

// Status icons
const (
	IconConnected = "●"
	IconReady     = "◐"
	IconStopped   = "○"
	IconCross     = "✗"
)
