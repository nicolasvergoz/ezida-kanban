## Context

Five separate code paths append a card to the bottom of its target
column instead of placing it at the top: `ezida add`
(`internal/commands/add.go:120`), `ezida move`
(`internal/commands/move.go:41`), `ezida edit --column`
(`internal/commands/edit.go:195`), `POST /api/cards`
(`internal/server/handlers.go:282` — the viewer's own "add card"
input), and the viewer's column-body drop
(`internal/server/web/app.jsx:879`). The first four all call
`board.AppendCardToColumn`, which counts existing cards in the
column and delegates to `board.InsertCardAt(b, c, c.Column, count)`
— i.e. "insert after everything." `board.MoveCard` (`board.go:331`)
already accepts an explicit position and is unaffected; it's used
only by the viewer's HTTP move handler and by card-to-card drops,
which are already positional.

`POST /api/cards` was not part of the original exploration (card
`nvwk16` scoped CLI `add`/`move`/`edit` and the viewer's drag-drop)
but shares `AppendCardToColumn` with CLI `add`, so repurposing the
helper's semantics silently changes its behavior too. Resolved
during apply: it gets the same top-placement treatment, for
symmetry with `ezida add` — see the added `viewer-server` delta
spec.

## Goals / Non-Goals

**Goals:**
- `add`, `move`, `edit --column`, and viewer column-body drop all
  place the card at position 0 of the target column.
- Card-to-card drag-drop (already positional) is untouched.
- One shared code change on the Go side — not three copies of
  "insert at 0."

**Non-Goals:**
- `ezida rm`, column reordering, and epic assignment are unaffected.
- No wire/schema change: `POST /api/cards/{id}/move` and
  `POST /api/cards` already accept a `position` field; only the
  value sent (viewer) or defaulted to (CLI) changes.

## Decisions

- **Rename/repurpose `AppendCardToColumn` semantics rather than add a
  new helper.** All three CLI call sites want the same "top of
  column" placement and currently share `AppendCardToColumn`. Change
  it to insert at position 0 (`InsertCardAt(b, c, c.Column, 0)`)
  instead of `count`. A rename (e.g. `PrependCardToColumn`) is
  preferred over keeping the old name with new behavior, since
  "Append" would now be a lie — but the rename is a local
  implementation detail, not a spec concern.
- **`ezida move` to the card's current column also re-places it at
  the top.** The existing requirement text doesn't special-case
  "moving to the column it's already in," and the current
  implementation doesn't either (it unconditionally deletes then
  re-inserts). This change keeps that: `ezida move a3f2k9 todo` on a
  card already in `todo` moves it to the top of `todo`, not a no-op
  in place. Consistent, no new branch needed.
- **Viewer fix is scoped to `List.onDrop` only.** `CardItem.onDrop`
  already computes `above`/`below` from cursor position and
  `stopPropagation()`s, so card-to-card drops never reach
  `List.onDrop`. Only the "drop `{ position: 0 }`" for a *column-body*
  drop.

## Note on test coverage

The column-body drop (`List.onDrop`) has no automated e2e assertion.
Per `CLAUDE.md`, Playwright's mouse synthesis does not reliably drive
native HTML5 `dataTransfer`, so cross-column card dragging is already
excluded from this project's e2e coverage — card-to-card drop tests
that exist assert `draggable` attributes and the move endpoint
contract, not an actual simulated drop. The same limitation applies
here; manual confirmation in the simulator is the check (tasks.md
§7–8). The Go-side contract (`POST /api/cards/{id}/move` accepting an
explicit `position`) is unchanged and already covered.

## Risks / Trade-offs

- **Breaking change for `ezida add`.** Anyone scripting around
  bottom-append order (e.g. assuming the last card in a column is
  the most recently added) breaks. Called out as **BREAKING** in the
  proposal; no migration path needed since it's an ordering
  convention, not a data format.
- **`git diff` noise on `kanban.toml`.** Every move/add now reorders
  the `[[cards]]` block near the top of its column instead of the
  bottom, which is a larger perceived diff for boards under version
  control. Accepted — this is the entire point of the change (put
  the touched card where you're looking).

## Open Questions

None outstanding — scope and same-column-move behavior were
resolved during exploration (see card `nvwk16`).
