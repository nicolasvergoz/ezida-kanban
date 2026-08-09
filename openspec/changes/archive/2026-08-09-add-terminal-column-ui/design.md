## Context

The terminal marker is a suffix on disk (`done*`) and a `done_columns`
string array on the wire; the name itself never carries the `*` anywhere
but in `kanban.toml`. `board.SetDoneColumn` is the only mutator, and
`board.RenameColumn` already carries the flag across a rename.

The viewer reads the flag (`toUiBoard` derives `list.done` from
`done_columns`) and renders a resting check mark in the list header. It
has never written it. Writing it is the whole change.

Two existing shapes constrain the work:

- `PATCH /api/columns/{name}` today takes `{"name": "<new>"}` and treats
  an empty or whitespace-only value as `INVALID_BODY`. That contract has
  live scenarios in `viewer-server`; the new field has to arrive without
  breaking them.
- The list header's rename is `EditableText`, a small generic with
  exactly one caller (the list title). The header itself is
  `draggable` — it is the list-reorder drag handle.

## Goals / Non-Goals

**Goals:**

- Toggle a column's terminal marker from the viewer, by two paths: while
  renaming, and from the `⋯` menu without renaming.
- A rename that also flips the marker is **one** request. No window in
  which the name landed and the marker did not.
- The demo page behaves like the real server.

**Non-Goals:**

- Any CLI change. `ezida columns done|undone` stays exactly as it is.
- Any change to `GET /api/board`, to `done_columns`, or to the on-disk
  format.
- Any change to the resting check mark rendered in the header.
- Fixing the latent `useClickOutside` bubble-phase defect in the
  priority and column dropdowns. Out of scope, still open.

## Decisions

### D1 — Both payload keys become optional, via pointers

`columnRenamePayload` becomes:

```go
type columnPatchPayload struct {
    Name *string `json:"name"`
    Done *bool   `json:"done"`
}
```

Absent means "leave alone". This is what lets the `⋯` menu send
`{"done": true}` and touch nothing else.

*Alternative considered:* keep `name` required and have the menu send
the current name unchanged, leaning on the existing `from == to` no-op.
Rejected — it makes the request lie about its intent, and it is racy:
the name in the body comes from the client's possibly-stale board, so a
concurrent rename turns a marker toggle into a rename back to a stale
name.

Back-compatibility falls out of the pointer: `{"name": ""}` is
*present and empty* and still yields `INVALID_BODY`, so every existing
scenario holds unchanged.

A body with **neither** key is `INVALID_BODY` (400). Only a broken
client can produce it, and answering 200 would hide the bug. No new
error code — the CLI's `NOTHING_TO_EDIT` belongs to the CLI vocabulary
and this route already owns `INVALID_BODY` for malformed intent.

### D2 — Rename first, then set the flag, on the *new* name

`RenameColumn` deliberately carries the terminal flag across a rename
(`done*` → `shipped*`). So the handler must run rename first and apply
`SetDoneColumn` to the post-rename name:

```
RenameColumn(b, from, to)      // carries the flag to `to`
b.Board.SetDoneColumn(target, *p.Done)   // target = to, or from if no rename
```

The reverse order is silently wrong: setting the flag on the old name
and then renaming would let `RenameColumn`'s carry-across overwrite the
value the client just asked for. `{"name":"shipped","done":false}` on a
`done*` column is the case that exposes it — correct order clears the
marker, reverse order re-applies it.

### D3 — Existence is checked once, up front

`SetDoneColumn` does not verify that the name is in `Columns` — its
doc comment claims the type makes that unrepresentable, but nothing
enforces it. A `{"done": true}` on a column that does not exist would
otherwise write a flag for a phantom name, which `EncodeColumns` then
silently drops on save: a 200 that did nothing.

So the handler verifies `from` is in `b.Board.Columns` before any
mutation and returns `*board.ColumnNotFoundError` otherwise — the same
error `RenameColumn` produces, so the wire code and status are
unchanged for the rename path.

This tightens one edge: `PATCH /api/columns/ghost` with
`{"name":"ghost"}` currently returns 200 (because `RenameColumn`
short-circuits on `from == to` before looking anything up) and now
returns 400 `COLUMN_NOT_FOUND`. That is the correct answer and it is
spelled out as a scenario rather than left as an accident.

### D4 — The check must commit on `mousedown`, not on click

The trap: clicking a control beside a focused input fires `blur` on the
input *first*. `EditableText` commits on blur. A naive `onClick` check
would therefore fire the rename PATCH, and only then toggle a piece of
state that no longer has a commit to ride along with — exactly the
class of defect the browser tests caught in `CardItem`'s double-click.

The control therefore handles `onMouseDown` with `preventDefault()`, so
focus never leaves the input, and toggles the staged value there. The
single commit still comes from the input's Enter or blur.

The staged value lives in `List` (it is `list.done` seeded at edit-open,
reset on Escape); `EditableText` gains two optional props — an
`accessory` node rendered beside the input while editing, and a staged
payload it hands back to `onChange` at commit time. The alternative was
inlining the editor into `List` and deleting the generic; not worth it
for one prop pair.

### D5 — The header must not turn the check into a drag handle

`.list-header` is `draggable` for list reorder. `viewer-ui` already
carries "Rename input is not a drag handle" for the same reason; the
accessory check gets the same treatment, so pressing it never starts a
column drag.

### D6 — The `⋯` menu entry writes immediately

`Terminal column` in the menu is not staged: it sends
`PATCH {"done": !list.done}` on activation and closes the menu. There
is no name to coordinate with, so there is nothing to batch.

`ListMenu`'s `useClickOutside` is correct here — the known bubble-phase
defect only bites inside the modal, which stops `mousedown` at its own
container. The menu is not in the modal. Recorded so nobody "fixes" it.

### D7 — The demo shim replays the field

`site/demo/demo-shim.js` handles `PATCH /api/columns/<old>` but knows
nothing of `done_columns`. It must tolerate an absent `name`, maintain
`state.done_columns` across rename and delete, and apply `done`. Change
3 established the rule: a control that is dead on the public demo page
is a regression, not an omission.

## Risks / Trade-offs

- **Blur beats click and the marker is silently lost.** → `mousedown` +
  `preventDefault` (D4), plus an e2e test asserting that renaming and
  toggling together produces exactly **one** `PATCH` carrying both keys.

- **The accessory starts a list drag instead of toggling.** → D5, plus a
  test that a press on the check does not begin a column reorder.

- **A board with no terminal column renders differently.** The resting
  marker is untouched and the new controls are inside the rename input
  and the menu, both of which are already-existing surfaces. The
  `plain.toml` pixel guard covers it.

- **The `ghost`/`ghost` no-op becomes a 400.** Behaviour change, but the
  old answer was an accident of short-circuit ordering and no client
  sends it. Spelled out in the delta.

- **`done` and `name` disagreeing about which column they address.**
  Not representable: the URL path names the column, `done` applies to it
  after rename. There is no second identifier to get out of sync.

## Migration Plan

None. No schema change, no data migration, no new dependency. The
endpoint is additive under D1, so an older page against a newer server
keeps working.
