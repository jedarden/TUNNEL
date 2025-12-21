package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	verbose    bool
	jsonOutput bool
)

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

	// Add all subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionsCmd)
}

func initCLI() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
	if verbose {
		viper.Set("verbose", true)
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

var configureCmd = &cobra.Command{
	Use:   "configure <method>",
	Short: "Configure a tunnel method interactively",
	Long:  `Configure a tunnel method through an interactive prompt.`,
	Example: `  tunnel configure cloudflared
  tunnel configure ngrok`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := args[0]
		return configureMethod(method)
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

// Implementation functions

func launchTUI(ctx context.Context) error {
	if verbose {
		fmt.Println("Launching TUI...")
	}

	// Import the TUI package and launch it
	// This will be handled by the TUI package
	// For now, we'll provide instructions for integration
	color.Yellow("TUI framework is implemented. Integration with Bubbletea coming next.")
	color.Cyan("\nTUI Components Ready:")
	fmt.Println("  - Dashboard view with active connections")
	fmt.Println("  - Browser for selecting connection methods")
	fmt.Println("  - Help system with keyboard shortcuts")
	fmt.Println("  - Lipgloss styling system")
	return nil
}

func startConnection(method string) error {
	if verbose {
		fmt.Printf("Starting connection with method: %s\n", method)
	}
	// TODO: Implement connection start
	if jsonOutput {
		output := map[string]interface{}{
			"status": "started",
			"method": method,
		}
		return printJSON(output)
	}
	color.Green("Started %s connection", method)
	return nil
}

func stopConnection(method string) error {
	if verbose {
		fmt.Printf("Stopping connection: %s\n", method)
	}
	// TODO: Implement connection stop
	if jsonOutput {
		output := map[string]interface{}{
			"status": "stopped",
			"method": method,
		}
		return printJSON(output)
	}
	color.Yellow("Stopped %s connection", method)
	return nil
}

func restartConnection(method string) error {
	if verbose {
		fmt.Printf("Restarting connection: %s\n", method)
	}
	// TODO: Implement connection restart
	if err := stopConnection(method); err != nil {
		return err
	}
	return startConnection(method)
}

func showStatus() error {
	// TODO: Implement status display
	if jsonOutput {
		output := map[string]interface{}{
			"connections": []map[string]interface{}{
				{"method": "cloudflared", "status": "inactive"},
				{"method": "ngrok", "status": "inactive"},
				{"method": "tailscale", "status": "inactive"},
				{"method": "bore", "status": "inactive"},
				{"method": "localhost.run", "status": "inactive"},
			},
		}
		return printJSON(output)
	}

	color.Cyan("=== Tunnel Status ===")
	fmt.Println()
	methods := []string{"cloudflared", "ngrok", "tailscale", "bore", "localhost.run"}
	for _, method := range methods {
		color.White("  %-15s: ", method)
		color.Red("inactive")
	}
	return nil
}

func listMethods() error {
	methods := []map[string]interface{}{
		{"name": "cloudflared", "description": "Cloudflare Tunnel", "enabled": true},
		{"name": "ngrok", "description": "ngrok tunnel", "enabled": true},
		{"name": "tailscale", "description": "Tailscale VPN", "enabled": true},
		{"name": "bore", "description": "bore tunnel", "enabled": true},
		{"name": "localhost.run", "description": "localhost.run tunnel", "enabled": true},
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{"methods": methods})
	}

	color.Cyan("=== Available Tunnel Methods ===")
	fmt.Println()
	for _, method := range methods {
		status := color.GreenString("enabled")
		if !method["enabled"].(bool) {
			status = color.RedString("disabled")
		}
		fmt.Printf("  %-15s - %s [%s]\n", method["name"], method["description"], status)
	}
	return nil
}

func configureMethod(method string) error {
	if verbose {
		fmt.Printf("Configuring method: %s\n", method)
	}
	// TODO: Implement interactive configuration
	color.Yellow("Interactive configuration not yet implemented for: %s", method)
	return nil
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
	// TODO: Implement authentication
	color.Yellow("Authentication not yet implemented for: %s", method)
	return nil
}

func setAPIKey(method string) error {
	if verbose {
		fmt.Printf("Setting API key for: %s\n", method)
	}
	// TODO: Implement API key setting
	fmt.Printf("Enter API key for %s: ", method)
	var apiKey string
	fmt.Scanln(&apiKey)

	configKey := fmt.Sprintf("providers.%s.api_key", method)
	return setConfig(configKey, apiKey)
}

func authStatus() error {
	methods := []string{"cloudflared", "ngrok", "tailscale", "bore"}
	statuses := make(map[string]interface{})

	for _, method := range methods {
		// TODO: Check actual auth status
		statuses[method] = "not authenticated"
	}

	if jsonOutput {
		return printJSON(statuses)
	}

	color.Cyan("=== Authentication Status ===")
	fmt.Println()
	for _, method := range methods {
		status := statuses[method].(string)
		if strings.Contains(status, "not") {
			fmt.Printf("  %-15s: %s\n", method, color.RedString(status))
		} else {
			fmt.Printf("  %-15s: %s\n", method, color.GreenString(status))
		}
	}
	return nil
}

func printJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
