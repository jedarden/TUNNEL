# Web UI Embedding & Serving Smoke Tests

Smoke tests covering the promise the README makes: the React frontend is
**embedded in the Go binary** and **served on `localhost:8080`**, through the
documented `make build` / `make build-dev` / `make frontend-dev` + `make dev`
workflows.

## Where the tests live

| File | What it verifies |
|------|------------------|
| `internal/web/embed/embed_test.go` | The embedded `dist` is re-rooted correctly, its entrypoint and referenced assets are valid, and a real build is byte-identical to `web/dist`. Isolated Makefile tests exercise `build-frontend` success/failure, `build-dev` placeholder output, and the `frontend-dev` command without network access. |
| `internal/web/static_test.go` | The production serving path (`web.MountFrontend`): `GET /` returns the embedded `index.html`, deep SPA routes fall back to `index.html`, referenced assets are served byte-identically with correct MIME types, and the UI is reachable over real loopback TCP. |
| `internal/web/embed/smokebin_test.go` | Builds and runs a small binary using the production Fiber mount, checks the embedded entrypoint is present in the executable, and fetches `/`, a deep SPA route, and real bundle assets over loopback HTTP. |
| `cmd/tunnel/webserver_test.go` | The documented address itself: `--port` defaults to 8080, the bind address defaults to `127.0.0.1` (loopback-only) with `--host` / `TUNNEL_WEB_LISTEN_ADDRESS` / config overrides honored, README + `configs/default.yaml` still promise `localhost:8080`, and a failed main-binary build fails this smoke test. |

## Build states the tests tolerate

`internal/web/embed/dist/` holds a tracked development placeholder or a real
Vite build, and the tests assert what each state can guarantee:

1. **Placeholder** (tracked `index.html`, written by `make build-dev` /
   restored by `make clean`): serving tests still pass — the page is served,
   SPA fallback works — and asset checks are vacuous.
2. **Real Vite build** (`make build-frontend` output): the full set of
   assertions applies, including the `web/dist` ↔ embed-dir byte comparison.
The placeholder `index.html` is **tracked in git** (generated build output
under `internal/web/embed/dist/` remains ignored) so a fresh clone compiles —
`//go:embed all:dist` needs at least one matching file — and `make test`
works before any frontend build.

## Consequences for the build workflow

- `make build-frontend` builds into `web/dist`, stages a complete copy in a
  temporary embed directory, and swaps it into place only after npm succeeds
  and an entrypoint exists. A failed build leaves the last embedded bundle
  intact.
- `make clean` restores the placeholder rather than deleting the embed dir,
  so `go build` keeps working after a clean.
- After `make build`/`make build-frontend` the tracked placeholder shows as
  modified (it now holds the real build's `index.html`); revert it with
  `git checkout -- internal/web/embed/dist/index.html` or `make build-dev`
  before committing, unless intentionally updating the placeholder.
