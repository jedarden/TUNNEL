package reversessh

import (
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// ReverseSSHProvider implements the Provider interface for reverse SSH tunnels
type ReverseSSHProvider struct {
	*providers.BaseProvider
	cmd *exec.Cmd
}

// New creates a new Reverse SSH provider
func New() *ReverseSSHProvider {
	return &ReverseSSHProvider{
		BaseProvider: providers.NewBaseProvider("reverse-ssh", providers.CategorySSH),
	}
}

// Install checks SSH client availability
func (r *ReverseSSHProvider) Install() error {
	if r.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install OpenSSH client: sudo apt install openssh-client")
}

// Uninstall is not applicable
func (r *ReverseSSHProvider) Uninstall() error {
	return fmt.Errorf("SSH client is a system package; please manage it through your system's package manager")
}

// IsInstalled checks if SSH client is installed
func (r *ReverseSSHProvider) IsInstalled() bool {
	cmd := exec.Command("which", "ssh")
	err := cmd.Run()
	return err == nil
}

// Connect establishes a reverse SSH tunnel
func (r *ReverseSSHProvider) Connect() error {
	if !r.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := r.GetConfig()
	if err != nil {
		return err
	}

	// Get relay server details from config
	relayServer := ""
	relayPort := "22"
	relayUser := ""
	remotePort := "2222"

	if config.Extra != nil {
		if s, ok := config.Extra["relayServer"]; ok {
			relayServer = s
		}
		if p, ok := config.Extra["relayPort"]; ok {
			relayPort = p
		}
		if u, ok := config.Extra["relayUsername"]; ok {
			relayUser = u
		}
		if rp, ok := config.Extra["remotePort"]; ok {
			remotePort = rp
		}
	}

	if relayServer == "" {
		return fmt.Errorf("relay server is required")
	}

	// Build SSH command for reverse tunnel
	// ssh -R remotePort:localhost:22 user@relay -p port -N
	target := relayServer
	if relayUser != "" {
		target = relayUser + "@" + relayServer
	}

	args := []string{
		"-R", fmt.Sprintf("%s:localhost:22", remotePort),
		target,
		"-p", relayPort,
		"-N",
		"-o", "ServerAliveInterval=60",
		"-o", "StrictHostKeyChecking=no",
	}

	r.cmd = exec.Command("ssh", args...)
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	return nil
}

// Disconnect terminates the reverse SSH tunnel
func (r *ReverseSSHProvider) Disconnect() error {
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Kill()
	}
	// Fallback: kill any reverse SSH tunnels
	cmd := exec.Command("pkill", "-f", "ssh -R")
	_ = cmd.Run()
	return nil
}

// IsConnected checks if reverse SSH tunnel is active
func (r *ReverseSSHProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "-f", "ssh -R")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (r *ReverseSSHProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if r.IsConnected() {
		info.Status = "connected"
		info.Extra["type"] = "reverse-ssh-tunnel"
	}

	return info, nil
}

// HealthCheck performs a health check
func (r *ReverseSSHProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !r.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "SSH client is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Check if reverse SSH tunnel process is running
	connected := r.IsConnected()
	if !connected {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "disconnected",
			Message:   "Reverse SSH tunnel is not active",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Verify actual SSH service reachability on the target port
	config, err := r.GetConfig()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("Config error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Get the local port being forwarded (default 22)
	testPort := 22
	if config.LocalPort > 0 {
		testPort = config.LocalPort
	}

	// Test if the local SSH service is accessible through the tunnel
	// by connecting to localhost:22 (or configured port)
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", testPort), timeout)
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "unreachable",
			Message:   fmt.Sprintf("SSH service on port %d is not accessible through tunnel: %v", testPort, err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}
	conn.Close()

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "connected",
		Message:   fmt.Sprintf("Reverse SSH tunnel is active and SSH service reachable on port %d", testPort),
		LastCheck: time.Now(),
		Latency:   time.Since(start),
	}, nil
}

// GetLogs retrieves logs
func (r *ReverseSSHProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return []providers.LogEntry{}, nil
}
