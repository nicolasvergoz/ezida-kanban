## Why

The viewer can read and rearrange columns but cannot author them.
Today, adding a column, renaming it, deleting an empty one, or
reordering the column strip all require the user to drop to the CLI
(`ezida columns add|rename|rm`). That is friction at the moment the
user is most engaged with the board — staring at it in the browser,
mid-flow, deciding the column model needs a tweak.

Phase 6 of the UI redesign batch closes that gap. It introduces four
column-management endpoints on the HTTP surface, four corresponding
`internal/board` helpers, and the inline affordances the Redacto
design system prescribes for list-level manipulation: a dashed
"Add list" placeholder after the last column, a 3-dots delete menu
on every list header, click-to-rename on the header title, and
drag-to-reorder via the header itself (separate Sortable instance
from card drag).

Last-write-wins (ADR 0002 §D3) carries forward unchanged — CLI and
viewer column edits remain concurrent without new locking. The four
new error codes are the ones minted in ADR 0003 §D9
(`CANNOT_DELETE_LAST_COLUMN`, `COLUMN_HAS_CARDS`,
`COLUMN_ALREADY_EXISTS`); the empty-name case reuses the existing
`INVALID_BODY` envelope rather than minting a new wire code (see
design.md TD2).

This phase is the last increment of the UI redesign batch and the
piece that brings the viewer to authoring parity with the CLI on the
column axis. After this lands, the only column-level operation that
remains CLI-only is bulk reorder via scripted pipelines, which is out
of scope for a UI surface.

## What Changes

- **Server**: four new endpoints under `/api/columns`:
  - `POST /api/columns` — create a column. Body `{"name": "<name>"}`.
    Validates non-empty and no duplicates. Returns 201 with the full
    updated column list.
  - `PATCH /api/columns/:name` — rename a column. Body
    `{"name": "<new-name>"}`. Cascades the rename across every card
    whose `column` field references the old name. Returns 200 with
    the full updated column list and a `renamed: {from, to}` echo.
  - `DELETE /api/columns/:name` — delete a column. Refuses if the
    column does not exist (404 `COLUMN_NOT_FOUND`), if it is the only
    remaining column (400 `CANNOT_DELETE_LAST_COLUMN`), or if it
    contains any cards (400 `COLUMN_HAS_CARDS`). Returns 200 with
    the post-delete column list.
  - `POST /api/columns/move` — reorder a column. Body
    `{"name": "<name>", "position": <int>}`. 0-indexed, clamped to
    `[0, N-1]` per ADR 0002 §D11. Returns 200 with the post-move
    column list.
- **Board helpers** (in `internal/board`, new file
  `internal/board/columns.go`):
  - `AddColumn(b *Board, name string) error` — appends after
    validation. Errors: `*EmptyColumnNameError`,
    `*ColumnAlreadyExistsError`.
  - `RenameColumn(b *Board, from, to string) error` — renames in
    `b.Board.Columns` and rewrites every referencing card's `column`
    field. Errors: `*EmptyColumnNameError`, `*ColumnNotFoundError`,
    `*ColumnAlreadyExistsError`. `from == to` is a no-op success.
  - `DeleteColumn(b *Board, name string) error` — errors:
    `*ColumnNotFoundError`, `*CannotDeleteLastColumnError`,
    `*ColumnHasCardsError`.
  - `MoveColumn(b *Board, name string, position int) error` — moves
    the named column to the clamped new index. Errors:
    `*ColumnNotFoundError`. Cards are not touched.
- **New typed errors** in `internal/board`:
  - `EmptyColumnNameError` → mapped to wire code `INVALID_BODY` (400).
    Rationale in design.md TD2.
  - `ColumnAlreadyExistsError{Name string}` → mapped to
    `COLUMN_ALREADY_EXISTS` (400).
  - `CannotDeleteLastColumnError{Name string}` → mapped to
    `CANNOT_DELETE_LAST_COLUMN` (400).
  - `ColumnHasCardsError{Name string, Cards []affectedCard}` →
    mapped to `COLUMN_HAS_CARDS` (400); `details.cards` carries the
    list of cards (`{id, title}`) currently in the column.
  - Existing `*ColumnNotFoundError` is reused for the
    not-found cases on rename/delete/move (404 for DELETE,
    400 for PATCH/move — matching the existing CARD/COLUMN pattern).
- **UI**: three new affordances on the column strip.
  - **Add-list placeholder** — a 296×48 dashed-border tile rendered
    after the last column. Click swaps it into an inline composer
    (input + Add/Cancel buttons, full-rounded surface, accent
    border). Enter or Add submits `POST /api/columns`. Escape or
    Cancel collapses back to the placeholder. Error response surfaces
    inline below the input.
  - **List-header 3-dots menu** — a button on the right side of every
    list header opens a small popover with one action ("Delete list"
    in danger color). Click → `DELETE /api/columns/:name`. Server
    error displays inline inside the menu (e.g. "Move cards first"
    on `COLUMN_HAS_CARDS`).
  - **Inline list rename** — clicking the list-header title swaps the
    span for an input pre-filled with the current name. Enter or blur
    commits via `PATCH /api/columns/:name` (if non-empty and changed);
    Escape or blur with empty/unchanged reverts. Error surfaces
    inline next to the input.
  - **Drag-reorder columns** — a second Sortable.js instance on the
    `.columns` container with `handle: '.list-header'` and a distinct
    `group` value so it does not interfere with the existing card
    Sortable. On drop, `POST /api/columns/move` fires with the new
    position. The SSE refetch updates state on success; failure
    refetches to revert.

## Capabilities

### New Capabilities

(none — this phase extends three existing capabilities)

### Modified Capabilities

- `viewer-server`: gains four routes
  (`POST /api/columns`, `PATCH /api/columns/:name`,
  `DELETE /api/columns/:name`, `POST /api/columns/move`) and three
  new wire error codes (`COLUMN_ALREADY_EXISTS`,
  `CANNOT_DELETE_LAST_COLUMN`, `COLUMN_HAS_CARDS`) plus a documented
  reuse of `INVALID_BODY` for the empty-name case.
- `viewer-ui`: gains four affordances on the column strip (Add-list
  placeholder + composer, 3-dots delete menu, inline list rename,
  drag-reorder via header). The column Sortable instance is separate
  from the existing card Sortable.
- `board-storage`: gains four helper functions
  (`AddColumn`, `RenameColumn`, `DeleteColumn`, `MoveColumn`) and
  three new typed errors (`EmptyColumnNameError`,
  `ColumnAlreadyExistsError`, `CannotDeleteLastColumnError`,
  `ColumnHasCardsError`). The existing `ColumnNotFoundError` is
  reused. The four helpers operate purely on `*Board` — loading
  and saving stay the caller's responsibility (HTTP handler or CLI).

## Impact

- **Code touched**:
  - `internal/board/columns.go` — new file containing the four
    helpers and the new typed errors (~180 lines).
  - `internal/board/columns_test.go` — new file covering each
    helper's success path and every error path (~250 lines).
  - `internal/server/handlers.go` — four new `handle*` functions
    (`handleColumnCreate`, `handleColumnRename`, `handleColumnDelete`,
    `handleColumnMove`), four new payload structs, four new lines in
    `routes()`, and three new `errors.As` arms in `httpError` for the
    new typed errors (~140 lines).
  - `internal/server/handlers_columns_test.go` — new file covering
    the four endpoints' HTTP envelope, status codes, response shape,
    and on-disk persistence (~300 lines).
  - `internal/server/web/index.html` — Add-list placeholder + composer
    markup after the last column, 3-dots menu button + popover inside
    each `.list-header`, click-to-rename span/input swap, `x-data`
    additions on `board()` for the new state fields (~50 lines).
  - `internal/server/web/app.js` — composer state
    (`composingList`, `listDraft`, `listError`), rename state
    (`renamingColumn`, `renameDraft`, `renameError`), 3-dots menu
    state (`openMenuColumn`, `menuError`), the four `fetch`
    wrappers, and the second Sortable instance mounting in
    `mountListSortable()` (~150 lines).
  - `internal/server/web/style.css` — dashed-border placeholder,
    composer surface, 3-dots icon button, popover surface, danger
    menu item color, inline-rename input styling (~70 lines).
- **APIs / contracts**: four new endpoints; three new wire error
  codes. The `/api/board` response is unchanged. All endpoints obey
  ADR 0001 §D8 / ADR 0002 §D7 (snake_case, error envelope,
  `application/json` content type). New codes match ADR 0003 §D9
  exactly.
- **Dependencies**: none (Sortable.js is already vendored from V2;
  Alpine is already vendored from V1).
- **Tests**: ~12 new unit tests across `internal/board` and
  `internal/server`. `go test ./... && go vet ./...` is the final
  gate.
- **Depends on**: UI-1 (`redesign-tokens-and-chrome`) for the token
  system every new surface consumes, and UI-4
  (`add-inline-card-create-delete`) for the composer state pattern
  (ADR 0003 §D10) reused here for the list composer.

References:

- [ADR 0003 §D9](../../decisions/0003-ui-redesign-batch.md) — error
  codes and JSON envelope.
- [ADR 0003 §D10](../../decisions/0003-ui-redesign-batch.md) —
  composer-as-`x-data` pattern.
- [ADR 0003 §D12](../../decisions/0003-ui-redesign-batch.md) —
  column delete safety rules.
- [ADR 0002 §D3](../../decisions/0002-viewer-batch.md) —
  last-write-wins semantics.
- [ADR 0002 §D11](../../decisions/0002-viewer-batch.md) — position
  clamping convention.
