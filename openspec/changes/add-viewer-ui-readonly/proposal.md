## Why

`add-viewer-server-skeleton` ships the HTTP surface but no real UI —
`web/index.html` is a placeholder, the page renders a heading and
nothing else. This change drops the actual read-only board view:
columns rendered horizontally, cards stacked vertically, priority
badges, tag chips. No interaction yet (no drag, no edit), but the
browser becomes useful: you can see your board.

Visual design is deliberately minimal in this phase (per the user's
"design will come later, stay minimal" instruction). A future
polish change layers theming, animations, and refined styling on
top of the structural HTML/CSS landed here.

## What Changes

- Replace `internal/server/web/index.html` placeholder with the real
  page: `<head>` linking `/static/style.css`, vendored Alpine,
  `/static/app.js`; `<body>` containing the top-bar shell and the
  Alpine root element that renders the board.
- Replace empty `internal/server/web/app.js` with the read-only
  Alpine component: fetch `/api/board` on init, expose
  `columns`, `priorities`, `cards`, and a `cardsByColumn(name)`
  helper. No event wiring yet.
- Replace empty `internal/server/web/style.css` with the minimal
  layout stylesheet: horizontal column row, vertical card stack
  inside each column, priority badge swatches (3 grayscale
  variants — color polish deferred), tag chip basics, top-bar
  thin row. No theme tokens, no media queries, no transitions.
- Add `internal/server/web/vendor/alpine.min.js` (vendored copy of
  Alpine.js, ~15 KB). Sortable.js is deferred to V2.
- Update `GET /` to set `Content-Type: text/html; charset=utf-8`
  explicitly (already done in V1) and confirm the page references
  vendored assets via `/static/vendor/alpine.min.js` and
  `/static/app.js`.

## Capabilities

### New Capabilities

- `viewer-ui`: the embedded web page that renders the board.
  Covers HTML structure, CSS layout primitives, vendored JS
  dependencies, the Alpine component contract (data shape +
  lifecycle hooks), and the UX contracts for read-only display
  (priority visual mapping, tag chips, empty column placeholder,
  card-count badge per column).

### Modified Capabilities

None. `viewer-server` is unchanged: it served the placeholder
before, serves the real page now, same routes.

## Impact

- `internal/server/web/` becomes substantive (~80 KB total once
  Alpine is vendored).
- Binary size grows by the vendored Alpine bundle.
- No Go-side change beyond confirming that `GET /static/vendor/*`
  resolves via the existing embed FS (no code change expected).
- No new external dependencies.
- The browser opened by `ezida serve` is now useful as a viewer
  even though no editing works yet.
