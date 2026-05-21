## Why

V3 ([add-card-inline-edit](../archive/2026-05-21-add-card-inline-edit/))
shipped the edit modal as an "always-open form": clicking a card
revealed inputs for every field at once, gated behind a global Save
button and a Cancel button that discarded the whole draft. This works,
but it conflicts with the Redacto editorial design language adopted in
the UI redesign batch (see
[ADR §D3](../../decisions/0003-ui-redesign-batch.md)): cards are
read-mostly, edits are small and local, and the form-style UI buries
the content under controls.

Phase UI-5 reshapes the existing modal into a Trello-style detail
view: rendered values at rest, click any field to enter inline edit
mode, blur or Enter commits that one field via PATCH, Escape reverts.
No global Save, no Cancel, no all-or-nothing draft. The modal itself
stays — ADR §D3 explicitly overrides `refs/design.md`'s "no modals"
rule for the card-detail case because a long description and read-only
metadata want a dedicated surface — but the modal's *contents* now
follow the Redacto direct-manipulation principle: every interaction
commits exactly what it claims to commit.

Behaviorally this is a modification of two V3 requirements
("Modal saves via PATCH" and the modal-inputs pre-fill requirement),
not new capability. Zero new server endpoints — the existing
`PATCH /api/cards/:id` contract (ADR 0002 §D8: present key replaces,
absent key untouched) is exactly the shape this UI needs.

## What Changes

- Modal markup is restructured around per-field `<span class="field">`
  rendered cells with inline editor swaps via `x-show` on
  per-field `editing.<name>` flags (Alpine sub-component pattern,
  [ADR §D10](../../decisions/0003-ui-redesign-batch.md)). At rest each
  field shows the value as text; clicking enters edit mode for that
  field only.
- Field state machine per field: `rendered` → `editing` →
  `saving` → (`rendered` on success | `error` on failure). One field at
  a time enters `editing`; clicking a different field while one is
  editing first commits (blur) the active one.
- Per-field commit semantics: blur or Enter sends `PATCH /api/cards/:id`
  with **exactly one key** (the field that changed). On 2xx, the
  field returns to `rendered` with the new value (server response is
  the source of truth — no optimistic update). On non-2xx, the field
  stays in `editing`, the inline editor is preserved, and the server's
  `error.message` renders inline directly under the field.
- `saving` affordance: a CSS-only state (subtle muted background or
  border tint on the field while a PATCH is in flight). No spinner,
  no toast, consistent with the V5-polish exclusion in ADR 0002 / 0003.
- Tag chip editor stays a chip pattern but each `add` and each
  `remove` commits its own PATCH immediately (one tag operation = one
  PATCH with the resulting `tags` array as the single key).
- Priority renders as inline text at rest (`high`, `medium`, `low`, or
  the literal `no priority`); click swaps to a `<select>`; blur
  commits.
- Description renders as a multi-line plain-text block; click swaps to
  a `<textarea>`; blur or `Cmd/Ctrl+Enter` commits.
- Title renders as a heading-styled span; click swaps to a single-line
  `<input>`; blur or Enter commits.
- Read-only metadata (id, column, created_at, updated_at) stays
  rendered as text — never editable from this modal.
- Cancel button is removed — there is no concept of a "draft to
  discard" since every field commits independently. Save button is
  removed for the same reason.
- Keyboard semantics:
  - `Esc` while a field is in `editing` reverts that field to
    `rendered` (does NOT close the modal).
  - `Esc` with no field in `editing` closes the modal.
  - `Enter` in the title input commits the title.
  - `Cmd/Ctrl+Enter` in the description textarea commits the
    description.
  - `Enter` / `comma` in the tag-add input commits the tag add.
- Modal-overlay close affordances kept: clicking the overlay outside
  the modal closes the modal (no draft → nothing to confirm).
- External-change behavior from V4 unchanged: an `event: board-changed`
  closes the modal without prompting.
- ZERO new endpoints. Reuses `PATCH /api/cards/:id` with the V3 wire
  contract (ADR 0002 §D8).

## Capabilities

### New Capabilities

- _(none — this phase modifies existing capabilities only)_

### Modified Capabilities

- `viewer-ui`: replaces the V3 "modal saves via PATCH on Save click"
  and "modal pre-fills inputs" requirements with click-to-edit
  per-field detail-view semantics; rewires keyboard shortcuts; removes
  the modal's Cancel/Save footer; tag chips commit per add/remove.

### Unchanged Capabilities

- `viewer-server`: `PATCH /api/cards/:id` contract unchanged. The
  endpoint already honors absent-key semantics (ADR 0002 §D8); the
  client now exercises that more aggressively (one key per request).

## Impact

- **Code touched**:
  - `internal/server/web/index.html` — modal markup rewritten:
    persistent inputs replaced with `<span class="field">` rendered
    cells + inline editor templates gated by `editing.<name>`; Save /
    Cancel footer removed.
  - `internal/server/web/app.js` — `saveCard()` split into
    `saveField(name, value)`; new state shape `editing: { title: false,
    description: false, priority: false, tags: false }` and per-field
    `saving: { ... }` / `errors: { ... }` maps; `openCard()` no longer
    seeds a global `draft` (each field reads from `card` directly at
    edit time); Esc handler distinguishes field-revert from
    modal-close; tag `addTag()` / `removeTag(t)` each issue a PATCH.
  - `internal/server/web/style.css` — `.field` rendered hover state
    (subtle background tint, `cursor: text`), `.field--editing` accent
    border, `.field--saving` muted affordance, `.field-error` inline
    message style.
  - `internal/server/server_test.go` — existing PATCH tests still
    pass (no server change). Optionally a structural test asserting
    the rendered modal HTML now contains `field` placeholders, not
    always-open `<input>` elements.
- **APIs / contracts**: none. `PATCH /api/cards/:id` body is now
  typically one key (e.g. `{"title": "..."}` or
  `{"tags": ["a","b"]}`), exercising the V3-pinned semantics.
- **Dependencies**: none.
- **Tests**: server tests unchanged; UI changes asserted via the modal
  markup smoke at the end of the task list.

## References

- [ADR 0003 §D3](../../decisions/0003-ui-redesign-batch.md) — modal
  stays for card detail; Trello-style click-to-edit inside.
- [ADR 0003 §D10](../../decisions/0003-ui-redesign-batch.md) — Alpine
  sub-component pattern for in-modal field state.
- [ADR 0002 §D8](../../decisions/0002-viewer-batch.md) — PATCH wire
  contract (present key replaces, absent key untouched).
- V3 phase (`add-card-inline-edit`, archived) — the form-style modal
  this phase replaces.
- `refs/design.md` — Redacto editorial reference; explicitly overridden
  on the "no modals" rule per ADR §D3.
