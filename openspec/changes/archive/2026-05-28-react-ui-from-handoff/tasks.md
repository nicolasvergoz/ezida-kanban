## 1. Vendor React, ReactDOM, Babel

- [x] 1.1 Download `react.production.min.js@18.3.1` to `internal/server/web/vendor/react.production.min.js`
- [x] 1.2 Download `react-dom.production.min.js@18.3.1` to `internal/server/web/vendor/react-dom.production.min.js`
- [x] 1.3 Download `babel.min.js@7.29.0` (standalone) to `internal/server/web/vendor/babel.min.js`
- [x] 1.4 Delete `internal/server/web/vendor/alpine.min.js`
- [x] 1.5 Delete `internal/server/web/vendor/sortable.min.js`
- [x] 1.6 Verify `internal/server/web/vendor/fonts/` is untouched (still vendored)

## 2. Replace web assets

- [x] 2.1 Copy `refs/handoff/web/index.html` → `internal/server/web/index.html`. Replace CDN `<script>` tags with `/static/vendor/...` references. Replace Google Fonts `<link>` tags with vendored font CSS (or inline `@font-face` in `styles.css`).
- [x] 2.2 Copy `refs/handoff/web/styles.css` → `internal/server/web/styles.css`. Inline `@font-face` rules pointing at `/static/vendor/fonts/`. Delete the old `internal/server/web/style.css`.
- [x] 2.3 Copy `refs/handoff/web/app.jsx` → `internal/server/web/app.jsx`. Apply the following edits in-place:
  - Remove `STORAGE_KEY`, `loadState`, `saveState` (localStorage).
  - Remove `CARD_PREFIX`, `nextCardId`, `uid` (server provides ids).
  - Remove `EditableText` usage on `board.title`; render `project_name` (read-only) instead, with `.toUpperCase()` on display only.
  - Replace hardcoded `PRIORITIES` with values resolved from `/api/board.priorities` + `/api/board.priority_colors`.
  - Add `toUiBoard(server)` adapter at top of file converting server-shape → mockup-shape (list id = column name, card.text = card.title, createdAt/updatedAt aliasing).
  - Replace every state mutation (`addCard`, `editCard`, `patchCard`, `removeCard`, `toggleTag`, `moveCard`, `addList`, `renameList`, `removeList`, `moveList`) with an async function that calls the matching REST endpoint, applies optimistic local update, and reconciles via SSE refetch on failure.
  - Add `useEffect` mount that fetches `/api/board`, calls `toUiBoard`, and sets state.
  - Add `useEffect` mount that opens `EventSource("/api/events")` and refetches `/api/board` on each `board-changed` event (debounced via `requestIdleCallback` or 50ms timeout).
  - Wire `ServerStatus` to the `EventSource` readyState ("online" while open, "offline" while connecting/errored). Remove the hardcoded `status="online"` and the "Storage" / "Version" rows in the popover (or update them to surface live values from a `/api/board` response — keep "Storage" as `kanban.toml`, drop version).
  - Delete the localStorage backfill on load and the `useEffect(() => saveState(board), [board])`.
- [x] 2.4 Delete `internal/server/web/app.js`.

## 3. Server-side glue

- [x] 3.1 In `internal/server/server.go` (or wherever MIME registration belongs), call `mime.AddExtensionType(".jsx", "application/javascript")` once at server init so `http.FileServerFS` serves `.jsx` with a JS Content-Type.
- [x] 3.2 Confirm `internal/server/embed.go` `//go:embed` directive still picks up `web/**` including the new `app.jsx` and `vendor/*.js` (no path changes expected; verify via `go build`).
- [x] 3.3 If `handlers.go` `handleIndex` references `style.css` or `app.js` by name (it shouldn't — it serves `index.html`), update accordingly.

## 4. Tests audit

- [x] 4.1 `grep -rn 'alpine\|sortable\|x-data\|x-init' internal/server/` and update or delete tests that asserted those.
- [x] 4.2 `grep -rn 'data-card-id\|data-column' internal/server/` and check the new React markup carries equivalent test hooks; add them if missing (cheap to add `data-card-id` / `data-column` attributes on the React elements so tests can keep their assertions).
- [x] 4.3 Run `go test ./...`. For each failing UI-level assertion: either fix the markup to satisfy it, or update the test to the new expectation, or delete the test if it was testing Alpine-specific behavior with no React analogue.
- [x] 4.4 REST-level tests (`handlers_*_test.go`, `sse_test.go`, `server_test.go` non-UI portions) MUST continue to pass without modification. If any break, treat it as a regression in the server-side glue, not a test problem.

## 5. Demo

- [x] 5.1 Update `site/demo/` symlinks:
  - `ln -sf ../../internal/server/web/app.jsx site/demo/app.jsx`
  - `ln -sf ../../internal/server/web/styles.css site/demo/styles.css`
  - `rm site/demo/app.js site/demo/style.css` (if those symlinks still exist)
  - Confirm `site/demo/vendor` symlink still resolves to `internal/server/web/vendor/` (it should).
- [x] 5.2 Update `site/demo/index.html` `<script>` and `<link>` tags so they reference the new vendored React/Babel files and the new stylesheet name. Keep `demo-shim.js` loaded BEFORE Babel/React (it monkey-patches `fetch` and `EventSource` which the React app calls on mount).
- [x] 5.3 Verify `site/demo/demo-shim.js` already intercepts every endpoint the new UI calls (`GET /api/board`, `POST /api/cards`, `POST /api/cards/{id}/move`, `PATCH /api/cards/{id}`, `DELETE /api/cards/{id}`, `POST /api/columns`, `POST /api/columns/move`, `PATCH /api/columns/{name}`, `DELETE /api/columns/{name}`, `GET /api/events`). If the new UI uses an endpoint the shim doesn't intercept, add it.
- [x] 5.4 If the `pages.yml` workflow references `style.css` or `app.js` by name, update.

## 6. Verify

- [x] 6.1 `go build ./...` succeeds.
- [x] 6.2 `go test ./...` passes (or every failure is an intentional spec change covered by tasks 4.x).
- [x] 6.3 Start the server in the project root: `go run ./cmd/ezida server` (or whatever the start command is). Open the URL printed to stdout in a browser. Confirm:
  - Topbar brand shows the project_name in uppercase.
  - Columns render with the cards from `kanban.toml`.
  - Theme toggle (light/system/dark) works and persists across reload.
  - Filter popover opens, filters cards, clears.
  - Click a card → modal opens with title, priority dot, column, description, tags, created/modified footer.
  - Edit title (commit on Enter / blur) → reflects in board after refresh.
  - Drag a card to another column → card moves; reload confirms persistence in `kanban.toml`.
  - Open the same server in a second tab, mutate in tab A → tab B updates via SSE within ~1s.
- [x] 6.4 If a step in 6.3 cannot be exercised in the runtime environment, explicitly state which step in the final report rather than claiming success.
