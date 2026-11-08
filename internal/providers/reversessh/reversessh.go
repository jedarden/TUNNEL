package reversessh

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

// connectStabilizeTimeout bounds how long Connect watches the process for
// immediate failure before reporting success.
const connectStabilizeTimeout = 1500 * time.Millisecond

// ReverseSSHProvider implements the Provider interface for reverse SSH
// tunnels (ssh -N -R): it runs the system ssh client so a port on the relay
// server forwards back to a service on this machine.
type ReverseSSHProvider struct {
	*providers.BaseProvider
	proc *procman.Manager
}

// New creates a new Reverse SSH provider
func New() *ReverseSSHProvider {
	return &ReverseSSHProvider{
		BaseProvider: providers.NewBaseProvider("reverse-ssh", providers.CategorySSH),
		proc:         procman.New("reverse-ssh"),
	}
}

// Install explains how to install the OpenSSH client
func (r *ReverseSSHProvider) Install() error {
	if r.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install the OpenSSH client: sudo apt install openssh-client")
}

// Uninstall is not applicable
func (r *ReverseSSHProvider) Uninstall() error {
	return fmt.Errorf("SSH client is a system package; please manage it through your system's package manager")
}

// IsInstalled reports whether the ssh client binary is on PATH
func (r *ReverseSSHProvider) IsInstalled() bool {
	_, err := exec.LookPath(sshBinary)
	return err == nil
}

// reverseSpec captures the resolved parameters of one reverse tunnel.
type reverseSpec struct {
	relayServer   string
	relayPort     int
	relayUsername string
	remotePort    int // port opened on the relay
	targetHost    string
	targetPort    int // local service the relay port forwards to
	identity      string
	hostKeys      string
}

func (r *ReverseSSHProvider) resolveSpec() (*reverseSpec, error) {
	config, err := r.GetConfig()
	if err != nil {
		return nil, err
	}
	if err := r.ValidateConfig(config); err != nil {
		return nil, err
	}

	// RemoteHost is accepted as an alias for Extra["relayServer"]. The
	// relayHost spelling is also accepted because it is used by the web UI.
	relayServer := firstNonEmpty(config.Extra["relayServer"], config.Extra["relayHost"], config.RemoteHost)
	remotePort := config.RemotePort
	if p := config.Extra["remotePort"]; p != "" {
		remotePort, _ = strconv.Atoi(p)
	}
	if remotePort == 0 {
		remotePort = 2222
	}

	spec := &reverseSpec{
		relayServer:   relayServer,
		relayPort:     22,
		relayUsername: firstNonEmpty(config.Extra["relayUsername"], config.Extra["relayUser"]),
		remotePort:    remotePort,
		targetHost:    firstNonEmpty(config.Extra["targetHost"], "localhost"),
		targetPort:    22,
		identity:      config.Extra["identityFile"],
		hostKeys:      config.Extra["strictHostKeyChecking"],
	}

	if p := firstNonEmpty(config.Extra["relayPort"], config.Extra["port"]); p != "" {
		port, err := strconv.Atoi(p)
		spec.relayPort = port
		if err != nil {
			return nil, fmt.Errorf("%w: invalid relay port %q", providers.ErrInvalidConfig, p)
		}
	}
	// LocalPort addresses the local service being exposed; it defaults
	// to the conventional SSH port of this machine.
	if config.LocalPort > 0 {
		spec.targetPort = config.LocalPort
	}

	return spec, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// buildReverseArgs renders the persistent ssh invocation for one reverse
// forward. Keeping this separate from Connect makes the exact command easy to
// test without starting a real SSH session.
func buildReverseArgs(spec *reverseSpec) []string {
	args := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
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
	if spec.relayPort != 22 {
		args = append(args, "-p", strconv.Itoa(spec.relayPort))
	}

	target := spec.relayServer
	if spec.relayUsername != "" {
		target = spec.relayUsername + "@" + spec.relayServer
	}
	return append(args,
		"-R", fmt.Sprintf("%d:%s:%d", spec.remotePort, spec.targetHost, spec.targetPort),
		target)
}

// Connect establishes a reverse SSH tunnel to the relay server
func (r *ReverseSSHProvider) Connect() error {
	if !r.IsInstalled() {
		return providers.ErrNotInstalled
	}

	spec, err := r.resolveSpec()
	if err != nil {
		return err
	}

	if err := r.proc.Start(sshBinary, buildReverseArgs(spec)); err != nil {
		return err
	}

	// Surface immediate failures (auth, unreachable relay, relay refusing
	// the remote bind) instead of reporting a dead tunnel as connected.
	deadline := time.Now().Add(connectStabilizeTimeout)
	for time.Now().Before(deadline) {
		if !r.proc.Running() {
			detail := strings.Join(r.proc.RecentErrors(3), "; ")
			if detail == "" {
				detail = "ssh exited during startup"
			}
			return fmt.Errorf("%w: %s", providers.ErrConnectionFailed, detail)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// Disconnect terminates the reverse SSH tunnel process
func (r *ReverseSSHProvider) Disconnect() error {
	return r.proc.Stop(2 * time.Second)
}

// IsConnected reports whether the reverse tunnel ssh process is alive
func (r *ReverseSSHProvider) IsConnected() bool {
	return r.proc.Running()
}

// GetConnectionInfo retrieves current connection information
func (r *ReverseSSHProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	spec, err := r.resolveSpec()
	if err != nil {
		// No usable config yet: report bare disconnected state.
		return info, nil
	}

	info.LocalIP = spec.targetHost
	info.RemoteIP = spec.relayServer
	info.Extra["type"] = "reverse-ssh-tunnel"
	info.Extra["relay_server"] = spec.relayServer
	info.Extra["relay_port"] = spec.relayPort
	info.Extra["remote_port"] = spec.remotePort
	info.Extra["target_host"] = spec.targetHost
	info.Extra["target_port"] = spec.targetPort
	info.Extra["target"] = fmt.Sprintf("%s:%d", spec.targetHost, spec.targetPort)
	if spec.relayUsername != "" {
		info.Extra["relay_username"] = spec.relayUsername
	}

	if r.proc.Running() {
		info.Status = "connected"
		info.ConnectedAt = r.proc.StartedAt()
		info.TunnelURL = fmt.Sprintf("%s:%d", spec.relayServer, spec.remotePort)
		info.Extra["pid"] = r.proc.PID()
	}

	return info, nil
}

// HealthCheck reports provider health. The tunnel is healthy while its ssh
// process lives; the local backing service is probed for reporting.
func (r *ReverseSSHProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !r.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "OpenSSH client is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	if !r.proc.Running() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "ready",
			Message:   "OpenSSH client available; no reverse tunnel active",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	spec, err := r.resolveSpec()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("Config error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	message := fmt.Sprintf("Reverse tunnel %s:%d -> %s:%d is active",
		spec.relayServer, spec.remotePort, spec.targetHost, spec.targetPort)
	metrics := map[string]interface{}{"pid": r.proc.PID()}

	// When the exposed service is local, probe it so the report can say
	// whether the far side would actually reach anything.
	if host := spec.targetHost; host == "localhost" || net.ParseIP(host) != nil {
		probeHost := host
		if probeHost == "localhost" {
			probeHost = "127.0.0.1"
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(probeHost, strconv.Itoa(spec.targetPort)), 250*time.Millisecond)
		if err != nil {
			metrics["target_service_reachable"] = false
			message += " (warning: target service not reachable)"
		} else {
			conn.Close()
			metrics["target_service_reachable"] = true
			message += " (target service reachable)"
		}
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
func (r *ReverseSSHProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return r.proc.Logs(since), nil
}

// validHostKeyPolicies are the StrictHostKeyChecking values accepted by ssh.
var validHostKeyPolicies = map[string]bool{
	"yes":        true,
	"no":         true,
	"ask":        true,
	"accept-new": true,
}

// ValidateConfig validates reverse-ssh-specific configuration: a relay must
// be configured, all forward ports must be valid, and optional authentication
// and host-key settings must be safe ssh values.
func (r *ReverseSSHProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := r.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}

	if relay := firstNonEmpty(config.Extra["relayServer"], config.Extra["relayHost"], config.RemoteHost); strings.TrimSpace(relay) == "" {
		return fmt.Errorf("%w: relay server is required for reverse-ssh", providers.ErrInvalidConfig)
	}
	if username := firstNonEmpty(config.Extra["relayUsername"], config.Extra["relayUser"]); username != "" && strings.TrimSpace(username) == "" {
		return fmt.Errorf("%w: relay username must not be blank", providers.ErrInvalidConfig)
	}

	for key, value := range map[string]string{
		"relayPort":  firstNonEmpty(config.Extra["relayPort"], config.Extra["port"]),
		"remotePort": firstNonEmpty(config.Extra["remotePort"], strconv.Itoa(config.RemotePort)),
	} {
		if value == "" || (key == "remotePort" && value == "0") {
			continue
		}
		if port, err := strconv.Atoi(value); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: invalid %s %q", providers.ErrInvalidConfig, key, value)
		}
	}
	if config.LocalPort < 0 || config.LocalPort > 65535 {
		return fmt.Errorf("%w: local port %d out of range", providers.ErrInvalidConfig, config.LocalPort)
	}
	if host := config.Extra["targetHost"]; host != "" && strings.TrimSpace(host) == "" {
		return fmt.Errorf("%w: targetHost must not be blank", providers.ErrInvalidConfig)
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
