package api

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/pkg/tunnel"
)

// The documented development workflow ("make frontend-dev" plus "make dev")
// relies on the Vite dev server proxying browser requests to the Go backend
// started by the binary. These tests pin that contract: the proxy target in
// web/vite.config.ts must keep pointing at the address the server binds to by
// default, the frontend's WebSocket endpoint must be a route the backend
// actually registers, and the docs must keep describing the real mapping.

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

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, parts...)...))
	if err != nil {
		t.Fatalf("reading %s: %v", filepath.Join(parts...), err)
	}
	return string(data)
}

// tsObjectBlock returns the contents of the brace-delimited object literal
// that follows the given key (e.g. "proxy:" or "'/api':") in a TypeScript
// config file. vite.config.ts contains no braces inside string literals, so
// plain depth counting is sufficient.
func tsObjectBlock(t *testing.T, src, key string) string {
	t.Helper()
	idx := strings.Index(src, key)
	if idx < 0 {
		t.Fatalf("%q not found in vite.config.ts", key)
	}
	rest := src[idx+len(key):]
	open := strings.Index(rest, "{")
	if open < 0 {
		t.Fatalf("no object literal after %q in vite.config.ts", key)
	}
	depth := 0
	for i := open; i < len(rest); i++ {
		switch rest[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return rest[open+1 : i]
			}
		}
	}
	t.Fatalf("unbalanced braces after %q in vite.config.ts", key)
	return ""
}

// defaultWebPort extracts the documented web port from the shipped default
// configuration, so the proxy test stays aligned with configs/default.yaml
// rather than hard-coding a second copy of the number.
func defaultWebPort(t *testing.T) string {
	t.Helper()
	cfg := readRepoFile(t, "configs", "default.yaml")
	m := regexp.MustCompile(`(?m)^\s*port:\s*(\d+)\s*$`).FindStringSubmatch(cfg)
	if m == nil {
		t.Fatal("configs/default.yaml no longer sets web.port; update it together with the vite proxy target")
	}
	return m[1]
}

// TestViteDevProxyTargetsBackendDefaultPort verifies the /api proxy entry in
// web/vite.config.ts points at the port the Go server listens on by default
// (configs/default.yaml) on localhost, with WebSocket upgrading enabled so
// the realtime endpoint works in dev mode.
func TestViteDevProxyTargetsBackendDefaultPort(t *testing.T) {
	vite := readRepoFile(t, "web", "vite.config.ts")

	entry := tsObjectBlock(t, tsObjectBlock(t, vite, "proxy:"), `'/api':`)

	wantTarget := "http://localhost:" + defaultWebPort(t)
	target := regexp.MustCompile(`target:\s*'([^']+)'`).FindStringSubmatch(entry)
	if target == nil {
		t.Fatalf("vite /api proxy entry has no target: %q", entry)
	}
	if target[1] != wantTarget {
		t.Errorf("vite /api proxy target = %q, want %q (configs/default.yaml web.port)", target[1], wantTarget)
	}
	if !regexp.MustCompile(`ws:\s*true`).MatchString(entry) {
		t.Error("vite /api proxy entry no longer sets ws: true; the /api/ws WebSocket would not be proxied in dev mode")
	}
}

// TestFrontendWebSocketDefaultMatchesBackendRoute verifies the WebSocket URL
// the frontend hook connects to by default is a route the API actually
// registers — reached through the /api proxy entry in dev mode and directly
// in production.
func TestFrontendWebSocketDefaultMatchesBackendRoute(t *testing.T) {
	hook := readRepoFile(t, "web", "src", "hooks", "useWebSocket.ts")
	m := regexp.MustCompile(`url\s*=\s*'([^']+)'`).FindStringSubmatch(hook)
	if m == nil {
		t.Fatal("useWebSocket.ts no longer has a default url; the dev proxy contract depends on it")
	}
	frontendPath := m[1]

	const token = "test-control-token"
	server := NewServer(&ServerConfig{
		Manager:   tunnel.NewManager(nil),
		Registry:  tunnel.NewRegistry(),
		AuthToken: token,
	})
	defer server.Close()
	app := fiber.New()
	SetupRoutes(app, server)

	registered := false
	for _, routes := range app.Stack() {
		for _, route := range routes {
			if route.Method == fiber.MethodGet && route.Path == frontendPath {
				registered = true
			}
		}
	}
	if !registered {
		t.Errorf("frontend WebSocket default %q matches no GET route registered by the API; "+
			"the documented proxy mapping would 404", frontendPath)
	}
}

// TestWebDevDocsMatchViteProxy keeps web/README.md — the document that tells
// developers where the dev-server proxy points — in sync with the actual
// vite.config.ts, and rejects the previously documented /ws mapping that
// never existed on the backend.
func TestWebDevDocsMatchViteProxy(t *testing.T) {
	readme := readRepoFile(t, "web", "README.md")
	vite := readRepoFile(t, "web", "vite.config.ts")
	port := defaultWebPort(t)

	// The documented API mapping must still hold.
	if !strings.Contains(readme, "`/api/*` → `http://localhost:"+port+"/api/*`") {
		t.Errorf("web/README.md no longer documents /api/* → http://localhost:%s/api/*", port)
	}
	// The realtime endpoint is /api/ws (see TestFrontendWebSocketDefaultMatchesBackendRoute).
	if !strings.Contains(readme, "`/api/ws` → `ws://localhost:"+port+"/api/ws`") {
		t.Errorf("web/README.md no longer documents the /api/ws WebSocket mapping to ws://localhost:%s/api/ws", port)
	}
	// Stale, never-implemented mappings must not creep back.
	if strings.Contains(readme, "`/ws` →") {
		t.Error("web/README.md documents a bare /ws proxy mapping, but the backend registers no /ws route (it serves /api/ws)")
	}
	if strings.Contains(readme, "VITE_WS_URL") {
		t.Error("web/README.md documents VITE_WS_URL, but no frontend code reads that variable (it pins /api/ws directly)")
	}

	// The documented dev-server address must match the configured port.
	devPort := regexp.MustCompile(`port:\s*(\d+)`).FindStringSubmatch(tsObjectBlock(t, vite, "server:"))
	if devPort == nil {
		t.Fatal("vite.config.ts server block no longer sets a port")
	}
	if !strings.Contains(readme, "http://localhost:"+devPort[1]) {
		t.Errorf("web/README.md no longer documents the dev server at http://localhost:%s", devPort[1])
	}

	// The in-source monitoring doc repeats the WebSocket endpoint; keep it honest too.
	monitoring := readRepoFile(t, "web", "src", "pages", "MONITORING.md")
	if strings.Contains(monitoring, "localhost:PORT/ws") && !strings.Contains(monitoring, "localhost:PORT/api/ws") {
		t.Error("web/src/pages/MONITORING.md documents the WebSocket at /ws; the backend serves /api/ws")
	}
}
