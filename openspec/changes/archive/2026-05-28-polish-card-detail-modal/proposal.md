## Why

The card detail modal is flat: no section labels, fields look passive, and the priority cell shows
a plain `no priority` span that does not look interactive. Users do not know what is editable
without clicking blindly. The footer's column / created / updated trio uses raw ISO strings,
which is hard to read at a glance and missing time-of-day. Column changes still require a drag
on the board even when the modal is open.

This change polishes the modal so the editable structure is discoverable on first glance, adds
column move from inside the modal, gives priority a colored-chip affordance, and rewrites the
footer to surface human-readable relative dates (with absolute date+time in the tooltip).

## What Changes

- Add small uppercase section labels above each editable field (Title, Description, Priority,
  Column, Tags) so users know what they are editing.
- Replace the plain priority span with a chip-style control: colored dot + label + caret. The
  rendered cell still enters the `<select>` editor on click — the click-to-edit contract is
  unchanged.
- Add a new Column field: chip-style trigger that opens the same `<select>` pattern listing every
  configured column. Selecting a different column fires `POST /api/cards/{id}/move`.
- Priority and Column triggers do NOT span the full modal width; they sit on a two-column meta
  grid that caps their width so the rendered control reads as a control, not a panel.
- Description: empty state renders as a dashed-border placeholder ("Add a description") that
  visibly differs from a real value. The redundant "Modifier" affordance from the HTML reference
  is NOT added — clicking the description still enters edit mode.
- Title: bumped to a heading-style ramp with a subtle hover background hinting at editability.
- Header: `<header>` shows only the monospace card id (clickable to copy, as today). The "CARD"
  prefix label from the HTML reference is dropped per request. A trash icon to the right of the
  id deletes the card after a confirm.
- Footer: split into two grouped items — Created / Updated — each rendering a relative string
  ("2 h ago", "just now") with the absolute `YYYY-MM-DD HH:MM` in the tooltip. Items separated by
  a thin vertical rule. Column moves out of the footer (now its own editable field).
- Overlay gains a backdrop blur + soft radial gradient. Modal gains a deeper pop shadow.
- All copy is English (the HTML reference was French; project is English-only).
- All colors reference existing design tokens (`--surface`, `--surface-2`, `--border`,
  `--text`, `--text-muted`, `--text-faint`, `--accent`, `--danger`). Priority dot colors reuse
  the per-priority colors already published by `/api/board` (`board.priority_colors`). No new hex
  literals outside the `:root` / `[data-theme="dark"]` token blocks.

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `viewer-ui`: card detail modal markup, behavior, and visual hierarchy. Adds section labels,
  priority chip affordance, column-move field, polished footer with relative dates, and a delete
  action in the modal header. Removes the read-only `column` row from the footer.

## Impact

- `internal/server/web/index.html` — modal markup restructured (sections, labels, priority/column
  controls, footer with relative dates, header trash).
- `internal/server/web/style.css` — new classes (`.modal-section`, `.modal-label`, `.modal-prio`,
  `.modal-col`, `.modal-foot-*`) plus tweaks to existing modal selectors. All via tokens.
- `internal/server/web/app.js` — Alpine component gets: column-move action calling
  `POST /api/cards/{id}/move`, delete-card action calling `DELETE /api/cards/{id}`, and a
  `formatRelative(iso)` helper for the footer.
- No backend changes. `/api/board`, `PATCH /api/cards/{id}`, `POST /api/cards/{id}/move`, and
  `DELETE /api/cards/{id}` already exist.
- No data migrations.
