## Why

`ezida add`, `ezida move`, `ezida edit --column`, and the viewer's
column-body drop all place a card at the *bottom* of the target
column. On a long column — `done` has 21 cards — the card you just
touched is the hardest one to find; you have to scroll all the way
down to see it. Placing it at the top instead puts it where you're
already looking.

## What Changes

- `ezida add` inserts the new card at position 0 of the target
  column instead of appending. **BREAKING** for anyone relying on
  the current bottom-append order via `board.AppendCardToColumn`.
- `POST /api/cards` (the viewer's own "add card" input) inserts the
  new card at position 0 too, for symmetry with `ezida add` — both
  go through the same shared helper. **BREAKING** for the same
  reason.
- `ezida move <id> <column>` re-places the card at position 0 of the
  target column instead of the end.
- `ezida edit --column=<name>` re-places the card at position 0 of
  the target column instead of the end, regardless of which other
  flags are combined with `--column` in the same invocation.
- Viewer: dropping a card onto the column body (blank space below
  the last card, or an empty column — not onto another card) inserts
  at position 0 instead of `list.cards.length`. Dropping a card onto
  another card is unchanged — that path is already positional
  (above/below by cursor Y).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `card-writing`: `ezida add` (spec:9) and `ezida move` (spec:121)
  both change from end-of-column placement to top-of-column
  placement.
- `board-config`: `ezida edit --column` (spec:9, the `--column` flag
  behavior and its "Edit changes column re-orders the card" scenario)
  changes from end-of-column placement to top-of-column placement.
- `viewer-ui`: "Cards are draggable across and within columns"
  (spec:341) gains a scenario for dropping on the column body
  (currently only card-to-card drop scenarios exist); that drop now
  resolves to position 0 instead of the column's current length.
- `viewer-server`: "`POST /api/cards` creates a new card" (spec:604)
  changes from end-of-column placement to top-of-column placement —
  it shares `board.AppendCardToColumn` with the CLI `add` path.

## Impact

- Go: `internal/board/board.go` (`AppendCardToColumn` /
  `InsertCardAt` — the shared helper), called from
  `internal/commands/add.go:120`, `internal/commands/move.go:41`,
  `internal/commands/edit.go:195`, and
  `internal/server/handlers.go:282` (`POST /api/cards`).
- JS: `internal/server/web/app.jsx:879` (`List.onDrop`, the
  column-body drop handler — `CardItem.onDrop` for card-to-card drop
  is untouched).
- Tests: Go unit tests for `add`, `move`, `edit --column` placement
  order; Playwright e2e coverage for the column-body drop position
  (`e2e/*.spec.ts`).
- No wire/schema change — `POST /api/cards/{id}/move` and
  `POST /api/cards` already accept a `position`; only the value the
  client sends (viewer) or the server defaults to (CLI) changes.
