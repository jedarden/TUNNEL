package vscodetunnel

import (
	"fmt"
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
	if !v.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "VS Code CLI is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	// Check if code CLI works
	cmd := exec.Command("code", "--version")
	output, err := cmd.Output()

	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("VS Code CLI error: %v", err),
			LastCheck: time.Now(),
		}, nil
	}

	version := strings.TrimSpace(string(output))
	connected := v.IsConnected()
	status := "ready"
	message := fmt.Sprintf("VS Code CLI available (version: %s)", strings.Split(version, "\n")[0])

	if connected {
		status = "connected"
		message = "VS Code tunnel is active"
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs
func (v *VSCodeTunnelProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return []providers.LogEntry{}, nil
}
