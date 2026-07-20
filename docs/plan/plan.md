# TUNNEL — Plan

> This file did not exist before 2026-07-20. TUNNEL was built without one; this
> document starts honestly, as a home for architecture decisions going forward,
> rather than a fabricated retroactive plan. See `README.md`, `docs/ARCHITECTURE.md`,
> `docs/PROVIDERS.md`, and `docs/connection-manager.md` for the existing (accurate,
> substantial) design documentation this repo already ships with.

## What TUNNEL is

A Go CLI/TUI that unifies ten SSH tunnel/VPN provider backends (Tailscale,
WireGuard, ZeroTier, Cloudflare Tunnel, ngrok, bore, VS Code Tunnels, SSH
port-forward, reverse SSH, bastion) behind one binary, with an embedded React
web UI (Fiber-served REST + WebSocket API on port 8080), automatic failover
between simultaneously-running providers, encrypted credential storage, and
hot-swap binary upgrades. Distributed as prebuilt binaries via GitHub Releases
(goreleaser); no hosted service — every deployment is a local install run by
the end user.

## Decisions

### ADR-1: 2026-07-20 — Default the embedded web UI to loopback-only and require an auth token for the control API

**Context**

TUNNEL embeds a Fiber web server exposing a REST + WebSocket API
(`internal/web/api/router.go`) that can, with no authentication whatsoever:
list/install/uninstall provider binaries, connect/disconnect tunnels, delete
connections, and enable/disable/reassign automatic failover
(`/api/providers/:name/install`, `/api/providers/:name/connect`,
`/api/connections/:id` DELETE, `/api/failover/enable`, etc.). The only
middleware registered (`internal/web/middleware/`) is CORS and request
logging — no auth, no rate limiting.

The README documents this as "served on `localhost:8080`," and the doctor/CLI
output echoes `http://localhost:...` back to the user. But the actual listen
call in `cmd/tunnel/cli.go` (`addr := fmt.Sprintf(":%d", actualPort)` →
`app.Listen(addr)`) binds an address with no host component, which in Go
binds **all interfaces**, not loopback. So on any machine with a routable
interface — a cloud VPS, a laptop on a shared/office/coffee-shop network, a
container with a bridged network — TUNNEL's control plane for SSH tunnels and
VPN connections is reachable from the network, unauthenticated, contradicting
both the documentation and the reasonable expectation for a tool whose entire
purpose is securing remote access.

This is the most significant gap found in this pass: the tool's stated job is
managing secure tunnels, and the management surface for those tunnels is
itself the least secure part of the system.

**Decision**

1. Bind the web server to `127.0.0.1` by default. Add an explicit
   `--host` / `web.listen_address` config option (mirroring the existing
   `ssh.listen_address` pattern already used for the SSH server in
   `cmd/tunnel/main.go`) for users who intentionally want LAN/remote access
   to the dashboard (e.g. checking tunnel status from a phone).
2. Generate a random bearer token on first run, store it via the existing
   credential store (`internal/core/credentials.go` — the same keyring/file
   backend already used for provider secrets), print it once on `tunnel`
   startup, and require it (`Authorization: Bearer <token>`) on all
   `/api/*` routes except a minimal unauthenticated `/api/system/info`
   health probe. The embedded React frontend reads the token from a
   same-origin-only bootstrap endpoint or a value injected at serve time,
   not from the URL or a cookie readable cross-origin.
3. Only widen exposure (bind non-loopback, or accept requests without the
   token) when the user has explicitly opted in via config/flag — never by
   default.

**Alternatives Considered**

- *Keep binding all interfaces but rely on host firewalls.* Rejected —
  TUNNEL's whole premise is that users can't assume a well-configured
  network; that's why it exists. Defaulting to "insecure unless the OS
  firewall saves you" inverts the product's value proposition.
- *Unix domain socket instead of TCP, with filesystem permissions as the
  auth boundary.* Considered for a future iteration — it's a stronger
  isolation primitive on Linux/macOS, but the current frontend/browser
  architecture (a React SPA making `fetch`/WebSocket calls) needs TCP to
  reach it from a browser; a unix socket would require a local proxy shim.
  Not rejected outright, but bearer-token-over-loopback-TCP is the smaller,
  faster-to-ship fix and doesn't preclude adding a socket option later.
- *mTLS between frontend and backend.* Rejected as disproportionate — this
  is a single-user local tool, not a multi-tenant service; the complexity
  (cert generation, rotation, browser trust) isn't justified when
  loopback-by-default + a bearer token already closes the actual exposure.
- *Do nothing, just fix the documentation to say "may bind non-loopback."*
  Rejected — the docs aren't the bug; a tunnel manager silently listening
  on all interfaces with zero auth is the bug.

**Consequences**

- Positive: closes a real, currently-shipping exposure — any TUNNEL user on
  a cloud instance or shared network today has an unauthenticated remote
  control plane for their SSH tunnels and VPN connections.
- Positive: establishes a pattern (loopback default + explicit opt-in +
  token) that the SSH-forwarding side of TUNNEL (`ssh.listen_address:
  0.0.0.0` in `cmd/tunnel/main.go`) should probably also be reviewed
  against, though that's a separate surface (raw SSH auth, not the web API)
  and out of scope here.
- Negative / migration cost: existing users who rely on reaching the
  dashboard from another host on their LAN (e.g. a homelab setup) will see
  it stop working after upgrade until they set `--host 0.0.0.0` explicitly
  — this is an intentional breaking change and should be called out
  prominently in the release notes for the version that ships it.
- Negative: the React frontend needs a small bootstrap change to acquire
  and attach the token; this touches `web/src/` request plumbing, not just
  the Go backend.

Implementation is tracked as beads (see `.beads/` in this repo, label
`artifact-improvement`), not performed as part of writing this ADR.

## Other improvement ideas from this pass (2026-07-20)

Filed as beads rather than expanded into ADRs — each is concrete and scoped,
not architectural in the way ADR-1 is:

- Failover health checks don't verify actual tunnel reachability (the signal
  driving automatic failover is a generic TCP probe that, by default,
  targets `localhost:22` regardless of which provider is running).
- CI (`.github/workflows/test.yml`, `release.yml`) has been red on every
  push since the initial release — three separate `go vet`/build errors,
  never fixed.
- Two divergent GoReleaser configs (`.goreleaser.yaml` vs `.goreleaser.yml`)
  mean the Homebrew/deb/rpm/Docker distribution targets likely never
  actually ship.
- Hot-swap binary upgrade watcher restarts into a new binary based on mtime
  change alone, with no integrity check.
- Connection metrics history is in-memory only (last 10 samples, wiped on
  restart) — no persisted uptime/failover-count trend data for the
  dashboard.

See the `artifact-improvement`-labeled beads in this repo's `.beads/`
workspace for full detail on each.
