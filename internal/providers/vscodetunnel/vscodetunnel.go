package vscodetunnel

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// VSCodeTunnelProvider implements the Provider interface for VS Code Tunnels
type VSCodeTunnelProvider struct {
	*providers.BaseProvider
}

// New creates a new VS Code Tunnel provider
func New() *VSCodeTunnelProvider {
	return &VSCodeTunnelProvider{
		BaseProvider: providers.NewBaseProvider("vscode-tunnel", providers.CategorySSH),
	}
}

// Install installs the VS Code CLI (code tunnel)
func (v *VSCodeTunnelProvider) Install() error {
	if v.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}

	// VS Code CLI is typically installed with VS Code or can be downloaded
	return fmt.Errorf("please install VS Code CLI from https://code.visualstudio.com/docs/remote/tunnels")
}

// Uninstall uninstalls VS Code tunnel
func (v *VSCodeTunnelProvider) Uninstall() error {
	if !v.IsInstalled() {
		return providers.ErrNotInstalled
	}
	return fmt.Errorf("please uninstall VS Code CLI manually")
}

// IsInstalled checks if VS Code CLI is installed
func (v *VSCodeTunnelProvider) IsInstalled() bool {
	cmd := exec.Command("code", "tunnel", "--help")
	err := cmd.Run()
	return err == nil
}

// Connect starts a VS Code tunnel
func (v *VSCodeTunnelProvider) Connect() error {
	if !v.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := v.GetConfig()
	if err != nil {
		return err
	}

	args := []string{"tunnel"}

	// Add machine name if provided
	if config.Extra != nil {
		if name, ok := config.Extra["machineName"]; ok && name != "" {
			args = append(args, "--name", name)
		}
	}

	cmd := exec.Command("code", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	// Wait for tunnel to start
	time.Sleep(5 * time.Second)

	return nil
}

// Disconnect stops the VS Code tunnel
func (v *VSCodeTunnelProvider) Disconnect() error {
	cmd := exec.Command("pkill", "-f", "code tunnel")
	_ = cmd.Run()
	return nil
}

// IsConnected checks if VS Code tunnel is running
func (v *VSCodeTunnelProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "-f", "code tunnel")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (v *VSCodeTunnelProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if v.IsConnected() {
		info.Status = "connected"
		info.Extra["type"] = "vscode-tunnel"
	}

	return info, nil
}

// HealthCheck performs a health check
func (v *VSCodeTunnelProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !v.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "VS Code CLI is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Check if code CLI works
	cmd := exec.Command("code", "--version")
	output, err := cmd.Output()

	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "cli_error",
			Message:   fmt.Sprintf("VS Code CLI error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	version := strings.TrimSpace(string(output))

	// Check if tunnel process is running
	connected := v.IsConnected()
	if !connected {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "disconnected",
			Message:   fmt.Sprintf("VS Code CLI available (version: %s) but tunnel is not active", strings.Split(version, "\n")[0]),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// VS Code tunnels typically expose a local port, test connectivity to the service
	// Default VS Code server port is usually dynamic, but we can test if any local port is accessible
	config, err := v.GetConfig()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "config_error",
			Message:   fmt.Sprintf("Config error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Try to test connectivity to common VS Code tunnel ports if configured
	testPort := config.LocalPort
	if testPort > 0 {
		timeout := 2 * time.Second
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", testPort), timeout)
		if err != nil {
			return &providers.HealthStatus{
				Healthy:   false,
				Status:    "unreachable",
				Message:   fmt.Sprintf("VS Code tunnel service on port %d is not reachable: %v", testPort, err),
				LastCheck: time.Now(),
				Latency:   time.Since(start),
			}, nil
		}
		conn.Close()
	}

	// If no specific port is configured, verify VS Code service connectivity
	// by checking if the tunnel can reach VS Code's relay service
	timeout := 3 * time.Second
	conn, err := net.DialTimeout("tcp", "tunnel.antlr.vscode.dev:443", timeout)
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "relay_unreachable",
			Message:   fmt.Sprintf("VS Code tunnel relay service is not reachable: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}
	conn.Close()

	message := fmt.Sprintf("VS Code tunnel is active (version: %s)", strings.Split(version, "\n")[0])
	if testPort > 0 {
		message += fmt.Sprintf(" with service reachable on port %d", testPort)
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "connected",
		Message:   message,
		LastCheck: time.Now(),
		Latency:   time.Since(start),
	}, nil
}

// GetLogs retrieves logs
func (v *VSCodeTunnelProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return []providers.LogEntry{}, nil
}
