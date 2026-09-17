# Prometheus Metrics Endpoint — Contract and Lifecycle

TUNNEL exposes an optional Prometheus scrape endpoint for connection metrics.
This document is the contract: endpoint, disabled behavior, port validation,
startup/shutdown, and the exported metric families. The implementation lives
in `internal/metrics`; the behavior described here is pinned by
`internal/metrics/metrics_test.go` and `pkg/config/config_test.go`.

## Configuration

```yaml
monitoring:
  metrics_enabled: false   # endpoint off by default
  metrics_port: 9090       # TCP port; 0 means "unset" → 9090
```

- `metrics_enabled: false` (the default) — no endpoint, no listener, nothing
  to shut down.
- `metrics_port` — the TCP port the endpoint binds. Validated at config load:
  any value other than `0` (unset) must be in `1`–`65535`, **whether or not
  the endpoint is enabled**. A typo'd port fails `tunnel` startup rather than
  surfacing later as an endpoint that can never bind. An unset port (`0`)
  falls back to `9090` (`config.DefaultMetricsPort`) during `Load`/`Reload`.

## Endpoint contract

| Property | Value |
|---|---|
| URL | `http://127.0.0.1:<metrics_port>/metrics` |
| Bind address | `127.0.0.1` only — never a routable interface |
| Methods | `GET` and `HEAD` on `/metrics`; anything else gets `405` (with `Allow`) on that path, and any other path gets `404` |
| Success response | `200`, body in Prometheus text exposition format |
| Content type | `text/plain; version=0.0.4; charset=utf-8` |
| Authentication | none |

**Why loopback and no auth:** the endpoint speaks the scrape format Prometheus
polls, and standard exporters (node_exporter &c.) are unauthenticated by
design; the security boundary is the loopback bind, which keeps metrics off
the network. This mirrors the web UI's default of `127.0.0.1:8080` (the UI has
a bearer token because it is interactive; the metrics endpoint has no secrets
beyond connection counts, latency and byte counters). If you need metrics from
another host, scrape through an SSH tunnel or a reverse proxy that applies its
own access control — there is no configuration knob to bind the endpoint
publicly.

## Lifecycle

**Startup.** The endpoint is constructed and started from
`startWebServer` in `cmd/tunnel/cli.go`, alongside the web UI server:

1. `metrics.NewAppServer` builds the server from the loaded application
   config. A nil config or `metrics_enabled: false` yields a *disabled*
   server: `Start`/`Stop` are no-ops and no port is ever bound.
2. An enabled server binds `127.0.0.1:<metrics_port>` and serves in the
   background; `Start` returns as soon as the listener is up.
3. **A bind failure is a warning, not a fatal error** — the port may already
   be taken by another Prometheus exporter, and metrics are an add-on. The
   message `Warning: metrics endpoint unavailable: …` is logged and the app
   (and every tunnel) runs on without the endpoint. Unlike the web UI, the
   metrics port does **not** auto-increment on conflict: a Prometheus scrape
   config pins the port, so a silently different port would be worse than no
   endpoint.
4. Misconfiguration that slips past config validation (for example, a missing
   metrics source) disables the endpoint with a `Metrics endpoint disabled: …`
   log line.

**Scrape time.** Each scrape snapshots the same in-memory metrics the JSON
API serves from `internal/core` (`DefaultMetricsCollector.Export()`), so both
surfaces share one data path. Malformed entries in a snapshot degrade to
zeroed fields; they never fail the scrape.

**Shutdown.** On application shutdown (SIGINT/SIGTERM, TUI exit, or hot-swap
binary upgrade), `Stop` closes the listener and waits for in-flight scrapes
to finish before the process exits. Cancelling the application context also
stops the endpoint. `Stop` is idempotent and a disabled or never-started
server is a no-op; `Start` after `Stop` rebinds (used by tests, and safe for
embedding).

**Restart-to-apply.** The endpoint reads `metrics_enabled`/`metrics_port`
once at startup. Editing the config file (the app watches it) does not bind,
rebind or unbind the endpoint — restart `tunnel` to apply.

## Exported metrics

All metric names are prefixed `tunnel_`. Label values are escaped per the
exposition format (`\`, `"`, newline).

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `tunnel_build_info` | gauge | `version` | Always `1`; identifies the running binary. Omitted when no version is supplied. |
| `tunnel_connections` | gauge | `state` | Registered connections per state (`connected`, `disconnected`, …, lower-cased from the internal state names). |
| `tunnel_connection_bytes_sent_total` | counter | `connection`, `method`, `primary` | Bytes sent per connection. |
| `tunnel_connection_bytes_received_total` | counter | `connection`, `method`, `primary` | Bytes received per connection. |
| `tunnel_connection_latency_milliseconds` | gauge | `connection`, `method`, `primary` | Average measured latency (ms) per connection. |
| `tunnel_connection_uptime_seconds` | gauge | `connection`, `method`, `primary` | Seconds since the connection started. |
| `tunnel_metrics_scrapes_total` | counter | — | Scrapes served since process start. |

`primary` is `"true"` for the connection currently holding the primary
(failover) role. Family order, sample order and label order are fixed, so a
given snapshot renders byte-for-byte identically — asserted by
`TestRenderExactOutput`.

### Example scrape

```
# HELP tunnel_build_info Build information for this TUNNEL process.
# TYPE tunnel_build_info gauge
tunnel_build_info{version="1.0.0"} 1
# HELP tunnel_connections Registered connections by state.
# TYPE tunnel_connections gauge
tunnel_connections{state="connected"} 2
# HELP tunnel_connection_bytes_sent_total Bytes sent per connection.
# TYPE tunnel_connection_bytes_sent_total counter
tunnel_connection_bytes_sent_total{connection="cloudflare-100",method="cloudflare",primary="true"} 1024
...
# HELP tunnel_metrics_scrapes_total Scrapes served since process start.
# TYPE tunnel_metrics_scrapes_total counter
tunnel_metrics_scrapes_total 1
```

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: tunnel
    scrape_interval: 15s
    static_configs:
      - targets: ["127.0.0.1:9090"]
```

(When Prometheus runs on another host, target an SSH tunnel or proxy to the
TUNNEL host's loopback endpoint; see the security note above.)

## What is intentionally not exported (yet)

- Failover events as time series (they are in the audit log and the JSON API
  today); only per-connection state and the `primary` label reflect failover
  in Prometheus.
- Process/Go runtime metrics (no `client_golang` dependency; the exposition
  format is rendered directly to keep the dependency tree flat).
