## Context

V1–V3 build a viewer that operates correctly as long as the only
mutator is the viewer itself. ADR 0002 §D9–§D10 already pinned the
external-change story: a single event type, debounced watcher,
self-write tolerated (no detection), heartbeat to keep proxies
alive. This change builds the implementation.

The brief frames this as a single-user, localhost scenario, which
keeps the watcher and broker simple — one watched file, one event
type, no per-client filtering.

## Goals / Non-Goals

**Goals:**
- External writes to `kanban.toml` (CLI, AI assistant, manual
  editor save) trigger a UI refresh within ~1 s.
- The browser keeps a persistent SSE connection and reconnects
  automatically after a server restart.
- The topbar surfaces the connection state.
- A modal open during an external change closes cleanly without
  data loss to the on-disk file (the on-disk file already won;
  the modal's draft is local-only).
- The server shuts down both the watcher and all SSE streams
  cleanly on `SIGINT`/`SIGTERM`.

**Non-Goals:**
- No diff stream — the event is "something changed", clients
  refetch the whole board (per ADR 0002 §D9).
- No self-write suppression. Every viewer write triggers a
  redundant refetch for the originating client. Acceptable cost.
- No toast on external change (deferred to the V5 polish phase
  that is excluded from this batch).
- No multi-file watching. Only `kanban.toml`.
- No filesystem-watching fallback for systems without inotify /
  fsevents — `fsnotify` already abstracts the cross-platform
  story for darwin and linux (ADR 0001 §D2 supported platforms).

## Decisions

### Package layout

```
internal/server/
  watcher.go   # type Watcher; New(path) (*Watcher, error); Run(ctx); Events()
  sse.go       # type Broker; broker.Subscribe(ctx) <-chan Event; broker.Broadcast()
                # plus the HTTP handler that bridges them
  server.go    # Run(opts): starts watcher + broker + HTTP, wires them
  handlers.go  # +ServeMux registration for /api/events
```

### Watcher

```go
type Watcher struct {
    path   string
    events chan struct{}
    fsw    *fsnotify.Watcher
}

func NewWatcher(path string) (*Watcher, error) {
    fsw, err := fsnotify.NewWatcher()
    if err != nil { return nil, err }
    if err := fsw.Add(path); err != nil {
        fsw.Close()
        return nil, err
    }
    return &Watcher{path: path, events: make(chan struct{}, 1), fsw: fsw}, nil
}

func (w *Watcher) Run(ctx context.Context) {
    defer w.fsw.Close()
    var debounceTimer *time.Timer
    fire := func() {
        select {
        case w.events <- struct{}{}:
        default: // drop if a previous event is still unread
        }
    }
    for {
        select {
        case <-ctx.Done():
            return
        case ev := <-w.fsw.Events:
            if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
                continue
            }
            if debounceTimer != nil { debounceTimer.Stop() }
            debounceTimer = time.AfterFunc(200*time.Millisecond, fire)
        case <-w.fsw.Errors:
            // log + continue; non-fatal
        }
    }
}

func (w *Watcher) Events() <-chan struct{} { return w.events }
```

`fsnotify.Create` and `Rename` matter because atomic write
(via temp + rename, ADR 0001 §D4) shows up as Rename on most
platforms.

Re-arming the watch after a rename: on platforms where the
watched inode is replaced by the rename target, the watcher
needs to call `fsw.Add(path)` again. Implementation note: after
each `Rename`/`Create` event, re-call `fsw.Add(path)` ignoring
`ErrExist`. Capture this in tasks.

### SSE broker

```go
type Event struct{} // empty — clients refetch on receipt

type Broker struct {
    mu       sync.Mutex
    clients  map[chan Event]struct{}
}

func NewBroker() *Broker { return &Broker{clients: map[chan Event]struct{}{}} }

func (b *Broker) Subscribe() (chan Event, func()) {
    ch := make(chan Event, 1)
    b.mu.Lock()
    b.clients[ch] = struct{}{}
    b.mu.Unlock()
    return ch, func() {
        b.mu.Lock()
        delete(b.clients, ch)
        b.mu.Unlock()
        close(ch)
    }
}

func (b *Broker) Broadcast() {
    b.mu.Lock()
    defer b.mu.Unlock()
    for ch := range b.clients {
        select {
        case ch <- Event{}:
        default: // client is slow; drop the event (next one will catch them up)
        }
    }
}
```

### SSE handler

```go
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    fmt.Fprintf(w, "retry: 2000\n\n")
    flusher.Flush()

    ch, unsubscribe := s.broker.Subscribe()
    defer unsubscribe()

    heartbeat := time.NewTicker(30 * time.Second)
    defer heartbeat.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case <-ch:
            fmt.Fprintf(w, "event: board-changed\ndata: \n\n")
            flusher.Flush()
        case <-heartbeat.C:
            fmt.Fprintf(w, ": ping\n\n")
            flusher.Flush()
        }
    }
}
```

### Wiring in `Run`

```go
func Run(opts Options) error {
    ln, port, err := bindPort(opts.Port)
    if err != nil { return err }
    fmt.Fprintf(os.Stdout, "→ Ezida viewer running at http://127.0.0.1:%d\n", port)

    w, err := NewWatcher(opts.Board)
    if err != nil { return err }
    broker := NewBroker()
    s := &server{boardPath: opts.Board, broker: broker}
    mux := s.routes()
    srv := &http.Server{Handler: mux}

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    go w.Run(ctx)
    go func() {
        for range w.Events() { broker.Broadcast() }
    }()

    if !opts.NoOpen { _ = Open(fmt.Sprintf("http://127.0.0.1:%d", port)) }

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
}
```

### Client (Alpine + EventSource)

```js
// inside board()
connected: false,
connectEvents() {
  const es = new EventSource('/api/events');
  es.addEventListener('board-changed', () => this.handleExternalChange());
  es.onopen = () => { this.connected = true; };
  es.onerror = () => { this.connected = false; /* browser will retry */ };
  this.eventSource = es;
},
handleExternalChange() {
  if (this.editing) this.closeCard();
  this.load();
},
```

`init` is updated to call `connectEvents()` after `load()`.

### Modal closes on external change

`closeCard()` (from V3) is the only modal-close path; calling it
from `handleExternalChange` is enough. No toast in this phase
(deferred polish).

## Risks / Trade-offs

- **Cross-platform watch quirks**: macOS FSEvents coalesces
  differently than Linux inotify. The 200 ms debounce + re-add
  after Rename should cover both. Tests live with this risk;
  any persistent issue surfaces during manual smoke.
- **Watcher needs the file to exist at startup**: `ezida serve`
  is only usable in a project with a `kanban.toml`. If the user
  runs `serve` and then `init`, the watcher's initial `Add` fails.
  Decision: `server.Run` returns the error to the CLI, which
  exits with a `BOARD_NOT_FOUND`-style message. The user must
  `init` first. Documented in the V4 README delta (out of scope
  here, captured as TODO).
- **Self-write storm**: every viewer write triggers a watcher event
  that broadcasts to every client (including the originator),
  causing a redundant refetch. One extra HTTP round-trip per
  write. Negligible on localhost.
- **EventSource on Safari**: Safari historically had quirks with
  long-lived EventSource (timeouts at certain idle intervals).
  The 30 s heartbeat is the mitigation. Acceptable for v1.
- **No reconnect backoff customization**: relying on the browser's
  default + `retry: 2000`. If the server is restarted with a
  different port (fallback chose 7778), the client reconnects to
  the original port and fails forever. Acceptable in v1 since
  `serve` doesn't normally restart mid-session.

## Migration Plan

Not applicable. New endpoints, no existing user state to migrate.

## Open Questions

- Whether to log watcher errors to stderr. Decision: yes, prefixed
  with `watcher:`, no rate limiting in v1.
- Whether to surface "external change" as a toast in the UI.
  Decision: deferred. The connection-status dot is enough signal
  in v1; toasts land with the polish phase that's not in this
  batch.
