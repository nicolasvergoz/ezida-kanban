## Why

`ezida` has a CLI and an embedded skill but no graphical surface. The
viewer batch (ADR 0002) adds a browser-based Kanban view backed by the
same `kanban.toml`. V1 builds the server foundation: an HTTP process
launched by `ezida serve`, capable of exposing the current board as
JSON, ready for the read-only UI (V1 second half), drag/move (V2),
inline edit (V3), and SSE hot-reload (V4) to layer on top.

This change ships **server-side only**: cobra wiring, HTTP boot, port
fallback, graceful shutdown, the `GET /api/board` endpoint, the
embedded-assets scaffold (empty placeholder `index.html` is enough),
and the cross-platform "open browser" helper. No HTML page authored
yet (that lands in `add-viewer-ui-readonly`).

## What Changes

- New cobra subcommand `ezida serve [--port=N] [--no-open]`, wired in
  `cmd/ezida/main.go`.
- New package `internal/server/` with:
  - `server.go`: HTTP server boot, port fallback (7777 → 7786 per
    ADR 0002 §D6), browser open on startup, graceful shutdown on
    SIGINT/SIGTERM with 5 s drain (ADR §D12).
  - `handlers.go`: `GET /` serves the embedded `index.html` placeholder;
    `GET /static/*` serves the embedded asset tree;
    `GET /api/board` returns the full board JSON per ADR 0002 §D7.
  - `browser.go`: cross-platform open helper (`open` on darwin,
    `xdg-open` on linux). Best-effort; failures only log.
  - `web/index.html`: minimal placeholder page (one `<h1>` and a
    `<script src="/static/app.js">`). Real UI lands in V1-UI.
  - `web/app.js`, `web/style.css`: empty stubs so the embed tree
    compiles.
  - `embed.go`: `//go:embed web` declaration owning the asset FS.
- New commands package entry: `internal/commands/serve.go` constructs
  the cobra command and delegates to `internal/server.Run`.
- New error codes in `internal/commands/errors.go` (or equivalent):
  `PORT_UNAVAILABLE` (all 11 fallback ports busy).
- No new external Go dependencies in this phase. `fsnotify` is
  deferred to V4.

## Capabilities

### New Capabilities

- `viewer-server`: HTTP server lifecycle, port binding and fallback,
  graceful shutdown, asset embedding contract, browser-open behavior,
  and the JSON contract for `GET /api/board`. Future viewer phases
  add to this spec (move endpoint, PATCH endpoint, SSE stream).

### Modified Capabilities

None. Existing specs (`board-storage`, `card-reading`, etc.) are
consumed read-only via `board.Load`.

## Impact

- New code under `internal/server/`, `internal/commands/serve.go`,
  and `cmd/ezida/main.go` (one `AddCommand` line).
- No new Go dependencies in `go.mod`.
- Binary size grows by the embedded placeholder assets (~1 KB
  including stubs; real growth comes in V1-UI and V2/V4).
- New runtime surface: localhost HTTP port. Documented as
  127.0.0.1-only (ADR 0002 §D6) — no firewall exposure.
- README and `docs/usage.md` need a `serve` section by the end of the
  batch; this phase notes the TODO but does not author docs yet.
