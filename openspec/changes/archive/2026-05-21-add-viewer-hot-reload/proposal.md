## Why

After V3 the viewer can read, drag, and edit, but only its own
mutations are visible — a CLI command in another terminal does not
update the open browser. Users have to F5. This change closes the
loop: the server watches `kanban.toml` for external changes
(`fsnotify`), broadcasts a `board-changed` SSE event, and the UI
refetches. The full developer story from the brief becomes real:
type `ezida add ...` in one terminal, watch the card appear in the
browser in another.

## What Changes

- `go.mod`: add the single new dependency for this batch,
  `github.com/fsnotify/fsnotify` (per ADR 0002 §D5).
- `internal/server/watcher.go` (new file): `Watcher` type that
  takes the board file path, starts an `fsnotify.Watcher`,
  debounces events 200 ms per ADR 0002 §D10, and exposes a
  `Events() <-chan struct{}` channel for downstream consumers.
- `internal/server/sse.go` (new file): the broker maintaining the
  registered SSE clients plus a `Broadcast()` method. `GET /api/events`
  registers a client, writes `retry: 2000` on connect, emits a
  `: ping` heartbeat every 30 s, and forwards every broker event
  as `event: board-changed\ndata: \n\n`.
- `internal/server/server.go`: starts the watcher and the broker
  alongside the HTTP server in `Run`, wires them together (watcher
  events → broker broadcast), shuts both down cleanly when the
  signal handler fires.
- `internal/server/handlers.go`: register `GET /api/events`.
- `internal/server/web/app.js`:
  - On `init`, open `new EventSource('/api/events')`.
  - On `event: board-changed`, call `load()` (which already
    re-mounts Sortable and re-renders).
  - On `EventSource` error/close, the browser auto-reconnects per
    SSE semantics. Track a `connected` boolean for the topbar
    indicator.
  - If the edit modal (from V3) is open at the time of
    `board-changed`, close it without prompting and reset
    `draft`/`error`.
- `internal/server/web/index.html`:
  - Add a `<span class="status-dot" :class="connected ? 'on' : 'off'">`
    next to the topbar title so the user sees connection health.
- `internal/server/web/style.css`:
  - Add `.status-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; margin-left: 8px }`,
    `.status-dot.on { background: #2a8 }`,
    `.status-dot.off { background: #aaa }`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `viewer-server`: adds the `GET /api/events` SSE endpoint, the
  internal watcher, and the broker contract (single event type
  `board-changed`, debounce, heartbeat).
- `viewer-ui`: adds the EventSource client, the connection-status
  indicator in the topbar, and the "modal closes on external
  change" behavior.

## Impact

- New Go dependency: `github.com/fsnotify/fsnotify` (cross-platform
  file watching, MIT licensed).
- New code in `internal/server/watcher.go` (~70 LOC), `sse.go`
  (~80 LOC), and ~30 LOC of plumbing in `server.go`.
- New code in `internal/server/web/app.js` (~25 LOC).
- New error: none new; the watcher's startup failure surfaces via
  `server.Run` returning the error.
- Binary size grows by the fsnotify dependency (small, mostly
  stdlib-style code).
- README / docs need a note about the hot-reload behavior — handled
  in the batch's documentation pass (outside this change).
