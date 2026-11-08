package bastion

import (
	"errors"
	"fmt"
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
	if provider.Name() != "bastion" {
		t.Fatalf("Name() = %q, want bastion", provider.Name())
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
				"jumpHost": "bastion.example.com",
			}},
			want: "provider name is required",
		},
		{
			name:   "missing jump host",
			config: &providers.ProviderConfig{Name: "bastion", RemoteHost: "target.example.com"},
			want:   "jump host is required",
		},
		{
			name: "missing target host",
			config: &providers.ProviderConfig{
				Name: "bastion",
				Extra: map[string]string{
					"jumpHost": "bastion.example.com",
				},
			},
			want: "target host is required",
		},
		{
			name: "invalid jump port",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				Extra: map[string]string{
					"jumpHost": "bastion.example.com",
					"jumpPort": "not-a-port",
				},
			},
			want: `invalid jump host port "not-a-port"`,
		},
		{
			name: "out of range jump port",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				Extra: map[string]string{
					"jumpHost": "bastion.example.com",
					"jumpPort": "70000",
				},
			},
			want: `invalid jump host port "70000"`,
		},
		{
			name: "out of range target port",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				RemotePort: 70000,
				Extra: map[string]string{
					"jumpHost": "bastion.example.com",
				},
			},
			want: "target port 70000 out of range",
		},
		{
			name: "blank jump user",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				Extra: map[string]string{
					"jumpHost": "bastion.example.com",
					"jumpUser": "   ",
				},
			},
			want: "jumpUser must not be blank",
		},
		{
			name: "blank target user",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				Extra: map[string]string{
					"jumpHost": "bastion.example.com",
					"user":     " ",
				},
			},
			want: "user must not be blank",
		},
		{
			name: "invalid identity file",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				Extra: map[string]string{
					"jumpHost":     "bastion.example.com",
					"identityFile": "   ",
				},
			},
			want: "identityFile must name a private key file",
		},
		{
			name: "invalid host key policy",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.com",
				Extra: map[string]string{
					"jumpHost":              "bastion.example.com",
					"strictHostKeyChecking": "maybe",
				},
			},
			want: `invalid strictHostKeyChecking "maybe"`,
		},
		{
			name: "valid documented configuration",
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.internal.example.com",
				RemotePort: 22,
				Extra: map[string]string{
					"jumpHost":              "bastion.example.com",
					"jumpPort":              "2222",
					"jumpUser":              "ops",
					"user":                  "deploy",
					"identityFile":          "/home/deploy/.ssh/id_ed25519",
					"strictHostKeyChecking": "yes",
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

func TestBuildJumpArgs(t *testing.T) {
	tests := []struct {
		name string
		spec *bastionSpec
		want []string
	}{
		{
			name: "defaults",
			spec: &bastionSpec{
				jumpHost:   "bastion.example.com",
				jumpPort:   22,
				target:     "target.example.com",
				targetPort: 22,
			},
			want: []string{
				"-N",
				"-J", "bastion.example.com",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=accept-new",
				"target.example.com",
			},
		},
		{
			name: "authentication and non-default jump settings",
			spec: &bastionSpec{
				jumpHost:   "bastion.example.com",
				jumpPort:   2222,
				jumpUser:   "ops",
				target:     "target.example.com",
				targetPort: 2200,
				user:       "deploy",
				identity:   "/home/deploy/.ssh/id_ed25519",
				hostKeys:   "yes",
			},
			want: []string{
				"-N",
				"-J", "ops@bastion.example.com:2222",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=yes",
				"-i", "/home/deploy/.ssh/id_ed25519",
				"-p", "2200",
				"deploy@target.example.com",
			},
		},
		{
			name: "custom jump port without a jump user",
			spec: &bastionSpec{
				jumpHost:   "bastion.example.com",
				jumpPort:   2223,
				target:     "target.example.com",
				targetPort: 22,
			},
			want: []string{
				"-N",
				"-J", "bastion.example.com:2223",
				"-o", "ServerAliveInterval=60",
				"-o", "ServerAliveCountMax=3",
				"-o", "StrictHostKeyChecking=accept-new",
				"target.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildJumpArgs(tt.spec); !slices.Equal(got, tt.want) {
				t.Errorf("buildJumpArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSpecUsesDocumentedDefaults(t *testing.T) {
	provider := New()
	config := &providers.ProviderConfig{
		Name:       "bastion",
		RemoteHost: "target.example.com",
		Extra: map[string]string{
			"jumpHost": "bastion.example.com",
			"jumpUser": "ops",
		},
	}
	if err := provider.Configure(config); err != nil {
		t.Fatal(err)
	}
	spec, err := provider.resolveSpec()
	if err != nil {
		t.Fatalf("resolveSpec() error = %v", err)
	}
	if spec.jumpHost != "bastion.example.com" || spec.jumpPort != 22 || spec.jumpUser != "ops" {
		t.Errorf("resolved jump route = %#v, want bastion.example.com:22 as ops", spec)
	}
	if spec.target != "target.example.com" || spec.targetPort != 22 {
		t.Errorf("resolved target = %#v, want target.example.com:22", spec)
	}
	if spec.hostKeys != "" || spec.identity != "" {
		t.Errorf("resolved optional settings = %#v, want empty host-key policy and identity", spec)
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
	configureBastion(t, provider, &providers.ProviderConfig{
		Name:       "bastion",
		RemoteHost: "target.example.com",
		Extra: map[string]string{
			"jumpHost": "bastion.example.com",
		},
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

func TestBastionLifecycleWithFakeSSH(t *testing.T) {
	fakeSSHOnPath(t, longLivedFakeSSH)
	provider := New()
	configureBastion(t, provider, &providers.ProviderConfig{
		Name:       "bastion",
		RemoteHost: "target.example.com",
		RemotePort: 2200,
		Extra: map[string]string{
			"jumpHost":              "bastion.example.com",
			"jumpPort":              "2222",
			"jumpUser":              "ops",
			"user":                  "deploy",
			"identityFile":          "/home/deploy/.ssh/id_ed25519",
			"strictHostKeyChecking": "yes",
		},
	})

	since := time.Now().Add(-time.Second)
	if err := provider.Connect(); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if !provider.IsConnected() {
		t.Fatal("IsConnected() = false after Connect()")
	}

	wantArgs := "-N -J ops@bastion.example.com:2222 -o ServerAliveInterval=60 -o ServerAliveCountMax=3 " +
		"-o StrictHostKeyChecking=yes -i /home/deploy/.ssh/id_ed25519 -p 2200 deploy@target.example.com"
	if got := provider.proc.Command(); !strings.Contains(got, wantArgs) {
		t.Errorf("managed command = %q, want it to contain %q", got, wantArgs)
	}

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != "connected" || info.RemoteIP != "target.example.com" {
		t.Errorf("connection info = %#v, want connected target details", info)
	}
	if info.TunnelURL != "target.example.com:2200" || info.ConnectedAt.IsZero() {
		t.Errorf("connection info URL/time = %q/%v, want target.example.com:2200 and a timestamp", info.TunnelURL, info.ConnectedAt)
	}
	for key, want := range map[string]interface{}{
		"type":        "bastion-host",
		"mode":        "jump-server",
		"jump_host":   "bastion.example.com",
		"jump_port":   2222,
		"jump_user":   "ops",
		"target_host": "target.example.com",
		"target_port": 2200,
		"user":        "deploy",
		"port":        2200,
	} {
		if got := info.Extra[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("Extra[%q] = %v, want %v", key, got, want)
		}
	}
	if pid, _ := info.Extra["pid"].(int); pid <= 0 {
		t.Errorf("Extra[pid] = %v, want a live process id", info.Extra["pid"])
	}
	if executable, _ := info.Extra["executable"].(string); executable == "" {
		t.Errorf("Extra[executable] = %v, want the resolved ssh path", info.Extra["executable"])
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatal(err)
	}
	if !health.Healthy || health.Status != "connected" {
		t.Errorf("HealthCheck() = %#v, want healthy connected status", health)
	}
	if want := "Bastion session to target.example.com via bastion.example.com:2222 is active"; !strings.Contains(health.Message, want) {
		t.Errorf("HealthCheck() message = %q, want jump-route details", health.Message)
	}
	if pid, _ := health.Metrics["pid"].(int); pid <= 0 {
		t.Errorf("health metrics pid = %v, want a live process id", health.Metrics["pid"])
	}
	if executable, _ := health.Metrics["executable"].(string); executable == "" {
		t.Errorf("health metrics executable = %v, want the resolved ssh path", health.Metrics["executable"])
	}

	logs, err := provider.GetLogs(since)
	if err != nil {
		t.Fatal(err)
	}
	logText := joinLogMessages(logs)
	for _, want := range []string{"started", wantArgs, "fake ssh started", "fake ssh: bastion path established"} {
		if !strings.Contains(logText, want) {
			t.Errorf("captured logs = %q, want %q", logText, want)
		}
	}
	for _, entry := range logs {
		if entry.Source != "bastion" {
			t.Errorf("log source = %q, want bastion", entry.Source)
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
	if executable, _ := health.Metrics["executable"].(string); executable == "" {
		t.Errorf("ready health metrics executable = %v, want the resolved ssh path", health.Metrics["executable"])
	}
	if !strings.Contains(health.Message, "no bastion session active") {
		t.Errorf("ready health message = %q, want an idle-session note", health.Message)
	}
}

func TestConnectSurfacesStartupFailureAndCapturesLogs(t *testing.T) {
	fakeSSHOnPath(t, "#!/bin/sh\nprintf 'bastion.example.com: Permission denied (publickey)\\n' >&2\nexit 255\n")
	provider := New()
	configureBastion(t, provider, &providers.ProviderConfig{
		Name:       "bastion",
		RemoteHost: "target.example.com",
		Extra: map[string]string{
			"jumpHost": "bastion.example.com",
		},
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

func TestConnectRejectsInvalidConfigBeforeSpawn(t *testing.T) {
	fakeSSHOnPath(t, longLivedFakeSSH)
	provider := New()
	configureBastion(t, provider, &providers.ProviderConfig{
		Name:       "bastion",
		RemoteHost: "target.example.com",
		Extra: map[string]string{
			"jumpHost":              "bastion.example.com",
			"strictHostKeyChecking": "maybe",
		},
	})

	err := provider.Connect()
	if !errors.Is(err, providers.ErrInvalidConfig) {
		t.Errorf("Connect() error = %v, want ErrInvalidConfig", err)
	}
	if provider.IsConnected() {
		t.Error("IsConnected() = true after a rejected configuration")
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
		Name: "bastion",
		Extra: map[string]string{
			"jumpHost": "bastion.example.com",
			"jumpPort": "2222",
		},
	}
	if err := provider.Configure(config); err != nil {
		t.Fatal(err)
	}
	retrieved, err := provider.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.Name != config.Name || retrieved.Extra["jumpHost"] != config.Extra["jumpHost"] {
		t.Errorf("GetConfig() = %#v, want configured bastion settings", retrieved)
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

const longLivedFakeSSH = `#!/bin/sh
printf 'fake ssh started: %s\n' "$*" >&2
trap 'printf "fake ssh stopped\n" >&2; exit 0' INT TERM
printf 'fake ssh: bastion path established\n'
while :; do sleep 0.05; done
`

func configureBastion(t *testing.T, provider *BastionProvider, config *providers.ProviderConfig) {
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
