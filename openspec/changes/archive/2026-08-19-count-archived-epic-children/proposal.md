## Why

Archiving finished work currently makes an epic look **less** complete.

`add-card-archiving-viewer` deliberately built the epic index over live cards
only, so that an archived child could not inflate a live parent's denominator.
That reasoning protected the wrong number. The dominant use of archiving is
exactly to clear out *finished* cards, so excluding them produces the opposite
of what a user expects:

> An epic has 5 children, 4 of them done. You archive those 4 to tidy the
> `done` column. The epic now reads **0/1** — it looks like nothing has been
> achieved, moments after you filed away four completed cards.

The work happened. Archiving is not deletion — it is precisely the operation
that keeps the record — so the record should keep counting.

## What Changes

- An epic's derived `done` / `total` counts its archived children alongside
  its live ones. An archived child counts toward `done` when the column it was
  archived from is a terminal column on the board **at read time** — the same
  rule a live card follows, applied to the column the archive recorded.
- `ezida get <id>` on an epic loads the archive, counts archived children in
  its `Progress`, and **lists** them alongside the live ones rather than only
  counting them — a total that does not match the visible list would be worse
  than the current understatement. Archived children are marked as archived in
  both text and JSON output.
- The viewer's progress bars (on the board and in the detail modal) count the
  same way. No wire change is needed: `GET /api/board` already carries
  `archived_cards` with their stored `column`, and `done_columns` — the client
  has everything required.
- A card whose only children are archived still reads as an epic, on both
  surfaces. It has children; they are simply filed away.
- Not in this change: `ezida list --epic=<id>`, which is a *filter* over a card
  set rather than a progress computation, and stays governed by the existing
  `--include-archived` / `--archived-only` flags.

## Capabilities

### Modified Capabilities

- `card-epics`: the derived-progress requirement gains archived children, and
  states the terminal-column rule that decides whether one counts as done.
- `card-reading`: `ezida get` reads the archive, reports archived children in
  `children` and `progress`, and marks them as archived.
- `viewer-ui`: progress bars count archived children; an epic whose children
  are all archived still renders as an epic.
- `documentation`: `docs/usage.md`'s epics section states that archived
  children keep counting, and names the deleted-column caveat below.

## Impact

- **Modified code**: `internal/board/epics.go` (`EpicProgress` /
  `ChildrenOf` gain archive awareness), `internal/commands/get.go` (loads the
  archive, renders archived children), `internal/output/json.go`
  (`ChildRef` gains an `archived` marker), `internal/server/web/app.jsx`
  (`buildEpicIndex` folds archived children in for live parents),
  `docs/usage.md`.
- **Existing tests touched**: `internal/board/epics_test.go` and
  `internal/commands/epics_test.go` assert current counts; any case with an
  archive present will need its expectation revisited. The bulk of them use
  boards with no archive at all and are unaffected by construction.
- **Dependencies**: none added.
- **Known limitation this introduces**: because done-ness is resolved at read
  time against the live board, deleting or un-marking the column an archived
  card came from silently drops it out of `done`. This is reachable through
  the very workflow the CLI change shipped — `ezida archive column done`
  followed by `ezida columns rm done`. Chosen deliberately over freezing a
  `was_done` flag into the archive file, to keep the project's "derived values
  are never stored" rule intact; documented rather than hidden.
