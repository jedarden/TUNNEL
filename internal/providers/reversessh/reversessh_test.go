package reversessh

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
	if provider == nil || provider.BaseProvider == nil || provider.proc == nil {
		t.Fatal("New() did not initialize the provider and process manager")
	}
	if provider.Name() != "reverse-ssh" {
		t.Fatalf("Name() = %q, want reverse-ssh", provider.Name())
	}
	if provider.Category() != providers.CategorySSH {
		t.Fatalf("Category() = %q, want %q", provider.Category(), providers.CategorySSH)
	}
	if provider.IsConnected() {
		t.Error("new provider should not be connected")
	}
}

func TestValidateConfig(t *testing.T) {
	provider := New()
	tests := []struct {
		name   string
		config *providers.ProviderConfig
		want   string
	}{
		{
			name: "nil config",
			want: "invalid configuration",
		},
		{
			name: "missing name",
			config: &providers.ProviderConfig{Extra: map[string]string{
				"relayServer": "relay.example.com",
			}},
			want: "provider name is required",
		},
		{
			name:   "missing relay server",
			config: &providers.ProviderConfig{Name: "reverse-ssh"},
			want:   "relay server is required",
		},
		{
			name: "invalid relay port",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				Extra:      map[string]string{"relayPort": "not-a-port"},
			},
			want: `invalid relayPort "not-a-port"`,
		},
		{
			name: "invalid remote port",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				Extra:      map[string]string{"remotePort": "70000"},
			},
			want: `invalid remotePort "70000"`,
		},
		{
			name: "invalid local target port",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				LocalPort:  70000,
			},
			want: "local port 70000 out of range",
		},
		{
			name: "invalid identity file",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				Extra:      map[string]string{"identityFile": "   "},
			},
			want: "identityFile must name a private key file",
		},
		{
			name: "invalid host key policy",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				Extra:      map[string]string{"strictHostKeyChecking": "maybe"},
			},
			want: `invalid strictHostKeyChecking "maybe"`,
		},
		{
			name: "valid documented configuration",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				LocalPort:  22,
				Extra: map[string]string{
					"relayPort":             "2222",
					"relayUsername":         "tunneluser",
					"remotePort":            "2223",
					"targetHost":            "127.0.0.1",
					"identityFile":          "/home/tunnel/.ssh/id_ed25519",
					"strictHostKeyChecking": "yes",
				},
			},
		},
		{
			name: "valid API remote port and UI aliases",
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.com",
				RemotePort: 2200,
				Extra: map[string]string{
					"relayHost": "relay.example.com",
					"relayUser": "tunneluser",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("ValidateConfig() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateConfig() error = %v, want text containing %q", err, tt.want)
			}
		})
	}
}

func TestBuildReverseArgs(t *testing.T) {
	tests := []struct {
		name string
		spec *reverseSpec
		want []string
	}{
		{
			name: "defaults",
			spec: &reverseSpec{
				relayServer: "relay.example.com",
				relayPort:   22,
				remotePort:  2222,
				targetHost:  "localhost",
				targetPort:  22,
			},
			want: []string{
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=accept-new",
				"-R", "2222:localhost:22",
				"relay.example.com",
			},
		},
		{
			name: "authentication and non-default relay settings",
			spec: &reverseSpec{
				relayServer:   "relay.example.com",
				relayPort:     2222,
				relayUsername: "tunnel",
				remotePort:    2200,
				targetHost:    "127.0.0.1",
				targetPort:    8080,
				identity:      "/home/tunnel/.ssh/id_ed25519",
				hostKeys:      "yes",
			},
			want: []string{
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=yes",
				"-i", "/home/tunnel/.ssh/id_ed25519",
				"-p", "2222",
				"-R", "2200:127.0.0.1:8080",
				"tunnel@relay.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildReverseArgs(tt.spec); !slices.Equal(got, tt.want) {
				t.Errorf("buildReverseArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSpecUsesDocumentedDefaultsAndAPIFields(t *testing.T) {
	provider := New()
	config := &providers.ProviderConfig{
		Name:       "reverse-ssh",
		RemoteHost: "relay.example.com",
		RemotePort: 2200,
		LocalPort:  8080,
		Extra: map[string]string{
			"relayUsername": "tunnel",
		},
	}
	if err := provider.Configure(config); err != nil {
		t.Fatal(err)
	}
	spec, err := provider.resolveSpec()
	if err != nil {
		t.Fatalf("resolveSpec() error = %v", err)
	}
	if spec.relayServer != "relay.example.com" || spec.relayPort != 22 || spec.remotePort != 2200 {
		t.Errorf("resolved relay = %#v, want relay.example.com:22:2200", spec)
	}
	if spec.targetHost != "localhost" || spec.targetPort != 8080 || spec.relayUsername != "tunnel" {
		t.Errorf("resolved target/auth = %#v, want localhost:8080 as tunnel", spec)
	}
}

func TestIsInstalledDetectsSSHExecutable(t *testing.T) {
	fakeSSHOnPath(t, "#!/bin/sh\nexit 0\n")
	if !New().IsInstalled() {
		t.Error("IsInstalled() = false with an ssh executable on PATH, want true")
	}

	t.Setenv("PATH", t.TempDir())
	if New().IsInstalled() {
		t.Error("IsInstalled() = true with no ssh executable on PATH, want false")
	}
}

func TestConnectRequiresInstalledSSH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	provider := New()
	configureReverse(t, provider, &providers.ProviderConfig{
		Name:       "reverse-ssh",
		RemoteHost: "relay.example.com",
	})

	err := provider.Connect()
	if !errors.Is(err, providers.ErrNotInstalled) {
		t.Errorf("Connect() error = %v, want ErrNotInstalled", err)
	}
	if provider.IsConnected() {
		t.Error("IsConnected() = true without an ssh executable")
	}
	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatal(err)
	}
	if health.Healthy || health.Status != "not_installed" || health.Message == "" {
		t.Errorf("HealthCheck() = %#v, want not_installed with a message", health)
	}
}

func TestReverseSSHLifecycleWithFakeSSH(t *testing.T) {
	fakeSSHOnPath(t, longLivedFakeSSH)
	provider := New()
	targetPort := freeLocalPort(t)
	configureReverse(t, provider, &providers.ProviderConfig{
		Name:       "reverse-ssh",
		RemoteHost: "relay.example.com",
		LocalPort:  targetPort,
		Extra: map[string]string{
			"relayPort":     "2222",
			"relayUsername": "tunnel",
			"remotePort":    "2200",
			"targetHost":    "127.0.0.1",
		},
	})

	since := time.Now().Add(-time.Second)
	if err := provider.Connect(); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if !provider.IsConnected() {
		t.Fatal("IsConnected() = false after Connect()")
	}

	wantArgs := fmt.Sprintf("-N -o ExitOnForwardFailure=yes -o ServerAliveInterval=60 -o ServerAliveCountMax=3 "+
		"-o StrictHostKeyChecking=accept-new -p 2222 -R 2200:127.0.0.1:%d tunnel@relay.example.com", targetPort)
	if got := provider.proc.Command(); !strings.Contains(got, wantArgs) {
		t.Errorf("managed command = %q, want it to contain %q", got, wantArgs)
	}

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != "connected" || info.LocalIP != "127.0.0.1" || info.RemoteIP != "relay.example.com" {
		t.Errorf("connection info = %#v, want connected relay/target details", info)
	}
	if info.TunnelURL != "relay.example.com:2200" || info.ConnectedAt.IsZero() {
		t.Errorf("connection info URL/time = %q/%v, want relay.example.com:2200 and a timestamp", info.TunnelURL, info.ConnectedAt)
	}
	for key, want := range map[string]interface{}{
		"type":           "reverse-ssh-tunnel",
		"relay_server":   "relay.example.com",
		"relay_port":     2222,
		"relay_username": "tunnel",
		"remote_port":    2200,
		"target_host":    "127.0.0.1",
		"target_port":    targetPort,
	} {
		if got := info.Extra[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("Extra[%q] = %v, want %v", key, got, want)
		}
	}
	if pid, _ := info.Extra["pid"].(int); pid <= 0 {
		t.Errorf("Extra[pid] = %v, want a live process id", info.Extra["pid"])
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatal(err)
	}
	if !health.Healthy || health.Status != "connected" {
		t.Errorf("HealthCheck() = %#v, want healthy connected status", health)
	}
	if want := fmt.Sprintf("Reverse tunnel relay.example.com:2200 -> 127.0.0.1:%d is active", targetPort); !strings.Contains(health.Message, want) {
		t.Errorf("HealthCheck() message = %q, want reverse-forward details", health.Message)
	}
	if reachable, ok := health.Metrics["target_service_reachable"].(bool); !ok || reachable {
		t.Errorf("target_service_reachable = %v, want false for the fake process", health.Metrics["target_service_reachable"])
	}

	logs, err := provider.GetLogs(since)
	if err != nil {
		t.Fatal(err)
	}
	logText := joinLogMessages(logs)
	for _, want := range []string{"started", wantArgs, "fake ssh started", "fake ssh: reverse forward established"} {
		if !strings.Contains(logText, want) {
			t.Errorf("captured logs = %q, want %q", logText, want)
		}
	}
	for _, entry := range logs {
		if entry.Source != "reverse-ssh" {
			t.Errorf("log source = %q, want reverse-ssh", entry.Source)
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
		t.Fatal(err)
	}
	if !strings.Contains(joinLogMessages(logs), "fake ssh stopped") {
		t.Errorf("logs after Disconnect() = %q, want teardown output", joinLogMessages(logs))
	}

	health, err = provider.HealthCheck()
	if err != nil {
		t.Fatal(err)
	}
	if health.Healthy || health.Status != "ready" {
		t.Errorf("HealthCheck() after Disconnect() = %#v, want ready", health)
	}
}

func TestConnectSurfacesStartupFailureAndCapturesLogs(t *testing.T) {
	fakeSSHOnPath(t, "#!/bin/sh\nprintf 'relay.example.com: Permission denied (publickey)\\n' >&2\nexit 255\n")
	provider := New()
	configureReverse(t, provider, &providers.ProviderConfig{
		Name:       "reverse-ssh",
		RemoteHost: "relay.example.com",
	})
	since := time.Now().Add(-time.Second)

	err := provider.Connect()
	if !errors.Is(err, providers.ErrConnectionFailed) {
		t.Fatalf("Connect() error = %v, want ErrConnectionFailed", err)
	}
	if !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("Connect() error = %v, want ssh diagnostic", err)
	}
	if provider.IsConnected() {
		t.Error("IsConnected() = true after startup failure")
	}
	logs, err := provider.GetLogs(since)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range logs {
		if strings.Contains(entry.Message, "Permission denied") {
			found = true
			if entry.Level != "Error" {
				t.Errorf("startup failure level = %q, want Error", entry.Level)
			}
		}
	}
	if !found {
		t.Errorf("captured logs = %q, want startup failure", joinLogMessages(logs))
	}
}

func TestGetConnectionInfoWithoutConfig(t *testing.T) {
	provider := New()
	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Status != "disconnected" || len(info.Extra) != 0 {
		t.Errorf("GetConnectionInfo() = %#v, want bare disconnected info", info)
	}
}

func TestGetLogsEmpty(t *testing.T) {
	logs, err := New().GetLogs(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if logs == nil {
		t.Fatal("GetLogs() returned nil")
	}
}

func TestConfigure(t *testing.T) {
	provider := New()
	config := &providers.ProviderConfig{
		Name: "reverse-ssh",
		Extra: map[string]string{
			"relayServer": "relay.example.com",
			"relayPort":   "2222",
		},
	}
	if err := provider.Configure(config); err != nil {
		t.Fatal(err)
	}
	retrieved, err := provider.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.Name != config.Name || retrieved.Extra["relayServer"] != config.Extra["relayServer"] {
		t.Errorf("GetConfig() = %#v, want configured reverse SSH settings", retrieved)
	}
}

func TestConfigureNil(t *testing.T) {
	if err := New().Configure(nil); err == nil {
		t.Error("Configure(nil) should return an error")
	}
}

func fakeSSHOnPath(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release local port: %v", err)
	}
	return port
}

const longLivedFakeSSH = `#!/bin/sh
printf 'fake ssh started: %s\n' "$*" >&2
trap 'printf "fake ssh stopped\n" >&2; exit 0' INT TERM
printf 'fake ssh: reverse forward established\n'
while :; do sleep 0.05; done
`

func configureReverse(t *testing.T, provider *ReverseSSHProvider, config *providers.ProviderConfig) {
	t.Helper()
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
}

func joinLogMessages(entries []providers.LogEntry) string {
	var sb strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&sb, "%s\n", entry.Message)
	}
	return sb.String()
}
