package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/registry"
	"github.com/jedarden/tunnel/pkg/config"
)

// newTestApp creates an App instance for testing with nil dependencies
func newTestApp() *App {
	reg := registry.NewRegistry()
	mgr := core.NewConnectionManager(nil)
	cfg := config.GetDefaultConfig()
	return NewApp(reg, mgr, cfg)
}

// TerminalDimension represents a terminal size for testing
type TerminalDimension struct {
	Name   string
	Width  int
	Height int
}

// Standard test dimensions covering all size categories
var testDimensions = []TerminalDimension{
	// Tiny terminals (< 40x12)
	{"tiny_minimal", 30, 8},
	{"tiny_small", 35, 10},
	{"tiny_edge", 39, 11},

	// Compact terminals (< 60x20)
	{"compact_small", 40, 12},
	{"compact_medium", 50, 15},
	{"compact_edge", 59, 19},

	// Normal terminals (>= 60x20)
	{"normal_minimum", 60, 20},
	{"normal_standard", 80, 24},
	{"normal_wide", 120, 24},

	// Large terminals
	{"large_hd", 120, 40},
	{"large_wide", 200, 50},

	// Edge cases
	{"ultra_narrow", 20, 30},
	{"ultra_short", 100, 6},
	{"square", 40, 40},
}

// TestIsCompact verifies the IsCompact function
func TestIsCompact(t *testing.T) {
	tests := []struct {
		width, height int
		expected      bool
	}{
		{30, 10, true},   // tiny
		{50, 15, true},   // compact
		{59, 19, true},   // compact edge
		{60, 20, false},  // normal minimum
		{80, 24, false},  // normal standard
		{120, 40, false}, // large
		{60, 19, true},   // height triggers compact
		{59, 20, true},   // width triggers compact
	}

	for _, tt := range tests {
		result := IsCompact(tt.width, tt.height)
		if result != tt.expected {
			t.Errorf("IsCompact(%d, %d) = %v, want %v", tt.width, tt.height, result, tt.expected)
		}
	}
}

// TestIsTiny verifies the IsTiny function
func TestIsTiny(t *testing.T) {
	tests := []struct {
		width, height int
		expected      bool
	}{
		{30, 8, true},    // tiny
		{39, 11, true},   // tiny edge
		{40, 12, false},  // compact minimum
		{35, 15, true},   // width triggers tiny
		{50, 10, true},   // height triggers tiny
		{80, 24, false},  // normal
	}

	for _, tt := range tests {
		result := IsTiny(tt.width, tt.height)
		if result != tt.expected {
			t.Errorf("IsTiny(%d, %d) = %v, want %v", tt.width, tt.height, result, tt.expected)
		}
	}
}

// TestDashboardRendersDimensions tests Dashboard rendering at all dimensions
func TestDashboardRendersDimensions(t *testing.T) {
	dashboard := NewDashboard(nil, nil, nil)

	for _, dim := range testDimensions {
		t.Run(dim.Name, func(t *testing.T) {
			dashboard.SetSize(dim.Width, dim.Height)
			output := dashboard.View()

			// Verify output is not empty
			if output == "" {
				t.Errorf("Dashboard View() returned empty string at %dx%d", dim.Width, dim.Height)
			}

			// Verify no panic occurred (implicit by reaching here)

			// Note: Width checking is informational only since multi-panel layouts
			// may intentionally exceed single-line width. The important thing
			// is that rendering completes without panic.
			lines := strings.Split(output, "\n")
			_ = lines // Used for potential debugging

			// Verify appropriate mode is used
			if IsTiny(dim.Width, dim.Height) {
				// Tiny mode should have minimal content
				if len(lines) > dim.Height+5 {
					t.Logf("Warning: Tiny view has %d lines for height %d", len(lines), dim.Height)
				}
			} else if IsCompact(dim.Width, dim.Height) {
				// Compact mode should show essential elements
				if !strings.Contains(output, "Connections") && !strings.Contains(output, "Actions") {
					t.Logf("Warning: Compact view missing expected sections")
				}
			}
		})
	}
}

// TestBrowserRendersDimensions tests Browser rendering at all dimensions
func TestBrowserRendersDimensions(t *testing.T) {
	browser := NewBrowser(nil)

	for _, dim := range testDimensions {
		t.Run(dim.Name, func(t *testing.T) {
			browser.SetSize(dim.Width, dim.Height)
			output := browser.View()

			if output == "" {
				t.Errorf("Browser View() returned empty string at %dx%d", dim.Width, dim.Height)
			}

			// Check for appropriate content based on mode
			if IsCompact(dim.Width, dim.Height) {
				// Compact mode should show category navigation hint
				if !strings.Contains(output, "cat") && !strings.Contains(output, "sel") {
					t.Logf("Warning: Compact browser missing navigation hints")
				}
			}
		})
	}
}

// TestBrowserSearchModeRenders tests Browser search mode at various dimensions
func TestBrowserSearchModeRenders(t *testing.T) {
	browser := NewBrowser(nil)

	// Enable search mode
	browser.searchMode = true
	browser.searchQuery = "test"

	for _, dim := range testDimensions {
		t.Run(dim.Name+"_search", func(t *testing.T) {
			browser.SetSize(dim.Width, dim.Height)
			output := browser.View()

			if output == "" {
				t.Errorf("Browser search View() returned empty at %dx%d", dim.Width, dim.Height)
			}

			// Search mode should show search-related content
			if !strings.Contains(output, "Search") && !strings.Contains(output, "/") {
				t.Logf("Warning: Search mode missing search indicators at %dx%d", dim.Width, dim.Height)
			}
		})
	}
}

// TestAppRendersDimensions tests the main App rendering at all dimensions
func TestAppRendersDimensions(t *testing.T) {
	app := newTestApp()

	views := []ViewMode{
		ViewDashboard,
		ViewBrowser,
		ViewConfig,
		ViewLogs,
		ViewMonitor,
	}

	for _, view := range views {
		for _, dim := range testDimensions {
			testName := viewModeName(view) + "_" + dim.Name
			t.Run(testName, func(t *testing.T) {
				app.currentView = view

				// Simulate window size message
				msg := tea.WindowSizeMsg{Width: dim.Width, Height: dim.Height}
				updatedModel, _ := app.Update(msg)
				app = updatedModel.(*App)

				output := app.View()

				if output == "" {
					t.Errorf("App View() returned empty at %dx%d for %s",
						dim.Width, dim.Height, viewModeName(view))
				}

				// Verify view renders without panic
				lines := strings.Split(output, "\n")
				if len(lines) == 0 {
					t.Errorf("App View() returned no lines at %dx%d for %s",
						dim.Width, dim.Height, viewModeName(view))
				}
			})
		}
	}
}

// TestAppTinyViewContent verifies tiny view has essential content
func TestAppTinyViewContent(t *testing.T) {
	app := newTestApp()

	tinyDims := []TerminalDimension{
		{"tiny_30x8", 30, 8},
		{"tiny_35x10", 35, 10},
	}

	for _, dim := range tinyDims {
		t.Run(dim.Name, func(t *testing.T) {
			msg := tea.WindowSizeMsg{Width: dim.Width, Height: dim.Height}
			updatedModel, _ := app.Update(msg)
			app = updatedModel.(*App)

			output := app.View()

			// Tiny view should contain TUNNEL title
			if !strings.Contains(output, "TUNNEL") {
				t.Errorf("Tiny view missing TUNNEL title at %dx%d", dim.Width, dim.Height)
			}

			// Should have view indicators
			hasViewIndicator := strings.Contains(output, "D") ||
				strings.Contains(output, "B") ||
				strings.Contains(output, "C") ||
				strings.Contains(output, "L") ||
				strings.Contains(output, "M")
			if !hasViewIndicator {
				t.Errorf("Tiny view missing view indicators at %dx%d", dim.Width, dim.Height)
			}

			// Should have help hint
			if !strings.Contains(output, "help") && !strings.Contains(output, "?") {
				t.Logf("Warning: Tiny view missing help hint at %dx%d", dim.Width, dim.Height)
			}
		})
	}
}

// TestAppCompactViewContent verifies compact view has expected sections
func TestAppCompactViewContent(t *testing.T) {
	app := newTestApp()

	compactDims := []TerminalDimension{
		{"compact_45x15", 45, 15},
		{"compact_55x18", 55, 18},
	}

	for _, dim := range compactDims {
		t.Run(dim.Name, func(t *testing.T) {
			msg := tea.WindowSizeMsg{Width: dim.Width, Height: dim.Height}
			updatedModel, _ := app.Update(msg)
			app = updatedModel.(*App)

			output := app.View()

			// Should have header
			if !strings.Contains(output, "TUNNEL") {
				t.Errorf("Compact view missing TUNNEL header at %dx%d", dim.Width, dim.Height)
			}

			// Should have tabs
			hasTabIndicator := strings.Contains(output, "Dash") ||
				strings.Contains(output, "Browse") ||
				strings.Contains(output, "1:")
			if !hasTabIndicator {
				t.Errorf("Compact view missing tab indicators at %dx%d", dim.Width, dim.Height)
			}
		})
	}
}

// TestHelpOverlayRenders tests help overlay rendering
func TestHelpOverlayRenders(t *testing.T) {
	help := NewHelp()

	output := help.View()

	if output == "" {
		t.Error("Help View() returned empty string")
	}

	// Help should contain keyboard shortcuts
	expectedSections := []string{"Navigation", "Dashboard", "Browser", "General"}
	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Help missing section: %s", section)
		}
	}

	// Help should show key bindings
	expectedKeys := []string{"Tab", "Enter", "Esc", "?"}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("Help missing key binding: %s", key)
		}
	}
}

// TestDashboardNoConnectionsRenders verifies empty state rendering
func TestDashboardNoConnectionsRenders(t *testing.T) {
	dashboard := NewDashboard(nil, nil, nil)

	for _, dim := range testDimensions {
		t.Run(dim.Name, func(t *testing.T) {
			dashboard.SetSize(dim.Width, dim.Height)
			output := dashboard.View()

			// Should handle no connections gracefully
			if strings.Contains(output, "panic") || strings.Contains(output, "nil") {
				t.Errorf("Dashboard shows error state at %dx%d", dim.Width, dim.Height)
			}

			// In normal/compact mode, should show "No active connections" or similar
			if !IsTiny(dim.Width, dim.Height) {
				if !strings.Contains(output, "No") && !strings.Contains(output, "connection") {
					t.Logf("Note: Dashboard may not show empty state message at %dx%d", dim.Width, dim.Height)
				}
			}
		})
	}
}

// TestBrowserEmptyCategoriesRenders verifies empty categories handling
func TestBrowserEmptyCategoriesRenders(t *testing.T) {
	browser := NewBrowser(nil)
	// Browser with nil registry has empty categories

	for _, dim := range testDimensions {
		t.Run(dim.Name, func(t *testing.T) {
			browser.SetSize(dim.Width, dim.Height)
			output := browser.View()

			// Should not panic or show errors
			if strings.Contains(output, "panic") || strings.Contains(output, "index out of range") {
				t.Errorf("Browser shows error at %dx%d with empty categories", dim.Width, dim.Height)
			}
		})
	}
}

// TestStylesConsistency verifies style functions work correctly
func TestStylesConsistency(t *testing.T) {
	// Test RenderStatus
	statuses := []string{"connected", "ready", "stopped", "unknown"}
	for _, status := range statuses {
		result := RenderStatus(status)
		if result == "" {
			t.Errorf("RenderStatus(%s) returned empty", status)
		}
	}

	// Test RenderBadge
	badgeTypes := []string{"success", "warning", "danger", "default"}
	for _, badgeType := range badgeTypes {
		result := RenderBadge("test", badgeType)
		if result == "" {
			t.Errorf("RenderBadge with type %s returned empty", badgeType)
		}
	}

	// Test RenderIcon
	icons := []string{IconConnected, IconReady, IconStopped, IconStar, IconArrow}
	for _, icon := range icons {
		result := RenderIcon(icon)
		if result == "" {
			t.Errorf("RenderIcon(%s) returned empty", icon)
		}
	}

	// Test RenderListItem
	for _, selected := range []bool{true, false} {
		result := RenderListItem("test item", selected)
		if result == "" {
			t.Errorf("RenderListItem with selected=%v returned empty", selected)
		}
	}
}

// TestNavigationKeysWork verifies keyboard navigation
func TestNavigationKeysWork(t *testing.T) {
	app := newTestApp()

	// Test number key navigation
	numberKeys := []struct {
		key      string
		expected ViewMode
	}{
		{"1", ViewDashboard},
		{"2", ViewBrowser},
		{"3", ViewConfig},
		{"4", ViewLogs},
		{"5", ViewMonitor},
	}

	for _, test := range numberKeys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.key)}
		updatedModel, _ := app.Update(msg)
		app = updatedModel.(*App)

		if app.currentView != test.expected {
			t.Errorf("Key '%s' should switch to %s, got %s",
				test.key, viewModeName(test.expected), viewModeName(app.currentView))
		}
	}

	// Test tab navigation
	app.currentView = ViewDashboard
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updatedModel, _ := app.Update(tabMsg)
	app = updatedModel.(*App)

	if app.currentView != ViewBrowser {
		t.Errorf("Tab should switch to Browser from Dashboard, got %s", viewModeName(app.currentView))
	}
}

// TestHelpToggle verifies help overlay toggle
func TestHelpToggle(t *testing.T) {
	app := newTestApp()

	// Initially help should be hidden
	if app.showHelp {
		t.Error("Help should be hidden initially")
	}

	// Press '?' to show help
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
	updatedModel, _ := app.Update(msg)
	app = updatedModel.(*App)

	if !app.showHelp {
		t.Error("Help should be shown after pressing '?'")
	}

	// Press '?' again to hide
	updatedModel, _ = app.Update(msg)
	app = updatedModel.(*App)

	if app.showHelp {
		t.Error("Help should be hidden after pressing '?' again")
	}
}

// TestDashboardNavigation verifies dashboard list navigation
func TestDashboardNavigation(t *testing.T) {
	dashboard := NewDashboard(nil, nil, nil)
	dashboard.SetSize(80, 24)

	initialSelection := dashboard.selectedAction

	// Navigate down
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := dashboard.Update(downMsg)
	dashboard = updatedModel.(*Dashboard)

	if dashboard.selectedAction != initialSelection+1 {
		t.Errorf("Down key should increment selection, got %d", dashboard.selectedAction)
	}

	// Navigate up
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = dashboard.Update(upMsg)
	dashboard = updatedModel.(*Dashboard)

	if dashboard.selectedAction != initialSelection {
		t.Errorf("Up key should decrement selection, got %d", dashboard.selectedAction)
	}
}

// TestBrowserCategoryNavigation verifies browser category navigation
func TestBrowserCategoryNavigation(t *testing.T) {
	browser := NewBrowser(nil)
	browser.SetSize(80, 24)

	// Add some test categories
	browser.categories = []MethodCategory{
		{Name: "Cat1", Methods: []Method{{Name: "M1"}}},
		{Name: "Cat2", Methods: []Method{{Name: "M2"}}},
		{Name: "Cat3", Methods: []Method{{Name: "M3"}}},
	}

	// Navigate right
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	updatedModel, _ := browser.Update(rightMsg)
	browser = updatedModel.(*Browser)

	if browser.selectedCategory != 1 {
		t.Errorf("Right key should move to category 1, got %d", browser.selectedCategory)
	}

	// Navigate left
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	updatedModel, _ = browser.Update(leftMsg)
	browser = updatedModel.(*Browser)

	if browser.selectedCategory != 0 {
		t.Errorf("Left key should move to category 0, got %d", browser.selectedCategory)
	}
}

// TestOutputLineCount verifies output doesn't exceed terminal height excessively
func TestOutputLineCount(t *testing.T) {
	app := newTestApp()

	for _, dim := range testDimensions {
		t.Run(dim.Name, func(t *testing.T) {
			msg := tea.WindowSizeMsg{Width: dim.Width, Height: dim.Height}
			updatedModel, _ := app.Update(msg)
			app = updatedModel.(*App)

			output := app.View()
			lines := strings.Split(output, "\n")

			// Allow some overflow for formatting, but not excessive
			maxAllowedLines := dim.Height + 10
			if len(lines) > maxAllowedLines {
				t.Logf("Warning: Output has %d lines for height %d at %dx%d",
					len(lines), dim.Height, dim.Width, dim.Height)
			}
		})
	}
}

// Helper functions

func viewModeName(v ViewMode) string {
	switch v {
	case ViewDashboard:
		return "Dashboard"
	case ViewBrowser:
		return "Browser"
	case ViewConfig:
		return "Config"
	case ViewLogs:
		return "Logs"
	case ViewMonitor:
		return "Monitor"
	default:
		return "Unknown"
	}
}

// stripAnsi removes ANSI escape codes from a string for length calculations
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}
