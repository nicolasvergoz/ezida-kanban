## Context

The viewer is a static Alpine.js + vanilla CSS bundle served from `internal/server/web/`. The
detail modal lives inside `index.html` (markup), `style.css` (one `.modal-overlay` /
`.modal` / `.field-row` block), and `app.js` (an Alpine `board()` component that owns
`openCardData`, `editing.*`, `drafts.*`, `saving.*`, and the click-to-edit commit helpers).

A standalone React mockup (`refs/CardDetailModal.standalone.html`) was provided as visual
reference. The mockup is React-only; we keep Alpine. We also reject several of its choices:
the French copy, the "CARD" prefix in the header, the redundant "Modifier" button above
description, the date strings that omit hours, and a few token choices that read poorly in
light mode.

The existing `viewer-ui` spec already pins the modal's outer contract (click opens, fields
render as text first then enter inline editors on click, no global Save/Cancel, Esc closes,
priority editor is a `<select>`, etc.). This change polishes the layout WITHIN those
guarantees and adds two new requirement blocks (column move from modal, delete from modal,
relative-date footer).

## Goals / Non-Goals

**Goals:**
- Make every editable field's structure discoverable on first glance (section labels).
- Give priority a chip-style affordance with a colored dot matching board configuration.
- Surface column move as a first-class field inside the modal, using the SAME click-to-edit
  pattern as priority.
- Replace ISO date strings with relative ("2 h ago") + absolute tooltip in the footer.
- Stop the priority/column controls from spanning the modal's full width.
- Preserve the existing click-to-edit gate, Esc behavior, blur-commit behavior, and tag chip
  exemption.
- Preserve the "id is clickable to copy" affordance from the header.
- All colors via design tokens. No new hex literals outside `:root` / `[data-theme="dark"]`.

**Non-Goals:**
- New backend endpoints. `POST /api/cards/{id}/move` and `DELETE /api/cards/{id}` already exist.
- A new dropdown widget. We keep the native `<select>` editor so the click-to-edit contract from
  the spec stays valid; we only restyle the rendered cell.
- Drag-to-reorder columns from the modal.
- Localization. Project remains English-only.
- Changing card-on-board appearance.

## Decisions

### Decision 1: Restyle the rendered priority cell as a chip but keep `<select>` for editing

The spec (`viewer-ui`, "Click priority enters priority edit mode" scenario) explicitly requires
that clicking the priority field reveals a `<select>` with `<option value="">` `no priority`
plus one option per `[board].priorities`. Switching to a custom listbox would invalidate that
scenario. So:

- Rendered state: `.field.modal-prio` — chip with a colored dot (from `board.priority_colors`
  served by `/api/board`) + label + caret. `:hover` shows a subtle background.
- Editing state: unchanged `<select>` per the spec.
- The dot is rendered with an inline `style="background: <hex>"` (the only place outside the
  token block where a hex value appears, sourced from server config — not a hardcoded literal in
  CSS).

**Alternative considered:** custom listbox dropdown matching the React mockup. Rejected: forces
a spec rewrite + accessibility work for `aria-haspopup`/`aria-expanded`/`aria-activedescendant`
without a clear UX gain over the chip + native `<select>`.

### Decision 2: Column move uses the same chip + `<select>` pattern as priority

A new field-row sits next to priority on a 2-column meta grid. Rendered state shows the current
column title as a chip with a small board icon + caret. Click switches to a `<select>` listing
every column. Choosing a different column fires `POST /api/cards/{id}/move` with
`{column: <new>, position: 0}` (head of the destination list). Same column = no-op.

**Why position 0:** the mockup picker has no "position" UI. Inserting at the top of the
destination column matches what the user just saw on screen (the card now reads as the
most-recently-touched card in that column) and matches how `addCard` already behaves.

### Decision 3: Footer rewrite — relative strings + tooltip with absolute date+time

`updated_at` / `created_at` arrive as RFC3339 strings from the server. New `formatRelative(iso)`
helper:

- < 1 min  → "just now"
- < 60 min → "N min ago"
- < 24 h   → "N h ago"
- < 7 d    → "N d ago"
- ≥ 7 d    → `toLocaleDateString` short form ("23 May 2026")

Absolute tooltip via `title=` attribute: `YYYY-MM-DD HH:MM`. Two items only — Created, Updated —
separated by a thin vertical rule. The previous `column: <code>` line moves out of the footer
because column is now an editable field.

### Decision 4: Header strips the "CARD" label; keeps id-as-button, adds trash icon

Per the prompt. The id span is the existing `.modal-id` (mono, clickable, copy-on-click). A
new `.modal-action.danger` button sits to its right showing a trash SVG; click triggers
`window.confirm("Delete this card?")` then `DELETE /api/cards/{id}`. Same affordance as the
hover-revealed delete on the card itself, just available from inside the modal.

### Decision 5: Token mapping (light + dark)

We do NOT introduce new tokens. Mapping from the React mockup's improvised values to existing
project tokens:

| Mockup color | Project token used |
|---|---|
| `surface` (modal bg) | `--surface` |
| `surface-2` (header / footer / chip bg) | `--surface-2` |
| `border` | `--border` |
| `border-strong` | `--border-strong` |
| `text` | `--text` |
| `text-muted` | `--text-muted` |
| `text-faint` (uppercase labels) | `--text-faint` |
| `accent` (focus ring) | `--accent` |
| danger button hover | `--danger` |
| Priority dot | `board.priority_colors.<id>` from `/api/board` (server-supplied hex) |

The light-mode failure in the React mockup was caused by `color-mix(... oklch ...)` blending
against an off-white `--bg-base`. Using the project's existing `--surface`/`--surface-2` pair
(which are pre-resolved hex in `:root`) avoids that.

### Decision 6: Section labels reuse the existing `.t-mono-label` typography utility

Per the `viewer-ui` "Typography utility classes" requirement, ad-hoc font declarations on
components must be avoided. `.modal-label` composes `.t-mono-label` (Geist Mono, 10 px,
uppercase, letter-spacing) on the markup side. Selector-side, `.modal-label` only sets
`color: var(--text-faint)` and `margin-bottom: var(--space-xxs)`.

## Risks / Trade-offs

- [Adding a `<select>` for column on every card open inflates DOM slightly] → Negligible; columns
  count is bounded (configured list in `kanban.toml`, typically 3–6) and is rendered via Alpine
  `<template x-for>` so the select exists only while editing.
- [Tooltip-only absolute date is a regression for users who copy-paste it] → Mitigated by
  keeping the absolute string easily readable on hover and matching the format used in card
  hover tooltips elsewhere.
- [`formatRelative` runs on render and won't tick] → Acceptable; the modal is short-lived and
  the SSE stream will replace `openCardData` on any external write anyway.
- [Delete from inside the modal can surprise users] → Mitigated by `window.confirm` (same gate
  as the existing hover-delete on the card).
- [Priority chip background derived from server-supplied hex] → No CSS rule references a hex
  literal; the value lives only on an inline `style` attribute injected at render time, which
  the existing spec's "components consume tokens, not hex" scenario covers (it inspects
  `style.css`, not inline attributes).

## Migration Plan

- Pure frontend change. Refresh the page; new modal renders.
- No data migration. Existing cards work unchanged.
- Rollback = revert the change set; specs/api unchanged so older clients keep working.
