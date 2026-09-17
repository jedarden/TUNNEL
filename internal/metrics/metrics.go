// Package metrics serves the optional Prometheus metrics endpoint.
//
// Contract (full details in docs/METRICS.md):
//
//   - Enabled by monitoring.metrics_enabled in the application config (off by
//     default). A disabled server never binds a port; Start and Stop are no-ops.
//   - When enabled, a second, minimal HTTP server listens on
//     127.0.0.1:<monitoring.metrics_port> (loopback only, unauthenticated —
//     the loopback bind is the security boundary, same model as the standard
//     exporters Prometheus scrapes) and answers GET /metrics with the
//     Prometheus text exposition format, version 0.0.4.
//   - Port validation: metrics_port must be 1-65535. Port 0 means "unset" and
//     falls back to the configured default (config.DefaultMetricsPort) at the
//     application-wiring layer; the raw NewServer constructor treats 0 as an
//     ephemeral port for tests and embedding.
//   - Lifecycle: Start binds and serves in the background; Stop drains
//     in-flight scrapes and closes the listener; cancelling the Start context
//     also stops the server. Start after Stop is supported.
package metrics

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/tunnel/pkg/config"
)

// MetricsPath is the URL path the exposition endpoint is served on.
const MetricsPath = "/metrics"

// ContentType is the media type of a scrape response.
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// DefaultListenHost is the only interface the endpoint ever binds. Metrics
// are unauthenticated by design, so they must stay off the network.
const DefaultListenHost = "127.0.0.1"

// SampleSource supplies the current metrics snapshot at scrape time. It has
// the shape of core.DefaultMetricsCollector.Export() — the same data the
// /api/metrics JSON endpoint serves — so both surfaces share one data path.
// A nil return or nil function yields a snapshot with no connection samples.
type SampleSource func() map[string]interface{}

// Config configures a metrics endpoint server.
type Config struct {
	// Enabled turns the endpoint on. When false the server is inert: it
	// never binds a port and Start/Stop are no-ops.
	Enabled bool
	// Host is the bind address. Empty defaults to loopback (127.0.0.1).
	Host string
	// Port is the TCP port. Application wiring substitutes
	// config.DefaultMetricsPort for 0 ("unset") before reaching this point;
	// raw NewServer callers may pass 0 to get an ephemeral port.
	// 1-65535 otherwise; anything else is rejected when Enabled.
	Port int
	// Version is reported in the tunnel_build_info metric.
	Version string
	// Logger receives startup and shutdown diagnostics. Defaults to the
	// standard logger.
	Logger *log.Logger
	// Export supplies the metrics snapshot per scrape. Required when Enabled.
	Export SampleSource
}

// Server serves the Prometheus exposition endpoint for a single process.
type Server struct {
	enabled bool
	host    string
	port    int
	version string
	logger  *log.Logger
	export  SampleSource

	mux     *http.ServeMux
	scrapes atomic.Int64

	mu         sync.Mutex
	serving    bool
	httpServer *http.Server
	listener   net.Listener
}

// NewServer validates cfg and returns the endpoint server. Disabled servers
// are always constructible; enabled ones require a sample source and a
// usable port.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = DefaultListenHost
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}

	s := &Server{
		enabled: cfg.Enabled,
		host:    cfg.Host,
		port:    cfg.Port,
		version: cfg.Version,
		logger:  cfg.Logger,
		export:  cfg.Export,
	}

	if !cfg.Enabled {
		// Inert by contract, but the handler still works so callers can
		// mount it elsewhere if they ever want to.
		s.buildMux()
		return s, nil
	}

	if cfg.Export == nil {
		return nil, fmt.Errorf("metrics enabled but no sample source configured")
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid metrics port %d: must be 1-65535 (or 0 for an ephemeral port)", cfg.Port)
	}

	s.buildMux()
	return s, nil
}

// NewAppServer builds the endpoint server from the application configuration.
// A nil config or one with monitoring.metrics_enabled false (the default)
// yields a disabled server whose Start and Stop are no-ops and which never
// binds a port. An error means the caller misconfigured an enabled endpoint
// (missing sample source, unusable port); callers should log it and continue
// without metrics.
func NewAppServer(appCfg *config.Config, version string, export SampleSource, logger *log.Logger) (*Server, error) {
	if appCfg == nil || !appCfg.Monitoring.MetricsEnabled {
		return NewServer(Config{Version: version, Logger: logger})
	}

	port := appCfg.Monitoring.MetricsPort
	if port == 0 {
		// "Unset" in the config file; pkg/config normalizes this on load,
		// but don't depend on every caller having gone through Load.
		port = config.DefaultMetricsPort
	}

	return NewServer(Config{
		Enabled: true,
		Port:    port,
		Version: version,
		Logger:  logger,
		Export:  export,
	})
}

// buildMux wires the route table once at construction.
func (s *Server) buildMux() {
	s.mux = http.NewServeMux()
	// The GET pattern also matches HEAD; any other method gets a 405 with an
	// Allow header, and any other path a 404, both from the standard mux.
	s.mux.HandleFunc("GET "+MetricsPath, s.handleScrape)
}

// Handler returns the HTTP handler serving the endpoint. Exposed for tests
// and for embedding the endpoint in another server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Enabled reports whether the endpoint was configured on. A disabled server
// never binds a port.
func (s *Server) Enabled() bool {
	return s.enabled
}

// Addr returns the address scrapers should target, "host:port". It is the
// bound address once Start has succeeded (which also resolves ephemeral
// ports), the configured address before that, and empty for a disabled
// server.
func (s *Server) Addr() string {
	if !s.enabled {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return net.JoinHostPort(s.host, strconv.Itoa(s.port))
}

// Start binds the endpoint and serves it in the background. It is a no-op
// for a disabled server or one already serving, and is safe to call again
// after Stop. Cancelling ctx stops the server, as does an explicit Stop.
// A bind failure is returned to the caller: metrics are optional, so the
// application treats this as a warning, not a fatal error.
func (s *Server) Start(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	if s.serving {
		s.mu.Unlock()
		return nil
	}
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("metrics endpoint listen on %s: %w", addr, err)
	}
	httpServer := &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.listener = listener
	s.httpServer = httpServer
	s.serving = true
	s.mu.Unlock()

	s.logger.Printf("Metrics endpoint serving on http://%s%s", s.Addr(), MetricsPath)

	go func() {
		// ErrServerClosed on graceful shutdown is the expected path.
		_ = httpServer.Serve(listener)
	}()

	go func() {
		<-ctx.Done()
		if err := s.Stop(context.Background()); err != nil {
			s.logger.Printf("metrics endpoint shutdown: %v", err)
		}
	}()

	return nil
}

// Stop stops serving and waits for in-flight scrapes to finish (bounded by
// ctx). It is a no-op when the server is disabled or not serving, and is
// safe to call more than once.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.serving {
		s.mu.Unlock()
		return nil
	}
	httpServer := s.httpServer
	s.serving = false
	s.httpServer = nil
	s.listener = nil
	s.mu.Unlock()

	s.logger.Printf("Metrics endpoint stopping")
	return httpServer.Shutdown(ctx)
}

// handleScrape increments the scrape counter, snapshots the sample source and
// renders the exposition body.
func (s *Server) handleScrape(w http.ResponseWriter, r *http.Request) {
	scrapes := s.scrapes.Add(1)

	var connections []connectionSample
	if s.export != nil {
		connections = parseConnections(s.export())
	}

	body := render(snapshot{
		version:     s.version,
		scrapes:     scrapes,
		connections: connections,
	})

	w.Header().Set("Content-Type", ContentType)
	// Content-Length is set implicitly by Write's buffering; set it
	// explicitly so scrapers can rely on a single unchunked response.
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

// snapshot is the rendered data for one scrape.
type snapshot struct {
	version     string
	scrapes     int64
	connections []connectionSample
}

// connectionSample is one connection's scrape data, defensively parsed out of
// the SampleSource map so a malformed export degrades to zeroed fields rather
// than breaking the endpoint.
type connectionSample struct {
	id            string
	method        string
	state         string
	bytesSent     int64
	bytesReceived int64
	latencyMS     float64
	uptimeSeconds float64
	isPrimary     bool
}

// parseConnections extracts connection samples from a SampleSource result.
// It accepts both the []map[string]interface{} shape Export builds and the
// []interface{} shape a JSON round-trip produces.
func parseConnections(export map[string]interface{}) []connectionSample {
	if export == nil {
		return nil
	}

	raw, ok := export["connections"]
	if !ok {
		return nil
	}

	var entries []interface{}
	switch list := raw.(type) {
	case []map[string]interface{}:
		entries = make([]interface{}, len(list))
		for i, e := range list {
			entries[i] = e
		}
	case []interface{}:
		entries = list
	default:
		return nil
	}

	samples := make([]connectionSample, 0, len(entries))
	for _, entry := range entries {
		fields, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		samples = append(samples, connectionSample{
			id:            asString(fields["id"]),
			method:        asString(fields["method"]),
			state:         strings.ToLower(asString(fields["state"])),
			bytesSent:     asInt64(fields["bytes_sent"]),
			bytesReceived: asInt64(fields["bytes_received"]),
			latencyMS:     asFloat64(fields["latency_ms"]),
			uptimeSeconds: asFloat64(fields["uptime_seconds"]),
			isPrimary:     asBool(fields["is_primary"]),
		})
	}
	return samples
}

// asString, asInt64, asFloat64 and asBool coerce the loosely-typed values in
// a SampleSource map, defaulting to the zero value on any mismatch.

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func asInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	default:
		return 0
	}
}

func asFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func asBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// render produces the Prometheus text exposition format (version 0.0.4) for
// one scrape. Family order, sample order and label order are fixed, so the
// output for a given snapshot is byte-for-byte deterministic.
func render(snap snapshot) string {
	var b strings.Builder

	if snap.version != "" {
		b.WriteString("# HELP tunnel_build_info Build information for this TUNNEL process.\n")
		b.WriteString("# TYPE tunnel_build_info gauge\n")
		b.WriteString("tunnel_build_info{version=\"" + escapeLabelValue(snap.version) + "\"} 1\n")
	}

	b.WriteString("# HELP tunnel_connections Registered connections by state.\n")
	b.WriteString("# TYPE tunnel_connections gauge\n")
	// Count per state, preserving first-seen order so the output is stable.
	var stateOrder []string
	stateCounts := make(map[string]int)
	for _, conn := range snap.connections {
		if stateCounts[conn.state] == 0 {
			stateOrder = append(stateOrder, conn.state)
		}
		stateCounts[conn.state]++
	}
	for _, state := range stateOrder {
		b.WriteString("tunnel_connections{state=\"" + escapeLabelValue(state) + "\"} " +
			strconv.Itoa(stateCounts[state]) + "\n")
	}

	b.WriteString("# HELP tunnel_connection_bytes_sent_total Bytes sent per connection.\n")
	b.WriteString("# TYPE tunnel_connection_bytes_sent_total counter\n")
	for _, conn := range snap.connections {
		b.WriteString("tunnel_connection_bytes_sent_total" + connectionLabels(conn) + " " +
			strconv.FormatInt(conn.bytesSent, 10) + "\n")
	}

	b.WriteString("# HELP tunnel_connection_bytes_received_total Bytes received per connection.\n")
	b.WriteString("# TYPE tunnel_connection_bytes_received_total counter\n")
	for _, conn := range snap.connections {
		b.WriteString("tunnel_connection_bytes_received_total" + connectionLabels(conn) + " " +
			strconv.FormatInt(conn.bytesReceived, 10) + "\n")
	}

	b.WriteString("# HELP tunnel_connection_latency_milliseconds Average measured latency per connection.\n")
	b.WriteString("# TYPE tunnel_connection_latency_milliseconds gauge\n")
	for _, conn := range snap.connections {
		b.WriteString("tunnel_connection_latency_milliseconds" + connectionLabels(conn) + " " +
			strconv.FormatFloat(conn.latencyMS, 'g', -1, 64) + "\n")
	}

	b.WriteString("# HELP tunnel_connection_uptime_seconds Seconds since each connection started.\n")
	b.WriteString("# TYPE tunnel_connection_uptime_seconds gauge\n")
	for _, conn := range snap.connections {
		b.WriteString("tunnel_connection_uptime_seconds" + connectionLabels(conn) + " " +
			strconv.FormatFloat(conn.uptimeSeconds, 'g', -1, 64) + "\n")
	}

	b.WriteString("# HELP tunnel_metrics_scrapes_total Scrapes served since process start.\n")
	b.WriteString("# TYPE tunnel_metrics_scrapes_total counter\n")
	b.WriteString("tunnel_metrics_scrapes_total " + strconv.FormatInt(snap.scrapes, 10) + "\n")

	return b.String()
}

// connectionLabels renders the fixed label set every per-connection metric
// carries. Values are quoted manually because they are already escaped —
// fmt's %q would apply Go escaping on top of the Prometheus escaping.
func connectionLabels(conn connectionSample) string {
	return `{connection="` + escapeLabelValue(conn.id) +
		`",method="` + escapeLabelValue(conn.method) +
		`",primary="` + strconv.FormatBool(conn.isPrimary) + `"}`
}

// escapeLabelValue applies the exposition format's label-value escaping
// (backslash, double quote, newline). fmt's %q is not used for the value
// itself because Go escaping and Prometheus escaping differ for control
// characters.
func escapeLabelValue(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}
