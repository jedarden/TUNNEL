package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// FilterMode represents the current filter mode
type FilterMode int

const (
	FilterNone FilterMode = iota
	FilterByLevel
	FilterByProvider
)

// AggregatedLogEntry combines log entries with provider information
type AggregatedLogEntry struct {
	Timestamp   time.Time
	Level       LogLevel
	Provider    string
	Message     string
	OriginalLog providers.LogEntry
}

// Logs is the logs view model
type Logs struct {
	logs             []AggregatedLogEntry
	scrollOffset     int
	width            int
	height           int
	filterMode       FilterMode
	selectedFilter   int
	availableFilters []string
	activeFilter     string
	lastRefresh      time.Time
	autoRefresh      bool

	// Dependencies
	registry *registry.Registry
}

// NewLogs creates a new logs view instance
func NewLogs(reg *registry.Registry) *Logs {
	l := &Logs{
		logs:             []AggregatedLogEntry{},
		scrollOffset:     0,
		width:            80,
		height:           24,
		filterMode:       FilterNone,
		selectedFilter:   0,
		availableFilters: []string{},
		activeFilter:     "",
		lastRefresh:      time.Now(),
		autoRefresh:      true,
		registry:         reg,
	}

	// Load initial logs
	l.refreshLogs()

	return l
}

// Init initializes the logs view
func (l *Logs) Init() tea.Cmd {
	// Start auto-refresh timer
	return l.tickCmd()
}

// tickCmd returns a command that sends a tick message after 3 seconds
func (l *Logs) tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// SetSize updates the logs view dimensions
func (l *Logs) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// Update handles messages for the logs view
func (l *Logs) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if l.filterMode != FilterNone {
			return l.handleFilterInput(msg)
		}

		switch msg.String() {
		case "j", "down":
			maxScroll := l.getMaxScroll()
			if l.scrollOffset < maxScroll {
				l.scrollOffset++
			}

		case "k", "up":
			if l.scrollOffset > 0 {
				l.scrollOffset--
			}

		case "g":
			// Go to top
			l.scrollOffset = 0

		case "G":
			// Go to bottom
			l.scrollOffset = l.getMaxScroll()

		case "f":
			// Toggle filter mode
			l.enterFilterMode()

		case "c":
			// Clear logs
			l.logs = []AggregatedLogEntry{}
			l.scrollOffset = 0

		case "r":
			// Manual refresh
			l.refreshLogs()
			l.scrollOffset = l.getMaxScroll() // Scroll to bottom on refresh
		}

	case TickMsg:
		// Auto-refresh logs every 3 seconds
		if l.autoRefresh {
			l.refreshLogs()
			l.lastRefresh = time.Time(msg)
		}
		// Return the tick command to continue auto-refresh
		return l, l.tickCmd()
	}

	return l, nil
}

// View renders the logs view
func (l *Logs) View() string {
	if l.filterMode != FilterNone {
		return l.renderFilterMode()
	}

	// Use compact layout for small terminals
	if IsCompact(l.width, l.height) {
		return l.renderCompactView()
	}

	return l.renderFullView()
}

// renderFullView renders the full logs view
func (l *Logs) renderFullView() string {
	var content strings.Builder

	// Header
	header := l.renderHeader()
	content.WriteString(header)
	content.WriteString("\n")

	// Filter status
	if l.activeFilter != "" {
		filterStatus := l.renderFilterStatus()
		content.WriteString(filterStatus)
		content.WriteString("\n")
	}

	// Log entries
	logs := l.renderLogEntries()
	content.WriteString(logs)

	// Help text
	content.WriteString("\n")
	helpText := l.renderHelp()
	content.WriteString(helpText)

	return content.String()
}

// renderCompactView renders a compact logs view for small terminals
func (l *Logs) renderCompactView() string {
	var content strings.Builder

	// Compact header
	content.WriteString(TitleStyle.Render("Logs"))
	if l.activeFilter != "" {
		content.WriteString(" ")
		content.WriteString(HelpDescStyle.Render("[" + l.activeFilter + "]"))
	}
	content.WriteString("\n")

	// Calculate available height for logs
	availableHeight := l.height - 3 // Header + help line

	// Render visible logs
	visibleLogs := l.getVisibleLogs()
	displayCount := availableHeight
	if displayCount > len(visibleLogs) {
		displayCount = len(visibleLogs)
	}

	for i := 0; i < displayCount; i++ {
		entry := visibleLogs[i]
		line := l.formatLogEntryCompact(entry)
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Compact help
	content.WriteString(HelpDescStyle.Render("j/k:scroll f:filter r:refresh c:clear"))

	return content.String()
}

// renderHeader renders the logs header
func (l *Logs) renderHeader() string {
	title := TitleStyle.Render("System Logs")

	// Show refresh indicator
	refreshInfo := ""
	if l.autoRefresh {
		timeSinceRefresh := time.Since(l.lastRefresh)
		refreshInfo = HelpDescStyle.Render(fmt.Sprintf(" (auto-refresh: %ds ago)", int(timeSinceRefresh.Seconds())))
	}

	// Show log count
	logCount := HelpDescStyle.Render(fmt.Sprintf(" • %d entries", len(l.logs)))

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
		refreshInfo,
		logCount,
	)
}

// renderFilterStatus renders the active filter status
func (l *Logs) renderFilterStatus() string {
	filterText := fmt.Sprintf("Filter: %s", l.activeFilter)
	return InfoStyle.Render(filterText)
}

// renderLogEntries renders the log entries table
func (l *Logs) renderLogEntries() string {
	var content strings.Builder

	// Calculate available height for logs
	availableHeight := l.height - 8 // Account for header, help, etc.
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Get visible logs (with scrolling)
	visibleLogs := l.getVisibleLogs()

	// Render table header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	tableHeader := fmt.Sprintf("%-20s %-6s %-15s %s",
		headerStyle.Render("Time"),
		headerStyle.Render("Level"),
		headerStyle.Render("Provider"),
		headerStyle.Render("Message"),
	)
	content.WriteString(tableHeader)
	content.WriteString("\n")

	// Separator
	separator := strings.Repeat("─", l.width)
	content.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render(separator))
	content.WriteString("\n")

	// Render log entries
	displayCount := availableHeight
	if displayCount > len(visibleLogs) {
		displayCount = len(visibleLogs)
	}

	if len(visibleLogs) == 0 {
		content.WriteString(InfoStyle.Render("No log entries"))
		content.WriteString("\n")
	} else {
		for i := 0; i < displayCount; i++ {
			entry := visibleLogs[i]
			line := l.formatLogEntry(entry)
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(visibleLogs) > displayCount {
		remaining := len(visibleLogs) - displayCount
		scrollInfo := HelpDescStyle.Render(fmt.Sprintf("... %d more entries (scroll with j/k)", remaining))
		content.WriteString(scrollInfo)
		content.WriteString("\n")
	}

	return content.String()
}

// renderHelp renders help text for the logs view
func (l *Logs) renderHelp() string {
	help := []string{
		HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll"),
		HelpKeyStyle.Render("g/G") + HelpDescStyle.Render(" top/bottom"),
		HelpKeyStyle.Render("f") + HelpDescStyle.Render(" filter"),
		HelpKeyStyle.Render("c") + HelpDescStyle.Render(" clear"),
		HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh"),
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
	)
}

// formatLogEntry formats a single log entry for display
func (l *Logs) formatLogEntry(entry AggregatedLogEntry) string {
	// Format timestamp
	timeStr := entry.Timestamp.Format("15:04:05 01/02")

	// Format level with color
	levelStr := l.formatLevel(entry.Level)

	// Truncate provider name if needed
	providerStr := entry.Provider
	if len(providerStr) > 15 {
		providerStr = providerStr[:12] + "..."
	}

	// Truncate message if needed
	messageStr := entry.Message
	maxMessageLen := l.width - 50
	if maxMessageLen < 20 {
		maxMessageLen = 20
	}
	if len(messageStr) > maxMessageLen {
		messageStr = messageStr[:maxMessageLen-3] + "..."
	}

	return fmt.Sprintf("%-20s %-6s %-15s %s",
		HelpDescStyle.Render(timeStr),
		levelStr,
		ListItemStyle.Render(providerStr),
		entry.Message,
	)
}

// formatLogEntryCompact formats a log entry for compact display
func (l *Logs) formatLogEntryCompact(entry AggregatedLogEntry) string {
	// Compact format: [time] level provider: message
	timeStr := entry.Timestamp.Format("15:04")
	levelStr := l.formatLevelCompact(entry.Level)

	// Truncate message to fit terminal width
	maxLen := l.width - 20
	if maxLen < 10 {
		maxLen = 10
	}
	message := entry.Message
	if len(message) > maxLen {
		message = message[:maxLen-3] + "..."
	}

	return fmt.Sprintf("%s %s %s: %s",
		HelpDescStyle.Render(timeStr),
		levelStr,
		ListItemStyle.Render(entry.Provider[:min(len(entry.Provider), 8)]),
		message,
	)
}

// formatLevel formats a log level with color
func (l *Logs) formatLevel(level LogLevel) string {
	switch level {
	case LogLevelInfo:
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render("INFO ")
	case LogLevelWarn:
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("WARN ")
	case LogLevelError:
		return lipgloss.NewStyle().Foreground(ColorDanger).Render("ERROR")
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("UNKN ")
	}
}

// formatLevelCompact formats a log level with color for compact display
func (l *Logs) formatLevelCompact(level LogLevel) string {
	switch level {
	case LogLevelInfo:
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render("I")
	case LogLevelWarn:
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("W")
	case LogLevelError:
		return lipgloss.NewStyle().Foreground(ColorDanger).Render("E")
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("?")
	}
}

// getVisibleLogs returns logs visible based on scroll offset and filters
func (l *Logs) getVisibleLogs() []AggregatedLogEntry {
	// Apply filters
	filteredLogs := l.applyFilters()

	// Apply scrolling
	if l.scrollOffset >= len(filteredLogs) {
		l.scrollOffset = len(filteredLogs) - 1
		if l.scrollOffset < 0 {
			l.scrollOffset = 0
		}
	}

	return filteredLogs[l.scrollOffset:]
}

// applyFilters applies the active filter to logs
func (l *Logs) applyFilters() []AggregatedLogEntry {
	if l.activeFilter == "" {
		return l.logs
	}

	filtered := []AggregatedLogEntry{}
	for _, entry := range l.logs {
		// Filter by level
		if string(entry.Level) == strings.ToLower(l.activeFilter) {
			filtered = append(filtered, entry)
			continue
		}

		// Filter by provider
		if entry.Provider == l.activeFilter {
			filtered = append(filtered, entry)
			continue
		}
	}

	return filtered
}

// getMaxScroll returns the maximum scroll offset
func (l *Logs) getMaxScroll() int {
	availableHeight := l.height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	filteredLogs := l.applyFilters()
	maxScroll := len(filteredLogs) - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	return maxScroll
}

// refreshLogs fetches and aggregates logs from all providers
func (l *Logs) refreshLogs() {
	if l.registry == nil {
		return
	}

	// Fetch logs from the last hour
	since := time.Now().Add(-1 * time.Hour)

	// Get all providers
	allProviders := l.registry.ListProviders()

	// Aggregate logs from all providers
	aggregated := []AggregatedLogEntry{}
	for _, provider := range allProviders {
		providerLogs, err := provider.GetLogs(since)
		if err != nil {
			// Skip providers that return errors
			continue
		}

		// Convert to aggregated log entries
		for _, log := range providerLogs {
			entry := AggregatedLogEntry{
				Timestamp:   log.Timestamp,
				Level:       normalizeLogLevel(log.Level),
				Provider:    provider.Name(),
				Message:     log.Message,
				OriginalLog: log,
			}
			aggregated = append(aggregated, entry)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(aggregated, func(i, j int) bool {
		return aggregated[i].Timestamp.After(aggregated[j].Timestamp)
	})

	l.logs = aggregated
	l.updateAvailableFilters()
}

// normalizeLogLevel converts various log level strings to LogLevel
func normalizeLogLevel(level string) LogLevel {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "info", "information", "i":
		return LogLevelInfo
	case "warn", "warning", "w":
		return LogLevelWarn
	case "error", "err", "e", "fatal", "critical":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// updateAvailableFilters updates the list of available filters
func (l *Logs) updateAvailableFilters() {
	filterMap := make(map[string]bool)

	// Add log levels
	filterMap["info"] = true
	filterMap["warn"] = true
	filterMap["error"] = true

	// Add providers
	for _, entry := range l.logs {
		filterMap[entry.Provider] = true
	}

	// Convert to sorted slice
	filters := []string{}
	for filter := range filterMap {
		filters = append(filters, filter)
	}
	sort.Strings(filters)

	l.availableFilters = filters
}

// enterFilterMode enters filter selection mode
func (l *Logs) enterFilterMode() {
	l.filterMode = FilterByLevel
	l.selectedFilter = 0
	l.updateAvailableFilters()
}

// renderFilterMode renders the filter selection interface
func (l *Logs) renderFilterMode() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("Select Filter"))
	content.WriteString("\n\n")

	// Show available filters
	if len(l.availableFilters) == 0 {
		content.WriteString(InfoStyle.Render("No filters available"))
		content.WriteString("\n")
	} else {
		for i, filter := range l.availableFilters {
			filterText := filter

			// Add badge to indicate filter type
			if filter == "info" || filter == "warn" || filter == "error" {
				filterText += " " + HelpDescStyle.Render("(level)")
			} else {
				filterText += " " + HelpDescStyle.Render("(provider)")
			}

			if i == l.selectedFilter {
				content.WriteString(SelectedItemStyle.Render(IconArrow + " " + filterText))
			} else {
				content.WriteString(ListItemStyle.Render("  " + filterText))
			}
			content.WriteString("\n")
		}
	}

	// Help
	content.WriteString("\n")
	content.WriteString(HelpDescStyle.Render("↑/↓: navigate, Enter: apply, Esc: cancel, x: clear filter"))

	return content.String()
}

// handleFilterInput handles keyboard input in filter mode
func (l *Logs) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		l.filterMode = FilterNone
		return l, nil

	case "x":
		// Clear filter
		l.activeFilter = ""
		l.filterMode = FilterNone
		l.scrollOffset = 0
		return l, nil

	case "enter":
		// Apply selected filter
		if l.selectedFilter >= 0 && l.selectedFilter < len(l.availableFilters) {
			l.activeFilter = l.availableFilters[l.selectedFilter]
		}
		l.filterMode = FilterNone
		l.scrollOffset = 0
		return l, nil

	case "up", "k":
		if l.selectedFilter > 0 {
			l.selectedFilter--
		}

	case "down", "j":
		if l.selectedFilter < len(l.availableFilters)-1 {
			l.selectedFilter++
		}
	}

	return l, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
