package cloudflare

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// CloudflareProvider implements the Provider interface for Cloudflare Tunnel
type CloudflareProvider struct {
	*providers.BaseProvider
}

// New creates a new Cloudflare Tunnel provider
func New() *CloudflareProvider {
	return &CloudflareProvider{
		BaseProvider: providers.NewBaseProvider("cloudflare", providers.CategoryTunnel),
	}
}

// Install installs cloudflared
func (c *CloudflareProvider) Install() error {
	if c.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}

	// Try different installation methods based on OS/package manager
	installMethods := []struct {
		name string
		cmd  string
		args []string
	}{
		// Debian/Ubuntu via apt
		{"apt", "bash", []string{"-c", "curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null && echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared $(lsb_release -cs) main' | sudo tee /etc/apt/sources.list.d/cloudflared.list && sudo apt-get update && sudo apt-get install -y cloudflared"}},
		// Direct binary download (Linux amd64)
		{"binary", "bash", []string{"-c", "curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /tmp/cloudflared && chmod +x /tmp/cloudflared && sudo mv /tmp/cloudflared /usr/local/bin/cloudflared"}},
		// Homebrew (macOS)
		{"brew", "brew", []string{"install", "cloudflared"}},
	}

	var lastErr error
	for _, method := range installMethods {
		cmd := exec.Command(method.cmd, method.args...)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		// Verify installation
		if c.IsInstalled() {
			return nil
		}
	}

	if lastErr != nil {
		return fmt.Errorf("installation failed: %w", lastErr)
	}
	return fmt.Errorf("installation failed: unknown error")
}

// Uninstall uninstalls cloudflared
func (c *CloudflareProvider) Uninstall() error {
	if !c.IsInstalled() {
		return providers.ErrNotInstalled
	}
	return fmt.Errorf("please uninstall cloudflared manually using your package manager")
}

// IsInstalled checks if cloudflared is installed
func (c *CloudflareProvider) IsInstalled() bool {
	cmd := exec.Command("cloudflared", "--version")
	err := cmd.Run()
	return err == nil
}

// Connect establishes a Cloudflare Tunnel connection
func (c *CloudflareProvider) Connect() error {
	if !c.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := c.GetConfig()
	if err != nil {
		return err
	}

	// Need either a token OR a tunnel name
	if config.AuthToken == "" && config.TunnelName == "" {
		return fmt.Errorf("tunnel token or tunnel name is required")
	}

	// Start tunnel as background process
	args := []string{"tunnel", "run"}

	if config.AuthToken != "" {
		// When using a token, the token contains all tunnel info
		// Command: cloudflared tunnel run --token <token>
		args = append(args, "--token", config.AuthToken)
	} else {
		// When using tunnel name (requires prior cloudflared login)
		// Command: cloudflared tunnel run <tunnel_name>
		args = append(args, config.TunnelName)
	}

	cmd := exec.Command("cloudflared", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	// Give it a moment to start
	time.Sleep(2 * time.Second)

	return nil
}

// Disconnect terminates the Cloudflare Tunnel connection
func (c *CloudflareProvider) Disconnect() error {
	if !c.IsInstalled() {
		return providers.ErrNotInstalled
	}

	// Find and kill cloudflared process
	cmd := exec.Command("pkill", "-f", "cloudflared tunnel run")
	_ = cmd.Run() // Ignore errors if no process found

	return nil
}

// IsConnected checks if Cloudflare Tunnel is connected
func (c *CloudflareProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "-f", "cloudflared tunnel run")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (c *CloudflareProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !c.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	config, err := c.GetConfig()
	if err != nil {
		return nil, err
	}

	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if c.IsConnected() {
		info.Status = "connected"
		info.Extra["tunnel_name"] = config.TunnelName
	}

	return info, nil
}

// HealthCheck performs a health check
func (c *CloudflareProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !c.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "cloudflared is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	connected := c.IsConnected()
	status := "disconnected"
	if connected {
		status = "connected"
	}

	return &providers.HealthStatus{
		Healthy:   connected,
		Status:    status,
		Message:   fmt.Sprintf("Cloudflare Tunnel is %s", status),
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (c *CloudflareProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	if !c.IsInstalled() {
		return []providers.LogEntry{}, nil
	}

	var logs []providers.LogEntry

	// Try journalctl for cloudflared service
	sinceArg := since.Format("2006-01-02 15:04:05")
	cmd := exec.Command("journalctl", "-u", "cloudflared", "--since", sinceArg, "-n", "100", "--no-pager", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		// If journalctl fails, return empty array gracefully
		return []providers.LogEntry{}, nil
	}

	// Parse journalctl JSON output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Extract timestamp
		var timestamp time.Time
		if ts, ok := entry["__REALTIME_TIMESTAMP"].(string); ok {
			// Microseconds since epoch
			if microseconds, err := json.Number(ts).Int64(); err == nil {
				timestamp = time.Unix(0, microseconds*1000)
			}
		}

		// Extract message
		message := ""
		if msg, ok := entry["MESSAGE"].(string); ok {
			message = msg
		}

		// Determine log level
		level := "Info"
		if priority, ok := entry["PRIORITY"].(string); ok {
			switch priority {
			case "0", "1", "2", "3":
				level = "Error"
			case "4":
				level = "Warning"
			default:
				level = "Info"
			}
		}

		// Also check message content for cloudflared-specific patterns
		if level == "Info" {
			msgLower := strings.ToLower(message)
			if strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed") || strings.Contains(msgLower, "fatal") || strings.Contains(msgLower, "panic") {
				level = "Error"
			} else if strings.Contains(msgLower, "warning") || strings.Contains(msgLower, "warn") {
				level = "Warning"
			}
		}

		if !timestamp.IsZero() && message != "" {
			logs = append(logs, providers.LogEntry{
				Timestamp: timestamp,
				Level:     level,
				Message:   message,
				Source:    "cloudflared",
			})
		}
	}

	// Limit to last 100 entries
	if len(logs) > 100 {
		logs = logs[len(logs)-100:]
	}

	return logs, nil
}

// ValidateConfig validates Cloudflare-specific configuration
func (c *CloudflareProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := c.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	if config.TunnelName == "" {
		return fmt.Errorf("tunnel_name is required for Cloudflare Tunnel")
	}
	return nil
}

// TunnelInfo represents tunnel information from cloudflared
type TunnelInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	Connections []struct {
		ColoName string `json:"colo_name"`
		ID       string `json:"id"`
	} `json:"connections"`
}

// ListTunnels lists available Cloudflare Tunnels
func (c *CloudflareProvider) ListTunnels() ([]TunnelInfo, error) {
	if !c.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	cmd := exec.Command("cloudflared", "tunnel", "list", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list tunnels", providers.ErrCommandFailed)
	}

	var tunnels []TunnelInfo
	if err := json.Unmarshal(output, &tunnels); err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrInvalidResponse, err)
	}

	return tunnels, nil
}
