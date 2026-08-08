## 1. HTTP wire

- [x] 1.1 Add `Epic string` and `Color string` (both `json:"...,omitempty"`) to `cardResponse` in `internal/server/handlers.go`
- [x] 1.2 Populate both in `cardToResponse`
- [x] 1.3 Add `DoneColumns []string` (`json:"done_columns"`) to `boardResponse`, always non-nil so it renders `[]` rather than `null`
- [x] 1.4 Populate `done_columns` in `handleBoard` from the board's terminal-column set, emitting bare names
- [x] 1.5 Assert in a test that no response field contains a `*` character on a board with terminal columns
- [x] 1.6 Assert in a test that a board with no epics produces a payload with no `epic` or `color` keys
- [x] 1.7 Assert in a test that no card object carries `epic_title`, `epic_color`, `children`, or `progress`
- [x] 1.8 Update the existing `server_test.go` assertions that enumerate top-level envelope keys, and the ones asserting `schema_version` equals 1

## 2. Export parity

- [x] 2.1 Add `Epic` and `Color` with the same `omitempty` tags to `output.ExportCard` in `internal/output/json.go`
- [x] 2.2 Add `done_columns` to the export envelope struct
- [x] 2.3 Populate both in `runExport` in `internal/commands/export.go`
- [x] 2.4 Add a test asserting `ezida export` and `GET /api/board` produce the same top-level key set and the same per-card key set for a board containing an epic

## 3. Adapter

- [x] 3.1 Carry `epic` and `color` through `toUiBoard` onto the UI-shape card in `internal/server/web/app.jsx`
- [x] 3.2 Derive each list's `done` flag from membership in `done_columns`
- [x] 3.3 Build an `id → card` Map once per board load inside the adapter
- [x] 3.4 Expose, from that Map: a card's parent card, a parent's children in payload order, and a parent's `done`/`total` counts
- [x] 3.5 Make an unresolvable `epic` id yield no parent rather than throwing
- [x] 3.6 Ensure the derived counts read the full payload, so an active filter never changes them

## 4. Card rendering

- [x] 4.1 Render the epic chip in `.card-foot` as the first element after the priority pill, before the tags
- [x] 4.2 Add the four-square epic glyph as an `Icon` component alongside the existing icon set
- [x] 4.3 Style `.card-epic-chip` in `styles.css`, deriving background, border, and text from the parent's hex via `color-mix` toward `--text`, with per-theme ratios mirroring `.card-tag-chip`
- [x] 4.4 Apply a `max-width` with ellipsis truncation and set the full parent title as the `title` attribute
- [x] 4.5 Render no chip when the card has no `epic` or when the referenced parent is absent
- [x] 4.6 Verify the tag chips' styling is byte-unchanged

## 5. Parent card rendering

- [x] 5.1 Render the epic glyph before the title on any card with at least one child
- [x] 5.2 Tint the card border from its own `color` via `color-mix` against `--border`
- [x] 5.3 Render the progress bar and the `done/total` counter, the counter in the mono face with tabular numerals
- [x] 5.4 Gate all three signals on having children, not on carrying a `color`
- [x] 5.5 Render an empty bar rather than no bar when `done_columns` is empty

## 6. Column header

- [x] 6.1 Render `IconCheck` between the list title and the count for any column in `done_columns`
- [x] 6.2 Style it with a muted foreground token so it reads as metadata
- [x] 6.3 Give it an accessible label identifying the column as terminal
- [x] 6.4 Confirm the `*` marker never reaches the DOM

## 7. Detail modal

- [x] 7.1 Render a labelled parent row — chip plus id — on a card carrying an `epic`
- [x] 7.2 Render a children list with each child's title and column, plus the progress bar and counter, on a card with children
- [x] 7.3 List children in payload order
- [x] 7.4 Render neither section on a card that is neither a child nor a parent
- [x] 7.5 Confirm no add, remove, reassign, or color control is present in either section
- [x] 7.6 Keep the section containers shaped so the next change can drop the picker and swatches into them

## 8. Demo and docs

- [x] 8.1 Regenerate `site/demo/board.json` with `ezida export` from a board that actually contains an epic, so the public demo shows the feature
- [x] 8.2 Confirm `site/demo/app.jsx` and `site/demo/styles.css` are still symlinks and inherit the rendering with no edit
- [x] 8.3 Add the epic-rendering entries to the "What the Web UI lets you do" list in `docs/usage.md`

## 9. Verification

- [x] 9.1 Run `gofmt -l .` and `go vet ./...` clean
- [x] 9.2 Run `go test ./...` green
- [x] 9.3 Load a real board with an epic in `ezida serve` and confirm chip, parent card, column marker, and both modal sections render in light and dark themes
- [x] 9.4 Confirm a board with no epics renders identically to the previous build, by screenshot comparison
- [x] 9.5 Confirm drag, reorder, inline edit, filter, and delete are all unaffected
