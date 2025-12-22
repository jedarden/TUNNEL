package tailscale

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// TailscaleProvider implements the Provider interface for Tailscale
type TailscaleProvider struct {
	*providers.BaseProvider
}

// New creates a new Tailscale provider
func New() *TailscaleProvider {
	return &TailscaleProvider{
		BaseProvider: providers.NewBaseProvider("tailscale", providers.CategoryVPN),
	}
}

// Install installs Tailscale
func (t *TailscaleProvider) Install() error {
	if t.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}

	// Try different installation methods
	installMethods := []struct {
		name string
		cmd  string
		args []string
	}{
		// Official Tailscale install script (Linux)
		{"script", "bash", []string{"-c", "curl -fsSL https://tailscale.com/install.sh | sh"}},
		// apt (Debian/Ubuntu)
		{"apt", "bash", []string{"-c", "curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.noarmor.gpg | sudo tee /usr/share/keyrings/tailscale-archive-keyring.gpg >/dev/null && curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.tailscale-keyring.list | sudo tee /etc/apt/sources.list.d/tailscale.list && sudo apt-get update && sudo apt-get install -y tailscale"}},
		// Homebrew (macOS)
		{"brew", "brew", []string{"install", "tailscale"}},
	}

	var lastErr error
	for _, method := range installMethods {
		cmd := exec.Command(method.cmd, method.args...)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		// Verify installation
		if t.IsInstalled() {
			return nil
		}
	}

	if lastErr != nil {
		return fmt.Errorf("installation failed: %w", lastErr)
	}
	return fmt.Errorf("installation failed: unknown error")
}

// Uninstall uninstalls Tailscale
func (t *TailscaleProvider) Uninstall() error {
	if !t.IsInstalled() {
		return providers.ErrNotInstalled
	}
	// Uninstallation should be done manually or via package manager
	return fmt.Errorf("please uninstall Tailscale manually using your package manager")
}

// IsInstalled checks if Tailscale is installed
func (t *TailscaleProvider) IsInstalled() bool {
	cmd := exec.Command("tailscale", "--version")
	err := cmd.Run()
	return err == nil
}

// Connect establishes a Tailscale connection
func (t *TailscaleProvider) Connect() error {
	if !t.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := t.GetConfig()
	if err != nil {
		return err
	}

	args := []string{"up"}

	// Add auth key if provided
	if config.AuthKey != "" {
		args = append(args, "--authkey", config.AuthKey)
	}

	// Enable SSH
	args = append(args, "--ssh")

	// Accept routes
	args = append(args, "--accept-routes")

	cmd := exec.Command("tailscale", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, string(output))
	}

	return nil
}

// Disconnect terminates the Tailscale connection
func (t *TailscaleProvider) Disconnect() error {
	if !t.IsInstalled() {
		return providers.ErrNotInstalled
	}

	cmd := exec.Command("tailscale", "down")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrCommandFailed, string(output))
	}

	return nil
}

// IsConnected checks if Tailscale is connected
func (t *TailscaleProvider) IsConnected() bool {
	info, err := t.GetConnectionInfo()
	if err != nil {
		return false
	}
	return info.Status == "Running"
}

// GetConnectionInfo retrieves current connection information
func (t *TailscaleProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !t.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	cmd := exec.Command("tailscale", "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get status", providers.ErrCommandFailed)
	}

	var status TailscaleStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrInvalidResponse, err)
	}

	info := &providers.ConnectionInfo{
		Status: status.BackendState,
		Extra:  make(map[string]interface{}),
	}

	if len(status.Self.TailscaleIPs) > 0 {
		info.LocalIP = status.Self.TailscaleIPs[0]
	}

	info.Extra["hostname"] = status.Self.HostName
	info.Extra["dns_name"] = status.Self.DNSName

	// Collect peer information
	var peers []string
	for _, peer := range status.Peer {
		peers = append(peers, peer.HostName)
	}
	info.Peers = peers

	return info, nil
}

// HealthCheck performs a health check
func (t *TailscaleProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !t.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "Tailscale is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	info, err := t.GetConnectionInfo()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   err.Error(),
			LastCheck: time.Now(),
		}, nil
	}

	healthy := info.Status == "Running"
	return &providers.HealthStatus{
		Healthy:   healthy,
		Status:    info.Status,
		Message:   fmt.Sprintf("Tailscale is %s", strings.ToLower(info.Status)),
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (t *TailscaleProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	if !t.IsInstalled() {
		return []providers.LogEntry{}, nil
	}

	var logs []providers.LogEntry

	// Try to get logs from journalctl for tailscaled service
	sinceArg := since.Format("2006-01-02 15:04:05")
	cmd := exec.Command("journalctl", "-u", "tailscaled", "--since", sinceArg, "-n", "100", "--no-pager", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		// If journalctl fails, return empty array gracefully
		return []providers.LogEntry{}, nil
	}

	// Parse journalctl JSON output (each line is a separate JSON object)
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
			if microseconds, err := strconv.ParseInt(ts, 10, 64); err == nil {
				timestamp = time.Unix(0, microseconds*1000)
			}
		}

		// Extract message
		message := ""
		if msg, ok := entry["MESSAGE"].(string); ok {
			message = msg
		}

		// Determine log level from priority
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

		// Determine level from message content if not already error/warning
		if level == "Info" {
			msgLower := strings.ToLower(message)
			if strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed") || strings.Contains(msgLower, "fatal") {
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
				Source:    "tailscaled",
			})
		}
	}

	// Limit to last 100 entries
	if len(logs) > 100 {
		logs = logs[len(logs)-100:]
	}

	return logs, nil
}

// ValidateConfig validates Tailscale-specific configuration
func (t *TailscaleProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := t.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	// AuthKey is optional for interactive authentication
	return nil
}

// TailscaleStatus represents the JSON output from tailscale status
type TailscaleStatus struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		HostName     string   `json:"HostName"`
		DNSName      string   `json:"DNSName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
	Peer map[string]struct {
		HostName string `json:"HostName"`
		DNSName  string `json:"DNSName"`
	} `json:"Peer"`
}
