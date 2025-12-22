package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and fix common issues",
	Long:  `Run diagnostics to check for common issues and suggest fixes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor()
	},
}

type checkResult struct {
	name    string
	status  string // "pass", "warn", "fail"
	message string
	fix     string // suggested fix
}

func runDoctor() error {
	results := []checkResult{}

	color.Cyan("=== TUNNEL Doctor ===")
	fmt.Println()
	color.White("Running diagnostics...\n")

	// Check 1: Configuration file
	results = append(results, checkConfigFile())

	// Check 2: Provider binaries
	results = append(results, checkProviderBinaries()...)

	// Check 3: Network connectivity
	results = append(results, checkNetworkConnectivity())

	// Check 4: SSH server
	results = append(results, checkSSHServer())

	// Check 5: Port availability
	results = append(results, checkPortAvailability())

	// Check 6: Permissions
	results = append(results, checkPermissions())

	// Check 7: System requirements
	results = append(results, checkSystemRequirements())

	// Print results
	fmt.Println()
	color.Cyan("=== Diagnostic Results ===")
	fmt.Println()

	passCount := 0
	warnCount := 0
	failCount := 0

	for _, result := range results {
		var statusColor func(format string, a ...interface{}) string
		var icon string

		switch result.status {
		case "pass":
			statusColor = color.GreenString
			icon = "✓"
			passCount++
		case "warn":
			statusColor = color.YellowString
			icon = "⚠"
			warnCount++
		case "fail":
			statusColor = color.RedString
			icon = "✗"
			failCount++
		}

		fmt.Printf("%s %s: %s\n", statusColor(icon), result.name, result.message)
		if result.fix != "" && result.status != "pass" {
			color.White("  Fix: %s\n", result.fix)
		}
	}

	// Summary
	fmt.Println()
	color.Cyan("=== Summary ===")
	fmt.Printf("Passed: %s  Warnings: %s  Failed: %s\n",
		color.GreenString("%d", passCount),
		color.YellowString("%d", warnCount),
		color.RedString("%d", failCount))

	if failCount > 0 {
		fmt.Println()
		color.Red("Some checks failed. Please address the issues above.")
		return nil // Don't exit with error, just inform
	}

	if warnCount > 0 {
		fmt.Println()
		color.Yellow("Some checks have warnings. TUNNEL should work but may have limited functionality.")
		return nil
	}

	fmt.Println()
	color.Green("All checks passed! TUNNEL is ready to use.")
	return nil
}

func checkConfigFile() checkResult {
	configFile := viper.ConfigFileUsed()

	if configFile == "" {
		return checkResult{
			name:    "Configuration File",
			status:  "warn",
			message: "No config file found, using defaults",
			fix:     "Run 'tunnel config edit' to create a configuration file",
		}
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return checkResult{
			name:    "Configuration File",
			status:  "warn",
			message: "Config file does not exist",
			fix:     "Run 'tunnel config edit' to create a configuration file",
		}
	}

	return checkResult{
		name:    "Configuration File",
		status:  "pass",
		message: fmt.Sprintf("Found at %s", configFile),
	}
}

func checkProviderBinaries() []checkResult {
	providers := []struct {
		name       string
		binary     string
		configKey  string
		required   bool
		installCmd string
	}{
		{
			name:       "Cloudflare Tunnel",
			binary:     "cloudflared",
			configKey:  "providers.cloudflared.binary_path",
			required:   false,
			installCmd: "Visit https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/",
		},
		{
			name:       "ngrok",
			binary:     "ngrok",
			configKey:  "providers.ngrok.binary_path",
			required:   false,
			installCmd: "Visit https://ngrok.com/download or run: snap install ngrok (Linux)",
		},
		{
			name:       "Tailscale",
			binary:     "tailscale",
			configKey:  "providers.tailscale.binary_path",
			required:   false,
			installCmd: "Visit https://tailscale.com/download or run: curl -fsSL https://tailscale.com/install.sh | sh",
		},
		{
			name:       "bore",
			binary:     "bore",
			configKey:  "providers.bore.binary_path",
			required:   false,
			installCmd: "Run: cargo install bore-cli (requires Rust)",
		},
	}

	results := []checkResult{}

	for _, provider := range providers {
		binaryPath := viper.GetString(provider.configKey)
		if binaryPath == "" {
			binaryPath = provider.binary
		}

		path, err := exec.LookPath(binaryPath)
		if err != nil {
			status := "warn"
			if provider.required {
				status = "fail"
			}
			results = append(results, checkResult{
				name:    provider.name,
				status:  status,
				message: fmt.Sprintf("Binary '%s' not found in PATH", binaryPath),
				fix:     provider.installCmd,
			})
			continue
		}

		results = append(results, checkResult{
			name:    provider.name,
			status:  "pass",
			message: fmt.Sprintf("Found at %s", path),
		})
	}

	return results
}

func checkNetworkConnectivity() checkResult {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try to connect to a reliable endpoint
	resp, err := client.Get("https://www.cloudflare.com")
	if err != nil {
		return checkResult{
			name:    "Internet Connectivity",
			status:  "fail",
			message: fmt.Sprintf("Cannot reach internet: %v", err),
			fix:     "Check your internet connection and firewall settings",
		}
	}
	defer resp.Body.Close()

	return checkResult{
		name:    "Internet Connectivity",
		status:  "pass",
		message: "Internet is reachable",
	}
}

func checkSSHServer() checkResult {
	port := viper.GetInt("ssh.port")
	if port == 0 {
		port = 22
	}

	// Try to connect to local SSH server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
	if err != nil {
		return checkResult{
			name:    "SSH Server",
			status:  "warn",
			message: fmt.Sprintf("SSH server not running on port %d", port),
			fix:     "Install and start SSH server: sudo apt-get install openssh-server && sudo systemctl start ssh",
		}
	}
	defer conn.Close()

	return checkResult{
		name:    "SSH Server",
		status:  "pass",
		message: fmt.Sprintf("SSH server is running on port %d", port),
	}
}

func checkPortAvailability() checkResult {
	port := viper.GetInt("ssh.port")
	if port == 0 {
		port = 22
	}

	// Check if we can bind to the port (if SSH is not running)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Port is in use, which is good if SSH is running
		return checkResult{
			name:    "Port Availability",
			status:  "pass",
			message: fmt.Sprintf("Port %d is in use (likely by SSH server)", port),
		}
	}
	listener.Close()

	return checkResult{
		name:    "Port Availability",
		status:  "warn",
		message: fmt.Sprintf("Port %d is available but nothing is listening", port),
		fix:     "Make sure SSH server is configured to listen on this port",
	}
}

func checkPermissions() checkResult {
	// Check if we can write to config directory
	configDir := filepath.Dir(viper.ConfigFileUsed())
	if configDir == "" || configDir == "." {
		configDir = os.ExpandEnv("$HOME/.config/tunnel")
	}

	// Try to create directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return checkResult{
			name:    "File Permissions",
			status:  "fail",
			message: fmt.Sprintf("Cannot create config directory: %v", err),
			fix:     fmt.Sprintf("Check permissions for %s", configDir),
		}
	}

	// Try to create a test file
	testFile := filepath.Join(configDir, ".test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return checkResult{
			name:    "File Permissions",
			status:  "fail",
			message: fmt.Sprintf("Cannot write to config directory: %v", err),
			fix:     fmt.Sprintf("Check write permissions for %s", configDir),
		}
	}
	os.Remove(testFile)

	return checkResult{
		name:    "File Permissions",
		status:  "pass",
		message: "Can read and write to config directory",
	}
}

func checkSystemRequirements() checkResult {
	// Check OS
	osInfo := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	// Check Go version (embedded in binary)
	goVersion := runtime.Version()

	// Check if running in container
	inContainer := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		inContainer = true
	}

	containerInfo := ""
	if inContainer {
		containerInfo = " (running in container)"
	}

	message := fmt.Sprintf("%s, Go %s%s", osInfo, goVersion, containerInfo)

	// Warn if not on supported OS
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return checkResult{
			name:    "System Requirements",
			status:  "warn",
			message: message,
			fix:     "TUNNEL is primarily tested on Linux and macOS",
		}
	}

	return checkResult{
		name:    "System Requirements",
		status:  "pass",
		message: message,
	}
}

