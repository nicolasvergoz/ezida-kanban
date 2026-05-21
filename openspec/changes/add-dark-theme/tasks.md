## 1. Dark token block in CSS

- [ ] 1.1 In `internal/server/web/style.css`, add a `[data-theme="dark"]` selector block directly below the existing `:root` token block from UI-1. Reassign the same custom-property names (`--bg-base`, `--surface`, `--surface-2`, `--border`, `--border-strong`, `--text`, `--text-muted`, `--text-faint`, `--accent`) to the dark-theme values from `refs/design.md` §"Colors" (bg-base-dark `#25282e`, surface-dark `#33363d`, surface-2-dark `#414448`, border-dark `#454850`, border-strong-dark `#5d5f66`, text-dark `#f1f1f2`, text-muted-dark `#a9aaad`, text-faint-dark `#7b7c81`, accent-soft `#82a8e3` for `--accent` in dark). Done when `grep '\[data-theme="dark"\]' internal/server/web/style.css` finds the block.
- [ ] 1.2 Verify no component rule outside of `:root` / `[data-theme="dark"]` embeds a hex literal. Done when `grep -nE '#[0-9a-fA-F]{3,8}' internal/server/web/style.css` reports only matches inside the two token blocks (and `@font-face`/`url(...)` lines if any). Fix any leftover hex in component selectors by replacing with `var(--token)`.

## 2. Theme toggle markup in topbar

- [ ] 2.1 In `internal/server/web/index.html`, locate the topbar's right-side container (the slot reserved by UI-1 for future controls). Insert a `<div class="theme-toggle" role="group" aria-label="Theme">` element containing exactly three `<button>` children, in DOM order: Light, System, Dark. Each button carries `data-theme-choice="light"|"system"|"dark"`, `type="button"`, `:aria-pressed="theme === 'light'"` / `'system'` / `'dark'`, and `@click="setTheme('light')"` / `'system'` / `'dark'`. Done when the rendered HTML contains all three buttons with the literal `data-theme-choice` values.
- [ ] 2.2 Inline three SVG icons inside each button (sun, monitor, moon) using `stroke="currentColor"` and `fill="none"` so the active-segment color cascades from `var(--text)`. Done when each button's inner HTML contains exactly one `<svg>` element.
- [ ] 2.3 Add CSS rules for `.theme-toggle` (track: `background: var(--surface-2)`, `border-radius: 999px`, `padding: 3px`, `display: inline-flex`, `gap: 0`) and `.theme-toggle button` (segment: `width: 26px`, `height: 26px`, `border: 0`, `background: transparent`, `border-radius: 999px`, `color: var(--text-muted)`, `cursor: pointer`, `display: inline-flex`, `align-items: center`, `justify-content: center`). Done when the toggle renders as a 3-pill segmented control in the topbar.
- [ ] 2.4 Add the active-segment rule: `.theme-toggle button[aria-pressed="true"] { background: var(--surface); color: var(--text); box-shadow: 0 1px 2px rgba(0,0,0,0.08); }`. Done when clicking a segment visibly fills it with `--surface` and applies the 1px close shadow.

## 3. Alpine theme controller

- [ ] 3.1 In `internal/server/web/app.js`, extend the `board()` returned object with two state fields: `theme: 'system'` (the user's choice) and a private `_mediaQuery: null` slot for the matchMedia handle. Done when the fields appear in the object literal and `app.js` still parses (`go test ./internal/server` keeps passing).
- [ ] 3.2 Add an `initTheme()` method that: (a) tries `localStorage.getItem('ezida.theme')` inside a try/catch, falling back to `null` on throw; (b) validates the value against the whitelist `['light','system','dark']`, mapping anything else (including `null`) to `'system'`; (c) assigns the result to `this.theme`; (d) creates `this._mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')` and attaches an arrow-function listener that calls `this.applyTheme()`; (e) calls `this.applyTheme()` once. Done when method exists and is callable.
- [ ] 3.3 Add `applyTheme()`: compute `effective = this.theme === 'system' ? (this._mediaQuery.matches ? 'dark' : 'light') : this.theme`. Then set `document.documentElement.setAttribute('data-theme', effective)`. Done when toggling `this.theme` from the Alpine devtools updates the `<html data-theme>` attribute live.
- [ ] 3.4 Add `setTheme(choice)`: validate against the whitelist (no-op on invalid), assign to `this.theme`, attempt `localStorage.setItem('ezida.theme', choice)` inside a try/catch (swallow the throw), then call `this.applyTheme()`. Done when clicking each segment updates both the active `aria-pressed` state and `<html data-theme>`.
- [ ] 3.5 Wire `initTheme()` into the existing `init()` / `load()` lifecycle on `board()` (likely via `x-init` in `index.html` or appended at the top of the existing init flow). Ensure it runs before — or independently of — the `/api/board` fetch, since the toggle should work even if the board fetch fails. Done when the toggle renders and reacts on first load with the network blocked.

## 4. Server tests

- [ ] 4.1 In `internal/server/server_test.go`, add `TestStaticStyleCSS_ContainsDarkSelector` that issues `GET /static/style.css` against the test server and asserts the response status is 200 and the body bytes contain the literal substring `[data-theme="dark"]`. Done when `go test ./internal/server -run TestStaticStyleCSS_ContainsDarkSelector` passes.
- [ ] 4.2 Add `TestIndex_ContainsThemeToggleButtons` that issues `GET /` and asserts the response body contains the three literal substrings `data-theme-choice="light"`, `data-theme-choice="system"`, `data-theme-choice="dark"`. Done when `go test ./internal/server -run TestIndex_ContainsThemeToggleButtons` passes.

## 5. Manual browser smoke (automated proof via Chrome MCP smoke)

- [ ] 5.1 Load the page with no stored preference; the System segment is active and the theme matches the OS. Toggle to Dark; tokens recompute and the page paints dark. Reload; Dark persists. Toggle to System; reload; System persists and effective theme follows OS.
- [ ] 5.2 With System selected, change the OS theme (or simulate `prefers-color-scheme` via DevTools); the page re-themes live without reload.
- [ ] 5.3 Disable `localStorage` (DevTools → Application → block storage); the toggle still works in-session and no exception is logged.

## 6. Acceptance gate

- [ ] 6.1 Run `go test ./... && go vet ./...`. Done when exit code is 0 and the two new test names appear in the output.
