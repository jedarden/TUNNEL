package embed

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// assetRefPattern matches the hashed /assets/... references a Vite build
// leaves in index.html (module scripts and stylesheets).
var assetRefPattern = regexp.MustCompile(`(?:src|href)="/assets/([^"]+)"`)

func readEmbeddedIndex(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(MustGetFS(), "index.html")
	if err != nil {
		t.Fatalf("index.html missing from embedded filesystem: %v", err)
	}
	return string(data)
}

// TestGetFSExposesDistRoot verifies the embedded filesystem is re-rooted at
// dist/ so the web server can serve index.html and assets/ directly.
func TestGetFSExposesDistRoot(t *testing.T) {
	fsys, err := GetFS()
	if err != nil {
		t.Fatalf("GetFS() error = %v", err)
	}

	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		t.Fatalf("index.html not readable at FS root (dist prefix not stripped?): %v", err)
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("embedded dist is empty; a fresh clone must at least carry the build-dev placeholder")
	}
}

// TestEmbeddedIndexHTMLIsPage verifies the served entrypoint exists, is
// non-empty and is an HTML document — for both real builds and placeholders.
func TestEmbeddedIndexHTMLIsPage(t *testing.T) {
	index := readEmbeddedIndex(t)
	if strings.TrimSpace(index) == "" {
		t.Fatal("embedded index.html is empty")
	}
	if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(index)), "<!doctype html") {
		t.Fatalf("embedded index.html does not look like an HTML document: %q", truncate(index, 80))
	}
}

// TestEmbeddedAssetsSelfConsistent verifies every /assets/... file that
// index.html references is actually embedded and non-empty. A placeholder
// page references no assets, so the test is vacuous there by construction.
func TestEmbeddedAssetsSelfConsistent(t *testing.T) {
	index := readEmbeddedIndex(t)
	refs := assetRefPattern.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Log("index.html references no /assets files (placeholder build); nothing to check")
		return
	}

	fsys := MustGetFS()
	checked := 0
	for _, ref := range refs {
		name := "assets/" + ref[1]
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Errorf("index.html references %s but it is not embedded: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("embedded asset %s is empty", name)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no referenced assets could be checked")
	}
	t.Logf("verified %d referenced asset(s) are embedded", checked)
}

// TestHasRealFrontendMatchesIndex verifies the real-vs-placeholder detection
// agrees with the actual embedded index.html content.
func TestHasRealFrontendMatchesIndex(t *testing.T) {
	index := readEmbeddedIndex(t)
	want := strings.Contains(index, `id="root"`) && strings.Contains(index, "/assets/")
	if want {
		assets, err := fs.ReadDir(MustGetFS(), "assets")
		if err != nil {
			want = false
		} else {
			want = false
			for _, asset := range assets {
				if !asset.IsDir() {
					want = true
					break
				}
			}
		}
	}
	if got := HasRealFrontend(); got != want {
		t.Fatalf("HasRealFrontend() = %v, want %v (index contains root marker: %v, asset refs: %v)",
			got, want, strings.Contains(index, `id="root"`), strings.Contains(index, "/assets/"))
	}
}

// TestEmbeddedFrontendMatchesWebDist verifies the copy step of
// `make build-frontend` (web/dist -> internal/web/embed/dist): once a real
// frontend is embedded, it must be byte-identical to the web/dist it was
// copied from, with no extra stale files on either side.
//
// It is skipped unless both sides hold real builds: web/dist is absent in a
// fresh clone, and a placeholder embed dir is the normal build-dev state.
func TestEmbeddedFrontendMatchesWebDist(t *testing.T) {
	webDist := filepath.Join(repoRoot(t), "web", "dist")
	webIndex, err := os.ReadFile(filepath.Join(webDist, "index.html"))
	if err != nil {
		t.Skipf("web/dist not built (%v); run make build-frontend", err)
	}
	if !strings.Contains(string(webIndex), `id="root"`) {
		t.Skip("web/dist exists but is not a real Vite build; nothing to compare")
	}
	if !HasRealFrontend() {
		t.Skip("embedded dist is a placeholder; frontend not embedded yet (run make build-frontend)")
	}

	// Both sides are real builds: every file in web/dist must be embedded
	// byte-identically, and the embed dir must not carry stale extra files.
	embedFS := MustGetFS()
	embedFiles := map[string]bool{}
	err = fs.WalkDir(embedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			embedFiles[path] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded fs: %v", err)
	}

	err = filepath.WalkDir(webDist, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(webDist, path)
		if err != nil {
			return err
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := fs.ReadFile(embedFS, rel)
		if err != nil {
			t.Errorf("web/dist file %s is missing from the embedded dist: %v", rel, err)
			return nil
		}
		if string(got) != string(want) {
			t.Errorf("embedded %s differs from web/dist (stale embed dir? rerun make build-frontend)", rel)
		}
		delete(embedFiles, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking web/dist: %v", err)
	}

	for extra := range embedFiles {
		t.Errorf("embedded dist contains %s which web/dist does not (stale artifact; rerun make build-frontend)", extra)
	}
}

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

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// TestBuildFrontendCopiesOutputAndPreservesTheLastBundle exercises the
// documented production build recipe without downloading npm packages. The
// fake npm command emits a small Vite-shaped bundle on success and fails on
// request, so the test verifies both the copy and failure paths.
func TestBuildFrontendCopiesOutputAndPreservesTheLastBundle(t *testing.T) {
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is required to exercise the documented frontend workflow: %v", err)
	}

	root := t.TempDir()
	copyMakefile(t, root)
	webDir := filepath.Join(root, "web")
	embedDir := filepath.Join(root, "internal", "web", "embed", "dist")
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(embedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeNPM(t, fakeBin)
	logPath := filepath.Join(root, "npm-args")
	if err := os.WriteFile(filepath.Join(embedDir, "index.html"), []byte("last-good-bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(embedDir, "stale.js"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(failBuild bool) error {
		t.Helper()
		cmd := exec.Command(makeBin, "-C", root, "build-frontend")
		cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "FAKE_NPM_LOG="+logPath)
		if failBuild {
			cmd.Env = append(cmd.Env, "FAKE_NPM_FAIL_BUILD=1")
		} else {
			cmd.Env = append(cmd.Env, "FAKE_NPM_FAIL_BUILD=0")
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("make output: %s", out)
		}
		return err
	}

	if err := run(false); err != nil {
		t.Fatalf("make build-frontend failed: %v", err)
	}
	gotIndex, err := os.ReadFile(filepath.Join(embedDir, "index.html"))
	if err != nil {
		t.Fatalf("reading copied index.html: %v", err)
	}
	if !strings.Contains(string(gotIndex), `id="root"`) {
		t.Fatalf("build-frontend did not copy the Vite entrypoint: %q", gotIndex)
	}
	if _, err := os.Stat(filepath.Join(embedDir, "assets", "app.js")); err != nil {
		t.Fatalf("build-frontend did not copy the referenced asset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(embedDir, "stale.js")); !os.IsNotExist(err) {
		t.Fatalf("stale embedded asset remains after a successful build: stat error = %v", err)
	}

	const lastGood = "keep-this-bundle-on-failure"
	if err := os.WriteFile(filepath.Join(embedDir, "index.html"), []byte(lastGood), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(true); err == nil {
		t.Fatal("make build-frontend succeeded even though npm run build failed")
	}
	gotIndex, err = os.ReadFile(filepath.Join(embedDir, "index.html"))
	if err != nil {
		t.Fatalf("failed frontend build removed the previous embedded index.html: %v", err)
	}
	if string(gotIndex) != lastGood {
		t.Fatalf("failed frontend build replaced the previous index.html: got %q, want %q", gotIndex, lastGood)
	}
}

// TestBuildDevWritesTheMakefilePlaceholder checks the build-dev recipe in an
// isolated directory. Its fake Go command avoids compiling the full app while
// still verifying the exact page written into the embed directory.
func TestBuildDevWritesTheMakefilePlaceholder(t *testing.T) {
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is required to exercise the documented dev build: %v", err)
	}

	root := t.TempDir()
	copyMakefile(t, root)
	cmd := exec.Command(makeBin, "-o", "deps", "-C", root,
		"GOBUILD=:", "LDFLAGS=", "BIN_DIR=./bin", "EMBED_DIR=./internal/web/embed/dist", "build-dev")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make build-dev failed: %v\n%s", err, out)
	}

	want, err := makefilePlaceholder(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "internal", "web", "embed", "dist", "index.html"))
	if err != nil {
		t.Fatalf("build-dev did not write the embed placeholder: %v", err)
	}
	if string(got) != want+"\n" {
		t.Fatalf("build-dev placeholder differs from PLACEHOLDER_INDEX:\n got: %q\nwant: %q", got, want+"\n")
	}
}

// TestFrontendDevInvokesTheDocumentedNPMCommand pins the make frontend-dev
// integration while keeping this test independent of a running Vite server.
func TestFrontendDevInvokesTheDocumentedNPMCommand(t *testing.T) {
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is required to exercise the documented frontend workflow: %v", err)
	}

	root := t.TempDir()
	copyMakefile(t, root)
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "npm-args")
	writeFakeNPM(t, fakeBin)
	cmd := exec.Command(makeBin, "-C", root, "frontend-dev")
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "FAKE_NPM_LOG="+logPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make frontend-dev failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("fake npm did not record its invocation: %v", err)
	}
	if strings.TrimSpace(string(got)) != "run dev" {
		t.Fatalf("make frontend-dev invoked npm with %q, want %q", strings.TrimSpace(string(got)), "run dev")
	}
}

func copyMakefile(t *testing.T, dstRoot string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("reading repository Makefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstRoot, "Makefile"), data, 0o644); err != nil {
		t.Fatalf("copying Makefile: %v", err)
	}
}

func writeFakeNPM(t *testing.T, binDir string) {
	t.Helper()
	script := `#!/bin/sh
if [ -n "${FAKE_NPM_LOG:-}" ]; then
	printf '%s\n' "$*" >> "$FAKE_NPM_LOG"
fi
if [ "$1" = ci ]; then
	 exit 0
fi
if [ "$1" = run ] && [ "$2" = build ]; then
	if [ "${FAKE_NPM_FAIL_BUILD:-0}" = 1 ]; then
		exit 23
	fi
	mkdir -p dist/assets || exit
	printf '%s' '<!doctype html><div id="root"></div><script type="module" src="/assets/app.js"></script>' > dist/index.html
	printf '%s' 'console.log("smoke")' > dist/assets/app.js
	exit 0
fi
if [ "$1" = run ] && [ "$2" = dev ]; then
	exit 0
fi
exit 2
`
	if err := os.WriteFile(filepath.Join(binDir, "npm"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake npm: %v", err)
	}
}

func makefilePlaceholder(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "PLACEHOLDER_INDEX='") || !strings.HasSuffix(line, "'") {
			continue
		}
		return strings.TrimSuffix(strings.TrimPrefix(line, "PLACEHOLDER_INDEX='"), "'"), nil
	}
	return "", fmt.Errorf("PLACEHOLDER_INDEX not found in %s", path)
}
