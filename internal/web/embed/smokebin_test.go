package embed

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestSmokeBinaryEmbedsAndServes is the end-to-end variant of the embedding
// check: it builds cmd/embedsmoke — a minimal Go binary whose only job is to
// carry the embedded web UI — verifies the compiled executable physically
// contains index.html, then runs it and fetches the frontend over loopback
// HTTP from the running process.
//
// cmd/embedsmoke has no dependencies beyond the standard library and the
// embed package, so this test exercises embedding and serving even when
// unrelated application packages (auth, providers) do not currently build.
func TestSmokeBinaryEmbedsAndServes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build/run in -short mode")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not in PATH: %v", err)
	}

	root := repoRoot(t)
	binary := filepath.Join(t.TempDir(), "tunnel-embedsmoke")
	build := exec.Command(goBin, "build", "-o", binary, "./cmd/embedsmoke")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building cmd/embedsmoke: %v\n%s", err, out)
	}

	// The embedded index.html must be present verbatim inside the binary.
	bin, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("reading built binary: %v", err)
	}
	index, err := fs.ReadFile(MustGetFS(), "index.html")
	if err != nil {
		t.Fatalf("reading embedded index.html: %v", err)
	}
	if len(index) == 0 || !bytes.Contains(bin, index) {
		t.Fatal("compiled binary does not contain the embedded index.html — the web UI is not embedded")
	}

	// Run it and fetch the frontend over loopback HTTP.
	smoke := exec.Command(binary, "-addr", "127.0.0.1:0")
	stdout, err := smoke.StdoutPipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	if err := smoke.Start(); err != nil {
		t.Fatalf("starting embedsmoke: %v", err)
	}
	t.Cleanup(func() {
		_ = smoke.Process.Kill()
		_ = smoke.Wait()
	})

	baseURL := readServingURL(t, stdout)
	if !strings.HasPrefix(baseURL, "http://127.0.0.1:") {
		t.Fatalf("embedsmoke reported serving URL %q, want a loopback http URL", baseURL)
	}

	for _, path := range []string{"/", "/providers"} {
		resp, err := http.Get(baseURL + strings.TrimPrefix(path, "/"))
		if err != nil {
			t.Fatalf("GET %s from running embedsmoke: %v", path, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("reading response for %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
		if !bytes.Equal(body, index) {
			t.Fatalf("GET %s from running embedsmoke does not match the embedded index.html", path)
		}
	}

	if HasRealFrontend() {
		for _, ref := range assetRefPattern.FindAllStringSubmatch(string(index), -1) {
			path := "/assets/" + ref[1]
			want, err := fs.ReadFile(MustGetFS(), strings.TrimPrefix(path, "/"))
			if err != nil {
				t.Fatalf("reading embedded asset %s: %v", path, err)
			}
			resp, err := http.Get(baseURL + strings.TrimPrefix(path, "/"))
			if err != nil {
				t.Fatalf("GET %s from running embedsmoke: %v", path, err)
			}
			got, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				t.Fatalf("reading response for %s: %v", path, err)
			}
			if resp.StatusCode != http.StatusOK || !bytes.Equal(got, want) {
				t.Fatalf("GET %s returned status %d and %d bytes, want 200 and %d bytes", path, resp.StatusCode, len(got), len(want))
			}
		}
		t.Log("built binary embeds and serves the real React build")
	} else {
		t.Log("built binary embeds and serves the placeholder page (run make build-frontend for the real UI)")
	}
}

// addressPattern matches the single "http://host:port/" line embedsmoke
// prints once it is listening.
var addressPattern = regexp.MustCompile(`^http://[0-9.]+:[0-9]+/$`)

func readServingURL(t *testing.T, r io.Reader) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		reader := bufio.NewReader(r)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			if addressPattern.MatchString(strings.TrimSpace(line)) {
				ch <- result{line: strings.TrimSpace(line)}
				return
			}
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("embedsmoke exited before reporting an address: %v", res.err)
		}
		return res.line
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for embedsmoke to report its serving address")
	}
	return ""
}
