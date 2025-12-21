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
	// WireGuard logs to kernel/system logs
	// This would require journalctl or similar
	return []providers.LogEntry{}, nil
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
