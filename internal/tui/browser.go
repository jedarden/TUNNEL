package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
)

// MethodCategory represents a category of connection methods
type MethodCategory struct {
	Name    string
	Methods []Method
}

// Method represents a connection method
type Method struct {
	Name        string
	Description string
	Recommended bool
	Status      string
	Category    string
}

// Browser is the method browser view model
type Browser struct {
	categories       []MethodCategory
	selectedCategory int
	selectedMethod   int
	searchQuery      string
	searchMode       bool
	filteredMethods  []Method
	width            int
	height           int

	// Dependencies
	registry *registry.Registry
}

// NewBrowser creates a new browser instance
func NewBrowser(reg *registry.Registry) *Browser {
	b := &Browser{
		categories:       []MethodCategory{},
		selectedCategory: 0,
		selectedMethod:   0,
		searchMode:       false,
		searchQuery:      "",
		filteredMethods:  []Method{},
		width:            80,
		height:           24,
		registry:         reg,
	}

	// Load categories and methods from registry
	b.loadProvidersFromRegistry()

	return b
}

// loadProvidersFromRegistry loads providers from the registry and organizes them by category
func (b *Browser) loadProvidersFromRegistry() {
	if b.registry == nil {
		// Fallback to empty categories
		b.categories = []MethodCategory{}
		return
	}

	// Get all providers from registry
	allProviders := b.registry.ListProviders()

	// Group providers by category
	categoryMap := make(map[string][]Method)
	categoryOrder := []string{"VPN/Mesh Networks", "Tunnel Services", "Direct/Traditional"}

	for _, provider := range allProviders {
		// Map provider category to display category
		var displayCategory string
		switch provider.Category() {
		case providers.CategoryVPN:
			displayCategory = "VPN/Mesh Networks"
		case providers.CategoryTunnel:
			displayCategory = "Tunnel Services"
		case providers.CategoryDirect:
			displayCategory = "Direct/Traditional"
		default:
			displayCategory = "Other"
		}

		// Determine status
		status := "available"
		if provider.IsConnected() {
			status = "connected"
		} else if provider.IsInstalled() {
			status = "installed"
		}

		// Determine if recommended (Tailscale, WireGuard, Cloudflare are recommended)
		recommended := false
		switch provider.Name() {
		case "Tailscale", "WireGuard", "Cloudflare Tunnel":
			recommended = true
		}

		method := Method{
			Name:        provider.Name(),
			Description: b.getProviderDescription(provider.Name()),
			Recommended: recommended,
			Status:      status,
			Category:    displayCategory,
		}

		categoryMap[displayCategory] = append(categoryMap[displayCategory], method)
	}

	// Build ordered categories
	var categories []MethodCategory
	for _, catName := range categoryOrder {
		if methods, exists := categoryMap[catName]; exists {
			categories = append(categories, MethodCategory{
				Name:    catName,
				Methods: methods,
			})
		}
	}

	// Add "Other" category if it exists
	if methods, exists := categoryMap["Other"]; exists {
		categories = append(categories, MethodCategory{
			Name:    "Other",
			Methods: methods,
		})
	}

	b.categories = categories
}

// getProviderDescription returns a description for a provider
func (b *Browser) getProviderDescription(name string) string {
	descriptions := map[string]string{
		"Tailscale":         "Zero-config VPN with NAT traversal",
		"WireGuard":         "Fast, modern VPN protocol",
		"ZeroTier":          "Global area network management",
		"Nebula":            "Overlay networking by Slack",
		"Cloudflare Tunnel": "Secure tunnels without public IPs",
		"ngrok":             "Instant public URLs for local servers",
		"bore":              "Simple TCP tunnel",
	}

	if desc, exists := descriptions[name]; exists {
		return desc
	}
	return "Network connection provider"
}

// Init initializes the browser
func (b *Browser) Init() tea.Cmd {
	return nil
}

// SetSize updates the browser dimensions
func (b *Browser) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// Update handles messages for the browser
func (b *Browser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if b.searchMode {
			return b.handleSearchInput(msg)
		}

		switch msg.String() {
		case "/":
			b.searchMode = true
			b.searchQuery = ""
			return b, nil

		case "left", "h":
			if b.selectedCategory > 0 {
				b.selectedCategory--
				b.selectedMethod = 0
			}

		case "right", "l":
			if b.selectedCategory < len(b.categories)-1 {
				b.selectedCategory++
				b.selectedMethod = 0
			}

		case "up", "k":
			if b.selectedMethod > 0 {
				b.selectedMethod--
			}

		case "down", "j":
			maxMethod := len(b.categories[b.selectedCategory].Methods) - 1
			if b.selectedMethod < maxMethod {
				b.selectedMethod++
			}

		case "enter":
			// Handle method selection
			return b, b.selectMethod()
		}
	}
	return b, nil
}

// View renders the browser
func (b *Browser) View() string {
	if b.searchMode {
		return b.renderSearchMode()
	}

	// Header
	var content strings.Builder
	content.WriteString(TitleStyle.Render("Connection Method Browser"))
	content.WriteString("\n\n")

	// Categories and methods
	categoriesView := b.renderCategories()
	methodsView := b.renderMethods()

	// Two column layout: categories on left, methods on right
	leftWidth := 30
	rightWidth := b.width - leftWidth - 6

	leftPanel := BoxStyle.Width(leftWidth).Height(b.height - 12).Render(categoriesView)
	rightPanel := ActivePanelStyle.Width(rightWidth).Height(b.height - 12).Render(methodsView)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	content.WriteString(panels)

	// Help text
	content.WriteString("\n")
	helpText := b.renderBrowserHelp()
	content.WriteString(helpText)

	return content.String()
}

// renderCategories renders the category list
func (b *Browser) renderCategories() string {
	var content strings.Builder

	content.WriteString(SubtitleStyle.Render("Categories"))
	content.WriteString("\n\n")

	for i, category := range b.categories {
		methodCount := fmt.Sprintf("(%d)", len(category.Methods))
		categoryText := fmt.Sprintf("%s %s", category.Name, HelpDescStyle.Render(methodCount))

		if i == b.selectedCategory {
			content.WriteString(SelectedItemStyle.Render(IconArrow + " " + categoryText))
		} else {
			content.WriteString(ListItemStyle.Render("  " + categoryText))
		}
		content.WriteString("\n")
	}

	return content.String()
}

// renderMethods renders the method list for the selected category
func (b *Browser) renderMethods() string {
	var content strings.Builder

	category := b.categories[b.selectedCategory]
	content.WriteString(SubtitleStyle.Render(category.Name))
	content.WriteString("\n\n")

	for i, method := range category.Methods {
		// Method name with star if recommended
		methodName := method.Name
		if method.Recommended {
			methodName = RenderIcon(IconStar) + " " + methodName
		}

		if i == b.selectedMethod {
			content.WriteString(SelectedItemStyle.Render(IconArrow + " " + methodName))
		} else {
			content.WriteString(ListItemStyle.Render("  " + methodName))
		}
		content.WriteString("\n")

		// Description (only show for selected method)
		if i == b.selectedMethod {
			description := "  " + method.Description
			content.WriteString(HelpDescStyle.Render(description))
			content.WriteString("\n")

			// Status badge
			statusBadge := RenderBadge(method.Status, "success")
			content.WriteString("  " + statusBadge)
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	return content.String()
}

// renderSearchMode renders the search interface
func (b *Browser) renderSearchMode() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("Search Methods"))
	content.WriteString("\n\n")

	// Search input
	searchPrompt := HelpKeyStyle.Render("/") + " "
	searchInput := FocusedInputStyle.Render(b.searchQuery + "█")
	content.WriteString(searchPrompt + searchInput)
	content.WriteString("\n\n")

	// Filtered results
	if b.searchQuery != "" {
		b.updateFilteredMethods()
		if len(b.filteredMethods) == 0 {
			content.WriteString(InfoStyle.Render("No methods found"))
		} else {
			content.WriteString(SubtitleStyle.Render(fmt.Sprintf("Found %d methods:", len(b.filteredMethods))))
			content.WriteString("\n\n")
			for _, method := range b.filteredMethods {
				methodName := method.Name
				if method.Recommended {
					methodName = RenderIcon(IconStar) + " " + methodName
				}
				content.WriteString(ListItemStyle.Render(methodName))
				content.WriteString("\n")
				content.WriteString(HelpDescStyle.Render("  " + method.Description))
				content.WriteString("\n")
				content.WriteString(HelpDescStyle.Render("  Category: " + method.Category))
				content.WriteString("\n\n")
			}
		}
	}

	// Help
	content.WriteString("\n")
	content.WriteString(HelpDescStyle.Render("Type to search, Esc to cancel"))

	return content.String()
}

// renderBrowserHelp renders help text for the browser
func (b *Browser) renderBrowserHelp() string {
	help := []string{
		HelpKeyStyle.Render("←/→") + HelpDescStyle.Render(" switch category"),
		HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" select method"),
		HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search"),
		HelpKeyStyle.Render("Enter") + HelpDescStyle.Render(" connect"),
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
	)
}

// handleSearchInput handles keyboard input in search mode
func (b *Browser) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		b.searchMode = false
		b.searchQuery = ""
		return b, nil

	case "enter":
		b.searchMode = false
		return b, nil

	case "backspace":
		if len(b.searchQuery) > 0 {
			b.searchQuery = b.searchQuery[:len(b.searchQuery)-1]
		}

	default:
		// Add character to search query
		if len(msg.String()) == 1 {
			b.searchQuery += msg.String()
		}
	}

	return b, nil
}

// updateFilteredMethods updates the filtered methods list based on search query
func (b *Browser) updateFilteredMethods() {
	b.filteredMethods = []Method{}
	query := strings.ToLower(b.searchQuery)

	for _, category := range b.categories {
		for _, method := range category.Methods {
			if strings.Contains(strings.ToLower(method.Name), query) ||
				strings.Contains(strings.ToLower(method.Description), query) {
				b.filteredMethods = append(b.filteredMethods, method)
			}
		}
	}
}

// selectMethod handles method selection
func (b *Browser) selectMethod() tea.Cmd {
	if b.registry == nil {
		return nil
	}

	// Get the selected method
	if b.selectedCategory >= len(b.categories) {
		return nil
	}
	category := b.categories[b.selectedCategory]

	if b.selectedMethod >= len(category.Methods) {
		return nil
	}
	method := category.Methods[b.selectedMethod]

	// Get the provider from the registry
	provider, err := b.registry.GetProvider(method.Name)
	if err != nil {
		return nil
	}

	// Connect the provider in a goroutine
	return func() tea.Msg {
		// If already connected, return
		if provider.IsConnected() {
			return nil
		}

		// If not installed, install first
		if !provider.IsInstalled() {
			if err := provider.Install(); err != nil {
				return nil
			}
		}

		// Connect the provider
		if err := provider.Connect(); err != nil {
			return nil
		}

		// Return a refresh message
		return RefreshConnectionsMsg{}
	}
}
