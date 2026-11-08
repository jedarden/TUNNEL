package wireguard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jedarden/tunnel/internal/providers/wireguard"
)

// writeFakeTool creates an executable shell script that succeeds when run,
// standing in for a WireGuard tools binary found on PATH.
func writeFakeTool(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
	return path
}

// TestIsInstalledRequiresBothTools pins availability detection to both
// WireGuard tools: Connect and Disconnect drive the interface through
// wg-quick, while status, statistics and log retrieval come from wg and the
// wg-quick@* systemd units. A PATH exposing only one of the two must not
// report the provider as installed.
func TestIsInstalledRequiresBothTools(t *testing.T) {
	tests := []struct {
		name  string
		tools []string
		want  bool
	}{
		{name: "both wg and wg-quick present", tools: []string{"wg", "wg-quick"}, want: true},
		{name: "wg without wg-quick", tools: []string{"wg"}, want: false},
		{name: "wg-quick without wg", tools: []string{"wg-quick"}, want: false},
		{name: "neither tool present", tools: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binDir := t.TempDir()
			for _, tool := range tt.tools {
				writeFakeTool(t, binDir, tool)
			}
			t.Setenv("PATH", binDir)

			provider := wireguard.New()
			if got := provider.IsInstalled(); got != tt.want {
				t.Fatalf("IsInstalled() with PATH containing %v = %v, want %v", tt.tools, got, tt.want)
			}
		})
	}
}

// TestIsInstalledIgnoresNonExecutableStubs covers the PATH entry that exists
// but is not executable: LookPath must reject it rather than count the tool
// as installed.
func TestIsInstalledIgnoresNonExecutableStubs(t *testing.T) {
	binDir := t.TempDir()
	for _, tool := range []string{"wg", "wg-quick"} {
		path := filepath.Join(binDir, tool)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
			t.Fatalf("write stub %s: %v", tool, err)
		}
	}
	t.Setenv("PATH", binDir)

	if provider := wireguard.New(); provider.IsInstalled() {
		t.Fatal("IsInstalled() = true for non-executable tool stubs, want false")
	}
}
