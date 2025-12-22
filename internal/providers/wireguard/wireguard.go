package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// WireGuardProvider implements the Provider interface for WireGuard
type WireGuardProvider struct {
	*providers.BaseProvider
	interfaceName string
}

// New creates a new WireGuard provider
func New() *WireGuardProvider {
	return &WireGuardProvider{
		BaseProvider:  providers.NewBaseProvider("wireguard", providers.CategoryVPN),
		interfaceName: "wg0",
	}
}

// Install installs WireGuard
func (w *WireGuardProvider) Install() error {
	if w.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install WireGuard manually using your package manager")
}

// Uninstall uninstalls WireGuard
func (w *WireGuardProvider) Uninstall() error {
	if !w.IsInstalled() {
		return providers.ErrNotInstalled
	}
	return fmt.Errorf("please uninstall WireGuard manually using your package manager")
}

// IsInstalled checks if WireGuard is installed
func (w *WireGuardProvider) IsInstalled() bool {
	cmd := exec.Command("wg", "version")
	err := cmd.Run()
	return err == nil
}

// Connect establishes a WireGuard connection
func (w *WireGuardProvider) Connect() error {
	if !w.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := w.GetConfig()
	if err != nil {
		return err
	}

	// Use config file if specified, otherwise use default interface
	iface := w.interfaceName
	if config.ConfigFile != "" {
		// Extract interface name from config file path
		// e.g., /etc/wireguard/wg0.conf -> wg0
		parts := strings.Split(config.ConfigFile, "/")
		filename := parts[len(parts)-1]
		iface = strings.TrimSuffix(filename, ".conf")
	}

	// Bring up the interface using wg-quick
	cmd := exec.Command("wg-quick", "up", iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, string(output))
	}

	w.interfaceName = iface
	return nil
}

// Disconnect terminates the WireGuard connection
func (w *WireGuardProvider) Disconnect() error {
	if !w.IsInstalled() {
		return providers.ErrNotInstalled
	}

	cmd := exec.Command("wg-quick", "down", w.interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Don't fail if already down
		if !strings.Contains(string(output), "is not a WireGuard interface") {
			return fmt.Errorf("%w: %s", providers.ErrCommandFailed, string(output))
		}
	}

	return nil
}

// IsConnected checks if WireGuard is connected
func (w *WireGuardProvider) IsConnected() bool {
	cmd := exec.Command("wg", "show", w.interfaceName)
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (w *WireGuardProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !w.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	info := &providers.ConnectionInfo{
		Status:        "disconnected",
		InterfaceName: w.interfaceName,
		Extra:         make(map[string]interface{}),
	}

	if !w.IsConnected() {
		return info, nil
	}

	info.Status = "connected"

	// Get interface details
	cmd := exec.Command("wg", "show", w.interfaceName)
	output, err := cmd.Output()
	if err != nil {
		return info, nil
	}

	// Parse WireGuard output
	lines := strings.Split(string(output), "\n")
	var peers []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "peer:") {
			peer := strings.TrimPrefix(line, "peer:")
			peers = append(peers, strings.TrimSpace(peer))
		} else if strings.HasPrefix(line, "endpoint:") {
			endpoint := strings.TrimPrefix(line, "endpoint:")
			info.RemoteIP = strings.TrimSpace(endpoint)
		}
	}

	info.Peers = peers

	// Get interface IP address
	cmd = exec.Command("ip", "addr", "show", w.interfaceName)
	output, err = cmd.Output()
	if err == nil {
		re := regexp.MustCompile(`inet\s+([0-9.]+)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			info.LocalIP = matches[1]
		}
	}

	return info, nil
}

// HealthCheck performs a health check
func (w *WireGuardProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !w.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "WireGuard is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	connected := w.IsConnected()
	status := "disconnected"
	if connected {
		status = "connected"
	}

	health := &providers.HealthStatus{
		Healthy:   connected,
		Status:    status,
		Message:   fmt.Sprintf("WireGuard is %s", status),
		LastCheck: time.Now(),
		Metrics:   make(map[string]interface{}),
	}

	if connected {
		// Get transfer statistics
		cmd := exec.Command("wg", "show", w.interfaceName, "transfer")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					sent, _ := strconv.ParseUint(parts[1], 10, 64)
					received, _ := strconv.ParseUint(parts[2], 10, 64)
					health.BytesSent = sent
					health.BytesReceived = received
					break
				}
			}
		}
	}

	return health, nil
}

// GetLogs retrieves logs since the specified time
func (w *WireGuardProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	if !w.IsInstalled() {
		return []providers.LogEntry{}, nil
	}

	var logs []providers.LogEntry

	// Try journalctl for wg-quick service first
	sinceArg := since.Format("2006-01-02 15:04:05")
	cmd := exec.Command("journalctl", "-u", "wg-quick@*", "--since", sinceArg, "-n", "100", "--no-pager")
	output, err := cmd.Output()
	if err == nil {
		logs = append(logs, parseSystemLogs(string(output), "wg-quick")...)
	}

	// Also try to get kernel logs via dmesg
	cmd = exec.Command("dmesg", "-T")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if !strings.Contains(strings.ToLower(line), "wireguard") {
				continue
			}

			// Parse dmesg line format: [timestamp] message
			var timestamp time.Time
			var message string

			// Try to parse timestamp
			re := regexp.MustCompile(`^\[([^\]]+)\]\s+(.*)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 2 {
				// dmesg -T outputs human-readable timestamps
				if ts, err := time.Parse("Mon Jan 2 15:04:05 2006", matches[1]); err == nil {
					timestamp = ts
					message = matches[2]
				}
			}

			if timestamp.IsZero() {
				// Fallback: use current time if parsing fails
				timestamp = time.Now()
				message = line
			}

			// Filter by time
			if timestamp.Before(since) {
				continue
			}

			// Determine log level
			level := "Info"
			msgLower := strings.ToLower(message)
			if strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed") || strings.Contains(msgLower, "fatal") {
				level = "Error"
			} else if strings.Contains(msgLower, "warning") || strings.Contains(msgLower, "warn") {
				level = "Warning"
			}

			logs = append(logs, providers.LogEntry{
				Timestamp: timestamp,
				Level:     level,
				Message:   message,
				Source:    "kernel",
			})
		}
	}

	// Limit to last 100 entries
	if len(logs) > 100 {
		logs = logs[len(logs)-100:]
	}

	return logs, nil
}

// parseSystemLogs parses standard syslog format
func parseSystemLogs(output, source string) []providers.LogEntry {
	var logs []providers.LogEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse syslog format: "Mon DD HH:MM:SS hostname service[pid]: message"
		// Or journalctl format: "Mon YYYY-MM-DD HH:MM:SS hostname service[pid]: message"
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) < 2 {
			continue
		}

		message := parts[1]

		// Try to parse timestamp from the beginning
		var timestamp time.Time
		fields := strings.Fields(parts[0])
		if len(fields) >= 3 {
			// Try different timestamp formats
			timeStr := strings.Join(fields[0:3], " ")
			formats := []string{
				"Jan 02 15:04:05",
				"2006-01-02 15:04:05",
			}

			for _, format := range formats {
				if ts, err := time.Parse(format, timeStr); err == nil {
					// If year is not in format, use current year
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
		msgLower := strings.ToLower(message)
		if strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed") || strings.Contains(msgLower, "fatal") {
			level = "Error"
		} else if strings.Contains(msgLower, "warning") || strings.Contains(msgLower, "warn") {
			level = "Warning"
		}

		logs = append(logs, providers.LogEntry{
			Timestamp: timestamp,
			Level:     level,
			Message:   message,
			Source:    source,
		})
	}

	return logs
}

// ValidateConfig validates WireGuard-specific configuration
func (w *WireGuardProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := w.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}

	// Check if config file exists if specified
	if config.ConfigFile != "" {
		if _, err := os.Stat(config.ConfigFile); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", config.ConfigFile)
		}
	}

	return nil
}
