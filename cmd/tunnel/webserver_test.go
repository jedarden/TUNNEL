package main

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	embeddedfs "github.com/jedarden/tunnel/internal/web/embed"
)

// repoRoot walks up from the test's working directory to the module root.
// Absolute paths are used because relative paths that leave the package
// directory do not resolve in all sandboxed test environments.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found in any parent of %s", dir)
		}
		dir = parent
	}
}

// TestWebUIDefaultAddressMatchesDocs pins the documented serving address:
// README.md and configs/default.yaml promise the web UI on localhost:8080,
// loopback only. If any of these intentionally change, update the docs in
// the same change.
func TestWebUIDefaultAddressMatchesDocs(t *testing.T) {
	// Flag defaults are registered by init() and must stay at 8080.
	if portFlag := rootCmd.PersistentFlags().Lookup("port"); portFlag == nil {
		t.Fatal("--port flag is not registered on the root command")
	} else if portFlag.DefValue != "8080" {
		t.Errorf("--port default = %q, want %q (README documents localhost:8080)", portFlag.DefValue, "8080")
	}
	if webPort != 8080 {
		t.Errorf("webPort default = %d, want 8080", webPort)
	}

	// Viper defaults must bind the UI to loopback unless overridden.
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
		setDefaults()
	})
	setDefaults()
	if got := viper.GetString("web.listen_address"); got != "127.0.0.1" {
		t.Errorf("web.listen_address default = %q, want 127.0.0.1", got)
	}
	if got := resolveWebHost(); got != "127.0.0.1" {
		t.Errorf("resolveWebHost() with defaults = %q, want 127.0.0.1 (loopback-only)", got)
	}
}

// TestWebHostOverrideIsHonored verifies the documented escape hatch for
// intentional LAN exposure: --host / TUNNEL_WEB_LISTEN_ADDRESS / config
// value must reach the bind address.
func TestWebHostOverrideIsHonored(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
		setDefaults()
	})
	setDefaults()

	t.Setenv("TUNNEL_WEB_LISTEN_ADDRESS", "0.0.0.0")
	if got := resolveWebHost(); got != "0.0.0.0" {
		t.Errorf("resolveWebHost() with TUNNEL_WEB_LISTEN_ADDRESS=0.0.0.0 = %q, want 0.0.0.0", got)
	}

	viper.Set("web.listen_address", "192.0.2.10")
	if got := resolveWebHost(); got != "192.0.2.10" {
		t.Errorf("resolveWebHost() with web.listen_address=192.0.2.10 = %q, want 192.0.2.10", got)
	}
}

// TestDocsStillPromiseLocalhost8080 keeps the README's documented address
// and the shipped default configuration in sync with the pinned defaults.
func TestDocsStillPromiseLocalhost8080(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	if !strings.Contains(string(readme), "localhost:8080") {
		t.Error("README.md no longer mentions localhost:8080; update it together with the default port")
	}

	cfg, err := os.ReadFile(filepath.Join(repoRoot(t), "configs", "default.yaml"))
	if err != nil {
		t.Fatalf("reading configs/default.yaml: %v", err)
	}
	cfgText := string(cfg)
	if !strings.Contains(cfgText, "listen_address: 127.0.0.1") {
		t.Error("configs/default.yaml no longer sets listen_address: 127.0.0.1")
	}
	if !strings.Contains(cfgText, "port: 8080") {
		t.Error("configs/default.yaml no longer sets port: 8080")
	}
}

// TestBinaryEmbedsWebUI is the literal embedding smoke test: it builds the
// real command and verifies the embedded index.html is present verbatim in
// the compiled binary's data — the React bundle rides inside the executable,
// not next to it.
func TestBinaryEmbedsWebUI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build in -short mode")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not in PATH: %v", err)
	}

	binary := filepath.Join(t.TempDir(), "tunnel-embed-smoke")
	build := exec.Command(goBin, "build", "-o", binary, ".")
	build.Dir, err = os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building cmd/tunnel to verify the embedded web UI: %v\n%s", err, out)
	}

	bin, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("reading built binary: %v", err)
	}

	index, err := fs.ReadFile(embeddedfs.MustGetFS(), "index.html")
	if err != nil {
		t.Fatalf("reading embedded index.html: %v", err)
	}
	if !bytes.Contains(bin, index) {
		t.Fatal("compiled binary does not contain the embedded index.html — the web UI is not embedded")
	}

	if embeddedfs.HasRealFrontend() {
		t.Log("binary embeds the real React build")
	} else {
		t.Log("binary embeds the placeholder page (run make build-frontend to embed the real UI)")
	}
}
