package bastion

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// BastionProvider implements the Provider interface for bastion/jump host
type BastionProvider struct {
	*providers.BaseProvider
}

// New creates a new Bastion provider
func New() *BastionProvider {
	return &BastionProvider{
		BaseProvider: providers.NewBaseProvider("bastion", providers.CategorySSH),
	}
}

// Install checks SSH server availability for bastion mode
func (b *BastionProvider) Install() error {
	if b.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install OpenSSH server: sudo apt install openssh-server")
}

// Uninstall is not applicable
func (b *BastionProvider) Uninstall() error {
	return fmt.Errorf("SSH is a system service; please manage it through your system's package manager")
}

// IsInstalled checks if SSH server is installed
func (b *BastionProvider) IsInstalled() bool {
	cmd := exec.Command("which", "sshd")
	err := cmd.Run()
	return err == nil
}

// Connect configures and starts bastion mode
func (b *BastionProvider) Connect() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	// Ensure SSH server is running
	cmd := exec.Command("pgrep", "sshd")
	if err := cmd.Run(); err != nil {
		startCmd := exec.Command("sudo", "systemctl", "start", "sshd")
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
		}
	}

	return nil
}

// Disconnect stops bastion mode
func (b *BastionProvider) Disconnect() error {
	// In bastion mode, we typically don't stop SSH
	// Just mark as disconnected
	return nil
}

// IsConnected checks if bastion (SSH server) is running
func (b *BastionProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "sshd")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (b *BastionProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if b.IsConnected() {
		info.Status = "connected"
		info.Extra["type"] = "bastion-host"
		info.Extra["mode"] = "jump-server"
		info.Extra["port"] = 22
	}

	return info, nil
}

// HealthCheck performs a health check
func (b *BastionProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !b.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "OpenSSH server is not installed for bastion mode",
			LastCheck: time.Now(),
		}, nil
	}

	connected := b.IsConnected()
	status := "ready"
	message := "SSH server is installed, bastion mode ready"

	if connected {
		status = "connected"
		message = "Bastion host is active and accepting connections"
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves bastion logs
func (b *BastionProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return []providers.LogEntry{}, nil
}
