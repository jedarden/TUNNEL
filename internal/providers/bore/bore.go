package bore

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// BoreProvider implements the Provider interface for bore
type BoreProvider struct {
	*providers.BaseProvider
	tunnelURL string
}

// New creates a new bore provider
func New() *BoreProvider {
	return &BoreProvider{
		BaseProvider: providers.NewBaseProvider("bore", providers.CategoryTunnel),
	}
}

// Install installs bore
func (b *BoreProvider) Install() error {
	if b.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}

	// Try different installation methods
	installMethods := []struct {
		name string
		cmd  string
		args []string
	}{
		// cargo install (if Rust is available)
		{"cargo", "cargo", []string{"install", "bore-cli"}},
		// Download pre-built binary (Linux amd64)
		{"binary", "bash", []string{"-c", "curl -fsSL https://github.com/ekzhang/bore/releases/latest/download/bore-v0.5.1-x86_64-unknown-linux-musl.tar.gz | tar -xz -C /tmp && sudo mv /tmp/bore /usr/local/bin/bore && chmod +x /usr/local/bin/bore"}},
		// Homebrew (macOS)
		{"brew", "brew", []string{"install", "bore-cli"}},
	}

	var lastErr error
	for _, method := range installMethods {
		cmd := exec.Command(method.cmd, method.args...)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		// Verify installation
		if b.IsInstalled() {
			return nil
		}
	}

	if lastErr != nil {
		return fmt.Errorf("installation failed: %w", lastErr)
	}
	return fmt.Errorf("installation failed: unknown error")
}

// Uninstall uninstalls bore
func (b *BoreProvider) Uninstall() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	cmd := exec.Command("cargo", "uninstall", "bore-cli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrCommandFailed, string(output))
	}

	return nil
}

// IsInstalled checks if bore is installed
func (b *BoreProvider) IsInstalled() bool {
	cmd := exec.Command("bore", "--version")
	err := cmd.Run()
	return err == nil
}

// Connect establishes a bore tunnel
func (b *BoreProvider) Connect() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := b.GetConfig()
	if err != nil {
		return err
	}

	// Default to port 22 for SSH if not specified
	localPort := config.LocalPort
	if localPort == 0 {
		localPort = 22
	}

	// Default to bore.pub as remote host
	remoteHost := config.RemoteHost
	if remoteHost == "" {
		remoteHost = "bore.pub"
	}

	// Build bore command
	args := []string{"local", fmt.Sprintf("%d", localPort), "--to", remoteHost}

	// Add remote port if specified
	if config.RemotePort > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", config.RemotePort))
	}

	// Start bore in background
	cmd := exec.Command("bore", args...)

	// Capture output to extract tunnel URL
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	// Wait a moment for bore to start and output the URL
	time.Sleep(2 * time.Second)

	// Try to read the tunnel URL from output
	// bore outputs something like: "listening at bore.pub:12345"
	buf := make([]byte, 1024)
	n, _ := stdout.Read(buf)
	if n > 0 {
		output := string(buf[:n])
		re := regexp.MustCompile(`listening at ([a-zA-Z0-9.-]+):(\d+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 2 {
			b.tunnelURL = fmt.Sprintf("%s:%s", matches[1], matches[2])
		}
	}

	return nil
}

// Disconnect terminates the bore tunnel
func (b *BoreProvider) Disconnect() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	// Kill bore process
	cmd := exec.Command("pkill", "-f", "bore local")
	_ = cmd.Run() // Ignore errors if no process found

	b.tunnelURL = ""
	return nil
}

// IsConnected checks if bore is connected
func (b *BoreProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "-f", "bore local")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (b *BoreProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !b.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if !b.IsConnected() {
		return info, nil
	}

	info.Status = "connected"

	if b.tunnelURL != "" {
		info.TunnelURL = b.tunnelURL

		// Parse host and port
		parts := strings.Split(b.tunnelURL, ":")
		if len(parts) == 2 {
			info.RemoteIP = parts[0]
		}
	}

	config, err := b.GetConfig()
	if err == nil {
		info.Extra["local_port"] = config.LocalPort
		info.Extra["remote_host"] = config.RemoteHost
	}

	return info, nil
}

// HealthCheck performs a health check
func (b *BoreProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !b.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "bore is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	connected := b.IsConnected()
	status := "disconnected"
	message := "bore tunnel is not active"

	if connected {
		status = "connected"
		message = "bore tunnel is active"

		if b.tunnelURL != "" {
			message = fmt.Sprintf("bore tunnel active at %s", b.tunnelURL)
		}
	}

	return &providers.HealthStatus{
		Healthy:   connected,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (b *BoreProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	if !b.IsInstalled() {
		return []providers.LogEntry{}, nil
	}

	// bore doesn't maintain a persistent log file by default
	// We can only capture logs if we have access to the running process output
	// For now, we'll check system logs for any bore-related entries

	var logs []providers.LogEntry

	// Try to get process output via ps and grep
	// This is a best-effort approach since bore logs to stdout
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return []providers.LogEntry{}, nil
	}

	// Look for bore process
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "bore local") {
			// Found the bore process
			// We can add a log entry indicating the process is running
			logs = append(logs, providers.LogEntry{
				Timestamp: time.Now(),
				Level:     "Info",
				Message:   "bore tunnel process is running: " + strings.TrimSpace(line),
				Source:    "bore",
			})
			break
		}
	}

	// Try journalctl for user logs if available
	cmd = exec.Command("journalctl", "--user", "--since", since.Format("2006-01-02 15:04:05"), "-n", "100", "--no-pager")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if !strings.Contains(strings.ToLower(line), "bore") {
				continue
			}

			// Parse timestamp from journalctl output
			var timestamp time.Time
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				timeStr := strings.Join(fields[0:3], " ")
				formats := []string{
					"Jan 02 15:04:05",
					"2006-01-02 15:04:05",
				}

				for _, format := range formats {
					if ts, err := time.Parse(format, timeStr); err == nil {
						if !strings.Contains(format, "2006") {
							timestamp = ts.AddDate(time.Now().Year(), 0, 0)
						} else {
							timestamp = ts
						}
						break
					}
				}
			}

			if timestamp.IsZero() {
				timestamp = time.Now()
			}

			// Determine log level
			level := "Info"
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "failed") {
				level = "Error"
			} else if strings.Contains(lineLower, "warning") || strings.Contains(lineLower, "warn") {
				level = "Warning"
			}

			// Extract message (everything after timestamp)
			message := line
			if len(fields) > 3 {
				message = strings.Join(fields[3:], " ")
			}

			logs = append(logs, providers.LogEntry{
				Timestamp: timestamp,
				Level:     level,
				Message:   message,
				Source:    "bore",
			})
		}
	}

	// Limit to last 100 entries
	if len(logs) > 100 {
		logs = logs[len(logs)-100:]
	}

	return logs, nil
}

// ValidateConfig validates bore-specific configuration
func (b *BoreProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := b.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	// All fields are optional with sensible defaults
	return nil
}
