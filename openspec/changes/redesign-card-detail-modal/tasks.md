## 1. Refactor modal markup (HTML)

- [ ] 1.1 In `internal/server/web/index.html`, replace the V3 modal body
  inside `.modal` with the per-field row structure from design MD10.
  Each editable field gets:
  - A rendered `<span class="field">` (or `<p class="field--multiline">`
    for description) with `@click="startEdit('<name>')"` and `x-show="!editing.<name>"`.
  - An inline editor element (`<input>` / `<textarea>` / `<select>`)
    with `x-show="editing.<name>"`, `x-model="drafts.<name>"`, blur
    and Enter / Cmd+Enter / change commit bindings, and
    `@keydown.escape.stop.prevent="revertField('<name>')"`.
  - A `<p class="field-error" x-show="errors.<name>" x-text="errors.<name>">`
    slot directly under the editor.

  Done when `GET /` body contains:
  - the literal substring `class="field-row field-row--title"`,
  - no occurrence of `<footer class="modal-footer">` containing
    `Save` / `Cancel`,
  - the description `<textarea>` carries `@keydown.meta.enter.prevent`
    and `@keydown.ctrl.enter.prevent`,
  - the priority `<select>` carries `@change="commitField('priority')"`.

- [ ] 1.2 Remove the V3 `<footer class="modal-footer">` block (Save +
  Cancel buttons). Done when `grep '>Save<\|>Cancel<' internal/server/web/index.html`
  returns no matches inside the modal block.

- [ ] 1.3 Keep the modal-overlay close affordances:
  `@keydown.escape.window="onEscape()"` on `.modal-overlay` and
  `@click.self="closeModal()"` on the same. Done when the modal still
  closes via overlay click and Esc-with-no-active-field.

- [ ] 1.4 Keep the read-only metadata `<footer class="modal-readonly">`
  block (column, created_at, updated_at) — text only, no editor swap.
  Done when the rendered modal still includes those three values.

- [ ] 1.5 Keep the tag chips block but remove any V3 "Save tags" /
  "Apply" button. Chips and tag-add input stay live (no
  click-to-edit gate). Done when the chip list and tag input render
  unconditionally inside the open modal.

## 2. Refactor Alpine state and handlers (JS)

- [ ] 2.1 In `internal/server/web/app.js`, replace the V3 modal state
  fields (`editing: boolean`, `draft`, `tagInput`, `error`) with the
  per-field shape from design MD1:
  - `open: boolean` (modal visibility)
  - `openCardData: object | null` (the source-of-truth card object)
  - `editing: { title, description, priority, tags }` (booleans)
  - `saving: { title, description, priority, tags }` (booleans)
  - `errors: { title, description, priority, tags }` (strings)
  - `drafts: { title, description, priority }` (in-flight values; tags
    has no draft because the chip editor mutates `openCardData.tags`
    directly via PATCH)
  - `tagInput: string`

  Done when `app.js` defines the new state shape in the returned
  object literal and `go build ./...` passes.

- [ ] 2.2 Implement `openCard(card)`:
  - Copy `card` (shallow + clone tags array) into `openCardData`.
  - Reset all `editing` / `saving` flags to false, all `errors` to `''`.
  - Reset all `drafts` and `tagInput`.
  - Set `open = true`.

  Done when clicking a card visibly opens the modal with the card's
  values rendered as text.

- [ ] 2.3 Implement `startEdit(name)`:
  - Commit any other currently-editing field first (one-at-a-time rule
    from design MD2): iterate `editing`, for the first true entry call
    `commitField(otherName)` and await it.
  - Set `drafts[name]` to `openCardData[name]` (string copy).
  - Set `editing[name] = true`.
  - `$nextTick(() => $refs[refName].focus())` to focus the swapped-in
    editor.

  Done when clicking a rendered field swaps it for its editor and
  focuses the editor.

- [ ] 2.4 Implement `commitField(name)`:
  - Read `drafts[name]`.
  - Call `await this.saveField(name, drafts[name])`.

- [ ] 2.5 Implement `revertField(name)`:
  - Clear `drafts[name]` and `errors[name]`.
  - Set `editing[name] = false`.
  - No PATCH issued.

  Done when pressing Esc on the active editor restores the rendered
  field to the last-saved value and no network call fires.

- [ ] 2.6 Implement `saveField(name, value)` per design MD4:
  - Set `errors[name] = ''`, `saving[name] = true`.
  - `fetch('/api/cards/<id>', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ [name]: value }) })`.
  - On non-2xx: read `error.message` from the JSON envelope (fall back
    to `HTTP <status>`), set `errors[name]`, leave `editing[name]`
    true. Return without closing the editor.
  - On 2xx: parse `{ card }` from the response, replace
    `openCardData` with it, set `editing[name] = false`, clear
    `drafts[name]`, then call `this.load()` to refresh the board
    behind the modal.
  - Always set `saving[name] = false` in `finally`.

  Done when:
  - editing a title and blurring sends a PATCH with body
    `{"title":"..."}` (single key) verified in the network panel,
  - the rendered title updates from the response value,
  - clearing the title and blurring keeps the editor open with the
    server's `MISSING_TITLE` message rendered in `.field-error`.

- [ ] 2.7 Replace V3's `saveCard()` and any caller of it. Remove the
  V3 `closeCard()` semantics that combined "cancel + close"; split
  into `revertField` (per-field) and `closeModal` (modal-level).
  Done when `grep saveCard internal/server/web/app.js` returns no
  matches.

## 3. Esc semantics (field-revert vs modal-close)

- [ ] 3.1 Implement `onEscape()` on the modal overlay:
  - If `Object.values(this.editing).some(v => v === true)`, pick the
    first true entry and call `this.revertField(name)`.
  - Otherwise call `this.closeModal()`.

  Done when:
  - Esc with title editor open reverts the title and modal stays open,
  - Esc with all fields rendered closes the modal.

- [ ] 3.2 Add `@keydown.escape.stop.prevent="revertField('<name>')"`
  on every inline editor element so Esc inside the editor is handled
  locally and does not bubble up to the overlay's `onEscape()`. This
  is a defense-in-depth measure — `onEscape()` would do the same
  thing, but stopping the event at the editor avoids accidental
  modal-close if the bubbling order changes.

  Done when Esc inside the title input reverts only the title and the
  modal stays open in a manual smoke.

- [ ] 3.3 Implement `closeModal()`:
  - Set `open = false`.
  - Clear `openCardData`, reset all `editing` / `saving` / `errors` /
    `drafts` / `tagInput` to their initial values.

  Done when overlay-click and Esc-with-no-active-field both close the
  modal cleanly and re-opening a card shows fresh state.

- [ ] 3.4 Preserve the V4 external-change behavior: when the SSE
  handler fires `event: board-changed`, call `closeModal()` before
  refetching `/api/board`. (This already exists in V4; verify it's
  still wired and now calls the new `closeModal()` and not the old
  `closeCard()`.)

  Done when `grep -n "closeCard\|closeModal" internal/server/web/app.js`
  shows only `closeModal` references.

## 4. Tag chip add / remove direct PATCH

- [ ] 4.1 Implement `addTag()`:
  - Trim `tagInput`. If empty, return (no PATCH).
  - If `openCardData.tags.includes(trimmed)`, clear `tagInput` and
    return (dedup, no PATCH).
  - Otherwise compute `next = [...openCardData.tags, trimmed]`, clear
    `tagInput`, and call `await this.saveField('tags', next)`.

  Done when typing `b` Enter on a card with `tags=["a"]` issues a
  PATCH with body `{"tags":["a","b"]}` and the chip list shows both.

- [ ] 4.2 Implement `removeTag(t)`:
  - Compute `next = openCardData.tags.filter(x => x !== t)`.
  - Call `await this.saveField('tags', next)`.

  Done when clicking the `×` on a chip issues a PATCH with body
  `{"tags": <array without that tag>}` and the chip disappears.

- [ ] 4.3 Confirm tag PATCH rejection is surfaced via `errors.tags`
  (handled automatically by `saveField` from task 2.6). The chip list
  reverts because `openCardData` is replaced only on 2xx.

  Done when typing an invalid tag (e.g. all whitespace, if the server
  rejects it as `INVALID_TAG`) leaves the chip list at the pre-add
  state and shows the server message under the tag row.

## 5. CSS

- [ ] 5.1 In `internal/server/web/style.css`, add `.field-row` (column
  flex container, vertical gap from `--space-xs`) and `.field` styles
  per design MD12. Hover state: subtle background tint
  (`color-mix(--surface ~85%, --accent ~15%)`), `cursor: text`. Done
  when hovering a rendered field shows a visible affordance distinct
  from idle.

- [ ] 5.2 Add `.field--input`, `.field--textarea`, `.field--select`
  styles: 1px accent border, surface background, token-based padding
  and radius. Done when the editors visually pop relative to the
  rendered text.

- [ ] 5.3 Add `.field--saving` style: 0.6 opacity, `pointer-events:
  none`. Done when committing a field briefly dims the editor while
  the PATCH is in flight.

- [ ] 5.4 Add `.field-error` style: `color: var(--danger)`,
  token-based font size, no margin. Done when the inline error renders
  in the danger color directly under the failed field.

- [ ] 5.5 Remove the V3 `.modal-error` rule (modal-level error block).
  Done when `grep "\.modal-error" internal/server/web/style.css`
  returns no matches.

- [ ] 5.6 Remove the V3 `.modal-footer` rule (Save / Cancel button
  row). Done when `grep "\.modal-footer" internal/server/web/style.css`
  returns no matches.

- [ ] 5.7 Confirm all colors / spacing / radii in the new rules come
  from token vars (no hex literals outside `:root` from UI-1). Done
  when `grep -nE '#[0-9a-fA-F]{3,6}' internal/server/web/style.css`
  shows only matches inside `@layer tokens` (or the `:root` block).

## 6. Server tests (unchanged contract; structural smoke)

- [ ] 6.1 Run the existing V3 PATCH tests
  (`go test ./internal/server -run "TestHandle_Patch"`). All MUST pass
  with no modification — this phase does not touch server code. Done
  when the test count matches V3's archive and exit code is 0.

- [ ] 6.2 (Optional) Add a single structural test asserting the
  rendered `GET /` HTML now contains `field-row--title` and does NOT
  contain the V3 `<input>` for the title at modal markup time (i.e.
  the always-open form is gone). Done when the test name (e.g.
  `TestIndex_ModalUsesFieldPlaceholders`) appears in `go test` output.

- [ ] 6.3 (Optional) Add a structural test asserting the rendered
  `GET /` HTML does NOT contain a `<button type="submit">` inside the
  modal block and does NOT contain a Cancel button. Done when the
  test passes.

## 7. Manual smoke (orchestrator-facing)

- [ ] 7.1 Click a card → modal opens with values rendered as plain
  text. No inputs visible until a field is clicked. Done when
  verified.

- [ ] 7.2 Click the title → input swaps in, focused, with current
  value. Edit and blur → PATCH fires with `{"title":"..."}` only →
  rendered title updates from the server response. Done when
  verified.

- [ ] 7.3 Clear the title → blur → editor stays open → inline error
  message appears under the title showing the `MISSING_TITLE` server
  message. Type a valid title → blur → editor swaps back to rendered.
  Done when verified.

- [ ] 7.4 Click the description, type something, press Esc → editor
  closes, rendered description shows the previous saved value, no
  PATCH fired. Done when verified.

- [ ] 7.5 Add a tag via Enter → one PATCH fires with the resulting
  tags array → chip appears. Click the chip's × → one PATCH fires
  with the smaller tags array → chip disappears. Done when verified.

- [ ] 7.6 Change priority via the dropdown → one PATCH fires with
  `{"priority":"..."}` → priority renders as the new value. Select
  `no priority` → PATCH with `{"priority":""}` → field renders `no
  priority`. Done when verified.

- [ ] 7.7 Esc with no field editing → modal closes, nothing changed
  on disk. Done when verified.

- [ ] 7.8 Click overlay outside modal → modal closes, no prompt, no
  pending PATCH issued from this action. Done when verified.

- [ ] 7.9 Open a card, edit title and blur (PATCH fires), then
  external CLI change to the same card → modal closes, board shows
  the merged state per last-write-wins. Done when verified.

## 8. Acceptance gate

- [ ] 8.1 Run `go test ./... && go vet ./...`. Done when exit code is
  0. Existing V3 PATCH tests must still appear in the output
  unchanged. New structural tests (if added in section 6) must
  appear and pass.
