# 0002. Viewer batch — cross-phase decisions

Date: 2026-05-21
Status: Accepted

## Context

This ADR records the cross-cutting decisions for the `viewer` batch, which
adds a web-based viewer/editor to `ezida` via a new `ezida serve`
subcommand. The viewer renders `kanban.toml` as an interactive Kanban
board in the user's browser, drag-and-drop reorders cards, edits cards
inline, and hot-reloads when the file changes externally.

The work is split into five phases that each produce a working,
testable increment (see `refs/VIEWER_BRIEF.md` §10). Phases are strictly
ordered: V1 server skeleton → V1 UI render → V2 drag/move → V3 inline
edit → V4 hot-reload via SSE. The brief's V5 "polish" phase is
deliberately excluded from this batch — the visual design is still
in flux and a spec-driven change is the wrong shape for an iterative
polish pass.

Constraints carried from `refs/PROJECT_BRIEF.md` and ADR 0001 still
apply: single static binary, no runtime network beyond `127.0.0.1`, no
server in the deployed sense (the viewer is launched per-developer per
project), TOML on disk via the existing `internal/board` package.

## Decisions

### D1. Capability split — `viewer-server` + `viewer-ui`

Two new capabilities. `viewer-server` covers the HTTP surface, port
fallback, JSON contract, SSE stream, and file watcher — everything that
can be exercised with `curl` and verified in Go tests. `viewer-ui`
covers the HTML page, embedded vendor JS/CSS, Alpine components,
Sortable.js integration, and UX rules (modals, toasts, keyboard
shortcuts) — verified subjectively in a browser.

**Alternatives considered:**
- Single `viewer` capability: spec grows monolithic; mixing
  protocol-level assertions with UX rules muddies traceability.
- Three-way split (`viewer-api` + `viewer-watch` + `viewer-ui`):
  over-segmented for v1; the watcher and SSE pieces are too small to
  carry their own capability.

**Affects:** all five phases.

### D2. Embed location — `internal/server/web/`

All frontend assets live under `internal/server/web/` rather than a
top-level `web/` directory. The `//go:embed` directive sits in
`internal/server` itself, no upward traversal, no path tricks. This
diverges from the brief's §8 layout, which placed `web/` at the repo
root — adopted for embed simplicity. README and developer docs need to
reflect the actual layout.

**Alternatives considered:**
- Top-level `web/`, embed declared in `cmd/ezida/main.go` and FS
  passed into `internal/server`: extra parameter to thread through,
  inverts the natural package ownership.
- Top-level `web/`, embed declared in a shim file under
  `internal/server`: requires symlink or duplicated path; brittle.

**Affects:** V1 server skeleton (embed wiring), V1 UI (asset layout),
V2/V3/V4 (vendor additions land under `internal/server/web/vendor/`).

### D3. Concurrent-write strategy — last-write-wins, no locking

Both the CLI and the viewer call `board.Save` (atomic temp + rename,
per ADR 0001 §D4). Neither side takes a file lock. If a CLI command
and a viewer write race, the second `Save` clobbers the first; the
losing edit is lost silently. Accepted for v1: single user, single
machine, edits are seconds apart in practice. The watcher (V4)
ensures both surfaces converge on the file's final state within ~1 s.

**Alternatives considered:**
- Optimistic concurrency via mtime/hash `If-Match` header: adds API
  surface and client-side conflict UI for a problem one user rarely
  hits.
- `flock` advisory locks in `board.Save`: cross-platform brittleness
  (rejected in ADR 0001 §D4 for the same reason); also blocks the
  watcher's read.

**Affects:** V1 server (no concurrency primitives added), V2/V3
(POST/PATCH handlers accept whatever state is on disk at write time),
V4 (watcher's "self-write detection" intentionally absent — both
sides treat external changes as authoritative).

### D4. Refactor `AppendCardToColumn` → `InsertCardAt` primitive in V2

V1 leaves `board.AppendCardToColumn` untouched. V2 (drag/reorder)
introduces `board.InsertCardAt(b *Board, c Card, column string, pos int)`
as the core primitive; `AppendCardToColumn` becomes a thin wrapper
that delegates to `InsertCardAt` with `pos = len(cards-in-column)`.
`board.MoveCard(b *Board, id, column string, pos int) error` builds
on the same primitive (remove + InsertCardAt). All three live in
`internal/board/board.go`; the public CLI API of P3
(`add`/`move`) does not change.

**Alternatives considered:**
- Introduce `InsertCardAt` in V1 ahead of need: speculative, no caller.
- Keep `AppendCardToColumn` separate and add `MoveCard` next to it:
  two near-duplicate implementations diverge over time.

**Affects:** V2 (introduces primitive, refactors append, adds move),
V3+V4 (consume `UpdateCard` helper added in V3 alongside the same
pattern).

### D5. Stack — stdlib HTTP + fsnotify + vendored Alpine/Sortable

The viewer adds exactly one Go dependency: `github.com/fsnotify/fsnotify`
for file watching (V4 only). HTTP server, routing, JSON, SSE, and
`html/template` are stdlib. Frontend: vanilla HTML/CSS, Alpine.js
(~15 KB) for reactivity, Sortable.js (~40 KB) for drag-drop —
both vendored as plain `.min.js` files under
`internal/server/web/vendor/` and served via `//go:embed`. No
bundler, no CDN at runtime, no Node toolchain for users or developers.

**Alternatives considered:**
- `gorilla/mux` or `chi` router: stdlib `http.ServeMux` (Go 1.22+
  patterns) is sufficient for ~6 routes.
- React/Vue/Svelte: requires build step, violates "no Node runtime".
- Loading Alpine/Sortable from CDN: introduces runtime network
  dependency, fails in offline contexts (planes, restricted
  networks).

**Affects:** V1 (HTTP server + Alpine vendor), V2 (Sortable vendor),
V4 (fsnotify dep + `go.mod` update).

### D6. HTTP bind — `127.0.0.1` only, port 7777 with auto-fallback +10

Server binds to `127.0.0.1` (never `0.0.0.0`, never a configurable
host in v1). Default port `7777`; on `EADDRINUSE`, try `7778`...`7786`
in order, log the chosen port to stdout, then continue. If all 11
ports fail, exit non-zero with a structured error.
`ezida serve --port=N` overrides the starting port; the fallback
window still spans 11 ports starting at `N`.

**Alternatives considered:**
- Bind on `0.0.0.0` to support LAN access: opens an unauthenticated
  surface, conflicts with §2 of the brief.
- Random port (port 0): unpredictable, breaks the "always open the
  same URL" muscle memory.
- Fail hard on `EADDRINUSE`: irritating when a previous `ezida serve`
  is still shutting down.

**Affects:** V1 (server boot, browser-open URL), V2/V3/V4 (no
re-impact: the port is decided once at start).

### D7. JSON contract — reuse ADR 0001 §D7 envelope, snake_case

The viewer's HTTP API follows the same conventions as the CLI's
`--json` output: snake_case keys, ISO 8601 UTC timestamps, structured
error envelope `{"error":{"code":"<UPPER_SNAKE>","message":"...","details":{...}}}`.
`GET /api/board` returns
`{schema_version, columns, priorities, cards_per_column, cards: [...]}`.
Card objects include the full `description` field (unlike CLI `list`
per ADR 0001 §D7) because the UI shows the description in the edit
modal and we have no second-fetch budget for every modal open.

Error code namespace is shared with the CLI: existing codes
(`COLUMN_NOT_FOUND`, `CARD_NOT_FOUND`, `INVALID_PRIORITY`, etc.) reused
verbatim; new codes added in this batch (`POSITION_OUT_OF_RANGE` in
V2, `MISSING_TITLE` reused in V3) follow the same convention.

**Alternatives considered:**
- Bespoke HTTP API conventions (camelCase, REST-style errors): forces
  a translation layer between two consumers of the same model.
- Omit description from `/api/board`: would save bytes but require a
  second `GET /api/cards/:id` on every modal open; the UI cost
  outweighs the wire cost.

**Affects:** V1 (board endpoint shape), V2 (move endpoint response),
V3 (PATCH response + new partial-update semantics), V4 (board
refetch payload unchanged).

### D8. PATCH semantics — present key replaces, absent key untouched

`PATCH /api/cards/:id` accepts a partial card object. Each key present
in the request body replaces the corresponding field; keys absent are
left untouched on the server. To clear `priority`, send `{"priority":""}`.
To clear all tags, send `{"tags":[]}`. To clear `description`, send
`{"description":""}`. Title is required and cannot be cleared (empty
title fails with `MISSING_TITLE`).

**Alternatives considered:**
- JSON Merge Patch semantics with `null` to clear: works but
  forces clients to distinguish `null` vs missing; empty string is
  natural for human-edited text fields.
- Always require the full card on PATCH (effectively PUT): wastes
  bytes and forces the client to round-trip the entire object.

**Affects:** V3 (PATCH handler implementation + spec).

### D9. SSE — single event type, browser auto-reconnect, server heartbeat

`GET /api/events` returns `text/event-stream` and emits a single event
type `board-changed` with an empty data payload whenever the watcher
fires after debounce. Server sends a `retry: 2000` directive on
connect (browser reconnects at 2 s on disconnect) and emits a
comment-only heartbeat (`: ping\n\n`) every 30 s to keep proxies and
load balancers from killing the connection (defensive — localhost
rarely needs it).

On `board-changed`, the client refetches `GET /api/board` and
re-renders. Any open modal closes without prompting; in-progress
drag aborts silently. A toast `"Board updated externally"` informs
the user.

**Alternatives considered:**
- WebSocket: bidirectional capability we do not need; heavier
  client/server code; SSE is purpose-built for "server pushes
  events".
- Long polling: works but wastes a request cycle per event.
- Differential events (send the diff, not "refetch"): smaller wire
  payload but adds a state-reconciliation bug surface; refetch is
  trivially correct.

**Affects:** V4 (SSE endpoint + watcher + client `EventSource`).

### D10. Watcher debounce — 200 ms, self-write tolerated

`fsnotify` watches `kanban.toml`. On any write event, the watcher
debounces 200 ms (coalesces a burst of events from a single editor
save into one) and then broadcasts `board-changed` to all SSE
clients. The viewer's own writes also trigger the watcher; this is
fine — connected clients refetch and observe the same state they
just produced. No "is this our own write?" detection in v1.

**Alternatives considered:**
- No debounce: editors that save via "write temp + rename" can fire
  3-4 events per save; clients would refetch redundantly.
- Self-write suppression via in-process flag: race-prone (the flag
  must be set before the write completes and unset only after the
  watcher fires; ordering is not guaranteed across goroutines and
  the kernel).

**Affects:** V4 (watcher implementation).

### D11. Position semantics — 0-indexed, clamp out-of-range

`POST /api/cards/:id/move` takes `{column, position}` where `position`
is the 0-indexed slot within the destination column after the move
completes. A `position` of `0` means "first card in the column";
`len(column-cards-after-move)` means "last". Values outside
`[0, len(column-cards-after-move)]` are clamped to the nearest valid
slot rather than rejected — Sortable.js can occasionally report
positions slightly off when columns mutate mid-drag, and the user
expects "drop near the bottom" to land at the bottom, not 400.

`board.MoveCard` and `InsertCardAt` apply the same clamping rule
internally so the CLI helper (introduced in V2 alongside the viewer)
behaves identically.

**Alternatives considered:**
- Reject out-of-range with `POSITION_OUT_OF_RANGE`: surfaces an
  error that the user did not cause; bad UX.
- 1-indexed positions: inconsistent with ADR 0001 §D13's 1-indexed
  `columns add --position`, but consistent with array semantics in
  JS/Go for the slot index. Trade-off: this is a programming-style
  field (array slot), not a human-friendly counter ("the Nth
  column"), so 0-indexed is more honest.

**Affects:** V2 (move endpoint + `MoveCard` + `InsertCardAt`).

### D12. Graceful shutdown — SIGINT/SIGTERM, 5 s drain

The server installs signal handlers for `SIGINT` and `SIGTERM`.
On either signal, it calls `http.Server.Shutdown(ctx)` with a 5 s
context, which stops accepting new connections and waits for
in-flight requests (including SSE streams) to complete. If 5 s
elapses, the server forces close. Stdout prints `Shutting down...`
on signal and `Bye.` on clean exit.

**Affects:** V1 (server boot includes signal wiring), V4 (SSE
streams must close cleanly on shutdown — the watcher's done channel
ensures broadcast goroutine returns).

### D13. CLI surface additions — minimal

Exactly one new subcommand: `ezida serve [--port=N] [--no-open]`.
No `--host`, no `--read-only`, no `--config`, no `--theme` in v1.
The brief's "Goals" sections of every phase already constrain scope;
this decision codifies it as a hard line.

**Alternatives considered:**
- `--host=0.0.0.0` flag for LAN testing: out of scope per D6.
- `--read-only` mode: implementation cost (gate every write
  handler) for a use case (read-only sharing) better served by
  the CLI's `ezida board` output.

**Affects:** V1 (cobra wiring).

## Consequences

**Positive:**
- Two clean capability boundaries (`viewer-server` + `viewer-ui`)
  keep protocol assertions separate from UX rules.
- One Go dependency added across five phases (`fsnotify` in V4 only).
- Frontend assets vendored: zero runtime network, zero build step,
  binary remains self-contained.
- Refactoring `AppendCardToColumn` into `InsertCardAt` lands exactly
  when a second caller appears (V2), avoiding speculative
  generalization.
- Shared JSON conventions (snake_case, error envelope, code
  namespace) mean AI tooling already aware of the CLI's contract
  understands the HTTP API for free.

**Negative / risks:**
- Last-write-wins (D3) can silently lose an edit if a CLI command
  and a viewer save collide within milliseconds. Acceptable for v1;
  revisit if real-world reports surface.
- Brief §8 placed `web/` at the repo root; D2 moves it to
  `internal/server/web/`. README and any future contributor docs
  must reflect the actual layout.
- Port fallback (D6) means the URL is not always `:7777`. The server
  prints the chosen URL on stdout, but users who bookmark the URL
  before the first run may bookmark a stale port.
- SSE broadcast (D9) refetches the full board on every change — for
  large boards this is more bytes than a diff stream would be, but
  diff sync is a state-machine bug surface we are not paying for
  in v1.
- Self-write tolerance (D10) means every viewer write produces a
  redundant refetch on all connected clients (including the
  originator). One extra HTTP round-trip per write per client.
  Negligible at <10 clients on localhost.

**Follow-up:**
- V5 "polish" deferred — handled outside this batch when the design
  intent stabilizes.
- Optimistic concurrency, read-only mode, LAN access, dark mode,
  mobile layout, multi-board navigation: all out of scope per
  brief §11 / §12.

## References

- Brief: `refs/VIEWER_BRIEF.md`
- Foundational ADR: `openspec/decisions/0001-kanban-v1-batch.md`
- Phases: `add-viewer-server-skeleton`, `add-viewer-ui-readonly`,
  `add-card-move-reorder`, `add-card-inline-edit`,
  `add-viewer-hot-reload`
