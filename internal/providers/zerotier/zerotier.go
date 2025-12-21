package zerotier

import (
	"encoding/json"
	"fmt"
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
	return fmt.Errorf("please install ZeroTier manually from https://www.zerotier.com/download")
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
	if !z.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "ZeroTier is not installed",
			LastCheck: time.Now(),
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
		}, nil
	}

	// Parse info output
	info := string(output)
	var nodeID string
	parts := strings.Fields(info)
	if len(parts) > 2 {
		nodeID = parts[2]
	}

	connected := z.IsConnected()
	status := "disconnected"
	if connected {
		status = "connected"
	}

	return &providers.HealthStatus{
		Healthy:   connected,
		Status:    status,
		Message:   fmt.Sprintf("ZeroTier node %s is %s", nodeID, status),
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (z *ZeroTierProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	// ZeroTier logs to system logs
	return []providers.LogEntry{}, nil
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
