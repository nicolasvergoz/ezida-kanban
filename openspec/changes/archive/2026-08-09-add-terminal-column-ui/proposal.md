## Why

Marking a column terminal is the one piece of board taxonomy that is
still CLI-only (`ezida columns done|undone`). The viewer already
*reports* the state — a check mark sits in the header of every terminal
column since `add-epics-wire-viewer` — and epic progress bars are read
straight off it, so a user who sees a wrong progress count has no way to
fix it from the page they are looking at. Every neighbouring operation
(add, rename, reorder, delete a column) is already reachable from the
list header.

## What Changes

- `PATCH /api/columns/{name}` accepts an optional `done` boolean
  alongside the existing `name`, and both keys become optional: a body
  may rename, may toggle the terminal marker, or may do both in one
  write. A body carrying neither is rejected.
- The inline rename input in the list header gains a clickable check
  beside it. Toggling it stages the intent; the commit that ends the
  rename sends **one** `PATCH` carrying both keys — there is no
  intermediate state in which the name landed but the marker did not.
- The `⋯` list menu — today a single `Delete list` entry — gains a
  `Terminal column` toggle, so the marker is reachable without first
  entering a rename.
- The demo shim (`site/demo/demo-shim.js`) replays the new field, so the
  control is not dead on the public demo page.
- No change to the resting check mark, to `done_columns` on the wire, or
  to the CLI. The `*` suffix stays a file-format detail.

## Capabilities

### New Capabilities

None — this extends surfaces that already exist.

### Modified Capabilities

- `viewer-server`: `PATCH /api/columns/:name` gains an optional `done`
  field and makes `name` optional; a body with neither key is
  `INVALID_BODY`.
- `viewer-ui`: the list-header rename gains a terminal-column check that
  commits in the same request; the `⋯` menu gains a terminal-column
  toggle.
- `demo-viewer`: the in-memory mutation set covers toggling a column's
  terminal status.

## Impact

- `internal/server/handlers.go` — `columnRenamePayload`,
  `handleColumnRename`.
- `internal/server/web/app.jsx` — `EditableText` (its only caller is the
  list title), `ListMenu`, `renameList` and the `List` header; the UI
  list shape already carries `done`.
- `internal/server/web/styles.css` — the staged/actionable check state
  next to the rename input and the menu row.
- `site/demo/demo-shim.js` — the `PATCH /api/columns/<old>` branch.
- Tests: `internal/server/*_test.go` for the endpoint, a new
  `e2e/terminal-column.spec.ts` for the two UI paths.
- No schema change, no migration, no CLI change, no wire-shape change to
  `GET /api/board`.
