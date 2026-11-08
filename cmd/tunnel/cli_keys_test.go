package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/tunnel/internal/core"
)

// Test-only key material; the private halves were discarded after the public
// lines below were generated.
const (
	testKeyAliceLaptop = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH7pY/80ztNJLFwn4TXihfDmA7mscj6CI7V9bhOM61ZH alice@laptop"
	testKeyAlicePhone  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILj2zTUAU6caekaK9Fiprdm36J+MmurPCSEndB1R6/tr alice@phone"
	testKeyBobDesktop  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN178iq6/pSdsI1NYgxqtfzUTmfLr287yT8nWL9bfttx bob@desktop"
)

// installTestKeyManager swaps the package-global key manager for one backed
// by a fresh authorized_keys file in a per-test directory, restoring the
// previous manager at cleanup. HOME is pointed at the same directory so the
// cobra initialization hook (which rebuilds the manager from config on every
// command execution) lands on the identical file instead of the real
// ~/.ssh/authorized_keys. A nil client keeps the manager's default HTTP
// client, which remote-import tests never exercise directly.
func installTestKeyManager(t *testing.T, client *http.Client) (*core.FileKeyManager, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".ssh", "authorized_keys")
	keyMgr, err := core.NewFileKeyManagerWithHTTPClient(path, nil, client)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}

	previous := keyManager
	keyManager = keyMgr
	t.Cleanup(func() { keyManager = previous })

	return keyMgr, path
}

// pipeStdin feeds input to the stdin reader used by the interactive key
// prompts, restoring the original stdin at cleanup.
func pipeStdin(t *testing.T, input string) {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatalf("create stdin file: %v", err)
	}
	if _, err := io.WriteString(file, input); err != nil {
		t.Fatalf("write stdin file: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("rewind stdin file: %v", err)
	}

	original := os.Stdin
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = original
		_ = file.Close()
	})
}

// captureStdout runs fn while collecting everything it writes to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer
	collected := make(chan string, 1)
	go func() {
		var buf strings.Builder
		_, _ = io.Copy(&buf, reader)
		collected <- buf.String()
	}()

	fn()

	os.Stdout = original
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	return <-collected
}

func withJSONOutput(t *testing.T, enabled bool) {
	t.Helper()

	original := jsonOutput
	jsonOutput = enabled
	t.Cleanup(func() { jsonOutput = original })
}

// remoteKeysServer serves body for every request and returns an HTTP client
// whose transport rewrites all requests to the test server, so the manager's
// hardcoded github.com / gitlab.com import URLs resolve locally instead of
// touching the network. requests, when non-nil, counts served requests.
func remoteKeysServer(t *testing.T, body string, requests *int) *http.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests != nil {
			*requests++
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: rewriteTransport{target: target},
	}
}

type rewriteTransport struct {
	target *url.URL
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rewritten := req.Clone(req.Context())
	rewritten.URL.Scheme = rt.target.Scheme
	rewritten.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(rewritten)
}

// keyMaterial returns the type and base64 blob of a public key line, i.e.
// the part that survives an import rewriting the comment with the source.
func keyMaterial(line string) string {
	fields := strings.Fields(line)
	return strings.Join(fields[:2], " ")
}

func authorizedKeysLines(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestKeysManualAddUpdatesAuthorizedKeys(t *testing.T) {
	_, path := installTestKeyManager(t, nil)
	pipeStdin(t, testKeyAliceLaptop+"\n")

	if err := addKey("alice"); err != nil {
		t.Fatalf("addKey: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat authorized_keys: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("authorized_keys mode = %v, want 0600", got)
	}

	lines := authorizedKeysLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("authorized_keys holds %d key lines, want 1: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], testKeyAliceLaptop) {
		t.Errorf("authorized_keys line %q does not contain the added key", lines[0])
	}

	fingerprint, err := keyManager.GetFingerprint(testKeyAliceLaptop)
	if err != nil {
		t.Fatalf("GetFingerprint: %v", err)
	}
	keys, err := keyManager.ListKeys("")
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].Fingerprint != fingerprint {
		t.Errorf("ListKeys = %+v, want the added key with fingerprint %s", keys, fingerprint)
	}
}

func TestKeysManualAddRejectsInvalidInput(t *testing.T) {
	_, path := installTestKeyManager(t, nil)

	// addKey reads exactly one line, so a multi-key paste past that line
	// never reaches the validator here; the parser-level rejection of
	// multi-key input is covered by core's TestValidateKeyRejectsMultipleKeys.
	cases := []struct {
		name  string
		input string
	}{
		{"empty", "\n"},
		{"not a key", "definitely not an OpenSSH public key\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pipeStdin(t, tc.input)
			if err := addKey("alice"); err == nil {
				t.Errorf("addKey accepted invalid input %q", tc.name)
			}
		})
	}

	if lines := authorizedKeysLines(t, path); len(lines) != 0 {
		t.Errorf("rejected input was written to authorized_keys: %q", lines)
	}
}

func TestKeysRevokeUpdatesAuthorizedKeys(t *testing.T) {
	keyMgr, path := installTestKeyManager(t, nil)

	laptop, err := keyMgr.ValidateKey(testKeyAliceLaptop)
	if err != nil {
		t.Fatalf("validate laptop key: %v", err)
	}
	phone, err := keyMgr.ValidateKey(testKeyAlicePhone)
	if err != nil {
		t.Fatalf("validate phone key: %v", err)
	}
	if err := keyMgr.AddKey("alice", *laptop); err != nil {
		t.Fatalf("seed laptop key: %v", err)
	}
	if err := keyMgr.AddKey("alice", *phone); err != nil {
		t.Fatalf("seed phone key: %v", err)
	}

	// Revoke by fingerprint.
	if err := revokeKey("alice", laptop.Fingerprint); err != nil {
		t.Fatalf("revokeKey by fingerprint: %v", err)
	}
	lines := authorizedKeysLines(t, path)
	if len(lines) != 1 || !strings.Contains(lines[0], testKeyAlicePhone) {
		t.Fatalf("after fingerprint revoke authorized_keys = %q, want only the phone key", lines)
	}

	// Revoke the remaining key by its one-based list index.
	if err := revokeKey("alice", "1"); err != nil {
		t.Fatalf("revokeKey by index: %v", err)
	}
	if lines := authorizedKeysLines(t, path); len(lines) != 0 {
		t.Errorf("after index revoke authorized_keys = %q, want empty", lines)
	}

	// Unknown key IDs are reported, not silently ignored.
	if err := revokeKey("alice", "SHA256:does-not-exist"); err == nil {
		t.Error("revokeKey accepted an unknown key ID")
	}
}

func TestKeysImportGitHubUpdatesAuthorizedKeysIdempotently(t *testing.T) {
	var requests int
	client := remoteKeysServer(t,
		testKeyAliceLaptop+"\n\ngarbage line\n"+testKeyAlicePhone+"\n",
		&requests,
	)
	_, path := installTestKeyManager(t, client)

	if err := importGitHubKeys("octocat"); err != nil {
		t.Fatalf("importGitHubKeys: %v", err)
	}

	lines := authorizedKeysLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("authorized_keys holds %d keys after import, want 2: %q", len(lines), lines)
	}
	for _, line := range lines {
		if !strings.Contains(line, "github.com/octocat") {
			t.Errorf("imported key %q has no github.com/octocat source comment", line)
		}
	}

	// A second import fetches the same profile again but adds nothing.
	before := strings.Join(lines, "\n")
	if err := importGitHubKeys("octocat"); err != nil {
		t.Fatalf("second importGitHubKeys: %v", err)
	}
	if requests != 2 {
		t.Errorf("remote profile fetched %d times, want 2", requests)
	}
	if after := strings.Join(authorizedKeysLines(t, path), "\n"); after != before {
		t.Errorf("re-import changed authorized_keys:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestKeysImportGitLabUpdatesAuthorizedKeysIdempotently(t *testing.T) {
	var requests int
	client := remoteKeysServer(t, testKeyBobDesktop+"\n", &requests)
	_, path := installTestKeyManager(t, client)

	if err := importGitLabKeys("bob"); err != nil {
		t.Fatalf("importGitLabKeys: %v", err)
	}

	lines := authorizedKeysLines(t, path)
	if len(lines) != 1 || !strings.Contains(lines[0], keyMaterial(testKeyBobDesktop)) {
		t.Fatalf("authorized_keys after GitLab import = %q, want the imported key", lines)
	}
	if !strings.Contains(lines[0], "gitlab.com/bob") {
		t.Errorf("imported key %q has no gitlab.com/bob source comment", lines[0])
	}

	// Re-importing the same profile must succeed without duplicating keys;
	// the import previously failed with "key already exists".
	if err := importGitLabKeys("bob"); err != nil {
		t.Fatalf("second importGitLabKeys: %v", err)
	}
	if requests != 2 {
		t.Errorf("remote profile fetched %d times, want 2", requests)
	}
	if lines := authorizedKeysLines(t, path); len(lines) != 1 {
		t.Errorf("re-import duplicated keys: %q", lines)
	}
}

func TestKeysImportRemoteFailures(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(notFound.Close)

	target, err := url.Parse(notFound.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: rewriteTransport{target: target}}

	installTestKeyManager(t, client)
	withJSONOutput(t, true)

	for _, tc := range []struct {
		name string
		run  func(string) error
	}{
		{"github", importGitHubKeys},
		{"gitlab", importGitLabKeys},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				if err := tc.run("ghost-user"); err != nil {
					t.Errorf("%s import returned error: %v", tc.name, err)
				}
			})
			if !strings.Contains(output, `"status": "error"`) {
				t.Errorf("JSON output %q does not report the error status", output)
			}
		})
	}
}

func TestKeysRevokeCommandAcceptsDocumentedAndLegacyArgOrder(t *testing.T) {
	keyMgr, path := installTestKeyManager(t, nil)

	key, err := keyMgr.ValidateKey(testKeyAliceLaptop)
	if err != nil {
		t.Fatalf("validate key: %v", err)
	}
	if err := keyMgr.AddKey("alice", *key); err != nil {
		t.Fatalf("seed key: %v", err)
	}

	withJSONOutput(t, true)
	// Cobra routes ExecuteC on a subcommand through the root, so args are
	// always set on rootCmd: executing a child directly would launch the
	// root command's default TUI run.
	runRevoke := func(args ...string) error {
		rootCmd.SetArgs(args)
		_, err := rootCmd.ExecuteC()
		return err
	}

	// Documented form: revoke <key-id> [user].
	if err := runRevoke("keys", "revoke", key.Fingerprint, "--user", "alice"); err != nil {
		t.Fatalf("documented revoke form: %v", err)
	}
	if lines := authorizedKeysLines(t, path); len(lines) != 0 {
		t.Fatalf("documented revoke left keys behind: %q", lines)
	}

	// Legacy form: revoke <user> <key-id>.
	if err := keyMgr.AddKey("alice", *key); err != nil {
		t.Fatalf("reseed key: %v", err)
	}
	if err := runRevoke("keys", "revoke", "alice", key.Fingerprint); err != nil {
		t.Fatalf("legacy revoke form: %v", err)
	}
	if lines := authorizedKeysLines(t, path); len(lines) != 0 {
		t.Errorf("legacy revoke left keys behind: %q", lines)
	}
}
