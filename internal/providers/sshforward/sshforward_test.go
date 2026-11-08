package sshforward

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

func TestNew(t *testing.T) {
	provider := New()
	if provider == nil {
		t.Fatal("New() returned nil")
	}
	if provider.BaseProvider == nil {
		t.Fatal("BaseProvider is nil")
	}
}

func TestName(t *testing.T) {
	provider := New()
	expected := "ssh-forward"
	if got := provider.Name(); got != expected {
		t.Errorf("Name() = %q, want %q", got, expected)
	}
}

func TestCategory(t *testing.T) {
	provider := New()
	expected := providers.CategorySSH
	if got := provider.Category(); got != expected {
		t.Errorf("Category() = %q, want %q", got, expected)
	}
}

func TestValidateConfig(t *testing.T) {
	provider := New()

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "invalid configuration",
		},
		{
			name: "missing name",
			config: &providers.ProviderConfig{
				RemoteHost: "db.example.test",
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "missing remote host",
			config: &providers.ProviderConfig{
				Name: "ssh-forward",
			},
			wantErr: true,
			errMsg:  "remote host is required",
		},
		{
			name: "valid config",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
			},
			wantErr: false,
		},
		{
			name: "local port out of range",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				LocalPort:  70000,
			},
			wantErr: true,
			errMsg:  "local port 70000 out of range",
		},
		{
			name: "negative remote port",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				RemotePort: -1,
			},
			wantErr: true,
			errMsg:  "remote port -1 out of range",
		},
		{
			name: "invalid ssh server port",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra: map[string]string{
					"port": "not-a-port",
				},
			},
			wantErr: true,
			errMsg:  `invalid SSH server port "not-a-port"`,
		},
		{
			name: "ssh server port out of range",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra: map[string]string{
					"port": "70000",
				},
			},
			wantErr: true,
			errMsg:  `invalid SSH server port "70000"`,
		},
		{
			name: "whitespace-only identity file",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra: map[string]string{
					"identityFile": "   ",
				},
			},
			wantErr: true,
			errMsg:  "identityFile must name a private key file",
		},
		{
			name: "unknown host key policy",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra: map[string]string{
					"strictHostKeyChecking": "maybe",
				},
			},
			wantErr: true,
			errMsg:  `invalid strictHostKeyChecking "maybe"`,
		},
		{
			name: "valid host key policy",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra: map[string]string{
					"strictHostKeyChecking": "yes",
					"identityFile":          "~/.ssh/id_ed25519",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestBuildForwardArgs(t *testing.T) {
	tests := []struct {
		name string
		spec *forwardSpec
		want []string
	}{
		{
			name: "defaults use accept-new and mirror the remote endpoint",
			spec: &forwardSpec{
				bindAddress: "127.0.0.1",
				localPort:   5432,
				remoteHost:  "db.example.test",
				remotePort:  5432,
				sshPort:     22,
			},
			want: []string{
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=accept-new",
				"-L", "127.0.0.1:5432:db.example.test:5432",
				"db.example.test",
			},
		},
		{
			name: "user, identity, server port and explicit policy are all passed",
			spec: &forwardSpec{
				user:        "deploy",
				bindAddress: "0.0.0.0",
				localPort:   15432,
				remoteHost:  "db.example.test",
				remotePort:  5432,
				sshPort:     2222,
				identity:    "/home/deploy/.ssh/id_ed25519",
				hostKeys:    "yes",
			},
			want: []string{
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=yes",
				"-i", "/home/deploy/.ssh/id_ed25519",
				"-p", "2222",
				"-L", "0.0.0.0:15432:db.example.test:5432",
				"deploy@db.example.test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildForwardArgs(tt.spec)
			if !slices.Equal(got, tt.want) {
				t.Errorf("buildForwardArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSpec(t *testing.T) {
	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		check   func(t *testing.T, spec *forwardSpec)
		wantErr bool
		errMsg  string
	}{
		{
			name: "local port defaults to the forwarded remote port",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				RemotePort: 5432,
			},
			check: func(t *testing.T, spec *forwardSpec) {
				if spec.localPort != 5432 {
					t.Errorf("localPort = %d, want 5432", spec.localPort)
				}
				if spec.bindAddress != "127.0.0.1" {
					t.Errorf("bindAddress = %q, want 127.0.0.1", spec.bindAddress)
				}
				if spec.sshPort != 22 {
					t.Errorf("sshPort = %d, want 22", spec.sshPort)
				}
			},
		},
		{
			name: "explicit endpoint and authentication settings are honored",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				RemotePort: 5432,
				LocalPort:  15432,
				Extra: map[string]string{
					"user":         "deploy",
					"port":         "2222",
					"bindAddress":  "0.0.0.0",
					"identityFile": "/home/deploy/.ssh/id_ed25519",
				},
			},
			check: func(t *testing.T, spec *forwardSpec) {
				if spec.user != "deploy" {
					t.Errorf("user = %q, want deploy", spec.user)
				}
				if spec.localPort != 15432 {
					t.Errorf("localPort = %d, want 15432", spec.localPort)
				}
				if spec.sshPort != 2222 {
					t.Errorf("sshPort = %d, want 2222", spec.sshPort)
				}
				if spec.bindAddress != "0.0.0.0" {
					t.Errorf("bindAddress = %q, want 0.0.0.0", spec.bindAddress)
				}
				if spec.identity != "/home/deploy/.ssh/id_ed25519" {
					t.Errorf("identity = %q, want the configured identity file", spec.identity)
				}
			},
		},
		{
			name: "missing remote host is rejected",
			config: &providers.ProviderConfig{
				Name: "ssh-forward",
			},
			wantErr: true,
			errMsg:  "remote host is required",
		},
		{
			name: "non-numeric ssh server port is rejected",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra:      map[string]string{"port": "22ssh"},
			},
			wantErr: true,
			errMsg:  `invalid SSH server port "22ssh"`,
		},
		{
			name: "out-of-range ssh server port is rejected",
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				Extra:      map[string]string{"port": "0"},
			},
			wantErr: true,
			errMsg:  `invalid SSH server port "0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := New()
			if err := provider.Configure(tt.config); err != nil {
				t.Fatalf("Configure() error = %v", err)
			}
			spec, err := provider.resolveSpec()
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("resolveSpec() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}
			if spec == nil {
				t.Fatal("resolveSpec() returned nil spec without error")
			}
			if tt.check != nil {
				tt.check(t, spec)
			}
		})
	}
}

// fakeSSHOnPath prepends a directory holding a fake ssh executable to PATH
// for the duration of the test, so provider lifecycle runs without a real
// OpenSSH client or server.
func fakeSSHOnPath(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// longLivedFakeSSH stays alive until it receives an interrupt, echoing its
// arguments to stderr and a runtime line to stdout so log capture of both
// streams can be verified.
const longLivedFakeSSH = `#!/bin/sh
printf 'fake ssh started: %s\n' "$*" >&2
trap 'printf "fake ssh stopped\n" >&2; exit 0' INT TERM
printf 'fake ssh: forward established\n'
while :; do sleep 0.05; done
`

// freeLocalPort reserves an ephemeral port and immediately releases it, so
// lifecycle tests never race a real service listening on a fixed port.
func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve ephemeral port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release ephemeral port: %v", err)
	}
	return port
}

func testForwardConfig(t *testing.T) (*providers.ProviderConfig, int) {
	t.Helper()
	localPort := freeLocalPort(t)
	return &providers.ProviderConfig{
		Name:       "ssh-forward",
		RemoteHost: "db.example.test",
		RemotePort: 5432,
		LocalPort:  localPort,
		Extra: map[string]string{
			"user": "deploy",
		},
	}, localPort
}

func logText(entries []providers.LogEntry) string {
	var sb strings.Builder
	for _, entry := range entries {
		sb.WriteString(entry.Message)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestIsInstalledDetectsSSHExecutable(t *testing.T) {
	fakeSSHOnPath(t, "#!/bin/sh\nexit 0\n")
	provider := New()
	if !provider.IsInstalled() {
		t.Error("IsInstalled() = false with an ssh executable on PATH, want true")
	}

	provider = New()
	t.Setenv("PATH", t.TempDir())
	if provider.IsInstalled() {
		t.Error("IsInstalled() = true with no ssh executable on PATH, want false")
	}
}

func TestConnectRequiresInstalledSSH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	provider := New()
	config, _ := testForwardConfig(t)
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	err := provider.Connect()
	if !errors.Is(err, providers.ErrNotInstalled) {
		t.Errorf("Connect() error = %v, want ErrNotInstalled", err)
	}
	if provider.IsConnected() {
		t.Error("IsConnected() = true without an ssh executable, want false")
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if health.Healthy || health.Status != "not_installed" || health.Message == "" {
		t.Errorf("HealthCheck() = %#v, want not_installed status with a message", health)
	}
}

func TestConnectLifecycleWithFakeSSH(t *testing.T) {
	fakeSSHOnPath(t, longLivedFakeSSH)
	provider := New()
	since := time.Now().Add(-time.Second)
	config, localPort := testForwardConfig(t)
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := provider.Connect(); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if !provider.IsConnected() {
		t.Fatal("IsConnected() = false after Connect()")
	}

	wantArgs := fmt.Sprintf("-N -o ExitOnForwardFailure=yes -o ServerAliveInterval=60 -o ServerAliveCountMax=3 "+
		"-o StrictHostKeyChecking=accept-new -L 127.0.0.1:%d:db.example.test:5432 deploy@db.example.test", localPort)
	if got := provider.proc.Command(); !strings.Contains(got, wantArgs) {
		t.Errorf("managed command = %q, want it to contain %q", got, wantArgs)
	}

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Fatalf("GetConnectionInfo() error = %v", err)
	}
	if info.Status != "connected" {
		t.Errorf("Status = %q, want connected", info.Status)
	}
	if info.LocalIP != "127.0.0.1" || info.RemoteIP != "db.example.test" {
		t.Errorf("LocalIP/RemoteIP = %q/%q, want 127.0.0.1/db.example.test", info.LocalIP, info.RemoteIP)
	}
	if want := fmt.Sprintf("127.0.0.1:%d", localPort); info.TunnelURL != want {
		t.Errorf("TunnelURL = %q, want %q", info.TunnelURL, want)
	}
	if info.ConnectedAt.IsZero() {
		t.Error("ConnectedAt is zero for a live forward")
	}
	wantExtra := map[string]interface{}{
		"type":         "ssh-local-forward",
		"bind_address": "127.0.0.1",
		"local_port":   localPort,
		"remote_host":  "db.example.test",
		"remote_port":  5432,
	}
	for key, want := range wantExtra {
		if got := info.Extra[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("Extra[%q] = %v, want %v", key, got, want)
		}
	}
	if pid, _ := info.Extra["pid"].(int); pid <= 0 {
		t.Errorf("Extra[pid] = %v, want a live process id", info.Extra["pid"])
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if !health.Healthy || health.Status != "connected" {
		t.Errorf("HealthCheck() = %#v, want healthy connected status", health)
	}
	if want := fmt.Sprintf("Forwarding 127.0.0.1:%d -> db.example.test:5432", localPort); !strings.Contains(health.Message, want) {
		t.Errorf("HealthCheck() message = %q, want it to contain %q", health.Message, want)
	}
	if pid, _ := health.Metrics["pid"].(int); pid <= 0 {
		t.Errorf("Metrics[pid] = %v, want a live process id", health.Metrics["pid"])
	}
	if open, ok := health.Metrics["local_port_open"].(bool); !ok {
		t.Errorf("Metrics[local_port_open] = %v, want a bool", health.Metrics["local_port_open"])
	} else if open {
		t.Error("Metrics[local_port_open] = true, want false: the fake ssh never listens")
	}

	logs, err := provider.GetLogs(since)
	if err != nil {
		t.Fatalf("GetLogs() error = %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("GetLogs() returned no entries")
	}
	if !strings.Contains(logs[0].Message, "started") || !strings.Contains(logs[0].Message, wantArgs) {
		t.Errorf("first log = %#v, want the started entry with %q", logs[0], wantArgs)
	}
	captured := logText(logs)
	for _, want := range []string{"fake ssh started", "fake ssh: forward established"} {
		if !strings.Contains(captured, want) {
			t.Errorf("captured logs = %q, want them to contain %q", captured, want)
		}
	}
	for _, entry := range logs {
		if entry.Source != "ssh-forward" {
			t.Errorf("log entry source = %q, want ssh-forward", entry.Source)
		}
	}

	if err := provider.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if provider.IsConnected() {
		t.Fatal("IsConnected() = true after Disconnect()")
	}

	logs, err = provider.GetLogs(since)
	if err != nil {
		t.Fatalf("GetLogs() after Disconnect() error = %v", err)
	}
	if !strings.Contains(logText(logs), "fake ssh stopped") {
		t.Errorf("captured logs after Disconnect() = %q, want the teardown entry", logText(logs))
	}

	health, err = provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() after Disconnect() error = %v", err)
	}
	if health.Healthy || health.Status != "ready" {
		t.Errorf("HealthCheck() = %#v, want ready status after Disconnect()", health)
	}
}

func TestConnectSurfacesStartupFailure(t *testing.T) {
	fakeSSHOnPath(t, "#!/bin/sh\nprintf 'deploy@db.example.test: Permission denied (publickey)\\n' >&2\nexit 255\n")
	provider := New()
	since := time.Now().Add(-time.Second)
	config, _ := testForwardConfig(t)
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	err := provider.Connect()
	if !errors.Is(err, providers.ErrConnectionFailed) {
		t.Fatalf("Connect() error = %v, want ErrConnectionFailed", err)
	}
	if !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("Connect() error = %v, want the ssh diagnostic", err)
	}
	if provider.IsConnected() {
		t.Error("IsConnected() = true after a failed startup, want false")
	}

	logs, err := provider.GetLogs(since)
	if err != nil {
		t.Fatalf("GetLogs() error = %v", err)
	}
	found := false
	for _, entry := range logs {
		if strings.Contains(entry.Message, "Permission denied") {
			found = true
			if entry.Level != "Error" {
				t.Errorf("startup failure log level = %q, want Error", entry.Level)
			}
		}
	}
	if !found {
		t.Errorf("GetLogs() = %q, want the captured startup failure", logText(logs))
	}
}

func TestGetConnectionInfoWithoutConfig(t *testing.T) {
	provider := New()

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Errorf("GetConnectionInfo() unexpected error = %v", err)
	}
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil info")
	}
	if info.Status != "disconnected" {
		t.Errorf("Status = %q, want disconnected without a usable config", info.Status)
	}
	if info.Extra == nil {
		t.Fatal("Extra map is nil")
	}
	if len(info.Extra) != 0 {
		t.Errorf("Extra = %v, want the bare disconnected state with no details", info.Extra)
	}
}

func TestConfigure(t *testing.T) {
	provider := New()

	config := &providers.ProviderConfig{
		Name:       "ssh-forward",
		RemoteHost: "db.example.test",
		LocalPort:  2222,
	}

	err := provider.Configure(config)
	if err != nil {
		t.Errorf("Configure() unexpected error = %v", err)
	}

	retrieved, err := provider.GetConfig()
	if err != nil {
		t.Errorf("GetConfig() unexpected error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetConfig() returned nil")
	}
	if retrieved.Name != config.Name {
		t.Errorf("Config.Name = %q, want %q", retrieved.Name, config.Name)
	}
	if retrieved.LocalPort != config.LocalPort {
		t.Errorf("Config.LocalPort = %d, want %d", retrieved.LocalPort, config.LocalPort)
	}
}

func TestConfigure_Nil(t *testing.T) {
	provider := New()

	err := provider.Configure(nil)
	if err == nil {
		t.Error("Configure(nil) should return error")
	}
}

func TestGetConfig_Default(t *testing.T) {
	provider := New()

	config, err := provider.GetConfig()
	if err != nil {
		t.Errorf("GetConfig() unexpected error = %v", err)
	}
	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}
}

func TestGetLogs(t *testing.T) {
	provider := New()

	since := time.Now().Add(-1 * time.Hour)
	logs, err := provider.GetLogs(since)

	if err != nil {
		t.Errorf("GetLogs() unexpected error = %v", err)
	}
	if logs == nil {
		t.Fatal("GetLogs() returned nil logs")
	}
}
