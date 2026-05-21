## Why

UI-1 (`redesign-tokens-and-chrome`) shipped a single source of truth for
viewer chrome: every surface, border, and text color is a CSS custom
property derived from `--bg-base` on `:root`. The viewer is still
light-only, which is jarring at night and ignores the developer's OS
preference. This phase wires the dark half of the token system and the
3-state topbar toggle (Light / System / Dark) prescribed by
`refs/design.md`, completing the "adaptive, system-aware theming"
principle the design system was built for.

System mode is the default — the page follows
`prefers-color-scheme` and re-renders live when the OS flips at dusk.
Manual overrides persist in `localStorage["ezida.theme"]` per ADR 0003
§D7. No server change: the theme is a pure client concern.

## What Changes

- Add dark color tokens (`--bg-base`, `--surface`, `--surface-2`,
  `--border`, `--border-strong`, `--text`, `--text-muted`, `--text-faint`,
  `--accent`) as overrides under `[data-theme="dark"]` in
  `internal/server/web/style.css`. Light tokens stay on `:root` from
  UI-1.
- Add a 3-segment theme toggle to the topbar (right side, after any
  future filter button slot): Sun (light), Monitor (system), Moon (dark).
  Track uses `--surface-2`, full-rounded, 3px padding; each segment is
  26×26 with an inline SVG icon. The active segment gets a `--surface`
  fill and a 1px close shadow.
- Wire an Alpine theme controller (extension of `board()` in
  `internal/server/web/app.js`) that:
  - reads `localStorage["ezida.theme"]` on init, defaulting to
    `"system"` if absent or unreadable;
  - sets `<html data-theme="...">` to the effective mode (`"light"` or
    `"dark"`), resolving `"system"` via
    `matchMedia("(prefers-color-scheme: dark)")`;
  - subscribes to `matchMedia.change` while in system mode so the
    page re-themes live with the OS;
  - writes the user's explicit choice (`"light" | "dark" | "system"`)
    to `localStorage` on toggle click.
- Server-rendered topbar HTML gains the 3 toggle buttons with stable
  `data-theme-choice` attributes and `aria-pressed` reflecting the
  active segment, so the Go server tests can assert their presence.

## Capabilities

### New Capabilities

None. The theme toggle is an extension of the existing viewer-ui
capability, not a new surface.

### Modified Capabilities

- `viewer-ui`: adds the 3-state theme toggle in the topbar, the
  `[data-theme="dark"]` token overrides, system-mode live updates from
  `matchMedia`, and `localStorage` persistence of the user's choice.

## Impact

- New CSS rules under `[data-theme="dark"]` in `style.css` (~25
  lines): one block of dark token values plus the toggle's active/idle
  segment styling.
- New markup in `index.html` (~30 lines): the 3-segment toggle inside
  the topbar's right zone, with inline SVG icons per design.md.
- New code in `app.js` (~40 lines): theme state on `board()`, init
  hook, `setTheme(choice)` method, `matchMedia` listener wiring,
  guarded `localStorage` reads/writes (no throw if storage is blocked).
- New `<html data-theme="...">` attribute set client-side on first
  paint; server renders with no `data-theme` so the initial paint is
  light (no FOUC mitigation in scope — first-paint flash is acceptable
  per ADR 0003 §D14 "light-first, behavior-second" phasing).
- Two new server tests: one asserts `/static/style.css` contains the
  literal `[data-theme="dark"]` selector; one asserts `GET /` body
  contains the 3 toggle buttons with the expected
  `data-theme-choice` values.
- Zero server logic change. No new Go dependencies, no new error
  codes, no new endpoints. CLI behavior unchanged.
- Depends on UI-1 (`redesign-tokens-and-chrome`): the CSS variable
  system on `:root` and the topbar's right-zone container must
  already exist.
