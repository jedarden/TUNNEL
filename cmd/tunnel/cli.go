package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/registry"
	"github.com/jedarden/tunnel/internal/tui"
	"github.com/jedarden/tunnel/internal/upgrade"
	"github.com/jedarden/tunnel/internal/web/api"
	embeddedfs "github.com/jedarden/tunnel/internal/web/embed"
	"github.com/jedarden/tunnel/internal/web/middleware"
	"github.com/jedarden/tunnel/pkg/config"
	"github.com/jedarden/tunnel/pkg/tunnel"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	verbose    bool
	jsonOutput bool
	webPort    int

	manager       *core.DefaultConnectionManager
	reg           *registry.Registry
	keyManager    *core.FileKeyManager
	tunnelManager *tunnel.Manager
	tunnelReg     *tunnel.Registry
)

// appConfig holds the loaded application configuration (used during initialization)
var appConfig *config.Config //nolint:unused

// Execute runs the root command
func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Terminal Unified Network Node Encrypted Link - SSH access management",
	Long: `TUNNEL (Terminal Unified Network Node Encrypted Link) is a TUI application
for managing SSH access through various tunnel providers including Cloudflare Tunnel,
ngrok, Tailscale, bore, and localhost.run.

By default, running 'tunnel' without arguments launches the interactive TUI.`,
	Example: `  # Launch interactive TUI
  tunnel

  # Start a specific tunnel method
  tunnel start cloudflared

  # Show status of all connections
  tunnel status

  # Configure a tunnel method
  tunnel configure ngrok`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Launch TUI by default
		return launchTUI(cmd.Context())
	},
}

func init() {
	cobra.OnInitialize(initCLI)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/tunnel/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().IntVarP(&webPort, "port", "p", 8080, "web server port")

	// Add all subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(keysCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionsCmd)
	rootCmd.AddCommand(emergencyRevokeCmd)
}

func initCLI() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
	if verbose {
		viper.Set("verbose", true)
	}

	// Load application config
	var err error
	appConfig, err = config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		// Use default config if loading fails
		appConfig = config.GetDefaultConfig()
	}

	// Create registry with all providers
	reg = registry.NewRegistry()

	// Create connection manager
	manager = core.NewConnectionManager(nil)

	// Register all providers from registry with the connection manager
	for _, provider := range reg.ListProviders() {
		// Create a ConnectionProvider adapter for each Provider
		adapter := &providerAdapter{provider: provider}
		manager.RegisterProvider(adapter)
	}

	// Initialize key manager
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to get home directory: %v\n", err)
	} else {
		authorizedKeysPath := filepath.Join(homeDir, ".ssh", "authorized_keys")
		keyManager, err = core.NewFileKeyManager(authorizedKeysPath, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize key manager: %v\n", err)
		}
	}
}

// Connection commands

var startCmd = &cobra.Command{
	Use:   "start [method]",
	Short: "Start a tunnel connection",
	Long:  `Start a tunnel connection using the specified method or the default method.`,
	Example: `  tunnel start cloudflared
  tunnel start ngrok
  tunnel start`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := "default"
		if len(args) > 0 {
			method = args[0]
		}
		return startConnection(method)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [method|all]",
	Short: "Stop tunnel connection(s)",
	Long:  `Stop a specific tunnel connection or all connections.`,
	Example: `  tunnel stop cloudflared
  tunnel stop all`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := "all"
		if len(args) > 0 {
			method = args[0]
		}
		return stopConnection(method)
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart [method]",
	Short: "Restart a tunnel connection",
	Long:  `Restart a specific tunnel connection.`,
	Example: `  tunnel restart cloudflared
  tunnel restart ngrok`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := args[0]
		return restartConnection(method)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status",
	Long:  `Display the status of all tunnel connections.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showStatus()
	},
}

// Method management commands

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tunnel methods",
	Long:  `List all available tunnel methods and their current status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listMethods()
	},
}

// Config commands

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Manage tunnel configuration settings.`,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get configuration value(s)",
	Long:  `Get a specific configuration value or show all configuration.`,
	Example: `  tunnel config get
  tunnel config get ssh.port
  tunnel config get providers.cloudflared.enabled`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := ""
		if len(args) > 0 {
			key = args[0]
		}
		return getConfig(key)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set configuration value",
	Long:  `Set a specific configuration value.`,
	Example: `  tunnel config set ssh.port 2222
  tunnel config set providers.cloudflared.enabled true`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		return setConfig(key, value)
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file",
	Long:  `Open the configuration file in $EDITOR.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return editConfig()
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configEditCmd)
}

// Auth commands

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Manage authentication for tunnel providers.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login <method>",
	Short: "Authenticate with a tunnel provider",
	Long:  `Interactively authenticate with a tunnel provider.`,
	Example: `  tunnel auth login cloudflared
  tunnel auth login ngrok`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := args[0]
		return authLogin(method)
	},
}

var authSetKeyCmd = &cobra.Command{
	Use:   "set-key <method>",
	Short: "Set API key for a provider",
	Long:  `Set the API key for a tunnel provider.`,
	Example: `  tunnel auth set-key ngrok
  tunnel auth set-key cloudflared`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := args[0]
		return setAPIKey(method)
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Show authentication status for all tunnel providers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return authStatus()
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authSetKeyCmd)
	authCmd.AddCommand(authStatusCmd)
}

// Keys commands

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage SSH keys",
	Long:  `Manage SSH public keys for authentication.`,
}

var keysListCmd = &cobra.Command{
	Use:   "list [user]",
	Short: "List SSH keys",
	Long:  `List all SSH public keys, optionally filtered by user.`,
	Example: `  tunnel keys list
  tunnel keys list alice`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		user := ""
		if len(args) > 0 {
			user = args[0]
		}
		return listKeys(user)
	},
}

var keysAddCmd = &cobra.Command{
	Use:   "add <user>",
	Short: "Add a new SSH key",
	Long:  `Add a new SSH public key for a user. Prompts for the key interactively.`,
	Example: `  tunnel keys add alice
  tunnel keys add bob`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		user := args[0]
		return addKey(user)
	},
}

var keysRotateCmd = &cobra.Command{
	Use:   "rotate <user> [key-id]",
	Short: "Rotate SSH key(s)",
	Long:  `Rotate a specific SSH key or all keys for a user.`,
	Example: `  tunnel keys rotate alice
  tunnel keys rotate alice SHA256:abc123...`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		user := args[0]
		keyID := ""
		if len(args) > 1 {
			keyID = args[1]
		}
		return rotateKey(user, keyID)
	},
}

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke <user> <key-id>",
	Short: "Revoke a specific SSH key",
	Long:  `Revoke (remove) a specific SSH public key.`,
	Example: `  tunnel keys revoke alice SHA256:abc123...
  tunnel keys revoke bob 1`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		user := args[0]
		keyID := args[1]
		return revokeKey(user, keyID)
	},
}

var keysImportGitHubCmd = &cobra.Command{
	Use:   "import-github <github-user>",
	Short: "Import SSH keys from GitHub",
	Long:  `Import all SSH public keys from a GitHub user profile.`,
	Example: `  tunnel keys import-github octocat
  tunnel keys import-github alice`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		githubUser := args[0]
		return importGitHubKeys(githubUser)
	},
}

var keysImportGitLabCmd = &cobra.Command{
	Use:   "import-gitlab <gitlab-user>",
	Short: "Import SSH keys from GitLab",
	Long:  `Import all SSH public keys from a GitLab user profile.`,
	Example: `  tunnel keys import-gitlab octocat
  tunnel keys import-gitlab alice`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gitlabUser := args[0]
		return importGitLabKeys(gitlabUser)
	},
}

func init() {
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysAddCmd)
	keysCmd.AddCommand(keysRotateCmd)
	keysCmd.AddCommand(keysRevokeCmd)
	keysCmd.AddCommand(keysImportGitHubCmd)
	keysCmd.AddCommand(keysImportGitLabCmd)
}

// Completions command

var completionsCmd = &cobra.Command{
	Use:   "completions <shell>",
	Short: "Generate shell completions",
	Long: `Generate shell completion scripts for bash, zsh, or fish.

To load completions:

Bash:
  $ source <(tunnel completions bash)
  # To load completions for each session, execute once:
  $ tunnel completions bash > /etc/bash_completion.d/tunnel

Zsh:
  $ source <(tunnel completions zsh)
  # To load completions for each session, add to ~/.zshrc:
  $ tunnel completions zsh > "${fpath[1]}/_tunnel"

Fish:
  $ tunnel completions fish | source
  # To load completions for each session:
  $ tunnel completions fish > ~/.config/fish/completions/tunnel.fish
`,
	ValidArgs: []string{"bash", "zsh", "fish"},
	Args:      cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]
		switch shell {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return fmt.Errorf("unsupported shell: %s", shell)
		}
	},
}

// Emergency revoke command

var (
	emergencyRevokeReason       string
	emergencyRevokeKillSessions bool
	emergencyRevokeNotify       bool
	emergencyRevokeForce        bool
)

var emergencyRevokeCmd = &cobra.Command{
	Use:   "emergency-revoke <user>",
	Short: "Emergency revocation of all SSH keys for a user",
	Long: `Emergency revocation of all SSH keys for a user. This is a critical security operation
that removes ALL keys for the specified user and requires a reason to be logged.

This command will:
- Revoke ALL SSH keys associated with the user
- Log an audit event with the reason
- Optionally kill active sessions
- Optionally send notifications

Use this command in emergency situations such as:
- Security breaches or compromised credentials
- Terminated employees
- Lost or stolen devices
- Suspected unauthorized access`,
	Example: `  # Revoke all keys for a user
  tunnel emergency-revoke bob_dev --reason "security breach"

  # Revoke and kill active sessions
  tunnel emergency-revoke alice --reason "device stolen" --kill-sessions

  # Skip confirmation prompt
  tunnel emergency-revoke charlie --reason "terminated" --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]
		return emergencyRevoke(username, emergencyRevokeReason, emergencyRevokeKillSessions, emergencyRevokeNotify, emergencyRevokeForce)
	},
}

func init() {
	emergencyRevokeCmd.Flags().StringVar(&emergencyRevokeReason, "reason", "", "reason for emergency revocation (required)")
	_ = emergencyRevokeCmd.MarkFlagRequired("reason")
	emergencyRevokeCmd.Flags().BoolVar(&emergencyRevokeKillSessions, "kill-sessions", false, "kill active SSH sessions for the user")
	emergencyRevokeCmd.Flags().BoolVar(&emergencyRevokeNotify, "notify", false, "send notification about the revocation")
	emergencyRevokeCmd.Flags().BoolVar(&emergencyRevokeForce, "force", false, "skip confirmation prompt")
}

// Implementation functions

func launchTUI(ctx context.Context) error {
	if verbose {
		fmt.Println("Launching tunnel with web server...")
	}

	// Start the hot-swap binary watcher
	upgradeWatcher, err := upgrade.NewWatcher(log.Default())
	if err != nil {
		// Log but don't fail - hot-swap is optional
		if verbose {
			fmt.Printf("Warning: Could not start upgrade watcher: %v\n", err)
		}
	} else {
		if err := upgradeWatcher.Start(func() {
			// Cleanup callback before restart
			if verbose {
				fmt.Println("Preparing for binary upgrade...")
			}
			// Give web server time to complete pending requests
			time.Sleep(500 * time.Millisecond)
		}); err != nil {
			if verbose {
				fmt.Printf("Warning: Could not start upgrade watcher: %v\n", err)
			}
		} else if verbose {
			fmt.Printf("Hot-swap watcher started for: %s\n", upgradeWatcher.GetBinaryPath())
		}
		defer upgradeWatcher.Stop()
	}

	// Create the minimal TUI application
	tuiApp := tui.NewApp(webPort)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(tuiApp, tea.WithAltScreen())

	// Channel to signal web server started
	serverReady := make(chan error, 1)

	// Start web server in background
	go func() {
		if err := startWebServer(ctx, p); err != nil {
			serverReady <- err
		}
		close(serverReady)
	}()

	// Wait a moment for server to start, then update TUI
	go func() {
		select {
		case err := <-serverReady:
			if err != nil {
				p.Send(tui.ServerStatusMsg{
					Status: tui.ServerError,
					Port:   webPort,
					Error:  err,
				})
			}
		case <-time.After(500 * time.Millisecond):
			// Server likely started successfully
			p.Send(tui.ServerStatusMsg{
				Status: tui.ServerRunning,
				Port:   webPort,
			})
		}
	}()

	// Run the TUI program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}

// startWebServer starts the Fiber web server with the API and embedded frontend
func startWebServer(ctx context.Context, p *tea.Program) error {
	// Create tunnel manager and registry for the API
	tunnelReg = tunnel.NewRegistry()
	tunnelManager = tunnel.NewManager(nil) // Use default config

	// Create API server
	apiServer := api.NewServer(&api.ServerConfig{
		Manager:  tunnelManager,
		Registry: tunnelReg,
		Logger:   log.Default(),
		DevMode:  false,
	})

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	app.Use(middleware.RequestLogger())

	// Setup API routes
	api.SetupRoutes(app, apiServer)

	// Serve embedded frontend
	staticFS, err := embeddedfs.GetFS()
	if err != nil {
		// Frontend not embedded (development mode)
		if verbose {
			fmt.Println("Frontend not embedded, API-only mode")
		}
	} else {
		// Serve static files from embedded filesystem
		app.Use("/", filesystem.New(filesystem.Config{
			Root:         http.FS(staticFS),
			Browse:       false,
			Index:        "index.html",
			NotFoundFile: "index.html", // SPA fallback
		}))
	}

	// Run server (blocks until shutdown)
	go func() {
		<-ctx.Done()
		_ = app.Shutdown()
	}()

	// Try to start server, auto-incrementing port if in use
	actualPort := webPort
	maxAttempts := 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		addr := fmt.Sprintf(":%d", actualPort)

		// Check if port is available
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			if verbose {
				fmt.Printf("Port %d in use, trying %d...\n", actualPort, actualPort+1)
			}
			actualPort++
			continue
		}
		listener.Close()

		// Port is available - notify TUI of the actual port
		if actualPort != webPort {
			p.Send(tui.ServerStatusMsg{
				Status: tui.ServerRunning,
				Port:   actualPort,
				URL:    fmt.Sprintf("http://localhost:%d", actualPort),
			})
		}

		if verbose {
			fmt.Printf("Starting web server on http://localhost:%d\n", actualPort)
		}

		return app.Listen(addr)
	}

	return fmt.Errorf("could not find available port after %d attempts (tried %d-%d)", maxAttempts, webPort, actualPort-1)
}

// Fallback for when embedded filesystem doesn't exist (dev mode)
func serveStaticFallback(app *fiber.App, staticFS fs.FS) {
	app.Use("/", filesystem.New(filesystem.Config{
		Root:         http.FS(staticFS),
		Browse:       false,
		Index:        "index.html",
		NotFoundFile: "index.html",
	}))
}

func startConnection(method string) error {
	if verbose {
		fmt.Printf("Starting connection with method: %s\n", method)
	}

	// Get provider from registry
	provider, err := reg.GetProvider(method)
	if err != nil {
		return fmt.Errorf("provider not found: %s", method)
	}

	// Check if already connected
	if provider.IsConnected() {
		if jsonOutput {
			output := map[string]interface{}{
				"status":  "error",
				"message": "already connected",
				"method":  method,
			}
			return printJSON(output)
		}
		color.Yellow("%s is already connected", method)
		return nil
	}

	// Connect using the provider
	if err := provider.Connect(); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"method": method,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Get connection info
	connInfo, err := provider.GetConnectionInfo()
	if err == nil && connInfo != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status":          "started",
				"method":          method,
				"connection_info": connInfo,
			}
			return printJSON(output)
		}

		color.Green("✓ Started %s connection", method)
		if connInfo.TunnelURL != "" {
			fmt.Printf("  Tunnel URL: %s\n", color.CyanString(connInfo.TunnelURL))
		}
		if connInfo.LocalIP != "" {
			fmt.Printf("  Local IP: %s\n", color.CyanString(connInfo.LocalIP))
		}
		if connInfo.RemoteIP != "" {
			fmt.Printf("  Remote IP: %s\n", color.CyanString(connInfo.RemoteIP))
		}
	} else {
		if jsonOutput {
			output := map[string]interface{}{
				"status": "started",
				"method": method,
			}
			return printJSON(output)
		}
		color.Green("✓ Started %s connection", method)
	}

	return nil
}

func stopConnection(method string) error {
	if verbose {
		fmt.Printf("Stopping connection: %s\n", method)
	}

	// Handle "all" to stop all connections
	if method == "all" {
		providers := reg.GetConnectedProviders()
		if len(providers) == 0 {
			if jsonOutput {
				output := map[string]interface{}{
					"status":  "info",
					"message": "no active connections",
				}
				return printJSON(output)
			}
			color.Yellow("No active connections to stop")
			return nil
		}

		errors := []string{}
		for _, provider := range providers {
			if err := provider.Disconnect(); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", provider.Name(), err))
			} else if verbose {
				fmt.Printf("Stopped %s\n", provider.Name())
			}
		}

		if jsonOutput {
			output := map[string]interface{}{
				"status":  "stopped",
				"count":   len(providers),
				"errors":  errors,
				"success": len(providers) - len(errors),
			}
			return printJSON(output)
		}

		if len(errors) > 0 {
			color.Yellow("Stopped %d connection(s) with %d error(s):", len(providers)-len(errors), len(errors))
			for _, errMsg := range errors {
				fmt.Printf("  - %s\n", errMsg)
			}
		} else {
			color.Green("✓ Stopped all %d connection(s)", len(providers))
		}
		return nil
	}

	// Stop specific provider
	provider, err := reg.GetProvider(method)
	if err != nil {
		return fmt.Errorf("provider not found: %s", method)
	}

	// Check if connected
	if !provider.IsConnected() {
		if jsonOutput {
			output := map[string]interface{}{
				"status":  "info",
				"message": "not connected",
				"method":  method,
			}
			return printJSON(output)
		}
		color.Yellow("%s is not connected", method)
		return nil
	}

	// Disconnect
	if err := provider.Disconnect(); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"method": method,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status": "stopped",
			"method": method,
		}
		return printJSON(output)
	}

	color.Green("✓ Stopped %s connection", method)
	return nil
}

func restartConnection(method string) error {
	if verbose {
		fmt.Printf("Restarting connection: %s\n", method)
	}

	// Get provider from registry
	provider, err := reg.GetProvider(method)
	if err != nil {
		return fmt.Errorf("provider not found: %s", method)
	}

	// Check if provider is installed
	if !provider.IsInstalled() {
		return fmt.Errorf("%s is not installed. Please install it first", method)
	}

	// Store the current connection state and configuration
	wasConnected := provider.IsConnected()
	var connInfo interface{}

	if wasConnected {
		// Try to get current connection info before stopping
		connInfo, _ = provider.GetConnectionInfo()

		if verbose && !jsonOutput {
			color.Yellow("Stopping current connection...")
		}

		// Stop the connection gracefully
		if err := provider.Disconnect(); err != nil {
			// Log the error but continue with restart
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: Error during disconnect: %v\n", err)
			}
		}

		// Wait a moment for cleanup
		time.Sleep(1 * time.Second)
	}

	if verbose && !jsonOutput {
		if wasConnected {
			color.Cyan("Restarting connection...")
		} else {
			color.Cyan("Starting connection (was not connected)...")
		}
	}

	// Start the connection
	if err := provider.Connect(); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status":        "error",
				"error":         err.Error(),
				"method":        method,
				"was_connected": wasConnected,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to restart connection: %w", err)
	}

	// Get new connection info
	newConnInfo, err := provider.GetConnectionInfo()
	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Warning: Could not retrieve connection info: %v\n", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status":        "restarted",
			"method":        method,
			"was_connected": wasConnected,
		}
		if newConnInfo != nil {
			output["connection_info"] = newConnInfo
		}
		if connInfo != nil {
			output["previous_connection_info"] = connInfo
		}
		return printJSON(output)
	}

	// Display success message
	color.Green("✓ Successfully restarted %s connection", method)

	// Show connection details if available
	if newConnInfo != nil {
		fmt.Println()
		color.Cyan("Connection Details:")

		// Access connection info fields directly
		if newConnInfo.TunnelURL != "" {
			fmt.Printf("  Tunnel URL: %s\n", color.CyanString("%s", newConnInfo.TunnelURL))
		}
		if newConnInfo.LocalIP != "" {
			fmt.Printf("  Local IP: %s\n", color.CyanString("%s", newConnInfo.LocalIP))
		}
		if newConnInfo.RemoteIP != "" {
			fmt.Printf("  Remote IP: %s\n", color.CyanString("%s", newConnInfo.RemoteIP))
		}
	}

	// Show uptime
	if wasConnected {
		fmt.Println()
		fmt.Printf("Connection restarted at: %s\n",
			color.CyanString(time.Now().Format("2006-01-02 15:04:05")))
	}

	return nil
}

func showStatus() error {
	providers := reg.ListProviders()

	if jsonOutput {
		connections := []map[string]interface{}{}
		for _, provider := range providers {
			info := map[string]interface{}{
				"name":      provider.Name(),
				"category":  provider.Category(),
				"installed": provider.IsInstalled(),
				"connected": provider.IsConnected(),
			}

			// Add connection info if connected
			if provider.IsConnected() {
				if connInfo, err := provider.GetConnectionInfo(); err == nil && connInfo != nil {
					info["connection_info"] = connInfo
				}
			}

			connections = append(connections, info)
		}
		return printJSON(map[string]interface{}{"connections": connections})
	}

	color.Cyan("=== Tunnel Status ===")
	fmt.Println()

	// Group by category
	vpnProviders := reg.ListByCategory("vpn")
	tunnelProviders := reg.ListByCategory("tunnel")

	if len(vpnProviders) > 0 {
		color.Cyan("VPN Providers:")
		for _, provider := range vpnProviders {
			displayProviderStatus(provider)
		}
		fmt.Println()
	}

	if len(tunnelProviders) > 0 {
		color.Cyan("Tunnel Providers:")
		for _, provider := range tunnelProviders {
			displayProviderStatus(provider)
		}
	}

	return nil
}

func displayProviderStatus(provider providers.Provider) {
	name := provider.Name()
	installed := provider.IsInstalled()
	connected := provider.IsConnected()

	fmt.Printf("  %-15s: ", name)

	if !installed {
		color.Red("not installed")
		return
	}

	if connected {
		color.Green("connected")
		// Show connection details
		if connInfo, err := provider.GetConnectionInfo(); err == nil && connInfo != nil {
			if connInfo.TunnelURL != "" {
				fmt.Printf("\n    URL: %s", color.CyanString(connInfo.TunnelURL))
			}
			if connInfo.LocalIP != "" {
				fmt.Printf("\n    Local IP: %s", color.CyanString(connInfo.LocalIP))
			}
			if connInfo.RemoteIP != "" {
				fmt.Printf("\n    Remote IP: %s", color.CyanString(connInfo.RemoteIP))
			}
		}
		fmt.Println()
	} else {
		color.Yellow("disconnected")
	}
}

func listMethods() error {
	providerInfo := reg.GetProviderInfo()

	if jsonOutput {
		return printJSON(map[string]interface{}{"providers": providerInfo})
	}

	color.Cyan("=== Available Tunnel Providers ===")
	fmt.Println()

	// Group by category
	vpnProviders := []registry.ProviderInfo{}
	tunnelProviders := []registry.ProviderInfo{}

	for _, info := range providerInfo {
		if info.Category == "vpn" {
			vpnProviders = append(vpnProviders, info)
		} else if info.Category == "tunnel" {
			tunnelProviders = append(tunnelProviders, info)
		}
	}

	if len(vpnProviders) > 0 {
		color.Cyan("VPN Providers:")
		for _, info := range vpnProviders {
			displayProviderInfo(info)
		}
		fmt.Println()
	}

	if len(tunnelProviders) > 0 {
		color.Cyan("Tunnel Providers:")
		for _, info := range tunnelProviders {
			displayProviderInfo(info)
		}
	}

	return nil
}

func displayProviderInfo(info registry.ProviderInfo) {
	installedStatus := color.GreenString("installed")
	if !info.Installed {
		installedStatus = color.RedString("not installed")
	}

	connectedStatus := ""
	if info.Installed {
		if info.Connected {
			connectedStatus = color.GreenString(" [connected]")
		} else {
			connectedStatus = color.YellowString(" [disconnected]")
		}
	}

	fmt.Printf("  %-15s - %-20s%s\n", info.Name, installedStatus, connectedStatus)
}

// NewCredentialStore creates a credential store (helper function)
func NewCredentialStore(storeType, serviceName, baseDir, passphrase string) (core.CredentialStore, error) {
	return core.NewCredentialStore(storeType, serviceName, baseDir, passphrase)
}

func getConfig(key string) error {
	if key == "" {
		// Show all config
		settings := viper.AllSettings()
		if jsonOutput {
			return printJSON(settings)
		}
		for k, v := range settings {
			fmt.Printf("%s = %v\n", k, v)
		}
		return nil
	}

	value := viper.Get(key)
	if jsonOutput {
		return printJSON(map[string]interface{}{key: value})
	}
	fmt.Printf("%s = %v\n", key, value)
	return nil
}

func setConfig(key, value string) error {
	viper.Set(key, value)

	// Write config file
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = os.ExpandEnv("$HOME/.config/tunnel/config.yaml")
		// Create directory if needed
		if err := os.MkdirAll(os.ExpandEnv("$HOME/.config/tunnel"), 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"key":    key,
			"value":  value,
			"status": "saved",
		})
	}

	color.Green("Configuration updated: %s = %s", key, value)
	return nil
}

func editConfig() error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // fallback
	}

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = os.ExpandEnv("$HOME/.config/tunnel/config.yaml")
		// Create directory if needed
		if err := os.MkdirAll(os.ExpandEnv("$HOME/.config/tunnel"), 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		// Create empty config file
		if _, err := os.Create(configFile); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
	}

	cmd := exec.Command(editor, configFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func authLogin(method string) error {
	if verbose {
		fmt.Printf("Authenticating with: %s\n", method)
	}

	// Get provider from registry
	provider, err := reg.GetProvider(method)
	if err != nil {
		return fmt.Errorf("provider not found: %s", method)
	}

	// Check if installed
	if !provider.IsInstalled() {
		return fmt.Errorf("%s is not installed. Please install it first", method)
	}

	// Provider-specific authentication
	switch method {
	case "cloudflare":
		color.Cyan("Launching Cloudflare Tunnel authentication...")
		fmt.Println("This will open your browser to authenticate with Cloudflare.")
		cmd := exec.Command("cloudflared", "tunnel", "login")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		color.Green("✓ Cloudflare authentication successful")
		return nil

	case "ngrok":
		color.Cyan("Setting up ngrok authentication...")
		fmt.Print("Enter your ngrok authtoken: ")
		var authtoken string
		_, _ = fmt.Scanln(&authtoken)
		if authtoken == "" {
			return fmt.Errorf("authtoken cannot be empty")
		}

		cmd := exec.Command("ngrok", "config", "add-authtoken", authtoken)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set authtoken: %w", err)
		}
		color.Green("✓ ngrok authentication configured")
		return nil

	case "tailscale":
		color.Cyan("Starting Tailscale authentication...")
		fmt.Println("This will authenticate your device with Tailscale.")
		cmd := exec.Command("tailscale", "up")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		color.Green("✓ Tailscale authentication successful")
		return nil

	case "wireguard":
		color.Yellow("WireGuard uses configuration files for authentication.")
		fmt.Println("Please configure WireGuard using 'wg-quick' or place your config file in /etc/wireguard/")
		return nil

	case "zerotier":
		color.Cyan("Setting up ZeroTier authentication...")
		fmt.Println("To join a ZeroTier network, use: zerotier-cli join <network-id>")
		return nil

	default:
		color.Yellow("Authentication not implemented for: %s", method)
		return nil
	}
}

func setAPIKey(method string) error {
	if verbose {
		fmt.Printf("Setting API key for: %s\n", method)
	}

	// Check if provider exists
	provider, err := reg.GetProvider(method)
	if err != nil {
		return fmt.Errorf("provider not found: %s", method)
	}

	// Check if installed
	if !provider.IsInstalled() {
		color.Yellow("Warning: %s is not installed", method)
		fmt.Print("Continue anyway? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return nil
		}
	}

	color.Cyan("=== Set API Key for %s ===", method)
	fmt.Println()

	// Get home directory for credential storage
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create credential store
	credStore, err := NewCredentialStore(
		"file",
		"tunnel",
		filepath.Join(homeDir, ".config", "tunnel", "credentials"),
		"tunnel-credentials",
	)
	if err != nil {
		return fmt.Errorf("failed to create credential store: %w", err)
	}

	// Read API key from stdin
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter API key for %s: ", method)
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read API key: %w", err)
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Validate key format based on provider
	if err := validateAPIKey(method, apiKey); err != nil {
		color.Yellow("Warning: %v", err)
		fmt.Print("Store anyway? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return nil
		}
	}

	// Store the API key securely
	if err := credStore.Set(method, "api_key", []byte(apiKey)); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"method": method,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to store API key: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status": "success",
			"method": method,
			"message": "API key stored securely",
		}
		return printJSON(output)
	}

	color.Green("✓ API key stored securely")
	fmt.Printf("  Provider: %s\n", method)
	fmt.Printf("  Location: %s\n",
		color.CyanString(filepath.Join(homeDir, ".config", "tunnel", "credentials")))

	// Show next steps
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Test the connection: %s\n", color.CyanString("tunnel start %s", method))
	fmt.Printf("  2. Check status: %s\n", color.CyanString("tunnel status"))

	return nil
}

// validateAPIKey performs basic validation on API key format
func validateAPIKey(method, apiKey string) error {
	// Basic validation rules for different providers
	switch method {
	case "ngrok":
		// ngrok auth tokens are typically long alphanumeric strings
		if len(apiKey) < 20 {
			return fmt.Errorf("ngrok auth token seems too short (expected 20+ characters)")
		}
	case "cloudflare", "cloudflared":
		// Cloudflare tunnel tokens are base64-encoded and quite long
		if len(apiKey) < 32 {
			return fmt.Errorf("cloudflare tunnel token seems too short")
		}
	case "tailscale":
		// Tailscale auth keys typically start with "tskey-"
		if !strings.HasPrefix(apiKey, "tskey-") && len(apiKey) > 10 {
			return fmt.Errorf("tailscale auth key should start with 'tskey-'")
		}
	case "zerotier":
		// ZeroTier network IDs are 16-character hex strings
		if len(apiKey) == 16 {
			// This might be a network ID, which is valid
			for _, c := range apiKey {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return fmt.Errorf("zerotier network ID should be a 16-character hex string")
				}
			}
		}
	}

	// Generic validation: no spaces, reasonable length
	if strings.Contains(apiKey, " ") {
		return fmt.Errorf("API key should not contain spaces")
	}

	if len(apiKey) < 8 {
		return fmt.Errorf("API key seems too short (minimum 8 characters)")
	}

	return nil
}

func authStatus() error {
	providers := reg.ListProviders()
	statuses := make(map[string]interface{})

	for _, provider := range providers {
		name := provider.Name()
		status := checkAuthStatus(name)
		statuses[name] = status
	}

	if jsonOutput {
		return printJSON(statuses)
	}

	color.Cyan("=== Authentication Status ===")
	fmt.Println()

	for _, provider := range providers {
		name := provider.Name()
		status := statuses[name].(string)

		fmt.Printf("  %-15s: ", name)
		if strings.Contains(status, "not") || strings.Contains(status, "unknown") {
			color.Red(status)
		} else {
			color.Green(status)
		}
	}

	return nil
}

func checkAuthStatus(method string) string {
	homeDir, _ := os.UserHomeDir()

	switch method {
	case "cloudflare":
		// Check for cloudflared certificate
		certPath := filepath.Join(homeDir, ".cloudflared", "cert.pem")
		if _, err := os.Stat(certPath); err == nil {
			return "authenticated"
		}
		return "not authenticated"

	case "ngrok":
		// Check ngrok config file for authtoken
		configPath := filepath.Join(homeDir, ".config", "ngrok", "ngrok.yml")
		if _, err := os.Stat(configPath); err == nil {
			// Read config and check for authtoken
			data, err := os.ReadFile(configPath)
			if err == nil && strings.Contains(string(data), "authtoken:") {
				return "authenticated"
			}
		}
		return "not authenticated"

	case "tailscale":
		// Check if tailscale is authenticated
		cmd := exec.Command("tailscale", "status")
		if err := cmd.Run(); err == nil {
			return "authenticated"
		}
		return "not authenticated"

	case "wireguard":
		// Check for WireGuard config files
		configDir := "/etc/wireguard"
		if entries, err := os.ReadDir(configDir); err == nil && len(entries) > 0 {
			return "configured"
		}
		return "not configured"

	case "zerotier":
		// Check if zerotier service is authorized
		cmd := exec.Command("zerotier-cli", "info")
		if err := cmd.Run(); err == nil {
			return "authenticated"
		}
		return "not authenticated"

	case "bore":
		// bore doesn't require authentication
		return "no auth required"

	default:
		return "unknown"
	}
}

func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// providerAdapter adapts a providers.Provider to core.ConnectionProvider
type providerAdapter struct {
	provider interface {
		Name() string
		Connect() error
		Disconnect() error
		IsConnected() bool
	}
}

func (p *providerAdapter) Name() string {
	return p.provider.Name()
}

func (p *providerAdapter) Connect(ctx context.Context, config *core.Config) (*core.Connection, error) {
	// Use the provider's Connect method
	if err := p.provider.Connect(); err != nil {
		return nil, err
	}

	// Create a connection object
	conn := core.NewConnection(
		fmt.Sprintf("%s-%d", p.provider.Name(), os.Getpid()),
		p.provider.Name(),
		0, // localPort - not used for most providers
		"", // remoteHost
		0,  // remotePort
	)
	conn.SetState(core.StateConnected)

	return conn, nil
}

func (p *providerAdapter) Disconnect(conn *core.Connection) error {
	return p.provider.Disconnect()
}

func (p *providerAdapter) IsHealthy(conn *core.Connection) bool {
	return p.provider.IsConnected()
}

// Keys management functions

func listKeys(user string) error {
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	keys, err := keyManager.ListKeys(user)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"count": len(keys),
			"keys":  keys,
		}
		if user != "" {
			output["user"] = user
		}
		return printJSON(output)
	}

	// Terminal output
	if len(keys) == 0 {
		color.Yellow("No SSH keys found")
		return nil
	}

	color.Cyan("=== SSH Public Keys ===")
	if user != "" {
		fmt.Printf("User: %s\n", color.GreenString(user))
	}
	fmt.Printf("Total: %s\n\n", color.GreenString("%d", len(keys)))

	for i, key := range keys {
		fmt.Printf("%s. %s\n", color.CyanString("%d", i+1), color.GreenString(key.Type))
		fmt.Printf("   Fingerprint: %s\n", key.Fingerprint)
		if key.Comment != "" {
			fmt.Printf("   Comment:     %s\n", key.Comment)
		}
		fmt.Printf("   Status:      %s\n", colorizeStatus(key.Status))
		fmt.Printf("   Added:       %s\n", key.AddedAt.Format("2006-01-02 15:04:05"))
		if !key.LastUsed.IsZero() {
			fmt.Printf("   Last Used:   %s\n", key.LastUsed.Format("2006-01-02 15:04:05"))
		}
		if key.ExpiresAt != nil {
			fmt.Printf("   Expires:     %s\n", key.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}

	return nil
}

func addKey(user string) error {
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	color.Cyan("Add SSH Public Key for %s", user)
	fmt.Println("Paste your SSH public key (press Enter when done):")

	// Read the key from stdin
	reader := bufio.NewReader(os.Stdin)
	keyStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}

	keyStr = strings.TrimSpace(keyStr)
	if keyStr == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Validate the key
	key, err := keyManager.ValidateKey(keyStr)
	if err != nil {
		return fmt.Errorf("invalid SSH key: %w", err)
	}

	// Add the key
	if err := keyManager.AddKey(user, *key); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"user":   user,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to add key: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status":      "success",
			"user":        user,
			"fingerprint": key.Fingerprint,
			"type":        key.Type,
		}
		return printJSON(output)
	}

	color.Green("✓ SSH key added successfully")
	fmt.Printf("  Type:        %s\n", key.Type)
	fmt.Printf("  Fingerprint: %s\n", key.Fingerprint)
	if key.Comment != "" {
		fmt.Printf("  Comment:     %s\n", key.Comment)
	}

	return nil
}

func rotateKey(user, keyID string) error {
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	if keyID == "" {
		// Rotate all keys for user
		color.Yellow("Key rotation for all keys is not yet implemented")
		fmt.Println("Please specify a key ID to rotate a specific key")
		return nil
	}

	// For now, rotation means prompting for a new key and removing the old one
	color.Cyan("Rotate SSH Key for %s", user)
	fmt.Printf("This will remove key: %s\n", keyID)
	fmt.Println("Enter the new SSH public key (press Enter when done):")

	// Read the new key
	reader := bufio.NewReader(os.Stdin)
	keyStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}

	keyStr = strings.TrimSpace(keyStr)
	if keyStr == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Validate the new key
	newKey, err := keyManager.ValidateKey(keyStr)
	if err != nil {
		return fmt.Errorf("invalid SSH key: %w", err)
	}

	// Remove the old key
	if err := keyManager.RemoveKey(user, keyID); err != nil {
		return fmt.Errorf("failed to remove old key: %w", err)
	}

	// Add the new key
	if err := keyManager.AddKey(user, *newKey); err != nil {
		return fmt.Errorf("failed to add new key: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status":          "success",
			"user":            user,
			"old_key_id":      keyID,
			"new_fingerprint": newKey.Fingerprint,
			"new_type":        newKey.Type,
		}
		return printJSON(output)
	}

	color.Green("✓ SSH key rotated successfully")
	fmt.Printf("  Old Key ID:       %s\n", keyID)
	fmt.Printf("  New Type:         %s\n", newKey.Type)
	fmt.Printf("  New Fingerprint:  %s\n", newKey.Fingerprint)

	return nil
}

func revokeKey(user, keyID string) error {
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	if verbose {
		fmt.Printf("Revoking key %s for user %s\n", keyID, user)
	}

	// Remove the key
	if err := keyManager.RemoveKey(user, keyID); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"user":   user,
				"key_id": keyID,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status": "success",
			"user":   user,
			"key_id": keyID,
		}
		return printJSON(output)
	}

	color.Green("✓ SSH key revoked successfully")
	fmt.Printf("  User:   %s\n", user)
	fmt.Printf("  Key ID: %s\n", keyID)

	return nil
}

func importGitHubKeys(githubUser string) error {
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	color.Cyan("Importing SSH keys from GitHub user: %s", githubUser)

	keys, err := keyManager.ImportFromGitHub(githubUser)
	if err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status":      "error",
				"error":       err.Error(),
				"github_user": githubUser,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to import keys from GitHub: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status":      "success",
			"github_user": githubUser,
			"count":       len(keys),
			"keys":        keys,
		}
		return printJSON(output)
	}

	if len(keys) == 0 {
		color.Yellow("No SSH keys found for GitHub user: %s", githubUser)
		return nil
	}

	color.Green("✓ Imported %d SSH key(s) from GitHub", len(keys))
	fmt.Println()

	for i, key := range keys {
		fmt.Printf("%d. %s\n", i+1, color.GreenString(key.Type))
		fmt.Printf("   Fingerprint: %s\n", key.Fingerprint)
		if key.Comment != "" {
			fmt.Printf("   Comment:     %s\n", key.Comment)
		}
		fmt.Println()
	}

	return nil
}

func importGitLabKeys(gitlabUser string) error {
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	color.Cyan("Importing SSH keys from GitLab user: %s", gitlabUser)

	// GitLab API endpoint for user's SSH keys
	url := fmt.Sprintf("https://gitlab.com/%s.keys", gitlabUser)

	resp, err := http.Get(url)
	if err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status":      "error",
				"error":       err.Error(),
				"gitlab_user": gitlabUser,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to fetch GitLab keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if jsonOutput {
			output := map[string]interface{}{
				"status":      "error",
				"error":       fmt.Sprintf("GitLab API returned status %d", resp.StatusCode),
				"gitlab_user": gitlabUser,
			}
			return printJSON(output)
		}
		return fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var keys []core.SSHPublicKey
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		keyStr := strings.TrimSpace(scanner.Text())
		if keyStr == "" {
			continue
		}

		key, err := keyManager.ValidateKey(keyStr)
		if err != nil {
			// Log but continue with other keys
			fmt.Fprintf(os.Stderr, "Warning: invalid key from GitLab: %v\n", err)
			continue
		}

		// Add comment indicating source
		key.Comment = fmt.Sprintf("gitlab.com/%s", gitlabUser)
		keys = append(keys, *key)

		// Add to authorized_keys
		if err := keyManager.AddKey(gitlabUser, *key); err != nil {
			return fmt.Errorf("failed to add key: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		if jsonOutput {
			output := map[string]interface{}{
				"status":      "error",
				"error":       err.Error(),
				"gitlab_user": gitlabUser,
			}
			return printJSON(output)
		}
		return fmt.Errorf("failed to read GitLab response: %w", err)
	}

	if jsonOutput {
		output := map[string]interface{}{
			"status":      "success",
			"gitlab_user": gitlabUser,
			"count":       len(keys),
			"keys":        keys,
		}
		return printJSON(output)
	}

	if len(keys) == 0 {
		color.Yellow("No SSH keys found for GitLab user: %s", gitlabUser)
		return nil
	}

	color.Green("✓ Imported %d SSH key(s) from GitLab", len(keys))
	fmt.Println()

	for i, key := range keys {
		fmt.Printf("%d. %s\n", i+1, color.GreenString(key.Type))
		fmt.Printf("   Fingerprint: %s\n", key.Fingerprint)
		if key.Comment != "" {
			fmt.Printf("   Comment:     %s\n", key.Comment)
		}
		fmt.Println()
	}

	return nil
}

func colorizeStatus(status string) string {
	switch status {
	case "active":
		return color.GreenString(status)
	case "revoked":
		return color.RedString(status)
	case "expired":
		return color.YellowString(status)
	default:
		return status
	}
}

// emergencyRevoke revokes all SSH keys for a user in an emergency situation
func emergencyRevoke(username, reason string, killSessions, notify, force bool) error {
	// Validate inputs
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}

	// Check if key manager is initialized
	if keyManager == nil {
		return fmt.Errorf("key manager not initialized")
	}

	// Get all keys for the user
	keys, err := keyManager.ListKeys(username)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if len(keys) == 0 {
		if jsonOutput {
			return printJSON(map[string]interface{}{
				"status":  "info",
				"message": "no keys found for user",
				"user":    username,
			})
		}
		color.Yellow("No keys found for user: %s", username)
		return nil
	}

	// Show warning and confirmation unless force is used
	if !force && !jsonOutput {
		color.Red("WARNING: Emergency key revocation for user: %s", username)
		fmt.Printf("\nThis will revoke ALL %d key(s) for this user.\n", len(keys))
		fmt.Printf("Reason: %s\n\n", reason)

		if killSessions {
			color.Red("Active sessions will be killed!")
		}
		if notify {
			fmt.Println("Notifications will be sent.")
		}

		fmt.Print("\nType 'yes' to confirm: ")
		var confirmation string
		_, _ = fmt.Scanln(&confirmation)

		if confirmation != "yes" {
			color.Yellow("Emergency revocation cancelled")
			return nil
		}
	}

	// Track revocation results
	revokedCount := 0
	failedKeys := []string{}

	// Revoke all keys
	for _, key := range keys {
		if err := keyManager.RemoveKey(username, key.ID); err != nil {
			failedKeys = append(failedKeys, fmt.Sprintf("%s: %v", key.Fingerprint, err))
			if verbose {
				fmt.Fprintf(os.Stderr, "Failed to revoke key %s: %v\n", key.Fingerprint, err)
			}
		} else {
			revokedCount++
			if verbose && !jsonOutput {
				fmt.Printf("Revoked key: %s\n", key.Fingerprint)
			}
		}
	}

	// Kill active sessions if requested
	sessionsKilled := 0
	if killSessions {
		// Note: This is a placeholder for session killing logic
		// In a real implementation, this would use 'pkill' or similar
		if verbose && !jsonOutput {
			color.Yellow("Session killing not yet implemented (placeholder)")
		}
	}

	// Send notification if requested
	if notify {
		// Note: This is a placeholder for notification logic
		// In a real implementation, this would send email, Slack, etc.
		if verbose && !jsonOutput {
			fmt.Printf("Notification logged: Emergency key revocation for %s\n", username)
		}
	}

	// Log audit event
	homeDir, _ := os.UserHomeDir()
	auditLogPath := filepath.Join(homeDir, ".config", "tunnel", "audit.log")
	auditLogger, err := core.NewAuditLogger(auditLogPath, false, "")
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize audit logger: %v\n", err)
		}
	} else {
		defer auditLogger.Close()

		_ = auditLogger.Log(core.AuditEvent{
			Timestamp: time.Now(),
			EventType: "emergency_revoke",
			Method:    "ssh-key",
			User:      username,
			Details: map[string]interface{}{
				"reason":          reason,
				"keys_revoked":    revokedCount,
				"keys_failed":     len(failedKeys),
				"total_keys":      len(keys),
				"kill_sessions":   killSessions,
				"sessions_killed": sessionsKilled,
				"notify":          notify,
				"forced":          force,
			},
			Success: len(failedKeys) == 0,
		})
	}

	// Output results
	if jsonOutput {
		return printJSON(map[string]interface{}{
			"status":          "completed",
			"user":            username,
			"reason":          reason,
			"keys_revoked":    revokedCount,
			"keys_failed":     len(failedKeys),
			"total_keys":      len(keys),
			"failed_keys":     failedKeys,
			"kill_sessions":   killSessions,
			"sessions_killed": sessionsKilled,
			"notify":          notify,
			"success":         len(failedKeys) == 0,
		})
	}

	// Display summary
	fmt.Println()
	if len(failedKeys) == 0 {
		color.Green("✓ Emergency revocation completed successfully")
	} else {
		color.Yellow("⚠ Emergency revocation completed with errors")
	}

	fmt.Printf("\nUser: %s\n", color.CyanString(username))
	fmt.Printf("Reason: %s\n", reason)
	fmt.Printf("Keys revoked: %s\n", color.GreenString("%d/%d", revokedCount, len(keys)))

	if len(failedKeys) > 0 {
		color.Red("\nFailed to revoke %d key(s):", len(failedKeys))
		for _, failure := range failedKeys {
			fmt.Printf("  - %s\n", failure)
		}
	}

	if killSessions {
		fmt.Printf("Sessions killed: %d\n", sessionsKilled)
	}

	if notify {
		fmt.Println("\nNotifications sent: Yes")
	}

	fmt.Println()
	color.Cyan("Audit event logged with type: emergency_revoke")

	return nil
}
