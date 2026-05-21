## Why

V1 ships a read-only viewer. The first real interaction users
expect is drag-and-drop: pick up a card, drop it in a different
column or at a different position within the same column.
ADR 0002 §D4 also pinned that the refactor of
`AppendCardToColumn` into a generic `InsertCardAt` primitive
lands when a second caller appears — that second caller is the
`MoveCard` helper introduced in this phase. The phase delivers
both: the server-side primitive + HTTP endpoint, and the
client-side Sortable.js wiring.

## What Changes

- `internal/board/board.go`:
  - Add `InsertCardAt(b *Board, c Card, column string, position int)`
    that inserts `c` so that, after the insert, it occupies
    `position` (0-indexed) among the cards whose `column` matches.
    Clamps `position` to `[0, len(column-cards-after-insert)]`
    (ADR 0002 §D11). Sets `c.Column = column` before insert.
  - Add `MoveCard(b *Board, id, column string, position int) error`
    that removes the card from its current slice position, refreshes
    `c.UpdatedAt`, sets `c.Column`, and re-inserts via
    `InsertCardAt`. Returns `*CardNotFoundError` if `id` is unknown
    and `*ColumnNotFoundError` if `column` is not in `[board].columns`.
  - Refactor `AppendCardToColumn` to delegate:
    `InsertCardAt(b, c, c.Column, len(cardsInColumn))`. Behavior is
    unchanged from existing callers' POV.
- `internal/server/handlers.go`:
  - Add `POST /api/cards/:id/move` accepting body
    `{"column": "<name>", "position": <int>}`. Loads the board,
    calls `board.MoveCard`, saves atomically via `board.Save`,
    returns the updated card as `{"card": {...}}`. Error mapping:
    unknown id → 404 `CARD_NOT_FOUND`; unknown column → 400
    `COLUMN_NOT_FOUND`; malformed body → 400 `INVALID_BODY`.
- `internal/server/web/vendor/sortable.min.js`: new vendored copy of
  Sortable.js (~40 KB) per ADR 0002 §D5.
- `internal/server/web/app.js`:
  - Augment `board()` Alpine component with `mountSortable()` hook
    called after columns render; instantiates one `Sortable` per
    column's `.cards` `<ul>` with shared `group: 'cards'`,
    `onEnd: handleDrop`.
  - `handleDrop(evt)` issues `POST /api/cards/<id>/move` with the
    destination column and new position. On failure, calls
    `load()` to reset state.
- `internal/server/web/index.html`: add the
  `<script defer src="/static/vendor/sortable.min.js"></script>` tag.
- `internal/server/web/style.css`: add a subtle drag-state class
  (`.sortable-ghost { opacity: 0.4 }`) and ensure card has
  `cursor: grab` then `cursor: grabbing` while held.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `board-storage`: adds `InsertCardAt` and `MoveCard` helpers,
  keeps `AppendCardToColumn` behavior (now delegates internally).
- `viewer-server`: adds `POST /api/cards/:id/move` endpoint.
- `viewer-ui`: adds drag-and-drop interaction across and within
  columns, optimistic update with server reconciliation on failure.

## Impact

- New code in `internal/board/board.go` (~60 LOC for the two
  helpers).
- New code in `internal/server/handlers.go` (~40 LOC for the
  endpoint).
- New code in `internal/server/web/app.js` (~30 LOC for Sortable
  wiring + drop handler).
- New vendored asset (~40 KB).
- No new Go module dependencies.
- New error codes: none — reuses existing `CARD_NOT_FOUND`,
  `COLUMN_NOT_FOUND` from CLI namespace; adds `INVALID_BODY` as a
  generic HTTP-layer malformed-request code.
- CLI behavior unchanged. `ezida move` still appends to end of the
  destination column (ADR 0001 §D12); the HTTP layer is the only
  caller that exercises explicit positions in v1.
