package sshforward

import (
	"fmt"
	"net"
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

// connectStabilizeTimeout bounds how long Connect waits for the forward to
// become usable before returning success (the process staying alive past
// this window is treated as connected; early exits surface as errors).
const connectStabilizeTimeout = 1500 * time.Millisecond

// SSHForwardProvider implements the Provider interface for SSH local port
// forwarding (ssh -N -L): it runs the system ssh client to bind a local
// listener that forwards to host:port on the remote side.
type SSHForwardProvider struct {
	*providers.BaseProvider
	proc *procman.Manager
}

// New creates a new SSH Forward provider
func New() *SSHForwardProvider {
	return &SSHForwardProvider{
		BaseProvider: providers.NewBaseProvider("ssh-forward", providers.CategorySSH),
		proc:         procman.New("ssh-forward"),
	}
}

// Install explains how to install the OpenSSH client (never auto-installed)
func (s *SSHForwardProvider) Install() error {
	if s.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install the OpenSSH client: sudo apt install openssh-client")
}

// Uninstall is not applicable for SSH
func (s *SSHForwardProvider) Uninstall() error {
	return fmt.Errorf("SSH is a system service; please manage it through your system's package manager")
}

// IsInstalled reports whether the ssh client binary is on PATH
func (s *SSHForwardProvider) IsInstalled() bool {
	_, err := exec.LookPath(sshBinary)
	return err == nil
}

// forwardSpec captures the resolved parameters of one local forward.
type forwardSpec struct {
	user        string
	bindAddress string
	localPort   int
	remoteHost  string
	remotePort  int
	sshPort     int
	identity    string
	hostKeys    string
}

func (s *SSHForwardProvider) resolveSpec() (*forwardSpec, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}

	if config.RemoteHost == "" {
		return nil, fmt.Errorf("%w: remote host is required for ssh-forward", providers.ErrInvalidConfig)
	}

	spec := &forwardSpec{
		user:        config.Extra["user"],
		bindAddress: config.Extra["bindAddress"],
		remoteHost:  config.RemoteHost,
		remotePort:  config.RemotePort,
		sshPort:     22,
		identity:    config.Extra["identityFile"],
		hostKeys:    config.Extra["strictHostKeyChecking"],
	}

	if spec.bindAddress == "" {
		spec.bindAddress = "127.0.0.1"
	}
	if spec.remotePort == 0 {
		spec.remotePort = 22
	}
	// The local listener defaults to mirroring the forwarded remote port.
	spec.localPort = config.LocalPort
	if spec.localPort == 0 {
		spec.localPort = spec.remotePort
	}
	if p := config.Extra["port"]; p != "" {
		port, err := strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: invalid SSH server port %q", providers.ErrInvalidConfig, p)
		}
		spec.sshPort = port
	}

	return spec, nil
}

// Connect starts the ssh client holding a local port forward open
func (s *SSHForwardProvider) Connect() error {
	if !s.IsInstalled() {
		return providers.ErrNotInstalled
	}

	spec, err := s.resolveSpec()
	if err != nil {
		return err
	}

	args := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=60",
		"-o", "ServerAliveCountMax=3",
	}
	if spec.hostKeys != "" {
		args = append(args, "-o", "StrictHostKeyChecking="+spec.hostKeys)
	} else {
		// accept-new: trust first contact, still refuse changed host keys
		args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	}
	if spec.identity != "" {
		args = append(args, "-i", spec.identity)
	}
	if spec.sshPort != 22 {
		args = append(args, "-p", strconv.Itoa(spec.sshPort))
	}

	target := spec.remoteHost
	if spec.user != "" {
		target = spec.user + "@" + spec.remoteHost
	}
	args = append(args, "-L", fmt.Sprintf("%s:%d:%s:%d", spec.bindAddress, spec.localPort, spec.remoteHost, spec.remotePort), target)

	if err := s.proc.Start(sshBinary, args); err != nil {
		return err
	}

	// Give immediate failures (bad key, unreachable host, occupied local
	// port) a chance to surface, and succeed early once the listener answers.
	deadline := time.Now().Add(connectStabilizeTimeout)
	for time.Now().Before(deadline) {
		if !s.proc.Running() {
			return s.connectFailure()
		}
		if s.localForwardReady(spec) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// connectFailure converts a dead process plus its captured stderr into an
// actionable connection error.
func (s *SSHForwardProvider) connectFailure() error {
	detail := strings.Join(s.proc.RecentErrors(3), "; ")
	if detail == "" {
		detail = "ssh exited during startup"
	}
	return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, detail)
}

// localForwardReady reports whether the local listener accepts connections.
// Non-loopback bind addresses are not probed from this process.
func (s *SSHForwardProvider) localForwardReady(spec *forwardSpec) bool {
	host := spec.bindAddress
	if host == "0.0.0.0" || host == "*" {
		host = "127.0.0.1"
	}
	if net.ParseIP(host) == nil && host != "localhost" {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(spec.localPort)), 250*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Disconnect terminates the ssh process backing the forward
func (s *SSHForwardProvider) Disconnect() error {
	return s.proc.Stop(2 * time.Second)
}

// IsConnected reports whether the forwarding ssh process is alive
func (s *SSHForwardProvider) IsConnected() bool {
	return s.proc.Running()
}

// GetConnectionInfo retrieves current connection information
func (s *SSHForwardProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	spec, err := s.resolveSpec()
	if err != nil {
		// No usable config yet: report bare disconnected state.
		return info, nil
	}

	info.LocalIP = spec.bindAddress
	info.RemoteIP = spec.remoteHost
	info.Extra["type"] = "ssh-local-forward"
	info.Extra["bind_address"] = spec.bindAddress
	info.Extra["local_port"] = spec.localPort
	info.Extra["remote_host"] = spec.remoteHost
	info.Extra["remote_port"] = spec.remotePort

	if s.proc.Running() {
		info.Status = "connected"
		info.ConnectedAt = s.proc.StartedAt()
		info.TunnelURL = fmt.Sprintf("%s:%d", spec.bindAddress, spec.localPort)
		info.Extra["pid"] = s.proc.PID()
	}

	return info, nil
}

// HealthCheck reports provider health: not_installed when the ssh client is
// missing, ready when installed but no forward is running, and connected
// while the forward process is alive (with a probe of the local listener).
func (s *SSHForwardProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !s.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "OpenSSH client is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	if !s.proc.Running() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "ready",
			Message:   "OpenSSH client available; no local forward active",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	spec, err := s.resolveSpec()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("Config error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// The forward is serving as long as the process lives; probe the local
	// listener for reporting, without failing health on a transient refusal
	// (ssh may still be completing the handshake).
	message := fmt.Sprintf("Forwarding %s:%d -> %s:%d", spec.bindAddress, spec.localPort, spec.remoteHost, spec.remotePort)
	metrics := map[string]interface{}{"pid": s.proc.PID()}
	if s.localForwardReady(spec) {
		metrics["local_port_open"] = true
		message += " (listener accepting connections)"
	} else {
		metrics["local_port_open"] = false
		message += " (listener not accepting yet)"
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "connected",
		Message:   message,
		LastCheck: time.Now(),
		Latency:   time.Since(start),
		Metrics:   metrics,
	}, nil
}

// GetLogs returns captured ssh client output since the given time
func (s *SSHForwardProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return s.proc.Logs(since), nil
}

// ValidateConfig validates SSH-forward-specific configuration
func (s *SSHForwardProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := s.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	for _, port := range []struct {
		name  string
		value int
	}{
		{"local port", config.LocalPort},
		{"remote port", config.RemotePort},
	} {
		if port.value < 0 || port.value > 65535 {
			return fmt.Errorf("%w: %s %d out of range", providers.ErrInvalidConfig, port.name, port.value)
		}
	}
	if p := config.Extra["port"]; p != "" {
		if port, err := strconv.Atoi(p); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: invalid SSH server port %q", providers.ErrInvalidConfig, p)
		}
	}
	return nil
}
