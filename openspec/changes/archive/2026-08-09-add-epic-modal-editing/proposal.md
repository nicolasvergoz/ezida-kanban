# Edit the epic relation and its color from the detail modal

## Why

`add-epics-wire-viewer` made epics visible and `add-epic-filter-focus` made them
navigable, but both are read-only. Every way to *create* an epic relation is still
the CLI: `ezida edit <id> --epic <parent>`, `ezida edit <id> --color <name>`. A user
who works in the viewer can see the chip, the parent card and the progress bar, and
can scope the board to one epic — and then has to leave for a terminal to attach a
single card.

The server already accepts the mutation. `PATCH /api/cards/{id}` decodes `epic` and
`color`, and `board.UpdateCard` validates both before writing. What is missing is the
UI that calls it, one honest error path, and the picker that lets a user name a card
without typing a six-character id from memory.

This is the last piece of the epics feature as originally scoped. It is almost
entirely frontend.

## What Changes

**The server stops reporting user errors as server errors.** `*board.InvalidEpicError`
and `*board.InvalidColorError` currently fall through `httpError`'s catch-all and
leave as **500 `IO_ERROR`** with an opaque message. They become **400 `INVALID_EPIC`**
and **400 `INVALID_COLOR`**, carrying the offending value in `details` like every
other typed error in that function. Without this the UI has nothing to display but
"something went wrong" for the four cases a user actually hits.

**One validation hole closes with it.** `CheckEpicTarget` rejects an unknown target,
a self-reference, and a target that already belongs to an epic — but not the mirror
case: giving a parent card an epic of its own, which pushes its existing children to
a second level. Today that is caught after mutation by `Validate`, so it also exits
as 500. `CheckEpicTarget` gains the rule, and it becomes an `INVALID_EPIC` like the
other three. The CLI inherits the fix for free, since it shares the validator.

**The client sends the two fields it currently drops.** `patchCard` in `app.jsx`
translates UI keys to server keys and silently discards anything it does not know;
`epic` and `color` are not in the list. They are added, with the empty string
meaning "clear", which is what the server already understands.

**Failures become visible instead of console-only.** Every mutation in `app.jsx`
ends in `catch (e) { console.error(e); }`. That is survivable for a rejected title,
where the field simply snaps back — it is not survivable here, where "no card on this
board carries that id" is the entire content of the interaction. The modal gains a
message region fed by the error envelope's `message`, cleared on the next successful
mutation or on close. Expected 400s no longer reach `console.error`, which also keeps
the browser tests' console-error guard meaningful.

**A card-search combobox, the first entity picker in Ezida.** Type to filter the
board's cards by title or by id, arrow keys to move, Enter to commit, Escape to
close. Candidates exclude what the server would refuse anyway: the card itself, any
card that already carries an epic, and any card that already has children. This is
the largest new component in the whole epics feature and the one piece meant to
outlive it — dependencies and linked files need the same control.

**The child side of the modal becomes editable.** The `Epic` section on a card that
carries one gains reassign (opens the picker seeded with the current parent) and
detach. A card carrying no epic gains an "Add to an epic" affordance in the same
place, so the section is no longer conditional on already having a relation.

**The parent side gains add and remove.** The `Children` section grows an add control
that opens the same picker over the cards that may become children, and a per-row
remove that clears that child's `epic`. Both are one `PATCH` on the *child*; the
parent is never written directly.

**A palette swatch row.** The eight `EpicPalette` colors as swatches plus a clear
control, on a card that is an epic. The wire takes hex only — `PATCH {"color":"blue"}`
is refused today and stays refused — so the swatches send `Hex`, and the palette
names travel only as labels. An off-palette hex already on a card renders as a ninth,
selected, unnamed swatch rather than disappearing.

**Explicitly out of scope.** No creating a card from inside the picker. No nesting
beyond one level. No per-epic ordering of children. No change to the chip, the parent
card treatment, the progress bar or the filter — this change writes what those
already read. Marking a terminal column from the UI stays its own change.

## Capabilities

### New Capabilities
<!-- None. Every change extends an existing viewer, server, or epics capability. -->

### Modified Capabilities
- `viewer-server`: `PATCH /api/cards/:id` gains typed 400 responses for an invalid
  epic target (`INVALID_EPIC`) and an invalid color (`INVALID_COLOR`), replacing the
  500 `IO_ERROR` both produce today.
- `card-epics`: the epic-target pre-check refuses a card that already has children,
  making the one-level nesting rule symmetric and reportable before mutation instead
  of after.
- `viewer-ui`: the modal's epic and children sections become editable; new
  requirements for the card-search combobox, the palette swatch row, and the modal's
  error message region.

## Impact

**Code**
- `internal/server/handlers.go`: two new `errors.As` arms in `httpError`.
- `internal/board/epics.go`: one rule in `CheckEpicTarget`.
- `internal/server/web/app.jsx`: `patchCard` key translation, the new `EpicPicker`
  component, the modal's `Epic` and `Children` sections, the swatch row, error state
  plumbing from `apiSend` to the modal.
- `internal/server/web/styles.css`: the picker's popover, list and highlight states;
  the swatch row; the error message region. `.modal-epic-parent` and
  `.modal-epic-children` already exist and were sized for this (change 2, design D8).

**Inherited for free**: `site/demo/app.jsx` and `site/demo/styles.css` are symlinks
to the embedded assets. The demo has no server, so its controls will render and
refuse to write — worth confirming they fail quietly rather than throwing.

**Tests**: `e2e/` gains a spec driving attach, reassign, detach, the invalid-target
rejection and the swatch row against the real server. The Go side gains error-mapping
tests in `internal/server/` and a `CheckEpicTarget` case in `internal/board/`.

**No breaking change.** The wire shape is untouched — no field added to
`cardResponse`, so `output.ExportCard` needs no matching edit. Two error responses
change status and code, from a 500 that no client could have been relying on.

**Docs**: `docs/usage.md`'s "What the Web UI lets you do" list gains the editing
entries.
