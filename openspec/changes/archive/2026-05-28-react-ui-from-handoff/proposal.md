## Why

The hand-validated React mockup at `refs/handoff/web/` is the new UI
direction: editorial typography, single-source-of-truth color tokens
derived from `bg-base`, polished card-detail modal, theme toggle, filter
popover. The current Alpine+Sortable.js UI at `internal/server/web/` is
functionally complete but visually and behaviorally diverged from the
validated design. Ship the new UI on top of the existing Go server
without changing the REST contract.

## What Changes

- Replace `internal/server/web/{index.html, style.css, app.js}` with
  a React 18 implementation derived from `refs/handoff/web/`. Files
  in `internal/server/web/` become `{index.html, styles.css, app.jsx}`.
- Vendor `react.production.min.js`, `react-dom.production.min.js`, and
  `babel.min.js` under `internal/server/web/vendor/`. **No CDN at
  runtime** — same offline-friendly policy as the existing Alpine
  vendor folder. **No build step**: JSX is transpiled in the browser
  by Babel-standalone, matching the mockup's stated zero-bundler
  constraint.
- Remove vendored Alpine.js and Sortable.js (no longer used). HTML5
  drag-and-drop replaces Sortable.
- Adapter layer in `app.jsx` translates between the server wire shape
  (`{columns[], cards[{column, title, ...}]}`) and the mockup's
  internal shape (`{title, lists:[{id=column-name, title=display,
  cards:[{text=title,...}]}]}`). All mutations call the existing
  granular REST endpoints (`POST /api/cards`, `POST
  /api/cards/{id}/move`, `PATCH /api/cards/{id}`, `DELETE
  /api/cards/{id}`, `POST /api/columns`, `POST /api/columns/move`,
  `PATCH /api/columns/{name}`, `DELETE /api/columns/{name}`).
- Subscribe to `/api/events` (SSE) and refetch `/api/board` on
  `board-changed`. Topbar connection-status indicator (online/offline)
  reflects EventSource state.
- Drop board-title editing from the mockup. Top-bar brand binds to
  the server-provided `project_name` (read-only).
- Drop the mockup's REDA-prefix card-id rebranding. IDs come from the
  server.
- Drop hardcoded mockup `PRIORITIES` (oklch). Priorities and their
  colors come from `/api/board.priorities` + `priority_colors`.
- Column-rename strictness preserved: server validation errors
  surface inline; no slugification.
- Update `site/demo/demo-shim.js` so the in-memory demo continues to
  back the new UI through the same intercepted REST endpoints.
- **BREAKING (UI tests)**: UI-level tests in `internal/server/`
  asserting Alpine DOM markup (`x-data`, Sortable, specific class
  names tied to the old structure) need to be updated or removed.
  REST-level tests are untouched.

## Capabilities

### New Capabilities
*(none)*

### Modified Capabilities
- `viewer-ui`: complete UI rewrite from Alpine to React. Design
  tokens, color ramp, typography classes, drag-and-drop semantics,
  card-detail modal, filter popover, theme toggle, SSE subscription,
  topbar brand-binding, vendoring policy — all re-stated against the
  React implementation. Sortable.js + Alpine.js requirements removed,
  replaced by React + Babel-standalone + ReactDOM vendoring
  requirements.
- `demo-viewer`: shim must intercept the same REST endpoints used by
  the new React UI and dispatch `board-changed` on mutation. Asset
  symlinks updated from `{app.js, style.css}` to `{app.jsx,
  styles.css}`.

## Impact

- **Code**: `internal/server/web/**` (full rewrite), `site/demo/**`
  (shim + symlink updates), `internal/server/{embed.go, handlers.go}`
  (serve `app.jsx` with correct Content-Type), `internal/server/*_test.go`
  (UI-level test updates).
- **APIs**: no change. Existing endpoints + JSON shapes preserved.
- **Dependencies**: drop `vendor/alpine.min.js`, `vendor/sortable.min.js`.
  Add `vendor/react.production.min.js`,
  `vendor/react-dom.production.min.js`, `vendor/babel.min.js`.
- **Storage**: no change (`kanban.toml` remains source of truth).
- **CLI commands**: unchanged.
