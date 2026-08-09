## Context

Three changes built the epics feature from the file up: `add-epics-schema-cli` put
`epic` and `color` on the card and taught the CLI to write them, `add-epics-wire-viewer`
carried them onto the wire and rendered them, `add-epic-filter-focus` scoped the board
to one epic. All viewer-side work so far has been read-only, and deliberately so
(change 2, design D8): the presentational layer was built first, with the modal's
`.modal-epic-parent` and `.modal-epic-children` containers already sized and their
empty states already settled, so the interactive component could be written against a
layout that was known to work.

This change writes the interactive component.

The server needs almost nothing. `PATCH /api/cards/{id}` already decodes `epic` and
`color` into `board.CardPatch` — pointer fields, so absent and set-to-empty are
distinguishable — and `board.UpdateCard` validates through `CheckEpicTarget` and the
hex pattern before touching anything. Attaching a child that turns its target into an
epic assigns the parent a palette color in the same write, through `EnsureEpicColor`.
Verified against a running server: a valid target returns 200, an unknown id, a
self-reference and a non-hex color are all refused, and `{"epic":""}` detaches.

What the server gets wrong is only *how* it refuses. `*board.InvalidEpicError` and
`*board.InvalidColorError` have no arm in `httpError`, so they exit through its
catch-all as 500 `IO_ERROR`. A UI built on top of that has no message worth showing.

The viewer side is a single-file React app with no state store: every mutation is a
`fetch` followed by a full board refetch, and an SSE `board-changed` event triggers the
same refetch. Every mutation ends in `catch (e) { console.error(e); }`, which is
tolerable when the failure mode is "the field snaps back" and useless when the failure
*is* the information.

## Goals / Non-Goals

**Goals:**
- Attach, reassign and detach an epic relation without leaving the viewer.
- Pick a target card by searching its title or id, not by recalling six characters.
- Choose an epic's color from the palette.
- Every refusal the server can produce is a 400 with a code and a sentence, and that
  sentence reaches the user.
- The picker is built to be reused for dependencies and linked files.

**Non-Goals:**
- No new endpoint, and no new field on the wire.
- No card creation from inside the picker.
- No nesting beyond one level, in either direction.
- No per-epic ordering of children.
- No change to the chip, the parent-card treatment, the progress bar, or the filter.
- No global toast or notification system — errors stay scoped to where they happen.
- No fix for the dead double-click on a card title; it stays pinned as a known defect.

## Decisions

### D1 — Two `errors.As` arms, not a new error mechanism

`httpError` gains an arm for `*board.InvalidEpicError` → 400 `INVALID_EPIC` with
`details: {"epic": <id>}`, and one for `*board.InvalidColorError` → 400 `INVALID_COLOR`
with `details: {"color": <value>}`. Both reuse `writeErrorJSON` and the envelope
`viewer-server` already specifies; both error types already document these exact codes
in their doc comments, which the CLI honours and the HTTP layer never did.

The alternative was to catch these in `handlePatch` before calling `httpError`, next to
the explicit `writeErrorJSON` calls in `handleCreate`. Rejected: `httpError` is the
place that knows the board error taxonomy, and it already holds nine such arms. Adding
a tenth and eleventh keeps every mapping in one readable list; splitting them across
handlers means the next endpoint to accept `epic` has to remember to repeat the map.

### D2 — The nesting rule becomes symmetric, and pre-mutation

`CheckEpicTarget` today refuses three things: `epicID == childID`, an unknown `epicID`,
and an `epicID` that itself carries an epic. It does not refuse the mirror case —
giving an epic a parent of its own, which pushes its existing children to a second
level. That case is caught, but only afterwards, by the whole-board `Validate` at the
end of `UpdateCard`, whose error is a board-level report and exits as 500.

`CheckEpicTarget` gains: if the child already has children, refuse with
`InvalidEpicError{ID: epicID, Reason: "that card has children of its own, and epic
nesting is limited to one level"}`. The rule then reads in one place, both directions,
before anything is written, and reports as `INVALID_EPIC` on the wire.

The alternative was to map validation errors to 400 as well. Rejected: a `Validate`
failure means the board on disk is inconsistent, which is a different class of problem
from "this particular id is not a legal target" and deserves to stay loud.

The CLI inherits the fix, since `ezida edit --epic` calls the same validator. That is
a behaviour change to a shared code path, so it belongs in the `card-epics` spec, not
only in `viewer-server`.

### D3 — The picker searches the board the client already holds

`GET /api/board` returns every card on every fetch. The picker filters that array in
memory: match on title (case-insensitive substring) or on id (prefix), then drop
candidates the server would refuse — the card itself, any card carrying an epic, and
any card with children.

No search endpoint, no debounce, no loading state. A board with a thousand cards
filters in under a frame, and Ezida's boards are files a human maintains.

Mirroring the server's rules client-side is duplication, and it is the right kind: the
picker's job is to not offer what cannot work, the server's job is to be the authority.
When they disagree the server wins and the error region says why — which is exactly the
case a stale board (an SSE event in flight) produces, and it is handled rather than
prevented.

### D4 — Every relation write is a PATCH on the child

The child's `epic` field is the only storage the relation has. Attaching from the
child side, attaching from the parent side, reassigning and detaching are therefore all
the same call: `PATCH /api/cards/{childId} {"epic": "<id>" | ""}`. The parent card is
never written by the client; when acquiring a child gives it a color, the server does
that inside the same write.

The parent-side "add a child" control is the same picker pointed the other way: it
lists candidate *children* and patches the one selected. The per-row remove patches
that row's card with `{"epic": ""}`. There is no parent-side endpoint to build, and no
ordering problem between two writes, because there is only ever one.

### D5 — Errors surface in the modal, and stop reaching the console

The modal gains an error region below the section that failed, fed by the `message`
field of the error envelope. It clears on the next successful mutation and on close.

This requires `patchCard` to stop swallowing failures. The mutation helpers gain an
optional error callback; `apiSend` already parses the JSON body of a non-2xx response
into its thrown `Error`, so the change is to carry the parsed envelope rather than
stringify it into a message.

Expected 400s must not reach `console.error`. Two reasons: it is noise for a case the
UI has handled, and `e2e/fixtures.ts` fails any test whose page logged a console error,
so a test asserting the rejection path would fail on the very error it is asserting.
Unexpected failures — a 500, a network error — keep logging.

A global toast was considered and rejected. The error is about a specific field in a
specific modal; a floating notification is both further from the cause and a new
subsystem to design, place and dismiss.

### D6 — Selection commits immediately; no confirm step

Choosing a card in the picker sends the PATCH. Clicking a swatch sends the PATCH.
Clicking the remove control on a child row sends the PATCH. Nothing is staged and there
is no Save button.

This matches the rest of the modal — the priority listbox and the column listbox both
commit on selection, and `viewer-ui` already requires a single-key PATCH per field
commit. The recovery from a mistake is the same control: reassign, or detach.

### D7 — Swatches send hex; palette names are labels

`PATCH {"color":"blue"}` is refused today, because `UpdateCard` validates `Color`
against `hexColorPattern` and only hex ever reaches the file. That stays true. The
swatch row is built from `board.EpicPalette` and sends each entry's `Hex`; the `Name`
is the accessible label and the tooltip.

Teaching the wire to accept palette names via `ResolveColor` was considered. Rejected:
it would put a second representation of a color on the wire, so a client reading a card
back would find hex where it sent a name — and the file, the export and the board
endpoint would all still be hex. One representation on the wire, resolved at the edge
that has a human in front of it.

The eight palette hexes are duplicated into the client. They are already effectively
duplicated — the chip renders stored hex — but the swatch row is the first place the
client needs the *set*. Duplicating eight constants is accepted over adding a
`GET /api/palette` or a top-level `palette` array to the board envelope for something
that changes when the Go source changes.

A card carrying an off-palette hex renders it as an additional, selected swatch rather
than showing nothing selected. Hand-edited colors are legal in `kanban.toml`, and a UI
that silently fails to represent the current value invites a user to overwrite it by
accident.

### D8 — The color row belongs to the parent side

The swatch row renders inside the `Children` section, on a card that has children.
Color on a childless card has no rendering consequence anywhere in the viewer — the
chip is drawn from the *parent's* color, and change 2's D5 gates every parent signal on
having children, not on carrying a color. Offering swatches on an ordinary card would
be a control whose effect is invisible until an unrelated future action.

The trade-off: a user cannot pre-color a card they are about to make an epic. They
color it after attaching the first child, which is also when the server has already
assigned one automatically — so the realistic action is *re*-coloring, and that is
available exactly where the color is visible.

### D9 — The `Epic` section always renders in the modal

Change 2 renders the parent row only on a card that carries an epic. That cannot stand
here: a card with no epic is precisely the card that needs the attach control, and
there would be no path from a fresh board to a first relation.

So the section renders on every card, showing either the current parent with reassign
and detach, or an "Add to an epic" affordance.

This does not weaken the invariant that a board without epics renders identically to
one built before the feature. That invariant is about the board surface — the cards,
the columns, the chips — which is what a user sees without acting. The modal is opened
deliberately, one card at a time, and it is where every other editable field already
lives. The guard test stays on the board surface; if it asserts on modal contents it is
narrowed to the board.

### D10 — Combobox semantics, and Escape closes the picker first

The picker is an input with `role="combobox"`, `aria-expanded`, and
`aria-activedescendant` pointing at the highlighted option in a `role="listbox"`.
Arrow keys move the highlight, Enter commits it, Escape closes the picker.

Escape ordering matters: the modal already closes on Escape, so an open picker must
consume the key and stop propagation. A user pressing Escape to abandon a search and
losing the whole modal is the kind of thing only real interaction catches — it gets a
browser test.

Visually it reuses `.modal-dropdown` / `.modal-dropdown-item` from the priority and
column listboxes, plus a search input and a highlighted state. No new visual vocabulary
beyond the highlight.

### D11 — The modal survives the refetch, keyed by id

Every mutation refetches the whole board and replaces the tree. The open modal already
re-resolves its card by id from the new board, which is why the parent row and the
progress bar update after a CLI edit lands via SSE. The picker's own state — the query
string, whether it is open — lives in the picker and is unaffected by a board that
changes underneath it; its candidate list simply recomputes.

The one case worth naming: an SSE event that deletes the card being edited. That path
exists today and closes the modal; nothing here changes it.

## Risks / Trade-offs

**The combobox is the largest new component in the feature and has the most keyboard
surface.** → Browser tests cover open, filter, arrow, Enter, Escape and the
click-outside close, driven against the real server rather than synthesised events.
`e2e/fixtures.ts`'s console-error guard catches the React warnings that this kind of
component tends to produce.

**Client-side candidate filtering can disagree with the server.** → By construction:
the board can change between fetch and commit. The server stays the authority, the
rejection returns a typed 400 with a sentence, and the error region shows it. Specified
as a scenario so the disagreement path is tested rather than assumed impossible.

**Detaching the last child leaves the parent colored but no longer an epic.** →
Correct and intentional. `EnsureEpicColor` never clears, an explicit color always
survives, and change 2's D5 already renders a colored childless card as an ordinary
card. The color is waiting there if the user attaches a child again.

**The demo page has controls that cannot write.** → `site/demo/` serves a static
`board.json` with no server behind it, and the assets are symlinks, so the new controls
appear there automatically. A failed `fetch` must land in the error region, not throw
past it. Worth an explicit check; the alternative — a build-time read-only flag — adds
a divergence between demo and product for a page nobody edits from.

**Two error responses change status and code.** → From 500 `IO_ERROR` to 400
`INVALID_EPIC` / `INVALID_COLOR`. No client can have depended on the old shape: the
viewer discarded it and the CLI never reaches the HTTP layer. Existing server tests
that assert a 500 on these inputs — if any exist — flip to 400.

**Removing a child while the filter is focused on its epic makes it disappear.** →
Accepted, and correct: the card no longer belongs to the focused scope. The modal stays
open on a card that is no longer in the filtered board, which is the same situation as
moving a card to a filtered-out column today.

## Migration Plan

No migration. No schema change, no wire field, no stored data touched. A user upgrading
gets editing controls in the modal and two error codes that previously arrived as 500s.
Downgrading loses the controls and leaves every relation written by them intact — they
are the same `epic` and `color` values the CLI writes.

## Open Questions

- Should the picker offer "create a new card as this epic's child"? It is the obvious
  next affordance and the reason to build a real combobox rather than a listbox, but it
  needs a column to create into, which is a second decision inside a picker.
- Should detach ask for confirmation? It is one field, restorable by reassigning, and
  the rest of the modal confirms nothing except delete. Starting without.
- Should the child rows in the `Children` section be reorderable? File order is the
  only order children have, and change 1 named per-epic reordering a non-goal. Worth
  revisiting only if a real board makes the order feel arbitrary.
- Does the `Epic` section on a card with no relation need to be visible at rest, or
  should it be revealed by a smaller affordance? Always-visible is the starting point;
  a real board with many relation-free cards is the test.
