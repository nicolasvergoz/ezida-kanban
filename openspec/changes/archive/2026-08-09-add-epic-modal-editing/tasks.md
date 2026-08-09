## 1. Server: typed refusals

- [x] 1.1 Add an `errors.As` arm for `*board.InvalidEpicError` in `httpError` (`internal/server/handlers.go`) → 400 `INVALID_EPIC`, `details: {"epic": ie.ID}`
- [x] 1.2 Add an arm for `*board.InvalidColorError` → 400 `INVALID_COLOR`, `details: {"color": ice.Value}`
- [x] 1.3 Update the `handlePatch` doc comment's error map to name both codes
- [x] 1.4 Test: `PATCH {"epic":"zzzzzz"}` → 400 `INVALID_EPIC`, message names the unknown id, `kanban.toml` byte-unchanged
- [x] 1.5 Test: `PATCH {"epic":"<self>"}` → 400 `INVALID_EPIC`
- [x] 1.6 Test: `PATCH {"epic":"<a card that already carries an epic>"}` → 400 `INVALID_EPIC`
- [x] 1.7 Test: `PATCH {"color":"blue"}` and `PATCH {"color":"#12"}` → 400 `INVALID_COLOR` with the offending value in `details`
- [x] 1.8 Test: `PATCH {"epic":"<valid>"}` → 200, and the target card acquires a hex `color` in the same write
- [x] 1.9 Test: `PATCH {"epic":""}` → 200 with no `epic` key on the returned card
- [x] 1.10 Flip any existing test that asserts 500 on these inputs

## 2. Board: the symmetric nesting rule

- [x] 2.1 In `CheckEpicTarget` (`internal/board/epics.go`), refuse when the child is itself referenced as an epic, returning `*InvalidEpicError` whose `Reason` explains the card has children of its own
- [x] 2.2 Place the new check after the self-reference and unknown-target checks, so the reported reason stays the most specific one
- [x] 2.3 Test: a card with children cannot be given an epic; the board is unmodified
- [x] 2.4 Test: a childless card can still be given an epic
- [x] 2.5 Test: `ezida edit <parent> --epic=<other>` exits non-zero with `INVALID_EPIC` and leaves `kanban.toml` byte-unchanged
- [x] 2.6 Confirm the case now exits `CheckEpicTarget`, not the post-mutation `Validate`

## 3. Client: send the fields, carry the errors

- [x] 3.1 Add `epic` and `color` to `patchCard`'s UI→server key translation in `internal/server/web/app.jsx`, empty string meaning clear
- [x] 3.2 Have `apiSend` attach the parsed error envelope to the thrown `Error` rather than stringifying it into the message
- [x] 3.3 Give the mutation helpers an optional failure callback so a caller can render the message
- [x] 3.4 Log to the console only when the failure is not a `4xx` carrying the envelope
- [x] 3.5 Confirm no existing caller's behaviour changes when it passes no callback

## 4. The card-search combobox

- [x] 4.1 Add an `EpicPicker` component: search input, filtered listbox, highlighted option
- [x] 4.2 Match a card when the query is a case-insensitive substring of the title or a prefix of the id
- [x] 4.3 Render title, id and column per row, so two same-titled cards are distinguishable
- [x] 4.4 Choosing an epic: exclude the edited card and any card carrying an `epic`; choosing a child: exclude the edited card, any card with children, and any card already attached to it
- [x] 4.5 Wire `ArrowDown`/`ArrowUp` to move the highlight and `Enter` to commit
- [x] 4.6 Make `Escape` close the picker and stop propagation so the modal stays open
- [x] 4.7 Close on click outside without committing
- [x] 4.8 Apply `role="combobox"`, `aria-expanded`, `aria-activedescendant`, and a `role="listbox"` on the results
- [x] 4.9 Style it in `styles.css` on top of `.modal-dropdown` / `.modal-dropdown-item`, adding only the search input and the highlight state
- [x] 4.10 Render an explicit empty state when the query matches nothing

## 5. Child side of the modal

- [x] 5.1 Render the `Epic` section on every card, not only on a card carrying one
- [x] 5.2 On a card with a parent: keep the chip and id, add reassign (opens the picker) and detach
- [x] 5.3 On a card with no parent and no children: render the attach affordance in the same place
- [x] 5.4 On a card that has children: render no attach affordance
- [x] 5.5 Commit every choice as a single `PATCH` on the open card, then refetch

## 6. Parent side of the modal

- [x] 6.1 Add an "Add a child" control to the `Children` section, opening the same picker over candidate children
- [x] 6.2 Add a per-row remove control that patches that child with `{"epic":""}`
- [x] 6.3 Confirm no request is ever issued against the parent card to establish or break a relation
- [x] 6.4 Confirm the counter and the progress bar recompute from the refetched board

## 7. Palette swatches

- [x] 7.1 Mirror the eight `EpicPalette` entries (name + hex) into `app.jsx` as a constant
- [x] 7.2 Render the swatch row inside the `Children` section, on a card with children only
- [x] 7.3 Send the hex on click; mark the swatch matching the card's current color as selected
- [x] 7.4 Render an off-palette hex as an additional, selected swatch
- [x] 7.5 Add a clear control sending `{"color":""}`
- [x] 7.6 Give each swatch the palette name as its accessible label and tooltip

## 8. The error region

- [x] 8.1 Add a message region to the modal, fed by the failed mutation's `error.message`
- [x] 8.2 Clear it on the next successful mutation and on close
- [x] 8.3 Place it below the section that failed rather than at the top of the modal
- [x] 8.4 Confirm an expected `4xx` leaves the console clean

## 9. Browser tests

- [x] 9.1 Attach a card to an epic from the child side and assert the chip appears on the board after the refetch
- [x] 9.2 Reassign to another epic and assert the chip's color and label change
- [x] 9.3 Detach and assert the chip disappears and the parent's counter drops
- [x] 9.4 Add and remove a child from the parent side
- [x] 9.5 Drive the picker with the keyboard: filter, arrow, Enter, and Escape leaving the modal open
- [x] 9.6 Force a refusal and assert the message renders and the console stays clean
- [x] 9.7 Click a swatch and assert the children's chip color changes, read via `rgbOf`
- [x] 9.8 Assert with `plain.toml` that the board surface is unchanged, and narrow the guard to the board if it asserts on modal contents

## 10. Demo, docs, verification

- [x] 10.1 Confirm the demo page's new controls fail into the error region instead of throwing, with no server behind them
- [x] 10.2 Add the editing entries to "What the Web UI lets you do" in `docs/usage.md`
- [x] 10.3 Run `./scripts/verify.sh` clean — gofmt, vet, go test, browser tests
- [x] 10.4 Exercise attach, reassign, detach, add-child, remove-child and recolor by hand in both themes — driven headless in both themes with screenshots of each state; a human still has to judge how it looks
- [x] 10.5 Confirm drag, reorder, inline edit, filter, epic focus and delete are all unaffected — the whole suite is green; native HTML5 drag stays outside what the browser tests can drive and needs manual confirmation
