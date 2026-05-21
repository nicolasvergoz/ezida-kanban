## Context

The viewer's V1-V4 stylesheet is functional but visually flat —
system fonts, single-shade surfaces, no token system. Phase UI-1 of
the redesign batch lands the visual foundation that every subsequent
phase consumes: design tokens, vendored typography, surfaces, and
chrome restyling. Cross-cutting decisions for the whole batch live in
[`openspec/decisions/0003-ui-redesign-batch.md`](../../decisions/0003-ui-redesign-batch.md);
this document captures only the per-phase how.

Constraints carried in:

- Stack frozen: Alpine + Sortable + vanilla CSS, no build step
  ([ADR §D2](../../decisions/0003-ui-redesign-batch.md)).
- Vendored assets only — no CDN at runtime
  ([ADR 0002 §D5](../../decisions/0002-viewer-batch.md)).
- Verify gate is `go test ./... && go vet ./...` (from
  `openspec/config.yaml`).

Visual truth is [`refs/design.md`](../../../refs/design.md); the JSX
shell in `refs/kanban-design/` is consulted only when `design.md` is
ambiguous on a detail
([ADR §D1](../../decisions/0003-ui-redesign-batch.md)).

## Goals / Non-Goals

**Goals:**

- Land the full CSS custom-property token system on `:root` (light
  values only) keyed to the Redacto palette / spacing / rounded /
  shadow ramps.
- Vendor Geist + Geist Mono and ship them via `//go:embed` so the
  binary stays self-contained.
- Restyle every existing UI element (topbar, columns, cards, tags,
  priorities, modal, loading, empty-column) using the new tokens.
- Add the drag-to-scroll affordance to the empty board surface
  ([ADR §D11](../../decisions/0003-ui-redesign-batch.md)).
- Surface the project name from the server and bind it to the topbar
  brand ([ADR §D4](../../decisions/0003-ui-redesign-batch.md)).

**Non-Goals:**

- Dark theme. Token names land here; overrides land in
  [add-dark-theme](../add-dark-theme/).
- Filter button / popover. Land in
  [add-board-filter](../add-board-filter/).
- Theme toggle UI. Lands in [add-dark-theme](../add-dark-theme/).
- Inline card create / delete. Land in
  [add-inline-card-create-delete](../add-inline-card-create-delete/).
- Trello-style click-to-edit inside the modal. Lands in
  [redesign-card-detail-modal](../redesign-card-detail-modal/).
- Column ops (add list, rename, drag headers, delete). Land in
  [add-inline-column-ops](../add-inline-column-ops/).

## Decisions

### Phase-D1. Token file = single `style.css`, layered with `@layer`

The token system lands in the existing `internal/server/web/style.css`
(no separate `tokens.css`). The file is reorganized into three CSS
`@layer` blocks for readability and override predictability:

1. `@layer tokens` — `:root { --bg-base, --surface, --border,
   --border-strong, --text, --text-muted, --text-faint, --accent,
   --accent-soft, --danger, --space-xxs..--space-4xl,
   --rounded-xs..--rounded-full, --shadow-card-idle,
   --shadow-card-hover, --shadow-popover, --shadow-card-rim,
   --grain-opacity, --topshade-strength }`. All hex literals exist
   here and only here.
2. `@layer typography` — `@font-face` rules + `.t-brand`,
   `.t-list-title`, `.t-card-text`, `.t-body-md`, `.t-button`,
   `.t-tag`, `.t-mono-counter`, `.t-mono-label` utility classes.
3. `@layer components` — every selector that styles the topbar,
   board, column, card, tag chip, modal, etc. References tokens
   exclusively via `var(--...)`. **No hex literal allowed outside
   `@layer tokens`** (enforceable by visual review of the diff in
   the smoke task — `rg '#[0-9a-fA-F]{3,6}'` outside the tokens
   block is the manual check).

This satisfies [ADR §D5](../../decisions/0003-ui-redesign-batch.md)
without splitting the asset into multiple files (avoiding embed-list
churn).

**Alternatives considered:**

- Separate `tokens.css` + `typography.css` + `components.css`:
  rejected — adds three new embed entries and three HTTP requests on
  page load. `@layer` gives the same readability gain in one file.
- Inline `<style>` tag in `index.html`: rejected — breaks browser
  cacheability and makes the file genuinely hard to read.

### Phase-D2. Fonts vendored at `web/vendor/fonts/`

Geist (300, 400, 500, 600, 700, 800) and Geist Mono (400, 500) ship as
`.woff2` files under `internal/server/web/vendor/fonts/`. The existing
`//go:embed web` directive in `internal/server/embed.go` already
embeds the subtree recursively, so no Go change is required for
embedding — the files just need to exist in the tree.

`@font-face` rules in `style.css` use relative paths
(`url('/static/vendor/fonts/Geist-Regular.woff2')`), `font-display:
swap` (browser falls back to system sans-serif while loading — no
FOIT), `font-weight: <weight>`, `font-style: normal`, `unicode-range:
U+0000-00FF` is not constrained (full Latin coverage shipped).

Source: the upstream Vercel/Geist repo provides `.woff2` files in
their open-source distribution (OFL license). The fonts are added to
the repo verbatim; no transformation.

**Alternatives considered:**

- Google Fonts CDN: rejected
  ([ADR §D6](../../decisions/0003-ui-redesign-batch.md)) — breaks
  offline mode.
- System font stack: rejected
  ([ADR §D6](../../decisions/0003-ui-redesign-batch.md)) — design.md
  is explicit about Geist as the typographic identity.
- Variable font (`Geist-Variable.woff2`): considered but rejected for
  this phase — eight static `.woff2` files (six Geist weights + two
  Geist Mono weights) total ~120 KB vs. ~95 KB for the variable
  variant, the gain is marginal, and explicit per-weight rules make
  hover/active state debugging simpler.

### Phase-D3. Project name resolution = `filepath.Base(filepath.Dir(absPath))` at boot

`projectName` is computed exactly once, in
`runWithContext` (`internal/server/server.go`), after the boardPath is
resolved. The pipeline:

```go
abs, err := filepath.Abs(boardPath)
if err != nil { abs = boardPath } // best-effort; never blocks boot
name := filepath.Base(filepath.Dir(abs))
if name == "" || name == "." || name == string(filepath.Separator) {
    name = "Ezida"
}
state := &serverState{boardPath: boardPath, projectName: name, broker: broker}
```

The new field lives on `serverState` and is read by `handleBoard`
when assembling the response. It is **immutable** for the lifetime of
the process — neither `fsnotify` events nor user-driven writes
re-evaluate it
([ADR §D4](../../decisions/0003-ui-redesign-batch.md)).

The fallback string `"Ezida"` is the brand name. It triggers in three
edge cases:

1. `boardPath` resolves to the root of a filesystem (rare in
   practice).
2. `boardPath` is a relative path with no directory component
   (`"kanban.toml"`) AND `filepath.Abs` fails (very rare). The
   normal case — `filepath.Abs("kanban.toml")` succeeding — yields a
   real parent directory.
3. The parent directory is named `"."` (only possible via crafted
   paths).

**Alternatives considered:**

- Re-evaluate `projectName` on every `/api/board` request: rejected —
  the value cannot change without restarting the server (board path
  is fixed at boot), so per-request work is wasted. Plus
  [ADR §D4](../../decisions/0003-ui-redesign-batch.md) makes the
  immutability explicit.
- Surface as an env var (`EZIDA_PROJECT_NAME`): rejected — adds a
  configuration surface for a value the filesystem already encodes.

### Phase-D4. Modal restyle = same DOM, new chrome

The V3 modal markup is preserved verbatim (same `.modal-overlay >
.modal > form > label/input` structure). Only CSS changes:

- `.modal-overlay` gets the new backdrop tint (10% bg-base × 100%
  width, no blur — keep the readability of cards behind it).
- `.modal` gets `surface` fill, `--rounded-xl` (14px) corners,
  `var(--shadow-popover)` (`0 12px 32px -8px` plus `0 2px 8px`
  ambient) — reads as floating above the column.
- `.modal-header h2` adopts `.t-list-title` (uppercase, tracked).
- Form labels / inputs / textarea / select stay structurally as V3
  ships them (label-wraps-input pattern), but inputs gain:
  - `--rounded-md` (8px) corners.
  - `1px solid var(--border)` resting, `1px solid var(--accent)` on
    focus (no native browser ring — `outline: none` + the accent
    border replaces it).
  - `.t-body-md` typography.
- Tag chips inside the modal adopt the new Redacto chip skin
  (rounded-full, 11px Geist 500, text-faint at rest) **but remain
  readonly visual mark in this phase — the chip's `×` remove button
  retains V3 behavior since the modal already had editable tags**.
  Wait — clarification: V3 already has editable chips inside the
  modal. This phase keeps that V3 behavior unchanged (chip click
  removes a tag, Enter in input adds one) — only the visual skin
  changes. The "tag chips remain readonly here" note in the brief
  refers to the *card surface* (tags on the card body in the board
  view stay non-editable). UI-5 reworks both surfaces.

**Alternatives considered:**

- Rewrite modal as a Trello-style click-to-edit detail view: rejected
  — that is the whole job of
  [redesign-card-detail-modal](../redesign-card-detail-modal/) (UI-5).
  Doing it here would explode the diff.

### Phase-D5. Drag-scroll handler = pointer events on `.board`, not body

The drag-scroll affordance attaches three pointer listeners to the
`.board` element (the `<div class="columns">` parent, renamed to
`.board` per Redacto):

```js
// app.js — wired in load() after first render via $nextTick.
let isDragging = false, startX = 0, startScroll = 0;
const board = this.$el.querySelector('.board');
board.addEventListener('pointerdown', (e) => {
  if (e.button !== 0) return;
  // Bail if pointerdown landed on an interactive descendant.
  if (e.target.closest('.card, .column-header, button, input, textarea, select, .modal')) return;
  if (e.target !== board && !e.target.classList.contains('board')) {
    // Allow drag-scroll when target is the board itself or a non-
    // interactive direct child (e.g. an empty-state spacer).
    // Otherwise bail to preserve native click semantics.
    return;
  }
  isDragging = true;
  startX = e.clientX;
  startScroll = board.scrollLeft;
  document.body.classList.add('is-scrolling');
  board.setPointerCapture(e.pointerId);
});
board.addEventListener('pointermove', (e) => {
  if (!isDragging) return;
  board.scrollLeft = startScroll - (e.clientX - startX);
});
board.addEventListener('pointerup', (e) => {
  if (!isDragging) return;
  isDragging = false;
  document.body.classList.remove('is-scrolling');
  board.releasePointerCapture(e.pointerId);
});
```

While `body.is-scrolling`, `.card { pointer-events: none }` so the
drag gesture is not hijacked by a child clickable element mid-drag
([ADR §D11](../../decisions/0003-ui-redesign-batch.md)). The
`is-scrolling` class is also removed in a `pointercancel` listener
for safety (e.g. system-initiated cancellation on touch devices —
desktop-only per scope, but cheap to add).

The handler is wired once after the first successful `load()` (the
`.board` element exists only after `loaded` flips true). Because
Alpine reuses the same `<div class="board">` across refetches (it is
keyed by the outer `x-data`, not by `x-for`), the listener does not
need re-binding.

**Alternatives considered:**

- Use mouse events (`mousedown`/`mousemove`/`mouseup`): rejected —
  pointer events unify mouse + pen + touch with no extra code, and
  `setPointerCapture` is the cleanest way to keep events flowing when
  the cursor leaves the element mid-drag.
- Attach to `document` and filter by target: rejected — leaks the
  handler beyond the viewer's surface and risks interfering with
  modal interactions.

### Phase-D6. Embed structure unchanged

`internal/server/embed.go` already declares `//go:embed all:web` (or
equivalent — verify before editing). Adding files under
`web/vendor/fonts/` requires no Go change because the embed directive
already recurses. If the directive is the older `//go:embed web` form
which excludes files starting with `.` or `_`, the font files are
plain `.woff2` and are picked up regardless.

The implementer must confirm the existing directive recurses into
nested subdirectories (a quick `go build && go test ./...` after
adding the first font file is the proof).

## Risks / Trade-offs

- **Risk**: Font payload bloats the binary by ~120 KB.
  → Mitigation: accepted in
  [ADR 0003 Consequences](../../decisions/0003-ui-redesign-batch.md);
  binary already carries Alpine + Sortable (~55 KB), this is the same
  order of magnitude.
- **Risk**: Visual regression in the modal — V3 tests assert DOM
  structure (e.g. `<header class="modal-header">`), but CSS changes
  could subtly break responsive layout on narrow viewports.
  → Mitigation: viewport is desktop-first per
  [ADR 0002 §D2](../../decisions/0002-viewer-batch.md); the final
  smoke task includes a Chrome MCP screenshot at 1280×800 to confirm
  modal renders correctly.
- **Risk**: `filepath.Base(filepath.Dir(abs))` returns `"/"` or `""`
  for boards at the filesystem root.
  → Mitigation: explicit fallback to `"Ezida"` for `""`, `"."`, and
  the platform separator. Verified by a unit test fixture using a
  temp dir whose name is intentionally `"."` (synthesized via direct
  `filepath.Base` test, not via real fs).
- **Trade-off**: `@layer` requires a relatively modern browser
  (Chrome 99+, Safari 15.4+, Firefox 97+).
  → Acceptable: the viewer is dev-tooling on a developer's machine;
  modern browser baseline matches the rest of the stack.
- **Trade-off**: All hex literals concentrated in one place means a
  future theme change (beyond light/dark) touches a single block —
  good for maintainability. The cost: linting "no hex outside
  tokens" is manual (no CI rule for it). Accepted because the file
  is small and reviewed every phase.
- **Risk**: Drag-scroll fights with Sortable's drag-start when the
  pointer lands on a card.
  → Mitigation: the `closest('.card, ...')` bail in the
  `pointerdown` handler ensures the listener returns early before
  Sortable's own listeners fire on the card. Sortable continues to
  work normally.

## Migration Plan

This phase is additive on the server side (one new field) and a
pure CSS / asset replacement on the UI side. No state migration, no
config migration, no data migration.

**Rollback**: revert the commits. The new `project_name` field is
purely informational — clients ignoring it are unaffected. CSS and
fonts are bundled in the binary; an older binary serves the old
viewer correctly.

## Open Questions

- None. The brief and the ADR fully constrain this phase.
