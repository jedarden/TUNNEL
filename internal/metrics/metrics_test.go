package metrics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/tunnel/pkg/config"
)

// freePort reserves an unused loopback port for a test to bind. The
// reservation is released before the test binds it, so a small race with
// other processes exists; that is standard for port-allocating tests.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// canDial reports whether anything is listening on addr.
func canDial(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// waitForDialState polls until the address reaches the expected listen state
// or the deadline expires.
func waitForDialState(t *testing.T, addr string, wantListening bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if canDial(addr) == wantListening {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("addr %s never reached wantListening=%v", addr, wantListening)
}

// TestDisabledServerIsInert pins the disabled contract: no listener is ever
// opened and lifecycle calls are no-ops.
func TestDisabledServerIsInert(t *testing.T) {
	server, err := NewServer(Config{Port: freePort(t)})
	if err != nil {
		t.Fatalf("NewServer(disabled) returned error: %v", err)
	}

	if server.Enabled() {
		t.Error("server reports Enabled with Enabled:false config")
	}
	if got := server.Addr(); got != "" {
		t.Errorf("disabled server Addr() = %q, want empty", got)
	}

	// Start must not bind anything.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("disabled Start() returned error: %v", err)
	}
	if got := server.Addr(); got != "" {
		t.Errorf("disabled server Addr() after Start = %q, want empty", got)
	}

	// Stop on a never-started server is a no-op.
	if err := server.Stop(context.Background()); err != nil {
		t.Errorf("disabled Stop() returned error: %v", err)
	}

	// The handler still renders a baseline body if ever mounted elsewhere.
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("disabled server handler status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestNewServerValidation pins which configurations are rejected.
func TestNewServerValidation(t *testing.T) {
	export := func() map[string]interface{} { return nil }

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "enabled with sample source and port",
			cfg:     Config{Enabled: true, Port: 9090, Export: export},
			wantErr: false,
		},
		{
			name:    "enabled with ephemeral port",
			cfg:     Config{Enabled: true, Port: 0, Export: export},
			wantErr: false,
		},
		{
			name:    "enabled without sample source",
			cfg:     Config{Enabled: true, Port: 9090},
			wantErr: true,
		},
		{
			name:    "enabled with negative port",
			cfg:     Config{Enabled: true, Port: -1, Export: export},
			wantErr: true,
		},
		{
			name:    "enabled with port above range",
			cfg:     Config{Enabled: true, Port: 65536, Export: export},
			wantErr: true,
		},
		{
			name:    "disabled without sample source",
			cfg:     Config{Port: 9090},
			wantErr: false,
		},
		{
			name:    "disabled with invalid port is inert, not an error",
			cfg:     Config{Port: -1},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && server == nil {
				t.Fatal("NewServer returned nil server and nil error")
			}
		})
	}
}

// TestScrapeLifecycle is the end-to-end lifecycle: start, scrape, stop,
// restart, stop again — over a real listener.
func TestScrapeLifecycle(t *testing.T) {
	port := freePort(t)
	server, err := NewServer(Config{
		Enabled: true,
		Port:    port,
		Version: "test",
		Export: func() map[string]interface{} {
			return map[string]interface{}{
				"total_connections": 0,
				"connections":       []map[string]interface{}{},
			}
		},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got, want := server.Addr(), addr; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
	waitForDialState(t, addr, true)

	resp, err := http.Get("http://" + addr + MetricsPath)
	if err != nil {
		t.Fatalf("scrape after Start: %v", err)
	}
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("scrape status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != ContentType {
		t.Errorf("Content-Type = %q, want %q", got, ContentType)
	}
	if !strings.Contains(body, "tunnel_metrics_scrapes_total 1") {
		t.Errorf("first scrape body does not count itself:\n%s", body)
	}
	resp.Body.Close()

	// Start is idempotent while serving.
	if err := server.Start(ctx); err != nil {
		t.Errorf("second Start() while serving returned error: %v", err)
	}

	// Stop closes the listener and drains.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitForDialState(t, addr, false)

	// Stop is idempotent.
	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("second Stop() returned error: %v", err)
	}

	// Start after Stop rebinds.
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start after Stop: %v", err)
	}
	waitForDialState(t, addr, true)
	resp, err = http.Get("http://" + addr + MetricsPath)
	if err != nil {
		t.Fatalf("scrape after restart: %v", err)
	}
	body = readAll(t, resp)
	resp.Body.Close()
	// The scrape counter survives restarts within the process.
	if !strings.Contains(body, "tunnel_metrics_scrapes_total 2") {
		t.Errorf("scrape counter did not survive restart:\n%s", body)
	}

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("final Stop: %v", err)
	}
}

// TestContextCancellationStopsServer pins that cancelling the Start context
// shuts the endpoint down.
func TestContextCancellationStopsServer(t *testing.T) {
	port := freePort(t)
	server, err := NewServer(Config{
		Enabled: true,
		Port:    port,
		Export:  func() map[string]interface{} { return nil },
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	addr := server.Addr()
	waitForDialState(t, addr, true)

	cancel()
	waitForDialState(t, addr, false)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Stop after context cancellation returned error: %v", err)
	}
}

// TestHandlerRoutes pins the endpoint contract: GET (and HEAD) on /metrics
// succeeds, other methods are rejected with 405, other paths with 404.
func TestHandlerRoutes(t *testing.T) {
	server, err := NewServer(Config{
		Enabled: true,
		Port:    9090,
		Export:  func() map[string]interface{} { return nil },
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := server.Handler()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "GET metrics", method: http.MethodGet, path: MetricsPath, wantStatus: http.StatusOK},
		{name: "HEAD metrics", method: http.MethodHead, path: MetricsPath, wantStatus: http.StatusOK},
		{name: "POST metrics", method: http.MethodPost, path: MetricsPath, wantStatus: http.StatusMethodNotAllowed},
		{name: "DELETE metrics", method: http.MethodDelete, path: MetricsPath, wantStatus: http.StatusMethodNotAllowed},
		{name: "GET root", method: http.MethodGet, path: "/", wantStatus: http.StatusNotFound},
		{name: "GET other path", method: http.MethodGet, path: "/api/metrics", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s status = %d, want %d", tt.method, tt.path, rec.Code, tt.wantStatus)
			}
		})
	}
}

// TestRenderExactOutput pins the full exposition body for a known snapshot:
// family order, sample order, label order and value formatting are all part
// of the endpoint contract.
func TestRenderExactOutput(t *testing.T) {
	server, err := NewServer(Config{
		Enabled: true,
		Port:    9090,
		Version: "v1.2.3",
		Export: func() map[string]interface{} {
			return map[string]interface{}{
				"timestamp":         int64(1700000000),
				"total_connections": 2,
				"connections": []map[string]interface{}{
					{
						"id":             "cloudflare-100",
						"method":         "cloudflare",
						"state":          "Connected",
						"bytes_sent":     int64(1024),
						"bytes_received": int64(2048),
						"latency_ms":     int64(42),
						"uptime_seconds": float64(3600),
						"is_primary":     true,
						"priority":       100,
					},
					{
						"id":             "tailscale-200",
						"method":         "tailscale",
						"state":          "Disconnected",
						"bytes_sent":     int64(0),
						"bytes_received": int64(5),
						"latency_ms":     int64(0),
						"uptime_seconds": float64(0),
						"is_primary":     false,
						"priority":       40,
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetricsPath, nil))

	// The scrape counter is part of the body; render the expected body with
	// the same counter value the handler used.
	want := "# HELP tunnel_build_info Build information for this TUNNEL process.\n" +
		"# TYPE tunnel_build_info gauge\n" +
		"tunnel_build_info{version=\"v1.2.3\"} 1\n" +
		"# HELP tunnel_connections Registered connections by state.\n" +
		"# TYPE tunnel_connections gauge\n" +
		"tunnel_connections{state=\"connected\"} 1\n" +
		"tunnel_connections{state=\"disconnected\"} 1\n" +
		"# HELP tunnel_connection_bytes_sent_total Bytes sent per connection.\n" +
		"# TYPE tunnel_connection_bytes_sent_total counter\n" +
		"tunnel_connection_bytes_sent_total{connection=\"cloudflare-100\",method=\"cloudflare\",primary=\"true\"} 1024\n" +
		"tunnel_connection_bytes_sent_total{connection=\"tailscale-200\",method=\"tailscale\",primary=\"false\"} 0\n" +
		"# HELP tunnel_connection_bytes_received_total Bytes received per connection.\n" +
		"# TYPE tunnel_connection_bytes_received_total counter\n" +
		"tunnel_connection_bytes_received_total{connection=\"cloudflare-100\",method=\"cloudflare\",primary=\"true\"} 2048\n" +
		"tunnel_connection_bytes_received_total{connection=\"tailscale-200\",method=\"tailscale\",primary=\"false\"} 5\n" +
		"# HELP tunnel_connection_latency_milliseconds Average measured latency per connection.\n" +
		"# TYPE tunnel_connection_latency_milliseconds gauge\n" +
		"tunnel_connection_latency_milliseconds{connection=\"cloudflare-100\",method=\"cloudflare\",primary=\"true\"} 42\n" +
		"tunnel_connection_latency_milliseconds{connection=\"tailscale-200\",method=\"tailscale\",primary=\"false\"} 0\n" +
		"# HELP tunnel_connection_uptime_seconds Seconds since each connection started.\n" +
		"# TYPE tunnel_connection_uptime_seconds gauge\n" +
		"tunnel_connection_uptime_seconds{connection=\"cloudflare-100\",method=\"cloudflare\",primary=\"true\"} 3600\n" +
		"tunnel_connection_uptime_seconds{connection=\"tailscale-200\",method=\"tailscale\",primary=\"false\"} 0\n" +
		"# HELP tunnel_metrics_scrapes_total Scrapes served since process start.\n" +
		"# TYPE tunnel_metrics_scrapes_total counter\n" +
		"tunnel_metrics_scrapes_total 1\n"

	if got := rec.Body.String(); got != want {
		t.Errorf("rendered body mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestRenderEscapesLabelValues covers the exposition format's label-value
// escaping for backslash, double quote and newline in connection IDs.
func TestRenderEscapesLabelValues(t *testing.T) {
	server, err := NewServer(Config{
		Enabled: true,
		Port:    9090,
		Export: func() map[string]interface{} {
			return map[string]interface{}{
				"connections": []interface{}{
					map[string]interface{}{
						"id":         `weird"id\x`,
						"method":     "test",
						"state":      "Connected",
						"bytes_sent": 1,
					},
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	body := rec.Body.String()

	wantLine := `tunnel_connection_bytes_sent_total{connection="weird\"id\\x",method="test",primary="false"} 1`
	if !strings.Contains(body, wantLine) {
		t.Errorf("escaped label line missing\nwant: %s\ngot:\n%s", wantLine, body)
	}
}

// TestRenderEmptySnapshot pins the no-connection body: every family keeps
// its HELP/TYPE lines so the endpoint stays self-describing.
func TestRenderEmptySnapshot(t *testing.T) {
	body := render(snapshot{version: "", scrapes: 7})

	if strings.Contains(body, "tunnel_build_info") {
		t.Error("empty version must not emit tunnel_build_info")
	}
	for _, family := range []string{
		"tunnel_connections",
		"tunnel_connection_bytes_sent_total",
		"tunnel_connection_bytes_received_total",
		"tunnel_connection_latency_milliseconds",
		"tunnel_connection_uptime_seconds",
	} {
		if !strings.Contains(body, "# TYPE "+family+" ") {
			t.Errorf("empty snapshot is missing TYPE line for %s", family)
		}
	}
	if !strings.Contains(body, "tunnel_metrics_scrapes_total 7") {
		t.Errorf("empty snapshot missing scrape counter:\n%s", body)
	}
	// No per-connection samples may appear.
	if strings.Contains(body, "tunnel_connection_bytes_sent_total{") {
		t.Errorf("empty snapshot emitted connection samples:\n%s", body)
	}
}

// TestNilExportRendersBaseline covers the defensive path: a sample source
// returning nil (no collector) still produces a valid scrape.
func TestNilExportRendersBaseline(t *testing.T) {
	server, err := NewServer(Config{
		Enabled: true,
		Port:    9090,
		Export:  func() map[string]interface{} { return nil },
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetricsPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "tunnel_metrics_scrapes_total 1") {
		t.Errorf("baseline scrape missing counter:\n%s", rec.Body.String())
	}
}

// TestNewAppServerFromConfig pins the application-wiring table: nil config
// and disabled config yield an inert server, enabled config yields a bound
// one on the configured port with 0 meaning the documented default.
func TestNewAppServerFromConfig(t *testing.T) {
	export := func() map[string]interface{} { return nil }

	tests := []struct {
		name        string
		cfg         *config.Config
		export      SampleSource
		wantEnabled bool
		wantAddr    string
		wantErr     bool
	}{
		{
			name:        "nil config",
			cfg:         nil,
			export:      export,
			wantEnabled: false,
		},
		{
			name: "metrics disabled (documented default)",
			cfg: &config.Config{
				Monitoring: config.MonitoringConfig{MetricsEnabled: false, MetricsPort: 9090},
			},
			export:      export,
			wantEnabled: false,
		},
		{
			name: "enabled on configured port",
			cfg: &config.Config{
				Monitoring: config.MonitoringConfig{MetricsEnabled: true, MetricsPort: 9105},
			},
			export:      export,
			wantEnabled: true,
			wantAddr:    "127.0.0.1:9105",
		},
		{
			name: "enabled with unset port falls back to default",
			cfg: &config.Config{
				Monitoring: config.MonitoringConfig{MetricsEnabled: true, MetricsPort: 0},
			},
			export:      export,
			wantEnabled: true,
			wantAddr:    fmt.Sprintf("127.0.0.1:%d", config.DefaultMetricsPort),
		},
		{
			name: "enabled without sample source",
			cfg: &config.Config{
				Monitoring: config.MonitoringConfig{MetricsEnabled: true, MetricsPort: 9090},
			},
			export:  nil,
			wantErr: true,
		},
		{
			name: "enabled with out-of-range port",
			cfg: &config.Config{
				Monitoring: config.MonitoringConfig{MetricsEnabled: true, MetricsPort: 70000},
			},
			export:  export,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewAppServer(tt.cfg, "test", tt.export, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewAppServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if server.Enabled() != tt.wantEnabled {
				t.Errorf("Enabled() = %v, want %v", server.Enabled(), tt.wantEnabled)
			}
			if tt.wantAddr != "" && server.Addr() != tt.wantAddr {
				t.Errorf("Addr() = %q, want %q", server.Addr(), tt.wantAddr)
			}
			if !tt.wantEnabled {
				// A disabled app server must never open a socket.
				if err := server.Start(context.Background()); err != nil {
					t.Errorf("disabled Start() returned error: %v", err)
				}
			}
		})
	}
}

// TestParseConnectionsShapes covers the two slice shapes a SampleSource may
// return plus the malformed ones that must degrade to an empty snapshot.
func TestParseConnectionsShapes(t *testing.T) {
	asExportMap := []map[string]interface{}{
		{"id": "a", "method": "ssh", "state": "Connected", "bytes_sent": 10, "is_primary": true},
	}
	asJSON := []interface{}{
		map[string]interface{}{
			"id": "b", "method": "ssh", "state": "Connected",
			"bytes_sent": float64(20), "latency_ms": float64(5), "is_primary": false,
		},
	}

	tests := []struct {
		name      string
		export    map[string]interface{}
		wantLen   int
		wantFirst string
		wantSent  int64
		wantState string
	}{
		{name: "nil export", export: nil, wantLen: 0},
		{name: "missing connections key", export: map[string]interface{}{}, wantLen: 0},
		{
			name:    "wrong connections type",
			export:  map[string]interface{}{"connections": "not-a-list"},
			wantLen: 0,
		},
		{
			name:      "export shape ([]map)",
			export:    map[string]interface{}{"connections": asExportMap},
			wantLen:   1,
			wantFirst: "a",
			wantSent:  10,
			wantState: "connected",
		},
		{
			name:      "json shape ([]interface)",
			export:    map[string]interface{}{"connections": asJSON},
			wantLen:   1,
			wantFirst: "b",
			wantSent:  20,
			wantState: "connected",
		},
		{
			name: "non-map entries skipped",
			export: map[string]interface{}{
				"connections": []interface{}{"junk", map[string]interface{}{"id": "c"}},
			},
			wantLen:   1,
			wantFirst: "c",
			wantState: "", // missing state parses as empty, not a panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samples := parseConnections(tt.export)
			if len(samples) != tt.wantLen {
				t.Fatalf("parseConnections len = %d, want %d", len(samples), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}
			first := samples[0]
			if first.id != tt.wantFirst {
				t.Errorf("first id = %q, want %q", first.id, tt.wantFirst)
			}
			if first.bytesSent != tt.wantSent {
				t.Errorf("bytesSent = %d, want %d", first.bytesSent, tt.wantSent)
			}
			if first.state != tt.wantState {
				t.Errorf("state = %q, want %q", first.state, tt.wantState)
			}
		})
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("reading scrape body: %v", err)
	}
	return string(body)
}

// repoRoot walks up from the test's working directory to the module root.
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

// TestDocsMatchContract keeps the shipped documentation and the default
// configuration in sync with the endpoint contract pinned by the tests
// above. Update the docs in the same change as any intentional change.
func TestDocsMatchContract(t *testing.T) {
	readFile := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		return string(data)
	}

	doc := readFile(filepath.Join("docs", "METRICS.md"))
	for _, want := range []string{
		"/metrics",
		"127.0.0.1",
		ContentType,
		"metrics_enabled",
		"metrics_port",
		"no-op", // disabled lifecycle
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/METRICS.md is missing %q", want)
		}
	}

	readme := readFile("README.md")
	if !strings.Contains(readme, "docs/METRICS.md") {
		t.Error("README.md does not reference docs/METRICS.md")
	}

	defaultCfg := readFile(filepath.Join("configs", "default.yaml"))
	if !strings.Contains(defaultCfg, "metrics_enabled: false") {
		t.Error("configs/default.yaml no longer ships metrics_enabled: false (the documented default)")
	}
	if !strings.Contains(defaultCfg, fmt.Sprintf("metrics_port: %d", config.DefaultMetricsPort)) {
		t.Errorf("configs/default.yaml no longer ships the default metrics port %d", config.DefaultMetricsPort)
	}
}
