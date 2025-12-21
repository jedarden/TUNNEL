package tailscale

import (
	"encoding/json"
	"fmt"
	"os/exec"
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
	// Installation should be done manually or via package manager
	return fmt.Errorf("please install Tailscale manually from https://tailscale.com/download")
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

	if status.Self.TailscaleIPs != nil && len(status.Self.TailscaleIPs) > 0 {
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
	// Tailscale doesn't provide easy log access via CLI
	// This would require reading system logs or using tailscaled debug endpoints
	return []providers.LogEntry{}, nil
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
