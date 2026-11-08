package bastion

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/providers/procman"
)

// sshBinary is the client binary this provider wraps. It is a package
// variable so tests can substitute a fake executable.
var sshBinary = "ssh"

// connectStabilizeTimeout bounds how long Connect watches the process for
// immediate failure before reporting success.
const connectStabilizeTimeout = 1500 * time.Millisecond

// BastionProvider implements the Provider interface for bastion/jump-host
// access (ssh -J): it holds a persistent ssh session to a target host
// routed through a jump host, keeping the authenticated path warm for
// forwarding and reachability checks.
type BastionProvider struct {
	*providers.BaseProvider
	proc *procman.Manager
}

// New creates a new Bastion provider
func New() *BastionProvider {
	return &BastionProvider{
		BaseProvider: providers.NewBaseProvider("bastion", providers.CategorySSH),
		proc:         procman.New("bastion"),
	}
}

// Install explains how to install the OpenSSH client
func (b *BastionProvider) Install() error {
	if b.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install the OpenSSH client: sudo apt install openssh-client")
}

// Uninstall is not applicable
func (b *BastionProvider) Uninstall() error {
	return fmt.Errorf("SSH is a system service; please manage it through your system's package manager")
}

// IsInstalled reports whether the ssh client binary is on PATH
func (b *BastionProvider) IsInstalled() bool {
	return sshExecutable() != ""
}

// sshExecutable returns the resolved path of the ssh client this provider
// runs, or "" when no executable is on PATH.
func sshExecutable() string {
	path, err := exec.LookPath(sshBinary)
	if err != nil {
		return ""
	}
	return path
}

// bastionSpec captures the resolved parameters of one jump-host session.
type bastionSpec struct {
	jumpHost   string
	jumpPort   int
	jumpUser   string
	target     string
	targetPort int
	user       string
	identity   string
	hostKeys   string
}

func (b *BastionProvider) resolveSpec() (*bastionSpec, error) {
	config, err := b.GetConfig()
	if err != nil {
		return nil, err
	}
	if err := b.ValidateConfig(config); err != nil {
		return nil, err
	}

	spec := &bastionSpec{
		jumpHost:   config.Extra["jumpHost"],
		jumpPort:   22,
		jumpUser:   config.Extra["jumpUser"],
		target:     config.RemoteHost,
		targetPort: config.RemotePort,
		user:       config.Extra["user"],
		identity:   config.Extra["identityFile"],
		hostKeys:   config.Extra["strictHostKeyChecking"],
	}
	if spec.targetPort == 0 {
		spec.targetPort = 22
	}
	if p := config.Extra["jumpPort"]; p != "" {
		spec.jumpPort, _ = strconv.Atoi(p)
	}

	return spec, nil
}

// buildJumpArgs renders the persistent ssh invocation for one jump-host
// session. Keeping this separate from Connect makes the exact ProxyJump
// command easy to test without starting a real SSH session.
func buildJumpArgs(spec *bastionSpec) []string {
	jump := spec.jumpHost
	if spec.jumpUser != "" {
		jump = spec.jumpUser + "@" + spec.jumpHost
	}
	if spec.jumpPort != 22 {
		jump = fmt.Sprintf("%s:%d", jump, spec.jumpPort)
	}

	target := spec.target
	if spec.user != "" {
		target = spec.user + "@" + spec.target
	}

	args := []string{
		"-N",
		"-J", jump,
		"-o", "ServerAliveInterval=60",
		"-o", "ServerAliveCountMax=3",
	}
	if spec.hostKeys != "" {
		args = append(args, "-o", "StrictHostKeyChecking="+spec.hostKeys)
	} else {
		args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	}
	if spec.identity != "" {
		args = append(args, "-i", spec.identity)
	}
	if spec.targetPort != 22 {
		args = append(args, "-p", strconv.Itoa(spec.targetPort))
	}
	return append(args, target)
}

// Connect opens a persistent ssh session to the target through the jump host
func (b *BastionProvider) Connect() error {
	if !b.IsInstalled() {
		return providers.ErrNotInstalled
	}

	spec, err := b.resolveSpec()
	if err != nil {
		return err
	}

	if err := b.proc.Start(sshBinary, buildJumpArgs(spec)); err != nil {
		return err
	}

	// Surface immediate failures (unreachable jump host, auth rejection)
	// instead of reporting a dead session as connected.
	deadline := time.Now().Add(connectStabilizeTimeout)
	for time.Now().Before(deadline) {
		if !b.proc.Running() {
			detail := strings.Join(b.proc.RecentErrors(3), "; ")
			if detail == "" {
				detail = "ssh exited during startup"
			}
			return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// Disconnect terminates the jump-host session
func (b *BastionProvider) Disconnect() error {
	return b.proc.Stop(2 * time.Second)
}

// IsConnected reports whether the jump-host ssh process is alive
func (b *BastionProvider) IsConnected() bool {
	return b.proc.Running()
}

// GetConnectionInfo retrieves current connection information
func (b *BastionProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	spec, err := b.resolveSpec()
	if err != nil {
		// No usable config yet: report bare disconnected state.
		return info, nil
	}

	info.RemoteIP = spec.target
	info.Extra["type"] = "bastion-host"
	info.Extra["mode"] = "jump-server"
	info.Extra["jump_host"] = spec.jumpHost
	info.Extra["jump_port"] = spec.jumpPort
	info.Extra["target_host"] = spec.target
	info.Extra["target_port"] = spec.targetPort
	if spec.jumpUser != "" {
		info.Extra["jump_user"] = spec.jumpUser
	}
	if spec.user != "" {
		info.Extra["user"] = spec.user
	}
	// Keep the legacy summary field for callers that only need the target SSH
	// port, while the detailed fields above describe the complete route.
	info.Extra["port"] = spec.targetPort

	if b.proc.Running() {
		info.Status = "connected"
		info.ConnectedAt = b.proc.StartedAt()
		info.TunnelURL = fmt.Sprintf("%s:%d", spec.target, spec.targetPort)
		info.Extra["pid"] = b.proc.PID()
		info.Extra["executable"] = sshExecutable()
	}

	return info, nil
}

// HealthCheck reports provider health: not_installed when the ssh client is
// missing, ready when installed with no session, connected while the
// jump-host session is alive. Every installed state reports the resolved
// ssh executable.
func (b *BastionProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !b.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "OpenSSH client is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}
	executable := sshExecutable()

	if !b.proc.Running() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "ready",
			Message:   fmt.Sprintf("OpenSSH client available at %s; no bastion session active", executable),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
			Metrics:   map[string]interface{}{"executable": executable},
		}, nil
	}

	spec, err := b.resolveSpec()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("Config error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
			Metrics:   map[string]interface{}{"executable": executable},
		}, nil
	}

	return &providers.HealthStatus{
		Healthy: true,
		Status:  "connected",
		Message: fmt.Sprintf("Bastion session to %s via %s:%d is active",
			spec.target, spec.jumpHost, spec.jumpPort),
		LastCheck: time.Now(),
		Latency:   time.Since(start),
		Metrics: map[string]interface{}{
			"pid":        b.proc.PID(),
			"executable": executable,
		},
	}, nil
}

// GetLogs returns captured ssh client output since the given time
func (b *BastionProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return b.proc.Logs(since), nil
}

// validHostKeyPolicies are the StrictHostKeyChecking values accepted by ssh.
var validHostKeyPolicies = map[string]bool{
	"yes":        true,
	"no":         true,
	"ask":        true,
	"accept-new": true,
}

// ValidateConfig validates bastion-specific configuration: a jump host and a
// target must be configured, all ports on the routed path must be valid, and
// the optional authentication and host-key settings must be spellings ssh
// accepts.
func (b *BastionProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := b.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	if config.Extra["jumpHost"] == "" {
		return fmt.Errorf("%w: jump host is required for bastion", providers.ErrInvalidConfig)
	}
	if config.RemoteHost == "" {
		return fmt.Errorf("%w: target host is required for bastion", providers.ErrInvalidConfig)
	}
	for field, user := range map[string]string{
		"jumpUser": config.Extra["jumpUser"],
		"user":     config.Extra["user"],
	} {
		if user != "" && strings.TrimSpace(user) == "" {
			return fmt.Errorf("%w: %s must not be blank", providers.ErrInvalidConfig, field)
		}
	}
	if p := config.Extra["jumpPort"]; p != "" {
		if port, err := strconv.Atoi(p); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: invalid jump host port %q", providers.ErrInvalidConfig, p)
		}
	}
	if config.RemotePort < 0 || config.RemotePort > 65535 {
		return fmt.Errorf("%w: target port %d out of range", providers.ErrInvalidConfig, config.RemotePort)
	}
	if key := config.Extra["identityFile"]; key != "" && strings.TrimSpace(key) == "" {
		return fmt.Errorf("%w: identityFile must name a private key file", providers.ErrInvalidConfig)
	}
	if policy := config.Extra["strictHostKeyChecking"]; policy != "" && !validHostKeyPolicies[policy] {
		return fmt.Errorf("%w: invalid strictHostKeyChecking %q (want yes, no, ask, or accept-new)",
			providers.ErrInvalidConfig, policy)
	}
	return nil
}
