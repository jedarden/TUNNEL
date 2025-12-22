package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette for dark theme
var (
	ColorPrimary   = lipgloss.Color("#7D56F4")
	ColorSecondary = lipgloss.Color("#7C3AED")
	ColorSuccess   = lipgloss.Color("#10B981")
	ColorWarning   = lipgloss.Color("#F59E0B")
	ColorDanger    = lipgloss.Color("#EF4444")
	ColorInfo      = lipgloss.Color("#3B82F6")
	ColorMuted     = lipgloss.Color("#6B7280")
	ColorText      = lipgloss.Color("#E5E7EB")
	ColorBorder    = lipgloss.Color("#4B5563")
	ColorBg        = lipgloss.Color("#1F2937")
	ColorBgLight   = lipgloss.Color("#374151")
)

// Status colors
var (
	StatusConnected = ColorSuccess
	StatusReady     = ColorWarning
	StatusStopped   = ColorDanger
	StatusUnknown   = ColorMuted
)

// Base styles
var (
	BaseStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorBg)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Italic(true).
			Padding(0, 1)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			MarginRight(1).
			MarginBottom(1)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			MarginRight(1).
			MarginBottom(1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2).
				MarginRight(1).
				MarginBottom(1)

	// List styles
	ListItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				PaddingLeft(1).
				MarginLeft(0)

	// Status styles
	StatusStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	StatusConnectedStyle = StatusStyle.
				Foreground(StatusConnected)

	StatusReadyStyle = StatusStyle.
				Foreground(StatusReady)

	StatusStoppedStyle = StatusStyle.
				Foreground(ColorDanger)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Background(ColorBgLight).
			Bold(true).
			Padding(0, 1).
			Width(100)

	// Help styles
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	// Tab styles
	TabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorMuted)

	ActiveTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorPrimary).
			Bold(true).
			Underline(true)

	// Badge styles
	BadgeStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 1).
			Bold(true)

	WarningBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorWarning).
				Padding(0, 1).
				Bold(true)

	DangerBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorDanger).
				Padding(0, 1).
				Bold(true)

	SuccessBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorSuccess).
				Padding(0, 1).
				Bold(true)

	// Icon styles
	IconStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Input styles
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	FocusedInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	// Message styles
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// Footer styles
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(1, 2).
			BorderTop(true).
			BorderForeground(ColorBorder)
)

// Status icons
const (
	IconConnected = "●"
	IconReady     = "◐"
	IconStopped   = "○"
	IconStar      = "★"
	IconArrow     = "→"
	IconCheck     = "✓"
	IconCross     = "✗"
	IconWarning   = "⚠"
	IconInfo      = "ℹ"
)

// RenderStatus returns a styled status indicator
func RenderStatus(status string) string {
	switch status {
	case "connected":
		return StatusConnectedStyle.Render(IconConnected + " Connected")
	case "ready":
		return StatusReadyStyle.Render(IconReady + " Ready")
	case "stopped":
		return StatusStoppedStyle.Render(IconStopped + " Stopped")
	default:
		return StatusStyle.Foreground(StatusUnknown).Render(IconStopped + " Unknown")
	}
}

// RenderBadge returns a styled badge
func RenderBadge(text string, badgeType string) string {
	switch badgeType {
	case "success":
		return SuccessBadgeStyle.Render(text)
	case "warning":
		return WarningBadgeStyle.Render(text)
	case "danger":
		return DangerBadgeStyle.Render(text)
	default:
		return BadgeStyle.Render(text)
	}
}

// RenderIcon returns a styled icon
func RenderIcon(icon string) string {
	return IconStyle.Render(icon)
}

// RenderListItem returns a styled list item
func RenderListItem(text string, selected bool) string {
	if selected {
		return SelectedItemStyle.Render(IconArrow + " " + text)
	}
	return ListItemStyle.Render("  " + text)
}
