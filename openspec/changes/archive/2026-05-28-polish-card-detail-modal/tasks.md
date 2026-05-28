## 1. Markup (index.html)

- [x] 1.1 Restructure modal body: wrap each editable field in a `.modal-section`
  group with a preceding `.modal-label` (Title, Description, Priority, Column, Tags).
- [x] 1.2 Replace the priority field's plain span with a chip layout: dot + label + caret SVG,
  keeping the `<select>` editor unchanged.
- [x] 1.3 Add the new Column field-row (label + rendered chip + `<select>` editor) on the same
  two-column meta grid as Priority.
- [x] 1.4 Drop the column line from the read-only footer; split footer into two `.modal-foot-item`
  groups (Created, Updated) separated by a `.modal-foot-sep`.
- [x] 1.5 Header: drop the "CARD" prefix; keep `.modal-id` with click-to-copy; add a
  `.modal-action.danger` trash button to the right.

## 2. Styles (style.css)

- [x] 2.1 Add `.modal-section`, `.modal-label`, `.modal-meta` (2-col grid), `.modal-prio`,
  `.modal-col` selectors using existing tokens. Cap priority/column rendered triggers at the grid
  cell width so they no longer span the modal.
- [x] 2.2 Empty-description state: render `.field--multiline.field--empty` with a dashed border
  and faint italic text so it visually differs from a real value.
- [x] 2.3 Add `.modal-foot`, `.modal-foot-item`, `.modal-foot-label`, `.modal-foot-value`,
  `.modal-foot-sep` selectors. Tighten typography per `.t-mono-label`.
- [x] 2.4 Overlay: switch background to a `color-mix` blend on top of a radial gradient and add
  `backdrop-filter: blur(...)`. Modal: deeper pop shadow.
- [x] 2.5 Header trash button: `.modal-action.danger` token-driven hover (`--danger`).
- [x] 2.6 Title: bump to `.t-list-title`-adjacent ramp with hover background; remove ad-hoc
  font-size on `.field` for the title row.
- [x] 2.7 Verify NO hex literal exists outside the `:root` / `[data-theme="dark"]` token blocks
  in `style.css`.

## 3. Behavior (app.js)

- [x] 3.1 Add `formatRelative(iso)` and `formatAbsolute(iso)` helpers (just now / N min ago /
  N h ago / N d ago / locale short date; absolute `YYYY-MM-DD HH:MM`).
- [x] 3.2 Extend the Alpine modal scope to support column editing: `editing.column`,
  `drafts.column`, `saving.column`, `errors.column`. On commit, fire
  `POST /api/cards/{id}/move` with `{column, position: 0}`; no-op when value unchanged.
- [x] 3.3 Add `deleteCard()` action that calls `window.confirm` and on confirm fires
  `DELETE /api/cards/{id}` then closes the modal.
- [x] 3.4 Wire the new fields' `startEdit/commitField/revertField` paths so they obey the
  existing one-field-at-a-time editing gate.

## 4. Verification

- [x] 4.1 `go build ./...` succeeds (Go embeds the static files).
- [x] 4.2 `go test ./...` passes (no test currently covers the modal markup, but make sure
  nothing regresses). NOTE: pre-existing `TestRun_PortFallback` failure is environmental
  (ports 7778/7779 bound by another `ezida serve` already running on the machine — unrelated
  to this change; no Go code was touched).
- [x] 4.3 Manual: open viewer, click a card, verify section labels, priority chip+dot, column
  selector, tooltip with date+time, delete confirm, id copy, light + dark themes.
