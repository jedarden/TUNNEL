package sshforward

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// SSHForwardProvider implements the Provider interface for SSH port forwarding
type SSHForwardProvider struct {
	*providers.BaseProvider
}

// New creates a new SSH Forward provider
func New() *SSHForwardProvider {
	return &SSHForwardProvider{
		BaseProvider: providers.NewBaseProvider("ssh-forward", providers.CategorySSH),
	}
}

// Install checks SSH availability (usually pre-installed)
func (s *SSHForwardProvider) Install() error {
	if s.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install OpenSSH server: sudo apt install openssh-server")
}

// Uninstall is not applicable for SSH
func (s *SSHForwardProvider) Uninstall() error {
	return fmt.Errorf("SSH is a system service; please manage it through your system's package manager")
}

// IsInstalled checks if SSH server is installed
func (s *SSHForwardProvider) IsInstalled() bool {
	cmd := exec.Command("which", "sshd")
	err := cmd.Run()
	return err == nil
}

// Connect starts SSH server if not running
func (s *SSHForwardProvider) Connect() error {
	if !s.IsInstalled() {
		return providers.ErrNotInstalled
	}

	// Check if sshd is running
	cmd := exec.Command("pgrep", "sshd")
	if err := cmd.Run(); err != nil {
		// Try to start sshd
		startCmd := exec.Command("sudo", "systemctl", "start", "sshd")
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
		}
	}

	return nil
}

// Disconnect stops SSH server
func (s *SSHForwardProvider) Disconnect() error {
	cmd := exec.Command("sudo", "systemctl", "stop", "sshd")
	return cmd.Run()
}

// IsConnected checks if SSH server is running
func (s *SSHForwardProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "sshd")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (s *SSHForwardProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if s.IsConnected() {
		info.Status = "connected"
		info.Extra["type"] = "ssh-server"
		info.Extra["port"] = 22
	}

	return info, nil
}

// HealthCheck performs a health check
func (s *SSHForwardProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !s.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "OpenSSH server is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	connected := s.IsConnected()
	status := "ready"
	message := "SSH server is installed but not running"

	if connected {
		status = "connected"
		message = "SSH server is running on port 22"
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves SSH logs
func (s *SSHForwardProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return []providers.LogEntry{}, nil
}
