package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/registry"
	"github.com/jedarden/tunnel/internal/tui"
	"github.com/jedarden/tunnel/pkg/config"
)

func main() {
	// Create the registry
	reg := registry.NewRegistry()

	// Create the connection manager
	mgr := core.NewConnectionManager(core.DefaultManagerConfig())

	// Load or create default config
	cfg := config.GetDefaultConfig()

	// Create the TUI application with dependencies
	app := tui.NewApp(reg, mgr, cfg)

	// Create the Bubbletea program
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Cleanup on exit
	if err := mgr.Shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "Error shutting down manager: %v\n", err)
	}
}
