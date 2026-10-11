package vscodetunnel

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/providers/procman"
)

// codeBinary is the VS Code CLI this provider wraps. It is a package
// variable so tests can substitute a fake executable.
var codeBinary = "code"

// connectStabilizeTimeout bounds how long Connect watches the process for
// immediate failure before reporting success.
const connectStabilizeTimeout = 1500 * time.Millisecond

// relayHost is the VS Code tunnel relay service the CLI maintains the
// tunnel through (informational reachability probe only).
const relayHost = "global.rel.tunnels.api.visualstudio.com:443"

// VSCodeTunnelProvider implements the Provider interface for VS Code
// Tunnels: it runs `code tunnel` to expose this machine to VS Code clients
// through Microsoft's relay service, reachable at
// https://vscode.dev/tunnel/<machine>.
type VSCodeTunnelProvider struct {
	*providers.BaseProvider
	proc *procman.Manager
}

// New creates a new VS Code Tunnel provider
func New() *VSCodeTunnelProvider {
	return &VSCodeTunnelProvider{
		BaseProvider: providers.NewBaseProvider("vscode-tunnel", providers.CategorySSH),
		proc:         procman.New("vscode-tunnel"),
	}
}

// Install explains how to install the VS Code CLI
func (v *VSCodeTunnelProvider) Install() error {
	if v.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install the VS Code CLI from https://code.visualstudio.com/docs/remote/tunnels")
}

// Uninstall explains that the CLI is uninstalled manually
func (v *VSCodeTunnelProvider) Uninstall() error {
	if !v.IsInstalled() {
		return providers.ErrNotInstalled
	}
	return fmt.Errorf("please uninstall the VS Code CLI manually")
}

// IsInstalled reports whether the VS Code CLI is on PATH
func (v *VSCodeTunnelProvider) IsInstalled() bool {
	_, err := exec.LookPath(codeBinary)
	return err == nil
}

// cliVersion returns the first line of `code --version`, or "" on failure.
func (v *VSCodeTunnelProvider) cliVersion() string {
	path, err := exec.LookPath(codeBinary)
	if err != nil {
		return ""
	}
	output, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)[0]
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
	if err := v.ValidateConfig(config); err != nil {
		return err
	}

	args := []string{"tunnel"}
	if name := config.Extra["machineName"]; name != "" {
		args = append(args, "--name", name)
	}
	if provider := config.Extra["provider"]; provider != "" {
		args = append(args, "--provider", provider)
	}
	// Non-interactive runs require accepting the license terms; allow
	// opting out for interactive first-run flows.
	acceptLicense := true
	if configured := config.Extra["acceptLicense"]; configured != "" {
		acceptLicense, _ = strconv.ParseBool(configured)
	}
	if acceptLicense {
		args = append(args, "--accept-server-license-terms")
	}

	if err := v.proc.Start(codeBinary, args); err != nil {
		return err
	}

	// Surface immediate failures (bad CLI install, rejected flags) instead
	// of reporting a dead tunnel as connected. Interactive authentication
	// prompts keep the process alive, so they pass this window.
	deadline := time.Now().Add(connectStabilizeTimeout)
	for time.Now().Before(deadline) {
		if !v.proc.Running() {
			detail := strings.Join(v.proc.RecentErrors(3), "; ")
			if detail == "" {
				detail = "code tunnel exited during startup"
			}
			return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// Disconnect stops this provider's tunnel process only
func (v *VSCodeTunnelProvider) Disconnect() error {
	return v.proc.Stop(2 * time.Second)
}

// IsConnected reports whether this provider's tunnel process is alive
func (v *VSCodeTunnelProvider) IsConnected() bool {
	return v.proc.Running()
}

// GetConnectionInfo retrieves current connection information
func (v *VSCodeTunnelProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	config, err := v.GetConfig()
	if err != nil {
		return nil, err
	}
	machineName := config.Extra["machineName"]

	info.Extra["type"] = "vscode-tunnel"
	if machineName != "" {
		info.Extra["machine_name"] = machineName
		// The documented web-client URL for a registered tunnel.
		info.TunnelURL = fmt.Sprintf("https://vscode.dev/tunnel/%s", machineName)
	}

	if v.proc.Running() {
		info.Status = "connected"
		info.ConnectedAt = v.proc.StartedAt()
		info.Extra["pid"] = v.proc.PID()
	}

	return info, nil
}

// HealthCheck reports provider health: not_installed when the CLI is
// missing, ready when installed with no tunnel, connected while the
// tunnel process is alive.
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

	if !v.proc.Running() {
		message := "VS Code CLI available; tunnel is not active"
		if version := v.cliVersion(); version != "" {
			message = fmt.Sprintf("VS Code CLI available (version: %s); tunnel is not active", version)
		}
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "ready",
			Message:   message,
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	config, err := v.GetConfig()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("Config error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	message := "VS Code tunnel is active"
	if name := config.Extra["machineName"]; name != "" {
		message = fmt.Sprintf("VS Code tunnel '%s' is active (via %s)", name, relayHost)
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "connected",
		Message:   message,
		LastCheck: time.Now(),
		Latency:   time.Since(start),
		Metrics:   map[string]interface{}{"pid": v.proc.PID()},
	}, nil
}

// GetLogs returns captured tunnel CLI output since the given time
func (v *VSCodeTunnelProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return v.proc.Logs(since), nil
}

// Configure validates provider-specific options before storing them.
func (v *VSCodeTunnelProvider) Configure(config *providers.ProviderConfig) error {
	if err := v.ValidateConfig(config); err != nil {
		return err
	}
	return v.BaseProvider.Configure(config)
}

// ValidateConfig validates VS Code tunnel-specific configuration
func (v *VSCodeTunnelProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := v.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	if config.Name != v.Name() {
		return fmt.Errorf("%w: provider name must be %q", providers.ErrInvalidConfig, v.Name())
	}
	if configured := config.Extra["acceptLicense"]; configured != "" {
		if _, err := strconv.ParseBool(configured); err != nil {
			return fmt.Errorf("%w: acceptLicense must be a boolean", providers.ErrInvalidConfig)
		}
	}
	// machineName and provider are passed as individual CLI arguments; the
	// code CLI remains the authority on which values it accepts.
	return nil
}
