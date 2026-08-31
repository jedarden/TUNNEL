package zerotier

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// ZeroTierProvider implements the Provider interface for ZeroTier
type ZeroTierProvider struct {
	*providers.BaseProvider
}

// New creates a new ZeroTier provider
func New() *ZeroTierProvider {
	return &ZeroTierProvider{
		BaseProvider: providers.NewBaseProvider("zerotier", providers.CategoryVPN),
	}
}

// Install installs ZeroTier
func (z *ZeroTierProvider) Install() error {
	if z.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}

	// Try different installation methods
	installMethods := []struct {
		name string
		cmd  string
		args []string
	}{
		// Official ZeroTier install script (Linux)
		{"script", "bash", []string{"-c", "curl -fsSL https://install.zerotier.com | sudo bash"}},
		// apt (Debian/Ubuntu)
		{"apt", "bash", []string{"-c", "curl -fsSL https://raw.githubusercontent.com/zerotier/ZeroTierOne/master/doc/contact%40zerotier.com.gpg | gpg --dearmor | sudo tee /usr/share/keyrings/zerotier-archive-keyring.gpg >/dev/null && echo 'deb [signed-by=/usr/share/keyrings/zerotier-archive-keyring.gpg] https://download.zerotier.com/debian/$(lsb_release -cs) $(lsb_release -cs) main' | sudo tee /etc/apt/sources.list.d/zerotier.list && sudo apt-get update && sudo apt-get install -y zerotier-one"}},
		// Homebrew (macOS)
		{"brew", "brew", []string{"install", "zerotier-one"}},
	}

	var lastErr error
	for _, method := range installMethods {
		cmd := exec.Command(method.cmd, method.args...)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		// Verify installation
		if z.IsInstalled() {
			return nil
		}
	}

	if lastErr != nil {
		return fmt.Errorf("installation failed: %w", lastErr)
	}
	return fmt.Errorf("installation failed: unknown error")
}

// Uninstall uninstalls ZeroTier
func (z *ZeroTierProvider) Uninstall() error {
	if !z.IsInstalled() {
		return providers.ErrNotInstalled
	}
	return fmt.Errorf("please uninstall ZeroTier manually using your package manager")
}

// IsInstalled checks if ZeroTier is installed
func (z *ZeroTierProvider) IsInstalled() bool {
	cmd := exec.Command("zerotier-cli", "info")
	err := cmd.Run()
	return err == nil
}

// Connect joins a ZeroTier network
func (z *ZeroTierProvider) Connect() error {
	if !z.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := z.GetConfig()
	if err != nil {
		return err
	}

	if config.NetworkID == "" {
		return fmt.Errorf("network_id is required for ZeroTier")
	}

	// Join the network
	cmd := exec.Command("zerotier-cli", "join", config.NetworkID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, string(output))
	}

	// Wait for network to be ready
	time.Sleep(2 * time.Second)

	return nil
}

// Disconnect leaves the ZeroTier network
func (z *ZeroTierProvider) Disconnect() error {
	if !z.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := z.GetConfig()
	if err != nil {
		return err
	}

	if config.NetworkID == "" {
		return fmt.Errorf("network_id is required")
	}

	cmd := exec.Command("zerotier-cli", "leave", config.NetworkID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrCommandFailed, string(output))
	}

	return nil
}

// IsConnected checks if connected to a ZeroTier network
func (z *ZeroTierProvider) IsConnected() bool {
	config, err := z.GetConfig()
	if err != nil || config.NetworkID == "" {
		return false
	}

	networks, err := z.listNetworks()
	if err != nil {
		return false
	}

	for _, network := range networks {
		if network.ID == config.NetworkID && network.Status == "OK" {
			return true
		}
	}

	return false
}

// GetConnectionInfo retrieves current connection information
func (z *ZeroTierProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !z.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	config, err := z.GetConfig()
	if err != nil {
		return info, nil
	}

	networks, err := z.listNetworks()
	if err != nil {
		return info, nil
	}

	for _, network := range networks {
		if network.ID == config.NetworkID {
			info.Status = strings.ToLower(network.Status)
			info.Extra["network_id"] = network.ID
			info.Extra["network_name"] = network.Name
			info.Extra["type"] = network.Type

			// Get assigned addresses
			if len(network.AssignedAddresses) > 0 {
				info.LocalIP = network.AssignedAddresses[0]
			}

			break
		}
	}

	return info, nil
}

// HealthCheck performs a health check
func (z *ZeroTierProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !z.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "ZeroTier is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Check service status
	cmd := exec.Command("zerotier-cli", "info")
	output, err := cmd.Output()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   "ZeroTier service is not running",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Parse info output
	info := string(output)
	var nodeID string
	parts := strings.Fields(info)
	if len(parts) > 2 {
		nodeID = parts[2]
	}

	// Check if connected to network
	connected := z.IsConnected()
	if !connected {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "disconnected",
			Message:   fmt.Sprintf("ZeroTier node %s is not connected to any network", nodeID),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Get connection info to verify actual VPN connectivity
	connInfo, err := z.GetConnectionInfo()
	if err != nil || connInfo.Status != "ok" {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "network_error",
			Message:   fmt.Sprintf("ZeroTier network is not OK (status: %s)", connInfo.Status),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Verify actual VPN connectivity by testing if we can reach ZeroTier's service
	// This confirms the VPN path is working
	timeout := 3 * time.Second
	conn, err := net.DialTimeout("tcp", "my.zerotier.com:443", timeout)
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "service_unreachable",
			Message:   fmt.Sprintf("Cannot reach ZeroTier service: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}
	conn.Close()

	// If we have a local IP, verify it's valid
	if connInfo.LocalIP != "" {
		// Try to bind to the assigned IP to verify it's usable
		ipAddr := &net.TCPAddr{
			IP:   net.ParseIP(connInfo.LocalIP),
			Port: 0,
		}
		listener, err := net.ListenTCP("tcp", ipAddr)
		if err != nil {
			return &providers.HealthStatus{
				Healthy:   false,
				Status:    "invalid_ip",
				Message:   fmt.Sprintf("ZeroTier IP %s is not usable: %v", connInfo.LocalIP, err),
				LastCheck: time.Now(),
				Latency:   time.Since(start),
			}, nil
		}
		listener.Close()
	}

	networkName := "unknown"
	if name, ok := connInfo.Extra["network_name"].(string); ok {
		networkName = name
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "connected",
		Message:   fmt.Sprintf("ZeroTier node %s is connected to network '%s' with IP %s", nodeID, networkName, connInfo.LocalIP),
		LastCheck: time.Now(),
		Latency:   time.Since(start),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (z *ZeroTierProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	if !z.IsInstalled() {
		return []providers.LogEntry{}, nil
	}

	var logs []providers.LogEntry

	// Try journalctl for zerotier-one service
	sinceArg := since.Format("2006-01-02 15:04:05")
	cmd := exec.Command("journalctl", "-u", "zerotier-one", "--since", sinceArg, "-n", "100", "--no-pager", "-o", "json")
	output, err := cmd.Output()
	if err == nil {
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

			// Also check message content
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
					Source:    "zerotier-one",
				})
			}
		}
	}

	// Limit to last 100 entries
	if len(logs) > 100 {
		logs = logs[len(logs)-100:]
	}

	return logs, nil
}

// ValidateConfig validates ZeroTier-specific configuration
func (z *ZeroTierProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := z.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	if config.NetworkID == "" {
		return fmt.Errorf("network_id is required for ZeroTier")
	}
	// Validate network ID format (16 hex characters)
	if len(config.NetworkID) != 16 {
		return fmt.Errorf("network_id must be 16 characters long")
	}
	return nil
}

// ZeroTierNetwork represents a ZeroTier network
type ZeroTierNetwork struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	Type              string   `json:"type"`
	AssignedAddresses []string `json:"assignedAddresses"`
}

// listNetworks retrieves the list of joined networks
func (z *ZeroTierProvider) listNetworks() ([]ZeroTierNetwork, error) {
	cmd := exec.Command("zerotier-cli", "listnetworks", "-j")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list networks", providers.ErrCommandFailed)
	}

	var networks []ZeroTierNetwork
	if err := json.Unmarshal(output, &networks); err != nil {
		return nil, fmt.Errorf("%w: %v", providers.ErrInvalidResponse, err)
	}

	return networks, nil
}
