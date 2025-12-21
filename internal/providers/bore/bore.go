package bore

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// BoreProvider implements the Provider interface for bore
type BoreProvider struct {
	*providers.BaseProvider
	tunnelURL string
}

// New creates a new bore provider
func New() *BoreProvider {
	return &BoreProvider{
		BaseProvider: providers.NewBaseProvider("bore", providers.CategoryTunnel),
	}
}

// Install installs bore
func (b *BoreProvider) Install() error {
	if b.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}

	// Try to install via cargo
	cmd := exec.Command("cargo", "install", "bore-cli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s\nPlease install bore manually: cargo install bore-cli",
			providers.ErrInstallFailed, string(output))
	}

	return nil
}

// Uninstall uninstalls bore
func (b *BoreProvider) Uninstall() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	cmd := exec.Command("cargo", "uninstall", "bore-cli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", providers.ErrCommandFailed, string(output))
	}

	return nil
}

// IsInstalled checks if bore is installed
func (b *BoreProvider) IsInstalled() bool {
	cmd := exec.Command("bore", "--version")
	err := cmd.Run()
	return err == nil
}

// Connect establishes a bore tunnel
func (b *BoreProvider) Connect() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := b.GetConfig()
	if err != nil {
		return err
	}

	// Default to port 22 for SSH if not specified
	localPort := config.LocalPort
	if localPort == 0 {
		localPort = 22
	}

	// Default to bore.pub as remote host
	remoteHost := config.RemoteHost
	if remoteHost == "" {
		remoteHost = "bore.pub"
	}

	// Build bore command
	args := []string{"local", fmt.Sprintf("%d", localPort), "--to", remoteHost}

	// Add remote port if specified
	if config.RemotePort > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", config.RemotePort))
	}

	// Start bore in background
	cmd := exec.Command("bore", args...)

	// Capture output to extract tunnel URL
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	// Wait a moment for bore to start and output the URL
	time.Sleep(2 * time.Second)

	// Try to read the tunnel URL from output
	// bore outputs something like: "listening at bore.pub:12345"
	buf := make([]byte, 1024)
	n, _ := stdout.Read(buf)
	if n > 0 {
		output := string(buf[:n])
		re := regexp.MustCompile(`listening at ([a-zA-Z0-9.-]+):(\d+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 2 {
			b.tunnelURL = fmt.Sprintf("%s:%s", matches[1], matches[2])
		}
	}

	return nil
}

// Disconnect terminates the bore tunnel
func (b *BoreProvider) Disconnect() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	// Kill bore process
	cmd := exec.Command("pkill", "-f", "bore local")
	_ = cmd.Run() // Ignore errors if no process found

	b.tunnelURL = ""
	return nil
}

// IsConnected checks if bore is connected
func (b *BoreProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "-f", "bore local")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (b *BoreProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !b.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if !b.IsConnected() {
		return info, nil
	}

	info.Status = "connected"

	if b.tunnelURL != "" {
		info.TunnelURL = b.tunnelURL

		// Parse host and port
		parts := strings.Split(b.tunnelURL, ":")
		if len(parts) == 2 {
			info.RemoteIP = parts[0]
		}
	}

	config, err := b.GetConfig()
	if err == nil {
		info.Extra["local_port"] = config.LocalPort
		info.Extra["remote_host"] = config.RemoteHost
	}

	return info, nil
}

// HealthCheck performs a health check
func (b *BoreProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !b.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "bore is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	connected := b.IsConnected()
	status := "disconnected"
	message := "bore tunnel is not active"

	if connected {
		status = "connected"
		message = "bore tunnel is active"

		if b.tunnelURL != "" {
			message = fmt.Sprintf("bore tunnel active at %s", b.tunnelURL)
		}
	}

	return &providers.HealthStatus{
		Healthy:   connected,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (b *BoreProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	// bore outputs to stdout when started
	return []providers.LogEntry{}, nil
}

// ValidateConfig validates bore-specific configuration
func (b *BoreProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := b.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	// All fields are optional with sensible defaults
	return nil
}
