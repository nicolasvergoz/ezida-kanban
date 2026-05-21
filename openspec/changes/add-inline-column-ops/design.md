## Context

The viewer's column strip has been read-only since V1. Every column
operation (`add`, `rename`, `rm`, no public reorder) has lived on the
CLI under `ezida columns`. The CLI's helpers thread through
`internal/commands/refgroup.go`, which is parameterized to also serve
priorities — that abstraction stays where it is; this phase adds a
direct, board-level API on `internal/board` so the HTTP handlers do
not depend on `internal/commands` (which would invert the import
direction; see `internal/board/board.go` header comment on
`CardNotFoundError`).

Column operations on the HTTP surface need to be small, validated,
and atomic with respect to disk. The handler pattern established by
V2 (`POST /api/cards/:id/move`) and V3 (`PATCH /api/cards/:id`) —
decode → load → mutate → save → respond — applies verbatim. The new
work is:

1. Mint four `internal/board` helpers and their error types.
2. Mount four routes and wire `httpError` to recognise the new
   error types.
3. Land the inline UI affordances per `refs/design.md` §"List",
   §"Add-list placeholder", §"Composers".
4. Add a second Sortable instance that does not collide with the
   existing card instance.

Visual specification: `refs/design.md` §"List", §"Add-list
placeholder", §"Composers". Behavioral specification: ADR 0003 §D9
(error codes), §D10 (composer pattern), §D12 (delete safety).

## Goals / Non-Goals

**Goals:**

- Bring the four column operations (create, rename, delete, move) to
  the HTTP surface with the same JSON envelope and status-code
  conventions as V2/V3.
- Provide a clean board-level API the handlers consume, so the HTTP
  layer stays a thin shell around `internal/board`.
- Surface every server-side refusal inline in the UI (composer error,
  rename error, menu error) — no toast, no modal, no `window.alert`.
- Drag-reorder list headers without breaking card drag.
- Keep last-write-wins (ADR 0002 §D3): no new locks, no client-side
  optimistic versioning.

**Non-Goals:**

- A "force delete" override for non-empty columns. The user moves
  cards first (ADR 0003 §D12).
- An undo / revert flow for any column op. The CLI parity does not
  have one either; v1 stays consistent.
- A bulk reorder endpoint (e.g. accept a full reordered list in one
  request). Move-by-name keeps the wire surface small and matches the
  card-move pattern.
- Touching `internal/commands/columns.go`. The CLI keeps its own
  parameterized `refgroup` helper unchanged. The two paths converge
  on disk via `board.Save`, not in code.
- Recomputing `project_name` on rename. Per ADR 0003 §D4, the
  project name is fixed at server start; column rename never affects
  it.

## Decisions

### TD1. Helpers live in `internal/board/columns.go`, not `internal/commands`

The four new helpers (`AddColumn`, `RenameColumn`, `DeleteColumn`,
`MoveColumn`) land in a new `internal/board/columns.go` file. The
existing `internal/commands/columns.go` keeps the CLI's
`refgroup`-based path unchanged.

Why duplicate effort instead of reusing the existing `refgroup`:
`refgroup` lives in `internal/commands` and depends on
`commands.affectedCard` / `commands.ColumnInUseError`. The HTTP
handler in `internal/server` cannot import `internal/commands`
without crossing the dependency direction the codebase has held
since V2 (board → commands → server, never the reverse). Mirroring
the small surface in `internal/board` is cheaper than refactoring
the CLI's helpers up to a third package.

The two paths still produce byte-identical disk state because both
funnel through `board.Save`, which runs `Validate` before writing.

### TD2. Empty-name case reuses `INVALID_BODY`, not a new `EMPTY_COLUMN_NAME`

`AddColumn("")` and `RenameColumn(_, "")` could either mint a new
wire code `EMPTY_COLUMN_NAME` or reuse the existing `INVALID_BODY`
envelope. We pick **INVALID_BODY**.

Rationale:

- The CLI's empty-name case for `ezida columns add` is rejected by
  Cobra's argument parsing before reaching `runColumnsAdd`, so there
  is no precedent CLI error code to mirror. The closest existing
  precedent is the JSON-decode failure path, which uses
  `INVALID_BODY`.
- ADR 0003 §D9 lists `CANNOT_DELETE_LAST_COLUMN`, `COLUMN_HAS_CARDS`,
  and `COLUMN_ALREADY_EXISTS` as new codes; `EMPTY_COLUMN_NAME` is
  explicitly left as a pick to this design doc. Minting one more
  code for what is functionally "your input body is invalid" inflates
  the wire enum without paying off — the message in
  `error.message` is the actionable signal.
- The board helper still mints a typed `*EmptyColumnNameError` so
  the HTTP layer can map it deterministically; the wire shape stays
  uniform.

The handler's `httpError` arm for `*EmptyColumnNameError` produces:

```json
{"error":{"code":"INVALID_BODY","message":"column name must be non-empty","details":null}}
```

with HTTP 400.

### TD3. Handler validation order

Each new handler follows the same order as the existing
PATCH/move handlers:

1. Decode the JSON body. Decode failure → `*InvalidBodyError`
   (`INVALID_BODY`, 400).
2. Trim and validate the input name(s). Empty after trim →
   `*EmptyColumnNameError` (`INVALID_BODY`, 400).
3. Load the board. Load failure → mapped by `httpError` to the
   existing `BOARD_NOT_FOUND` / `SCHEMA_VERSION_MISMATCH` / etc.
4. Call the board helper. Helper errors propagate verbatim through
   `httpError`.
5. Save. Save failure → `IO_ERROR` (500).
6. Encode the response.

This order means every "is the input shaped correctly" check fires
before any disk I/O — a malformed body never causes a `Load`. The
board mutation is in-memory only; `Save` is the single write.

### TD4. `DELETE /api/columns/:name` status codes split COLUMN_NOT_FOUND from the other refusals

Per spec convention (404 for "the named resource does not exist", 400
for "the request is shaped fine but the operation is refused"):

| Error                        | HTTP | Code                       |
|------------------------------|------|----------------------------|
| `ColumnNotFoundError`        | 404  | `COLUMN_NOT_FOUND`         |
| `CannotDeleteLastColumnError`| 400  | `CANNOT_DELETE_LAST_COLUMN`|
| `ColumnHasCardsError`        | 400  | `COLUMN_HAS_CARDS`         |

The existing handler mapping in `httpError` puts
`ColumnNotFoundError` at 400. **We change that for DELETE without
breaking PATCH `/api/cards/:id`'s contract** by routing through the
same typed error: the HTTP layer always sees the same Go error type.
Two acceptable approaches:

- **A.** Inspect the request method inside `httpError` and pick the
  status based on it. Rejected — couples generic error mapping to
  the route.
- **B.** Add a small wrapper type
  `boardColumnDeleteNotFoundError{*ColumnNotFoundError}` used only
  inside `handleColumnDelete`. Rejected — wrapper-of-wrapper noise
  for one site.
- **C.** Keep `ColumnNotFoundError` at 400 in `httpError` and have
  `handleColumnDelete` short-circuit: it checks the column's
  existence itself before calling `DeleteColumn` and emits a 404
  envelope inline. **Chosen** — the membership check is one loop
  that the handler is already doing in spirit anyway (the helper
  would do the same loop), and the explicit 404 keeps the wire
  contract clear.

The other endpoints (`PATCH /api/columns/:name`, `POST /api/columns/move`)
keep the existing 400 mapping for `ColumnNotFoundError` — those are
"the body referenced a name that does not exist", which matches the
existing PATCH `/api/cards/:id/move`'s 400 mapping for unknown
columns.

### TD5. Rename cascade

`RenameColumn(b, from, to)` performs:

1. `from == to` → return nil (no-op success).
2. `to` trimmed-empty → `*EmptyColumnNameError`.
3. Locate `from` in `b.Board.Columns`. Not found →
   `*ColumnNotFoundError{Column: from}`.
4. Check `to` is not already in `b.Board.Columns` (unless equal to
   `from`, already handled in step 1). Duplicate →
   `*ColumnAlreadyExistsError{Name: to}`.
5. Mutate `b.Board.Columns[idx] = to`.
6. Walk `b.Cards`; for every card whose `Column == from`, set
   `Column = to`. Do **not** refresh `UpdatedAt` on the affected
   cards — a column rename is a board-level rebrand, not a card
   edit. This matches the CLI's `refgroup.rename` behavior (no
   `UpdatedAt` bump there either).
7. Return nil.

The cascade is in-memory only; `board.Save` then writes the whole
file atomically.

### TD6. `MoveColumn` clamping and "no-op" semantics

`MoveColumn(b, name, position)`:

1. Locate `name` in `b.Board.Columns`. Not found →
   `*ColumnNotFoundError`.
2. Compute `target = clamp(position, 0, len(columns)-1)`.
3. If `target == curIdx` → return nil (no-op, no error). The save
   still rewrites the file (same shape) — that is acceptable, the
   call site's contract is "ensure the column ends up at this
   position".
4. Otherwise, slice-out the column at `curIdx`, slice-insert at
   `target`. Standard `append`+`copy` pattern, identical in spirit
   to `InsertCardAt`.

No `UpdatedAt` field exists on the board itself, so nothing else to
refresh.

### TD7. `ColumnHasCardsError` carries affected cards in `details`

The wire envelope for `COLUMN_HAS_CARDS` includes the list of cards
currently in the refused column so the UI can render a helpful
message ("3 cards block this delete" rather than just "column has
cards"). Shape:

```json
{
  "error": {
    "code": "COLUMN_HAS_CARDS",
    "message": "column \"todo\" still has 3 cards; move them first",
    "details": {
      "column": "todo",
      "cards": [
        {"id": "a3f2k9", "title": "Refactor auth"},
        {"id": "b1c4d8", "title": "Write tests"},
        {"id": "c5e7f2", "title": "Update docs"}
      ]
    }
  }
}
```

This matches the existing CLI `COLUMN_IN_USE` payload (board-config
spec §"`ezida columns rm`"). The UI only needs the count for v1
("Move 3 cards first"), but exposing the list keeps the contract
useful for future surfaces (an Undo-style "show me the blockers"
view).

`affectedCard` is duplicated from `internal/commands` as a small
struct local to `internal/board/columns.go`:

```go
type affectedCard struct {
    ID    string `json:"id"`
    Title string `json:"title"`
}
```

### TD8. List Sortable instance vs card Sortable instance

Two Sortable instances live on the page after this phase:

| Instance              | Mount target          | Handle           | `group`        | Reorder fires            |
|-----------------------|-----------------------|------------------|----------------|--------------------------|
| Cards (V2, unchanged) | `.cards` (one per column) | card body  | `'cards'`      | `POST /api/cards/:id/move`|
| Lists (new in UI-6)   | `.columns`            | `.list-header`   | `'lists'`      | `POST /api/columns/move`  |

Why this does not collide:

- Different mount target (`.columns` vs `.cards`) — Sortable scopes
  drag detection to its container; pointer events on a `.card`
  inside `.cards` are claimed by the card instance first.
- Different `group` value — Sortable refuses to accept a dragged
  element if its group does not match the destination's group, so a
  card cannot accidentally be dropped onto the list-container, and
  vice versa.
- Different handle — the list instance only initiates a drag from
  `.list-header`. A pointer-down on a card or anywhere inside
  `.cards` cannot start a column drag.

The list Sortable is mounted in a new `mountListSortable()` method,
called from the existing `$nextTick` after the board renders. The
existing `mountSortable()` (for cards) is unchanged.

Teardown: a new `_listSortable` field holds the single instance;
`mountListSortable` destroys the prior instance (if any) before
creating a new one, mirroring `mountSortable`'s pattern. Both teardowns
fire on every `load()` refetch.

### TD9. Add-list placeholder + composer state machine

The placeholder and composer live on the same DOM position (after
the last column). State on `board()`:

- `composingList: false` — when true, render the composer; when
  false, render the dashed placeholder.
- `listDraft: ''` — the input's bound value.
- `listError: ''` — server-side error message, rendered inline below
  the input.

Transitions:

- Click placeholder → `composingList = true; listDraft = ''; listError = ''`.
  After Alpine flushes DOM, focus the input via `$nextTick`.
- Enter key in input → `submitNewList()`.
- Add button click → `submitNewList()`.
- Escape key in input → `cancelNewList()`.
- Cancel button click → `cancelNewList()`.
- Click outside the composer surface → leave composer open (no
  outside-click dismissal in v1; matches UI-4's card composer
  behavior so the two patterns stay consistent).

`submitNewList()`:

1. Trim `listDraft`. If empty → `listError = "name required"`, stay
   open.
2. `POST /api/columns` with `{"name": trimmed}`.
3. On 2xx → close composer (`composingList = false; listDraft = '';
   listError = ''`). The SSE refetch picks up the new column.
4. On non-2xx → `listError = err.message || "HTTP <status>"`, stay
   open.

`cancelNewList()`: `composingList = false; listDraft = ''; listError = ''`.

### TD10. Inline list rename state machine

State on `board()` (one rename at a time across the page):

- `renamingColumn: null` — the column name currently being renamed,
  or `null` when no rename is active.
- `renameDraft: ''` — the input's bound value.
- `renameError: ''` — server-side error rendered inline next to the
  input.

Transitions:

- Click `.column-name` span → `startRename(col)`:
  `renamingColumn = col; renameDraft = col; renameError = ''`.
  Focus the input via `$nextTick` and select all text so the user
  can type-to-replace.
- Enter key in input → `commitRename()`.
- Escape key in input → `cancelRename()`.
- Blur on input → `commitRename()` if `renameDraft.trim() !==
  renamingColumn && renameDraft.trim() !== ''`, else `cancelRename()`.

`commitRename()`:

1. Compute `trimmed = renameDraft.trim()`.
2. If `trimmed === renamingColumn || trimmed === ''` →
   `cancelRename()`.
3. `PATCH /api/columns/<renamingColumn>` with
   `{"name": trimmed}`.
4. On 2xx → `renamingColumn = null; renameDraft = ''; renameError = ''`.
   The SSE refetch picks up the rename.
5. On non-2xx → `renameError = err.message || "HTTP <status>"`;
   keep the input open. The user can fix the value and press Enter
   again, or press Escape to revert.

`cancelRename()`: `renamingColumn = null; renameDraft = ''; renameError = ''`.

A guard prevents Sortable from initiating a drag while a rename is
active: the list-header's `:class="renamingColumn === col ?
'is-renaming' : ''"` adds an `is-renaming` class; the list Sortable
is configured with `filter: '.is-renaming'` so the header is not
draggable mid-rename.

### TD11. 3-dots menu state, placement, dismissal

State on `board()`:

- `openMenuColumn: null` — name of the column whose menu is open,
  or `null` if no menu is open. Only one menu is open at a time.
- `menuError: ''` — error message rendered inside the menu (e.g.
  "Move 3 cards first" on `COLUMN_HAS_CARDS`).

Markup: each `.list-header` ends with a `<button class="list-menu-btn">`
(3-dots icon, inline SVG with `currentColor`). Clicking it toggles
`openMenuColumn` between `col` and `null`. When open, a sibling
`<div class="list-menu" x-show="openMenuColumn === col">` renders
absolutely-positioned below the button with a single
`<button class="menu-item danger">Delete list</button>` action.

Dismissal:

- Click outside the menu (overlay or any non-menu element) →
  `openMenuColumn = null; menuError = ''`. Implemented via a
  document-level `@click.outside` modifier on the menu container
  (Alpine's `x-on:click.outside`).
- Escape key on the document → close menu.
- Successful delete → close menu (and refetch via SSE).

`deleteList(col)`:

1. `menuError = ''`.
2. `DELETE /api/columns/<col>`.
3. On 2xx → `openMenuColumn = null`. SSE refetch picks up the
   deletion.
4. On non-2xx → `menuError = err.message || "HTTP <status>"`. Menu
   stays open so the user reads the message and decides next step
   (cancel or move cards first via drag).

The menu does **not** close automatically on error — the message
needs to be visible. The user closes it by clicking outside or
pressing Escape.

### TD12. Drag-reorder via list-header — POST `/api/columns/move`

The list Sortable's `onEnd` handler reads `evt.newIndex` (the
0-indexed new position in `.columns`) and the dragged column's name
from `evt.item.dataset.column`. The `<section class="column">`
element gets a new `data-column` attribute so the drop handler can
read it without DOM traversal.

`handleListDrop(evt)`:

1. `const name = evt.item.dataset.column`.
2. `const position = evt.newIndex`.
3. `POST /api/columns/move` with `{"name": name, "position": position}`.
4. On 2xx → `await this.load()` (refetch reconciles JS model).
5. On non-2xx → `console.error(...)` and `await this.load()` —
   server is source of truth; the visual drop is reverted by the
   refetch (ADR 0002 §D3).

A drag-from-anywhere protection: `mountListSortable` sets
`handle: '.list-header'` and `filter: '.is-renaming, .list-menu-btn,
.list-menu, .column-name input'` so a click on the menu button, the
menu popover, or the rename input does not initiate a drag.

### TD13. Server `routes()` and `httpError` extension

`routes()` gains four lines:

```go
mux.HandleFunc("POST /api/columns", s.handleColumnCreate)
mux.HandleFunc("PATCH /api/columns/{name}", s.handleColumnRename)
mux.HandleFunc("DELETE /api/columns/{name}", s.handleColumnDelete)
mux.HandleFunc("POST /api/columns/move", s.handleColumnMove)
```

`httpError` gains three new `errors.As` arms (before the existing
generic fallbacks):

```go
var ene *board.EmptyColumnNameError
if errors.As(err, &ene) {
    writeErrorJSON(w, http.StatusBadRequest, "INVALID_BODY",
        "column name must be non-empty", nil)
    return
}
var caee *board.ColumnAlreadyExistsError
if errors.As(err, &caee) {
    writeErrorJSON(w, http.StatusBadRequest, "COLUMN_ALREADY_EXISTS",
        err.Error(), map[string]any{"name": caee.Name})
    return
}
var cdle *board.CannotDeleteLastColumnError
if errors.As(err, &cdle) {
    writeErrorJSON(w, http.StatusBadRequest, "CANNOT_DELETE_LAST_COLUMN",
        err.Error(), map[string]any{"name": cdle.Name})
    return
}
var che *board.ColumnHasCardsError
if errors.As(err, &che) {
    writeErrorJSON(w, http.StatusBadRequest, "COLUMN_HAS_CARDS",
        err.Error(),
        map[string]any{"column": che.Name, "cards": che.Cards})
    return
}
```

The existing `ColumnNotFoundError` arm is unchanged (still 400 for
PATCH/move sites). `handleColumnDelete` short-circuits the 404 case
itself per TD4.

### TD14. CSS surfaces

New rules in `style.css`:

- `.add-list-placeholder` — 296×48 dashed border using
  `border: 1.5px dashed var(--border-strong)`, `border-radius:
  var(--rounded-lg)`, `display: flex; align-items: center;
  justify-content: center`, `color: var(--text-muted)`, `cursor:
  pointer`. Hover lifts color to `var(--text)`.
- `.list-composer` — same 296px width as a column, `background:
  var(--surface)`, `border: 1px solid var(--accent)`, `border-radius:
  var(--rounded-xl)`, `padding: var(--space-sm)`. Contains the
  input and the Add/Cancel buttons.
- `.list-menu-btn` — 28×28 transparent button with three dots
  (inline SVG), `color: var(--text-muted)`; hover →
  `color: var(--text)`.
- `.list-menu` — absolute-positioned popover, `background:
  var(--surface)`, `border: 1px solid var(--border)`,
  `border-radius: var(--rounded-md)`, `box-shadow: <popover-elev>`,
  `padding: var(--space-xs)`, minimum width 160px.
- `.list-menu .menu-item.danger` — `color: var(--danger)`,
  full-width, left-aligned, transparent button surface; hover →
  `background: color-mix(in oklab, var(--danger) 8%, transparent)`.
- `.list-menu .menu-error` — `color: var(--danger)`, `font-size:
  var(--font-size-xs)`, padding to match menu item.
- `.list-header .column-name` — when `.is-renaming`, the span is
  replaced by an `<input>` with `background: var(--surface-2)`,
  `border: 1px solid var(--accent)`, sized to match the span's
  typography.

No hex literals outside the existing `:root` block.

## Risks / Trade-offs

- **Two Sortable instances on the same page**: mitigated by
  different mount target + different handle + different group
  (TD8). Manual smoke confirms a card drag does not move a column
  and vice versa.
- **Rename mid-drag**: a column being renamed cannot also be
  dragged because the `is-renaming` class triggers Sortable's
  `filter` (TD10). The reverse (drag mid-rename) is impossible
  because the input element is the focus owner; a pointer-down on a
  non-header surface dismisses focus → blur fires → rename commits
  or reverts.
- **Stale state after SSE refetch with composer open**: the user
  could be mid-type in the Add-list composer when an external
  change (CLI added a column) refetches the board. Per ADR 0002
  §D3, server is source of truth — but the composer state is
  user-local. We **preserve** the composer state across refetch
  (do not auto-close it). If the user submits a name that now
  collides with a CLI-added column, the server returns
  `COLUMN_ALREADY_EXISTS` and the inline error makes the conflict
  obvious. Same logic for inline rename and 3-dots menu.
- **`COLUMN_HAS_CARDS` payload size**: a column with thousands of
  cards produces a multi-KB `details.cards` array. Acceptable for
  v1 — the typical viewer usage tops out at low double digits per
  column, and the wire envelope already supports large card details
  via `cardResponse`.
- **Two-step delete (open menu, click Delete)**: the friction is
  intentional. Per ADR 0003 §D12, deletion is refused on non-empty
  columns; the menu also surfaces the refusal inline. No
  confirmation step is needed because the operation is server-side
  validated and reversible by re-creating the column (only the
  empty case can complete).
- **Last-write-wins on column rename**: CLI renames `todo` → `wip`
  at the same instant the viewer renames `todo` → `backlog`. The
  later `board.Save` wins; the earlier rename's cards are renamed
  to whichever value ended up on disk. This is the documented
  behavior (ADR 0002 §D3); no new lock is introduced.
- **No new e2e test for the two-instance Sortable interaction**:
  the visual smoke at the end of the task list is the gate.
  Adding a Playwright/Chrome-MCP test for drag-and-drop interaction
  is deferred to a follow-up; the unit tests cover the
  endpoint surface comprehensively.
