## Context

The Ezida CLI ships a Go HTTP server (`internal/server`) that embeds
its web assets via `//go:embed internal/server/web/**` and serves a
single-page UI plus a JSON REST + SSE API backed by `kanban.toml`.

The current UI is **Alpine.js + Sortable.js**, vendored locally
(`internal/server/web/vendor/{alpine.min.js, sortable.min.js}`). It
implements every required behavior (drag, modal, filter, theme, SSE
refetch) but visually diverged from the validated design.

The validated design is delivered as a React 18 prototype in
`refs/handoff/web/` (`index.html` + `styles.css` + `app.jsx`). It uses
`localStorage` as its data source and a slightly different state shape
than the server returns. The HANDOFF doc proposed a backend rewrite to
`PUT /api/board` whole-state replace — we explicitly reject that. We
keep the granular REST surface intact and adapt the React UI to it.

The Go server endpoints are unchanged. The only server-side touch is
`embed.go` / `handlers.go` adjustments to serve a `.jsx` file with the
correct Content-Type so Babel-standalone picks it up.

## Goals / Non-Goals

**Goals:**
- Ship the validated React UI byte-for-pixel close to the mockup.
- Preserve the existing REST + SSE contract.
- Preserve the offline-friendly "no CDN at runtime" policy by
  vendoring React, ReactDOM, and Babel-standalone locally.
- Preserve the "no bundler, no Node build step" constraint: JSX is
  transpiled in the browser.
- Keep the demo (`site/demo/`) functional via its existing
  fetch/EventSource monkey-patch shim.

**Non-Goals:**
- Backend rewrite (`PUT /api/board`, JSON-on-disk replacement of
  `kanban.toml`, etc.) — explicitly rejected.
- Editable board title — `project_name` stays read-only, derived
  from the working directory.
- Compile-time JSX (a Node build step, esbuild, etc.) — rejected to
  preserve the zero-bundler invariant.
- CDN-loaded React / Babel at runtime — rejected to preserve offline
  use.

## Decisions

### D1. Vendor React + Babel-standalone; transpile in browser
**Choice:** ship `react.production.min.js`, `react-dom.production.min.js`,
`babel.min.js` (Babel v7.x standalone) under
`internal/server/web/vendor/`. `index.html` loads each from `/static/vendor/…`.
`app.jsx` is loaded with `<script type="text/babel" src="…">`.

**Why:** matches the mockup as-shipped (no JSX compile step), keeps
the embed manifest simple, and preserves offline behavior. Babel
standalone is ~3 MB minified — acceptable for a local-only tool.

**Alternatives rejected:**
- *htm + Preact* (~10 KB, no Babel): would require rewriting the
  mockup's JSX into `html\`...\`` template literals. Diverges from
  the validated source.
- *Pre-compile JSX → JS* via a `go generate` step: violates the
  "no build step" constraint and complicates contribution.
- *CDN React + Babel*: blocks offline use of the local CLI tool.

### D2. Adapter layer in `app.jsx` instead of mockup-shape on disk
The server returns:
```
{
  schema_version: 1,
  columns: ["backlog","todo","ongoing","done"],
  priorities: ["low","medium","high"],
  priority_colors: { low: "#22c55e", medium: "#f59e0b", high: "#ef4444" },
  cards_per_column: { backlog: 2, ... },
  cards: [
    { id, title, column, priority, tags, description, created_at, updated_at },
    ...
  ],
  project_name: "ezida-kanban"
}
```

The React mockup wants:
```
{
  title: "Redacto",
  lists: [
    { id: "l1", title: "BACKLOG",
      cards: [{ id, text, tags, priority, description, createdAt, updatedAt }, ...]
    }, ...
  ]
}
```

**Choice:** add `toUiBoard(server)` / `toServerCard(ui)` adapters in
`app.jsx`. List `id` IS the column name (server identifier). List
`title` is `name.toUpperCase()` for display only. Card `text` is the
server `title`. Field aliases (`createdAt`↔`created_at`,
`updatedAt`↔`updated_at`) are handled in the adapter. Wire calls
always go in server shape; UI state is always in mockup shape.

**Why:** the mockup React tree is large and well-tested visually.
Editing every component to consume snake_case keys would touch every
file in the patch and risk visual regressions. The adapter is small
(~30 lines), local, and changes only one boundary.

### D3. Mutation strategy: granular REST + SSE-driven refetch
- **Create card** → `POST /api/cards` (body: `{ column, title }`).
- **Patch card** (title, description, priority, tags) →
  `PATCH /api/cards/{id}` (single-key patches; server already
  validates that pattern).
- **Move card** → `POST /api/cards/{id}/move`
  (body: `{ column, position }`).
- **Delete card** → `DELETE /api/cards/{id}`.
- **Create column** → `POST /api/columns`.
- **Rename column** → `PATCH /api/columns/{name}` (body: `{ name }`).
- **Delete column** → `DELETE /api/columns/{name}`.
- **Reorder column** → `POST /api/columns/move`.

After every successful mutation, the server emits a `board-changed`
SSE event. The UI subscribes once on mount via `EventSource('/api/events')`
and refetches `/api/board` on each event (debounced with `requestIdleCallback`
to coalesce rapid bursts). This is identical to the current Alpine
implementation and keeps cross-tab sync free.

**Optimistic UI for drag:** when a card or column is dragged-and-dropped,
the local state updates immediately; the REST call fires in parallel.
On a non-2xx response, refetch from `/api/board` to reconcile. This
matches the current "drop failure resets the UI from the server"
requirement.

### D4. Drop board-title editing and CARD_PREFIX
The mockup's `EditableText` on `board.title` and its `nextCardId` /
`CARD_PREFIX="REDA"` are removed. The top-bar brand binds to the
server-provided `project_name`; card IDs come from the server (`POST
/api/cards` response) so no client-side ID generation is needed.

### D5. Priorities and priority colors from server
The mockup hardcodes `PRIORITIES = [low, medium, high, urgent]` with
`oklch(…)` swatches. The server exposes whichever priorities the user
configured in `kanban.toml` (typically `low, medium, high`) with their
resolved colors. The UI sources both arrays from
`/api/board.priorities` + `/api/board.priority_colors` and renders the
priority listbox + dots from that. No `urgent`, no hardcoded oklch,
no default fallback list other than the server's own defaults.

### D6. Column rename: strict
The server already enforces column-name uniqueness, non-emptiness, and
the snake_case identifier convention via
`board.{EmptyColumnNameError, ColumnAlreadyExistsError, ...}`. The UI
surfaces these errors inline on the list-header inline editor, reverts
the local title on rejection, and re-fetches on success (SSE will fire
anyway). No client-side slugification, no auto-uppercase coercion on
the wire (display-only `.toUpperCase()` in CSS / JSX).

### D7. Remove Sortable + Alpine
HTML5 drag-and-drop in `app.jsx` (already present in the mockup)
fully replaces Sortable.js. Alpine.js was the entire view layer of
the old UI; React replaces it. Both vendor files and any
`script src` references are removed. Spec requirements that named
them are removed in the delta.

### D8. Demo shim continues to work as-is, with light updates
`site/demo/demo-shim.js` already intercepts `/api/board`,
`/api/cards*`, `/api/columns*`, and `/api/events`. The new UI calls
the **same** endpoints, so the shim continues to work without a
contract change. Two updates only:
1. Symlinks under `site/demo/` need to point at `app.jsx` and
   `styles.css` (was `app.js` and `style.css`).
2. `site/demo/index.html` (which is a thin wrapper around
   `internal/server/web/index.html`) needs its `<script>` tags
   aligned with the new vendor list (React + ReactDOM + Babel
   instead of Alpine + Sortable).

### D9. Serve `.jsx` with a JS Content-Type
Babel-standalone reads `<script type="text/babel" src="app.jsx">`
and fetches the file itself. Browsers tolerate any text MIME for
this case, but to keep the dev console quiet we register `.jsx`
under `mime.AddExtensionType` once at server init, mapped to
`application/javascript`. No handler change is needed because
`http.FileServerFS` already serves the file; we just decorate the
extension table.

## Risks / Trade-offs

- **Babel-standalone size (~3 MB)** → Mitigation: this is a
  local-only tool, the binary is run from the user's machine, no
  cold-start network cost. The minified Babel ships once with the
  Go binary.
- **Babel transpile cost on page load** → Mitigation: a one-time
  ~200 ms hit on first paint. Acceptable for a desktop dev tool.
  If it becomes painful, switch to D2-alt (`htm + Preact`) later.
- **Existing UI tests assert Alpine DOM structure** → Mitigation:
  audit `internal/server/*_test.go`, drop assertions tied to
  `x-data`, Sortable, and Alpine-specific classes. Keep REST-level
  tests. New UI semantics are covered by spec scenarios; we are
  not adding browser-level integration tests in this change.
- **HTML5 drag-and-drop quirks across browsers** → Mitigation: the
  mockup ships polished cross-browser handlers (`dragstart`,
  `dragover`, `drop`, `dragend`, plus a custom `kanban:drag-cleanup`
  event to recover from `stopPropagation` on child cards). Keep
  these patterns intact.
- **Adapter drift if server JSON shape evolves** → Mitigation:
  adapter is ~30 lines in one file. Any new field gets aliased
  there. Document the adapter as the single boundary.

## Migration Plan

1. Land the change behind no flag: it's a full replace.
2. After merge, users running `ezida server` get the new UI on
   first reload. No DB / state migration is needed because we kept
   `kanban.toml` as the source of truth.
3. Rollback path: revert the merge commit. Vendored files and
   spec deltas all go together.

## Open Questions

None at this stage. The four product-level decisions
(stack, API, features, demo) are settled. The four implementation
decisions (rename strictness, board title, old UI fate, demo
strategy) are settled. Anything else is downstream tactical detail
inside `tasks.md`.
