# 0003. UI redesign batch — cross-phase decisions

Date: 2026-05-21
Status: Accepted

## Context

This ADR records the cross-cutting decisions for the `ui-redesign`
batch, which rebrands the viewer UI to the Redacto editorial design
system and layers in interactive features (theme toggle, board filter,
inline card create/delete, Trello-style detail modal, inline column
operations).

The visual truth lives in `refs/design.md` (tokens, typography,
components, do/don'ts). A JSX shell in `refs/kanban-design/` exists as
a static visual reference only — it is not a behavioral spec and is
not imported as code. The behavioral truth remains the existing
OpenSpec specs (`viewer-server`, `viewer-ui`) plus ADR 0002.

The work is split into six phases, each a working, testable
increment:

1. `redesign-tokens-and-chrome` — pure visual rebrand (no behavior
   change).
2. `add-dark-theme` — system-aware theme toggle.
3. `add-board-filter` — client-side filter popover.
4. `add-inline-card-create-delete` — inline composer at column foot +
   hover delete; introduces `POST /api/cards` and
   `DELETE /api/cards/:id`.
5. `redesign-card-detail-modal` — Trello-style click-to-edit fields
   inside the detail modal (replaces V3's always-open form).
6. `add-inline-column-ops` — Add-list placeholder, inline list
   rename, 3-dots menu, drag-reorder list headers; introduces
   `POST /api/columns`, `PATCH /api/columns/:name`,
   `DELETE /api/columns/:name`, `POST /api/columns/move`.

Constraints carried from ADR 0001 and ADR 0002 still apply: single
static binary, `127.0.0.1`-only bind, vendored frontend (no CDN at
runtime), Alpine + Sortable stack, TOML on disk via `internal/board`.

## Decisions

### D1. Visual truth is `refs/design.md`; JSX is fallback reference only

The Redacto design system documented in `refs/design.md` is the
canonical visual specification for every phase. The JSX shell in
`refs/kanban-design/` exists only as a runtime visual reference (open
in a browser to see Redacto vibes) when `design.md` is ambiguous on
a detail. The JSX is **not** imported as code, not ported component
by component, and not consulted for behavior — only for pixels.

**Alternatives considered:**
- Adopt the JSX as React components: rejected — violates ADR 0002 §D5
  (no build step, no Node toolchain).
- Treat `design.md` and JSX as equally authoritative: rejected —
  introduces drift risk and ambiguity about which to follow on
  conflict. Single source of truth wins.

**Affects:** all six phases.

### D2. Stack frozen — Alpine + Sortable, no build step

ADR 0002 §D5 (stdlib + fsnotify + vendored Alpine/Sortable) carries
forward unchanged. The UI redesign ports Redacto patterns into Alpine
templates and vanilla CSS. No bundler, no JSX transpilation, no React
introduced. Token system implemented via CSS custom properties on
`:root` (light theme) and `[data-theme="dark"]` selector overrides.

**Alternatives considered:**
- Add Vite + Preact at dev-time only, embed compiled bundle:
  rejected — adds a Node toolchain that the existing contributor
  workflow does not have, and the binary still ships static assets so
  the runtime gain is zero. Cost without reward.
- Switch to Lit (Web Components, no build): rejected — would require
  rewriting V1-V5 markup just to add tokens; Alpine's `x-data` /
  `x-for` already covers the reactive needs.

**Affects:** all six phases.

### D3. Modal stays for card detail; everything else inline

Redacto design prescribes "no modals anywhere" with double-click
inline edit on the card body. We override this single rule: card
detail and editing happen in a modal opened by single-click on the
card body (kept from V3). Inside the modal, fields use Trello-style
click-to-edit affordances (UI-5). All other interactions follow
Redacto: inline card composer at column foot, inline list rename,
inline tag chips, hover delete, drag-scroll empty board surface.

**Alternatives considered:**
- Full Redacto fidelity (double-click for body inline edit, no
  modal): rejected — the modal pattern is already established with
  V3 spec + tests, and the card detail surface is genuinely a
  read-mostly view (description can be long, metadata is shown). A
  modal is the right shape for it.
- Side panel detail (Trello mobile pattern): rejected — viewport is
  desktop-first per ADR 0002, no responsive design in scope.

**Affects:** UI-1 (modal restyle), UI-4 (inline create/delete
coexist with modal), UI-5 (modal becomes detail+click-to-edit).

### D4. Project name = `filepath.Base` of board path

The topbar brand renders the project name, computed server-side as
`filepath.Base(filepath.Dir(<resolved boardPath>))` — i.e. the
parent-directory name of `kanban.toml`. Falls back to the literal
string `"Ezida"` if the basename is empty or `.`. Surfaced in the
`/api/board` response as a new top-level string field
`project_name`. Hot-reload (SSE) does not re-evaluate this — it is
fixed at server start.

**Alternatives considered:**
- Use git remote / repo name (`git config --get remote.origin.url`):
  rejected — adds a git dependency, breaks for non-git projects,
  ambiguous for monorepos with multiple kanban boards.
- Embed `[board].name` in `kanban.toml`: rejected — adds a schema
  field used in exactly one place; the filesystem already carries
  this metadata for free.
- Client-side via `document.title`: rejected — leaks
  shell/server-side info into the schema by another name.

**Affects:** UI-1 (server adds field, UI consumes it).

### D5. Token system — CSS custom properties, light first

Design tokens land as CSS custom properties on `:root`:

```css
:root {
  --bg-base: #fbfaf8;
  --surface: color-mix(in oklab, var(--bg-base) 35%, white);
  --border: color-mix(in oklab, var(--bg-base) 88%, black 12%);
  /* ... */
  --space-xs: 4px;  --space-sm: 6px;  --space-md: 8px;
  --rounded-lg: 10px;  --rounded-xl: 14px;
  /* ... */
}
```

Dark theme overrides set the same custom-property names under
`[data-theme="dark"]`. Components reference variables exclusively —
no hex literals in `style.css` outside the `:root` blocks. Typography
tokens become utility classes (`.t-brand`, `.t-card`, `.t-mono-counter`)
keyed to the type ramp in `design.md`. UI-1 ships the full token
system (light values) and the utility classes; UI-2 adds the dark
overrides without touching consumers.

**Alternatives considered:**
- Inline SCSS / PostCSS: rejected — needs a build step (ADR 0002 §D5).
- Tailwind via CDN: rejected — runtime network dependency, conflicts
  with vendoring rule.

**Affects:** UI-1 (build the system), UI-2 (extend with dark),
all subsequent UI changes (consume via vars).

### D6. Fonts — Geist + Geist Mono, vendored locally

Geist (300-800) and Geist Mono (400/500) are vendored as `.woff2`
files under `internal/server/web/vendor/fonts/` and embedded via
`//go:embed`. `@font-face` rules in `style.css` reference relative
paths under `/static/vendor/fonts/`. The browser falls back to system
sans-serif while fonts load (no FOIT).

**Alternatives considered:**
- Google Fonts CDN: rejected — runtime network dependency, breaks
  offline mode (ADR 0002 §D5 reasoning).
- System font stack only: rejected — design.md is explicit about
  Geist as the typographic identity, and the editorial feel depends
  on it.

**Affects:** UI-1 (vendor + load).

### D7. Theme persistence — `localStorage["ezida.theme"]`, system default

Theme preference persists in `localStorage` under key
`ezida.theme` with values `"light" | "dark" | "system"`. Default
(no stored value) is `"system"`, which derives from
`matchMedia("(prefers-color-scheme: dark)")`. The 3-state toggle UI
in the topbar writes the chosen value on every click; system mode
listens to `matchMedia.change` events for live OS-level updates.

**Alternatives considered:**
- Server-side persistence: rejected — viewer is per-developer
  per-project, no user model exists, localStorage is the right
  scope.
- Cookie: rejected — no server need to know.

**Affects:** UI-2.

### D8. Filter state — transient, not persisted

The board filter (UI-3) lives in the Alpine component state only.
Closing the popover does not clear the filter; reloading the page
does. Filter is a client-side substring match on card title +
description + tags (case-insensitive). No persistence by design — a
filter is a query, not a preference.

**Alternatives considered:**
- Persist filter in localStorage: rejected — confuses fresh page
  loads with no obvious "filter is active" cue beyond the badge.
- Server-side filter (URL query param): rejected — full board still
  needed for the card count, no bandwidth advantage at single-user
  scale.

**Affects:** UI-3.

### D9. New endpoints reuse existing JSON contract

All endpoints introduced in UI-4 and UI-6 follow the JSON contract
fixed in ADR 0001 §D7 and ADR 0002 §D7: snake_case response payloads,
`{"error": {"code": "...", "message": "...", "details": {...}}}`
envelope on failures, `Content-Type: application/json`. New error
codes minted in this batch:

- `CANNOT_DELETE_LAST_COLUMN` (UI-6, attempt to remove the only
  remaining column).
- `COLUMN_HAS_CARDS` (UI-6, attempt to delete a column that still
  has cards; client must move them first).
- `COLUMN_ALREADY_EXISTS` (UI-6, POST /api/columns with a name that
  collides).

Existing codes (`CARD_NOT_FOUND`, `COLUMN_NOT_FOUND`, `MISSING_TITLE`,
`INVALID_PRIORITY`, `INVALID_TAG`, `INVALID_BODY`, `VALIDATION_FAILED`)
are reused wherever applicable.

**Alternatives considered:**
- Mint per-endpoint custom envelopes: rejected — breaks the single
  contract that makes CLI + viewer parity work.

**Affects:** UI-4, UI-6.

### D10. Inline composer = Alpine sub-component, not a separate file

The card composer (UI-4) and list composer (UI-6) are implemented as
Alpine `x-data` blocks scoped to their parent column / board, not as
extracted partial templates. Reason: the existing `app.js` already
holds the `board()` root component; introducing partials requires
templating infrastructure the stack doesn't have. Composer state
(`composing`, `draft`, `error`) lives on the parent column's data
scope as additional keys.

**Alternatives considered:**
- Extract composers into a separate Alpine `register` plugin:
  rejected — adds plugin lookup overhead and a partial split that
  doesn't pay off at this size.

**Affects:** UI-4, UI-6.

### D11. Drag-scroll surface — listener on `.board`, not body

The "drag empty board surface to scroll horizontally" affordance
(UI-1) attaches a `pointerdown`/`pointermove`/`pointerup` handler
on the `.board` element. The handler activates only when the
`pointerdown.target` is the `.board` itself or a descendant with no
interactive role — clicks on cards, list headers, buttons, and
composers do **not** initiate drag-scroll (event delegation checks
`e.target.closest('.card, .list-header, button, .composer')`).
During an active scroll, `body.classList.add('is-scrolling')`
disables pointer events on `.card` children so the drag is not
hijacked.

**Alternatives considered:**
- Trackpad-style: just enable `overflow-x: auto` and rely on horizontal
  scroll gesture: rejected — Magic Mouse users have no horizontal
  gesture; design.md explicitly mandates drag-to-scroll.
- Map wheel to horizontal scroll: rejected — design.md explicitly
  forbids this ("Don't map mouse wheel to horizontal scroll").

**Affects:** UI-1.

### D12. Column delete safety — refuse non-empty + last column

`DELETE /api/columns/:name` (UI-6) refuses to delete if the column
contains any cards (`COLUMN_HAS_CARDS`) or if it is the only remaining
column (`CANNOT_DELETE_LAST_COLUMN`). The UI surfaces these by leaving
the column in place and showing the error message in the column's
3-dots menu (no toast, no modal). No "force delete" override exists in
v1 — the user must move cards first via drag.

**Alternatives considered:**
- Delete + cascade cards: rejected — destructive and reversible only
  via undo, which doesn't exist.
- Allow delete of empty last column: rejected — leaves the board in
  a degenerate state with no drop target for new cards.

**Affects:** UI-6.

### D13. Card delete safety — soft confirm via hover affordance only

`DELETE /api/cards/:id` (UI-4) commits immediately on click of the
hover-revealed × button. No confirmation dialog, no undo, no
"are you sure". The hover-only affordance is the friction — accidental
clicks on cards open the modal (existing V3 behavior), not delete.

**Alternatives considered:**
- Confirm modal on delete: rejected — adds friction that contradicts
  Redacto's "direct manipulation" principle and the rest of the UI
  has no other confirms.
- Undo toast (5 s revert window): rejected — adds toast infrastructure
  that v1 doesn't have; the design.md non-goal "V5 polish (toasts...)"
  is excluded for the same reason.

**Affects:** UI-4.

### D14. Phase ordering — light-first, behavior-second

The six phases ship in two informal groups:

- **Look-and-feel batch** (UI-1 → UI-2 → UI-3): zero or near-zero
  new behavior, maximum visual ROI. Can stop here and have a
  presentable v1.
- **Interaction batch** (UI-4 → UI-5 → UI-6): each adds a new
  endpoint surface and corresponding inline affordances.

UI-2 / UI-3 / UI-4 / UI-6 all depend only on UI-1's tokens. UI-5
depends on UI-4 (the inline-edit pattern must exist before the modal
can borrow it). Sub-agents applying via `/opsx:batch-apply` should
respect this dependency line; the manifest encodes it.

**Affects:** all six phases.

## Consequences

- Visual identity stabilizes around a single token system; future
  themes (more than light/dark) become cheap.
- The vendored fonts add ~120 KB to the embedded asset payload.
  Acceptable given the binary already includes Alpine + Sortable
  (~55 KB).
- Adding inline create/delete (UI-4) and column ops (UI-6) brings
  the viewer to feature parity with the CLI for the common path.
  The CLI remains canonical for scripting and headless contexts.
- The modal-stays-for-detail decision (D3) means we carry forward
  V3's modal infrastructure. If a later phase decides to drop
  modals entirely, UI-5's click-to-edit work transfers to inline
  body editing with minimal rework.
- Six phases is a lot. The "stop after UI-3" exit ramp is real —
  the look-and-feel batch alone delivers most of the visual goal.
- Adding column endpoints (UI-6) makes the server a fuller authoring
  surface. Concurrent CLI + viewer column edits remain last-write-wins
  per ADR 0002 §D3 — no new locking introduced.

## References

- Brief: inline (this batch was specified in conversation, not in a
  brief file)
- Visual design: `refs/design.md`
- Visual reference shell: `refs/kanban-design/`
- Phases: `redesign-tokens-and-chrome`, `add-dark-theme`,
  `add-board-filter`, `add-inline-card-create-delete`,
  `redesign-card-detail-modal`, `add-inline-column-ops`
- Prior ADRs: `0001-kanban-v1-batch.md`, `0002-viewer-batch.md`
