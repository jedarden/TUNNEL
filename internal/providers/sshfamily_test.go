package providers_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/providers/bastion"
	"github.com/jedarden/tunnel/internal/providers/reversessh"
	"github.com/jedarden/tunnel/internal/providers/sshforward"
	"github.com/jedarden/tunnel/internal/providers/vscodetunnel"
)

// TestSSHFamilyLifecycle exercises the common subprocess-backed lifecycle of
// every SSH-family provider without requiring a real relay or SSH server.
// The fake clients stay alive until Disconnect sends them an interrupt and
// write their arguments to stderr, which also verifies provider log capture.
func TestSSHFamilyLifecycle(t *testing.T) {
	fakeBinDir := t.TempDir()
	writeFakeClient(t, filepath.Join(fakeBinDir, "ssh"))
	writeFakeClient(t, filepath.Join(fakeBinDir, "code"))
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	tests := []struct {
		name     string
		provider providers.Provider
		config   *providers.ProviderConfig
		wantArgs string
	}{
		{
			name:     "vscode tunnel",
			provider: vscodetunnel.New(),
			config: &providers.ProviderConfig{
				Name: "vscode-tunnel",
				Extra: map[string]string{
					"machineName": "integration-machine",
				},
			},
			wantArgs: "tunnel --name integration-machine --accept-server-license-terms",
		},
		{
			name:     "ssh local forward",
			provider: sshforward.New(),
			config: &providers.ProviderConfig{
				Name:       "ssh-forward",
				RemoteHost: "db.example.test",
				RemotePort: 5432,
				LocalPort:  15432,
				Extra: map[string]string{
					"user": "deploy",
				},
			},
			wantArgs: "-L 127.0.0.1:15432:db.example.test:5432 deploy@db.example.test",
		},
		{
			name:     "reverse ssh",
			provider: reversessh.New(),
			config: &providers.ProviderConfig{
				Name:       "reverse-ssh",
				RemoteHost: "relay.example.test",
				LocalPort:  2222,
				Extra: map[string]string{
					"relayUsername": "tunnel",
					"remotePort":    "22222",
				},
			},
			wantArgs: "-R 22222:localhost:2222 tunnel@relay.example.test",
		},
		{
			name:     "bastion",
			provider: bastion.New(),
			config: &providers.ProviderConfig{
				Name:       "bastion",
				RemoteHost: "target.example.test",
				RemotePort: 2200,
				Extra: map[string]string{
					"jumpHost": "bastion.example.test",
				},
			},
			wantArgs: "-J bastion.example.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.provider.Configure(tt.config); err != nil {
				t.Fatalf("Configure() error = %v", err)
			}
			since := time.Now().Add(-time.Second)

			if err := tt.provider.Connect(); err != nil {
				t.Fatalf("Connect() error = %v", err)
			}
			if !tt.provider.IsConnected() {
				t.Fatal("IsConnected() = false after Connect()")
			}

			info, err := tt.provider.GetConnectionInfo()
			if err != nil {
				t.Fatalf("GetConnectionInfo() error = %v", err)
			}
			if info == nil || info.Status != "connected" {
				t.Fatalf("GetConnectionInfo() = %#v, want connected info", info)
			}
			if info.ConnectedAt.IsZero() {
				t.Error("ConnectedAt is zero for a live provider")
			}

			health, err := tt.provider.HealthCheck()
			if err != nil {
				t.Fatalf("HealthCheck() error = %v", err)
			}
			if health == nil || !health.Healthy || health.Status != "connected" {
				t.Fatalf("HealthCheck() = %#v, want healthy connected status", health)
			}

			logs, err := tt.provider.GetLogs(since)
			if err != nil {
				t.Fatalf("GetLogs() error = %v", err)
			}
			if len(logs) == 0 {
				t.Fatal("GetLogs() returned no startup entry")
			}
			if !strings.Contains(logs[0].Message, "started") {
				t.Errorf("first log = %#v, want startup entry", logs[0])
			}
			var logText strings.Builder
			for _, entry := range logs {
				logText.WriteString(entry.Message)
				logText.WriteByte('\n')
			}
			if !strings.Contains(logText.String(), tt.wantArgs) {
				t.Errorf("captured logs = %q, want command arguments %q", logText.String(), tt.wantArgs)
			}

			if err := tt.provider.Disconnect(); err != nil {
				t.Fatalf("Disconnect() error = %v", err)
			}
			if tt.provider.IsConnected() {
				t.Fatal("IsConnected() = true after Disconnect()")
			}
		})
	}
}

func TestSSHFamilyStartupFailureIsReported(t *testing.T) {
	fakeBinDir := t.TempDir()
	failingSSH := filepath.Join(fakeBinDir, "ssh")
	if err := os.WriteFile(failingSSH, []byte("#!/bin/sh\nprintf 'permission denied\\n' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider := sshforward.New()
	if err := provider.Configure(&providers.ProviderConfig{
		Name:       "ssh-forward",
		RemoteHost: "db.example.test",
		RemotePort: 5432,
		LocalPort:  15432,
	}); err != nil {
		t.Fatal(err)
	}

	err := provider.Connect()
	if !errors.Is(err, providers.ErrConnectionFailed) {
		t.Fatalf("Connect() error = %v, want ErrConnectionFailed", err)
	}
	if provider.IsConnected() {
		t.Fatal("IsConnected() = true after startup failure")
	}
	logs, logErr := provider.GetLogs(time.Now().Add(-time.Second))
	if logErr != nil {
		t.Fatalf("GetLogs() error = %v", logErr)
	}
	var logText strings.Builder
	for _, entry := range logs {
		logText.WriteString(entry.Message)
		logText.WriteByte('\n')
	}
	if !strings.Contains(logText.String(), "permission denied") {
		t.Errorf("logs = %q, want startup failure detail", logText.String())
	}
}

func writeFakeClient(t *testing.T, path string) {
	t.Helper()
	const script = `#!/bin/sh
printf 'fake client started: %s\n' "$*" >&2
trap 'printf "fake client stopped\n" >&2; exit 0' INT TERM
while :; do sleep 0.05; done
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake client: %v", err)
	}
}
