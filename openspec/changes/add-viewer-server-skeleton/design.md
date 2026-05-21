## Context

`ezida` is a single static Go binary distributed via `install.sh` (ADR
0001 §D17–§D20). The CLI subcommands wire into `cmd/ezida/main.go`
through `internal/commands`. The storage layer (`internal/board`) is
the only path to `kanban.toml`; this phase consumes it read-only.

All cross-cutting choices for the viewer batch live in
`openspec/decisions/0002-viewer-batch.md`. This design covers only the
P-specific architecture for the server skeleton: package layout, port
fallback algorithm, embed wiring, and the shutdown protocol.

## Goals / Non-Goals

**Goals:**
- Stand up an HTTP server on `127.0.0.1:7777` (with fallback) that can
  be launched from `ezida serve` and stopped cleanly with `Ctrl+C`.
- Expose `GET /api/board` returning the board JSON per ADR 0002 §D7.
- Establish the `internal/server/web/` embed root with at least one
  served asset so V1-UI can drop the real `index.html`/`app.js` in
  without touching server code.
- Open the user's browser to the chosen URL on startup unless
  `--no-open` is set.
- Provide the structured-error path for `PORT_UNAVAILABLE` so the
  CLI's existing `output.Fail` surface handles it identically to
  other errors.

**Non-Goals:**
- No HTML/CSS authoring — the placeholder `index.html` is one `<h1>`.
- No mutation endpoints (move, PATCH) — V2/V3.
- No SSE, no file watcher — V4 (and the `internal/server/web/vendor/`
  subtree is empty in this phase).
- No write-path code in `internal/server` — V1 is read-only.
- No request logging middleware. Single-user localhost; defer until a
  real need surfaces.

## Decisions

### File layout

```
cmd/ezida/main.go                     # +1 AddCommand(NewServeCmd)
internal/commands/serve.go            # cobra wiring → server.Run
internal/commands/errors.go           # +PortUnavailableError type
internal/server/
  server.go                           # Run(opts) error, bind loop, shutdown
  handlers.go                         # routes: /, /static/*, /api/board
  browser.go                          # Open(url) cross-platform helper
  embed.go                            # //go:embed web/* asset FS
  web/
    index.html                        # placeholder (one heading)
    app.js                            # empty stub
    style.css                         # empty stub
  server_test.go                      # port fallback + /api/board test
```

### `server.Run` signature

```go
package server

type Options struct {
    Port    int    // starting port; default 7777
    NoOpen  bool   // skip browser open
    Board   string // path to kanban.toml (typically "./kanban.toml")
}

// Run blocks until SIGINT/SIGTERM, then drains and returns.
func Run(opts Options) error
```

The cobra command in `internal/commands/serve.go` parses flags,
resolves the board path (current directory + `kanban.toml`), and
calls `server.Run`. Cobra returns the error to `output.Fail` per
the existing CLI contract.

### Port fallback algorithm

```go
start := opts.Port
if start == 0 { start = 7777 }
const window = 11 // 7777..7787 if start=7777

for p := start; p < start+window; p++ {
    ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
    if err == nil {
        return p, ln, nil
    }
    // only retry on "address in use"; surface other errors immediately
    if !isAddrInUse(err) { return 0, nil, err }
}
return 0, nil, &PortUnavailableError{Start: start, Window: window}
```

`isAddrInUse` checks `errno == EADDRINUSE` via `errors.As` against
`*net.OpError` → `*os.SyscallError`. Avoiding string matches.

### Embed wiring

```go
// internal/server/embed.go
package server

import "embed"

//go:embed web
var webFS embed.FS
```

`handlers.go` mounts `/static/` via `http.FileServerFS(fs.Sub(webFS, "web"))`
and serves `/` by reading `web/index.html` directly through `webFS`.

V1-UI replaces `web/index.html` with the real page, adds vendored
Alpine, edits CSS. No server-side change needed because the embed
declaration captures the whole subtree.

### `GET /api/board`

```go
func (s *server) handleBoard(w http.ResponseWriter, r *http.Request) {
    b, err := board.Load(s.boardPath)
    if err != nil {
        s.writeError(w, err)
        return
    }
    payload := struct {
        SchemaVersion   int                 `json:"schema_version"`
        Columns         []string            `json:"columns"`
        Priorities      []string            `json:"priorities"`
        CardsPerColumn  map[string]int      `json:"cards_per_column"`
        Cards           []board.Card        `json:"cards"`
    }{...}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(payload)
}
```

`board.Card` already carries JSON-friendly TOML tags; explicit JSON
tags on the response struct keep the wire shape snake_case. Cards
include the `description` field per ADR 0002 §D7.

### Graceful shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(),
    syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() {
    <-ctx.Done()
    sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = srv.Shutdown(sdCtx)
}()

if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
    return err
}
return nil
```

`signal.NotifyContext` (Go 1.16+) handles signal masking and
`srv.Shutdown` drains in-flight requests within 5 s (ADR 0002 §D12).

### Browser open

```go
// browser.go
func Open(url string) error {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin":
        cmd = exec.Command("open", url)
    case "linux":
        cmd = exec.Command("xdg-open", url)
    default:
        return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
    }
    return cmd.Start()
}
```

Best-effort: a failure to launch the browser logs a warning but does
not abort the server. Windows path absent on purpose — no Windows
support in v1 (ADR 0001 §D2).

## Risks / Trade-offs

- **Port fallback racing**: between `net.Listen` returning success
  and `http.Serve` accepting, no other process can grab the port
  because we hold the listener. Safe.
- **Browser open failures on headless Linux** (no `xdg-open`,
  `DISPLAY` unset): handled by logging; users on SSH should pass
  `--no-open` regardless.
- **Embed of empty subtrees**: `//go:embed web` requires at least one
  file. The placeholder `index.html` and stub `app.js`/`style.css`
  satisfy this and act as the contract V1-UI fills in.
- **JSON encoding allocates `cards_per_column` map per request**:
  trivial for v1 board sizes; revisit only if profiling demands it.
- **No request logging**: a 404 or 500 disappears silently. Acceptable
  for v1; add structured logging if/when the viewer leaves the "demo
  in front of one user" stage.

## Migration Plan

Not applicable. New code path; no existing users; no rollback target.

## Open Questions

None within this phase. The board file path defaults to
`./kanban.toml` (current working directory at server start); a future
`--board=<path>` flag is trivial to add but out of scope per ADR
0002 §D13.
