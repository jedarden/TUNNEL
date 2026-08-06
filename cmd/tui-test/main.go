package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedarden/tunnel/internal/tui"
)

func main() {
	// Create the TUI application with default port
	app := tui.NewApp(8080)

	// Create the Bubbletea program
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
