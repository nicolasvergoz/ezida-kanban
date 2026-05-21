## Context

UI-1 lands the token system on `:root` per ADR 0003 §D5: every
component reads `var(--bg-base)`, `var(--surface)`, `var(--text)`, etc.
No component embeds a hex literal. The viewer is therefore a single
selector swap away from a second theme — the work in this phase is to
write the dark values, wire a toggle, and persist the user's choice
exactly as ADR 0003 §D7 prescribes.

The visual specification for the toggle is locked by `refs/design.md`
§"Theme Toggle" (3 pill segments, `--surface-2` track, full-rounded,
3px padding, 26×26 segments, active segment gets `--surface` fill and
a 1px close shadow). The dark palette is locked by `refs/design.md`
§"Colors" (bg-base-dark `#25282e`, with all derived surfaces computed
via `color-mix`).

The stack is fixed by ADR 0002 §D5 / ADR 0003 §D2: Alpine + vanilla
CSS, no build step, no framework swap. Sortable is unaffected — the
theme switch is a pure style + data-attribute operation.

## Goals / Non-Goals

**Goals:**

- Provide three explicit theme states (light, system, dark) selectable
  from the topbar.
- Default to `"system"`, deriving from
  `matchMedia("(prefers-color-scheme: dark)")` on first paint.
- Update live when the OS theme changes while the user is in system
  mode (e.g. macOS auto night-shift at dusk).
- Persist the user's explicit choice across reloads via
  `localStorage["ezida.theme"]`.
- Add dark token values as `[data-theme="dark"]` overrides without
  touching any consumer rule from UI-1.
- Fail gracefully when `localStorage` is unavailable (private browsing,
  storage blocked): in-memory state still works for the session.

**Non-Goals:**

- Server-side theme awareness, persistence, or cookie. The viewer is
  per-developer per-machine; localStorage is the right scope (ADR 0003
  §D7).
- Avoiding the first-paint light flash for users whose stored choice
  is `"dark"`. A blocking inline script in `<head>` would prevent it
  but adds a non-Alpine code path; the phase ordering note in ADR 0003
  §D14 accepts the flash for now.
- New themes beyond light/dark (high-contrast, sepia, brand variants).
  The system supports them via additional `[data-theme="..."]` blocks,
  but none are in scope.
- Filter button, inline composer, modal redesign — those belong to
  UI-3 / UI-4 / UI-5.

## Decisions

### TD1. Selector strategy — `[data-theme="dark"]` on `<html>`

Dark token values override the light values via a single rule block
keyed on `html[data-theme="dark"]` (selector written as
`[data-theme="dark"]` for brevity; specificity matches `:root` exactly
since both are single class/attr selectors on the root, but the
attribute selector wins on cascade order). The Alpine controller
writes the attribute on `document.documentElement`.

Why `<html>` not `<body>`: CSS variables on `<html>` cascade to every
element including content rendered outside `body` (none currently, but
future Alpine teleport / portals would inherit). It is the
conventional spot.

Why attribute selector and not a class: classes are easier to inspect
in DevTools, but the attribute form `data-theme="..."` extends
naturally to future modes (`"high-contrast"`, etc.) without name
collisions with utility classes like `.t-card`.

Affects: `style.css` (new rule block), `app.js` (writes the attribute).

### TD2. Effective mode derivation

The controller distinguishes two values:

- `choice` ∈ `{"light", "system", "dark"}` — what the user selected
  (the persisted value, drives toggle UI's `aria-pressed`).
- `effective` ∈ `{"light", "dark"}` — what the page actually
  renders (drives the `<html data-theme="...">` attribute).

Resolution:

```
effective = (choice === "system")
  ? (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
  : choice
```

Why two values: the toggle must show System as active even when the
page renders dark, and `matchMedia` may flip while the user keeps
their System choice unchanged.

### TD3. matchMedia listener wiring

The controller stores the `MediaQueryList` once
(`window.matchMedia("(prefers-color-scheme: dark)")`) and registers a
`change` listener at init. The listener calls `applyTheme()`, which
re-computes `effective` from the current `choice`. When `choice` is
`"light"` or `"dark"`, the listener is a no-op (effective is fixed).
The listener is never removed during the page's lifetime — the page
itself dies on reload.

Why not add/remove the listener on choice change: bookkeeping cost
outweighs the no-op. `addEventListener` once and re-check inside is
simpler and correct.

### TD4. Persistence with graceful localStorage failure

Read/write are guarded by `try/catch`:

- Read at init: `try { return localStorage.getItem("ezida.theme") }
  catch { return null }`. `null` (key absent OR storage unreadable)
  resolves to `"system"`.
- Write on toggle click: `try { localStorage.setItem("ezida.theme",
  choice) } catch { /* swallow */ }`. The in-memory `choice` state
  still drives the UI for the rest of the session.

Stored values are validated: only `"light"`, `"dark"`, `"system"`
are accepted. Anything else is treated as absent and resolves to
`"system"`.

### TD5. Controller lives on `board()`, not as a separate component

Per ADR 0003 §D10 (composers as `x-data` on the parent), additional
viewer behavior extends `board()` instead of introducing parallel
Alpine roots. The controller adds:

- State: `theme` (the `choice`), `themeEffective` (derived, used by
  toggle's `aria-pressed` if needed).
- Init hook: read storage, set `theme`, wire matchMedia listener, call
  `applyTheme()` once.
- Method: `setTheme(choice)` — validate, store the new value, persist
  to localStorage, call `applyTheme()`.
- Method: `applyTheme()` — compute effective, write
  `document.documentElement.dataset.theme = effective`.

Why on `board()` and not a tiny standalone Alpine block: the topbar
markup is already inside the `board()` scope from UI-1 (the project
name comes from `boardData.project_name`). Re-using the same scope
avoids cross-component event plumbing.

### TD6. Token override block stays minimal — no consumer rules touch

The `[data-theme="dark"]` block contains only token reassignments. No
component selector (`.card`, `.list-header`, `.topbar`, etc.) gets a
dark-mode variant. This matches ADR 0003 §D5 ("Components reference
variables exclusively"): if a component still hardcodes a hex
literal after UI-1, it is UI-1's bug to fix, not a dark-mode override
to bandage.

The toggle itself is the one component this phase introduces, so its
rules live in the regular CSS namespace and read `var(--surface)`,
`var(--surface-2)`, `var(--text)` like every other component.

### TD7. SVG icons inline, not vendored

The three icons (sun, monitor, moon) are inline SVG in the
`index.html` template. They use `currentColor` for stroke, so the
active segment's `var(--text)` color cascades automatically. No new
vendored asset, no icon font.

Why inline: three icons, ~15 lines each, used once. A sprite sheet
would be over-engineered.

### TD8. First-paint behavior is light + flash for dark users

The server returns HTML with no `data-theme` attribute. The first
paint is therefore the light theme (the `:root` values). The Alpine
controller runs after DOM ready and sets `data-theme="dark"` if
applicable, triggering a re-paint. Dark users see ~50ms of light
flicker on initial load.

This is accepted for UI-2 per ADR 0003 §D14 (look-and-feel batch
trades polish for shipping cadence). UI-2 does not introduce the
flash; it merely surfaces it because dark mode now exists. A later
"polish" change could solve it with an inline head script if user
feedback demands.

## Risks / Trade-offs

- **First-paint light flash for dark users** → accepted per TD8; can
  be revisited with an inline head script if it becomes a complaint.
- **localStorage blocked / corrupted** → guarded with try/catch and
  whitelist validation per TD4; the session still works, the choice
  just doesn't persist.
- **matchMedia listener leak** → not a real risk, the listener lives
  for the page's lifetime and is replaced by reload; no SPA-style
  navigation exists.
- **Token drift between light and dark** → mitigated by keeping the
  dark block as a single rule that overrides the same named tokens.
  CI test (assert `[data-theme="dark"]` literal selector exists) is a
  smoke check; pixel parity is the developer's eye on first load.
- **Active-segment focus ring colliding with the `--surface` fill** →
  the toggle uses `var(--accent)` for `:focus-visible` outlines per
  ADR 0003 §D5 (accent reserved for focus rings); not visually
  ambiguous because the fill is surface and the ring is accent.
- **Three states are more than the typical two** → users who only
  want "follow OS" never have to touch the toggle (system is
  default); users who want a fixed mode click once and forget. The
  three-state model is locked by design.md.
