package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	categories := []MethodCategory{
		{
			Name: "VPN/Mesh Networks",
			Methods: []Method{
				{
					Name:        "Tailscale",
					Description: "Zero-config VPN with NAT traversal",
					Recommended: true,
					Status:      "available",
					Category:    "VPN/Mesh Networks",
				},
				{
					Name:        "WireGuard",
					Description: "Fast, modern VPN protocol",
					Recommended: true,
					Status:      "available",
					Category:    "VPN/Mesh Networks",
				},
				{
					Name:        "ZeroTier",
					Description: "Global area network management",
					Recommended: false,
					Status:      "available",
					Category:    "VPN/Mesh Networks",
				},
				{
					Name:        "Nebula",
					Description: "Overlay networking by Slack",
					Recommended: false,
					Status:      "available",
					Category:    "VPN/Mesh Networks",
				},
			},
		},
		{
			Name: "Tunnel Services",
			Methods: []Method{
				{
					Name:        "Cloudflare Tunnel",
					Description: "Secure tunnels without public IPs",
					Recommended: true,
					Status:      "available",
					Category:    "Tunnel Services",
				},
				{
					Name:        "ngrok",
					Description: "Instant public URLs for local servers",
					Recommended: false,
					Status:      "available",
					Category:    "Tunnel Services",
				},
				{
					Name:        "bore",
					Description: "Simple TCP tunnel",
					Recommended: false,
					Status:      "available",
					Category:    "Tunnel Services",
				},
			},
		},
		{
			Name: "Direct/Traditional",
			Methods: []Method{
				{
					Name:        "VS Code Tunnels",
					Description: "Remote development tunnels",
					Recommended: false,
					Status:      "available",
					Category:    "Direct/Traditional",
				},
				{
					Name:        "SSH Port Forward",
					Description: "Traditional SSH tunneling",
					Recommended: false,
					Status:      "available",
					Category:    "Direct/Traditional",
				},
			},
		},
	}

	return &Browser{
		categories:       categories,
		selectedCategory: 0,
		selectedMethod:   0,
		searchMode:       false,
		searchQuery:      "",
		filteredMethods:  []Method{},
		width:            80,
		height:           24,
	}
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
	method := b.categories[b.selectedCategory].Methods[b.selectedMethod]
	// TODO: Implement actual connection logic
	_ = method
	return nil
}
