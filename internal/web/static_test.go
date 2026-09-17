package web

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	embeddedfs "github.com/jedarden/tunnel/internal/web/embed"
)

// assetRefPattern mirrors the Vite output shape: hashed <script src> and
// <link href> references under /assets/.
var assetRefPattern = regexp.MustCompile(`(?:src|href)="/assets/([^"]+)"`)

// newFrontendApp builds a Fiber app with the embedded frontend mounted the
// same way cmd/tunnel does. The test is skipped when no frontend is embedded.
func newFrontendApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	if err := MountFrontend(app); err != nil {
		t.Skipf("web UI not embedded: %v", err)
	}
	return app
}

func embeddedIndex(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(embeddedfs.MustGetFS(), "index.html")
	if err != nil {
		t.Fatalf("reading embedded index.html: %v", err)
	}
	return string(data)
}

func getBody(t *testing.T, app *fiber.App, path string) (int, string, http.Header) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("app.Test(%q) error = %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body for %q: %v", path, err)
	}
	return resp.StatusCode, string(body), resp.Header
}

// TestIndexServedAtRoot verifies the documented entrypoint: GET / answers
// 200 with the embedded index.html. This holds for the real React build and
// for the build-dev placeholder alike.
func TestIndexServedAtRoot(t *testing.T) {
	app := newFrontendApp(t)
	want := embeddedIndex(t)

	status, body, header := getBody(t, app, "/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", status, http.StatusOK)
	}
	if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET / Content-Type = %q, want text/html", ct)
	}
	if body != want {
		t.Fatalf("GET / body does not match the embedded index.html (got %d bytes, want %d bytes)",
			len(body), len(want))
	}

	if embeddedfs.HasRealFrontend() {
		if !strings.Contains(body, `id="root"`) {
			t.Fatal("real frontend is embedded but GET / body lacks the React root mount point")
		}
		t.Log("serving real React build from /")
	} else {
		t.Log("serving placeholder page from / (dev mode; run make build-frontend for the real UI)")
	}
}

// TestSPAFallbackServesIndexForDeepRoutes verifies unknown client-side routes
// fall back to index.html so the React router can pick them up — required for
// the UI to stay functional on refresh of a deep link.
func TestSPAFallbackServesIndexForDeepRoutes(t *testing.T) {
	app := newFrontendApp(t)
	want := embeddedIndex(t)

	for _, path := range []string{"/providers", "/settings/credentials", "/logs/tailscale/entries"} {
		status, body, _ := getBody(t, app, path)
		if status != http.StatusOK {
			t.Errorf("GET %q (SPA fallback) status = %d, want %d", path, status, http.StatusOK)
			continue
		}
		if body != want {
			t.Errorf("GET %q did not return index.html as SPA fallback", path)
		}
	}
}

// TestEmbeddedAssetsServed verifies each asset referenced by index.html is
// served with a 200, the exact embedded bytes and a usable MIME type.
func TestEmbeddedAssetsServed(t *testing.T) {
	app := newFrontendApp(t)
	index := embeddedIndex(t)
	refs := assetRefPattern.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Log("no /assets references (placeholder build); nothing to check")
		return
	}

	for _, ref := range refs {
		name := ref[1]
		wantBytes, err := fs.ReadFile(embeddedfs.MustGetFS(), "assets/"+name)
		if err != nil {
			t.Errorf("referenced asset %s not embedded: %v", name, err)
			continue
		}

		path := "/assets/" + name
		status, body, header := getBody(t, app, path)
		if status != http.StatusOK {
			t.Errorf("GET %q status = %d, want %d", path, status, http.StatusOK)
			continue
		}
		if body != string(wantBytes) {
			t.Errorf("GET %q returned %d bytes, embedded file is %d bytes", path, len(body), len(wantBytes))
		}

		ct := header.Get("Content-Type")
		switch {
		case strings.HasSuffix(name, ".js") && !strings.Contains(ct, "javascript"):
			t.Errorf("GET %q Content-Type = %q, want a javascript type", path, ct)
		case strings.HasSuffix(name, ".css") && !strings.HasPrefix(ct, "text/css"):
			t.Errorf("GET %q Content-Type = %q, want text/css", path, ct)
		}
	}
}

// TestServingOverLoopbackTCP serves the frontend through a real TCP listener
// on loopback — the transport the README documents ("served on
// localhost:8080") — rather than only through Fiber's in-memory test adapter.
// An ephemeral port is used so the test cannot collide with a running
// instance; the fixed default port is pinned separately in cmd/tunnel tests.
func TestServingOverLoopbackTCP(t *testing.T) {
	app := newFrontendApp(t)
	want := embeddedIndex(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening on loopback: %v", err)
	}
	go func() { _ = app.Listener(listener) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	base := fmt.Sprintf("http://%s", listener.Addr().String())
	for _, path := range []string{"/", "/providers"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s%s over TCP: %v", base, path, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("reading TCP response for %q: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %q over TCP status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
		if string(body) != want {
			t.Fatalf("GET %q over TCP did not return the embedded index.html", path)
		}
	}
}
