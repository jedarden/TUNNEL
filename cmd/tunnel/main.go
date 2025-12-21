package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
)

var (
	// Version information (set by build flags)
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
	GoVersion = "unknown"
)

func main() {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT (Ctrl+C) and SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down gracefully...")
		cancel()
		os.Exit(0)
	}()

	// Initialize configuration
	if err := initConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing configuration: %v\n", err)
		os.Exit(1)
	}

	// Execute root command
	if err := Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initConfig initializes viper configuration
func initConfig() error {
	// Set config name and paths
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add config search paths
	viper.AddConfigPath("$HOME/.config/tunnel")
	viper.AddConfigPath("$HOME/.tunnel")
	viper.AddConfigPath(".")

	// Set environment variable prefix
	viper.SetEnvPrefix("TUNNEL")
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Read config file (it's okay if it doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; that's okay, we'll use defaults
	}

	return nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// General defaults
	viper.SetDefault("verbose", false)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_file", "")

	// SSH defaults
	viper.SetDefault("ssh.port", 22)
	viper.SetDefault("ssh.listen_address", "0.0.0.0")
	viper.SetDefault("ssh.authorized_keys_file", "$HOME/.ssh/authorized_keys")

	// Provider defaults
	viper.SetDefault("providers.cloudflared.enabled", true)
	viper.SetDefault("providers.cloudflared.binary_path", "cloudflared")

	viper.SetDefault("providers.ngrok.enabled", true)
	viper.SetDefault("providers.ngrok.binary_path", "ngrok")

	viper.SetDefault("providers.tailscale.enabled", true)
	viper.SetDefault("providers.tailscale.binary_path", "tailscale")

	viper.SetDefault("providers.bore.enabled", true)
	viper.SetDefault("providers.bore.binary_path", "bore")
	viper.SetDefault("providers.bore.server", "bore.pub")

	viper.SetDefault("providers.localhost.enabled", true)

	// TUI defaults
	viper.SetDefault("tui.show_help", true)
	viper.SetDefault("tui.refresh_interval", 1)
	viper.SetDefault("tui.theme", "default")

	// Monitoring defaults
	viper.SetDefault("monitoring.enabled", true)
	viper.SetDefault("monitoring.check_interval", 30)
	viper.SetDefault("monitoring.auto_reconnect", true)
}
