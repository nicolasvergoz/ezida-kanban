## 1. Font vendoring

- [x] 1.1 Create directory `internal/server/web/vendor/fonts/`.
  Done when `test -d internal/server/web/vendor/fonts` exits 0.
- [x] 1.2 Add Geist `.woff2` files for weights 300, 400, 500, 600,
  700, 800 under `internal/server/web/vendor/fonts/` (file names
  e.g. `Geist-Light.woff2`, `Geist-Regular.woff2`,
  `Geist-Medium.woff2`, `Geist-SemiBold.woff2`, `Geist-Bold.woff2`,
  `Geist-ExtraBold.woff2`). Source: Vercel/Geist open-source
  distribution (OFL license). Done when `ls
  internal/server/web/vendor/fonts/Geist-*.woff2 | wc -l` returns
  `6`.
- [x] 1.3 Add Geist Mono `.woff2` files for weights 400 and 500
  (`GeistMono-Regular.woff2`, `GeistMono-Medium.woff2`). Done when
  `ls internal/server/web/vendor/fonts/GeistMono-*.woff2 | wc -l`
  returns `2`.
- [x] 1.4 Verify the existing `//go:embed` directive picks up the
  new font files. Done when `go build ./...` succeeds AND a quick
  manual probe (start the server, `curl -sI
  http://127.0.0.1:7777/static/vendor/fonts/Geist-Regular.woff2`)
  returns `HTTP/1.1 200 OK` with `Content-Type: font/woff2`. The
  curl probe is part of the final smoke gate (task 10.x) so this
  step's done-condition is just `go build ./...` succeeding.

## 2. Token system in style.css

- [x] 2.1 Replace `internal/server/web/style.css` with a token-driven
  Redacto stylesheet organized via `@layer tokens, typography,
  components`. The `@layer tokens` block MUST define on `:root`
  every token enumerated in the viewer-ui delta (Design Tokens
  requirement): `--bg-base`, `--surface`, `--surface-2`, `--border`,
  `--border-strong`, `--text`, `--text-muted`, `--text-faint`,
  `--accent`, `--accent-soft`, `--danger`, `--space-xxs` through
  `--space-4xl`, `--rounded-xs` through `--rounded-full`,
  `--shadow-card-idle`, `--shadow-card-hover`, `--shadow-popover`.
  Hex literals MUST appear only inside this block. Done when `grep
  -n '^[^/]*#[0-9a-fA-F]\{3,6\}' internal/server/web/style.css |
  grep -vE '^[^:]*:\s*[0-9]+:[[:space:]]*(--|/\*|\*|@font-face)'`
  returns no matches outside the `@layer tokens` block (manual
  diff review; the smoke task captures this).
- [x] 2.2 Inside `@layer typography`, define `@font-face` rules for
  every weight vendored in section 1 with `font-display: swap` and
  relative `url('/static/vendor/fonts/...')` paths. Then define
  utility classes `.t-brand`, `.t-list-title`, `.t-card-text`,
  `.t-body-md`, `.t-button`, `.t-tag`, `.t-mono-counter`,
  `.t-mono-label` matching the ramp in `refs/design.md`. Done when
  the file contains all 8 `@font-face` blocks and all 8 `.t-*`
  selectors.

## 3. Page surfaces (background, grain, top-shade)

- [x] 3.1 In `@layer components`, set `body` background to
  `var(--bg-base)` and add a fixed `body::before` (or a dedicated
  `.grain` overlay div) bearing a tiled SVG noise data-URI with
  `opacity: 0.04`, `pointer-events: none`, `z-index: -1`. Done
  when the served page's `body` computed `background-color`
  resolves to the bg-base value AND a grain pseudo-element exists.
- [x] 3.2 Add a `.topshade` decorative element (either a `body::after`
  pseudo or a `<div class="topshade">` in `index.html` if pseudo-
  element layering conflicts with `position: fixed`) — a 64px-tall
  `linear-gradient` from `color-mix(in oklab, var(--bg-base) 90%,
  transparent)` at the top down to `transparent`, `pointer-events:
  none`, sitting above the board but below the topbar. Done when
  the page has a non-interactive top-shade layer with the correct
  height and gradient.

## 4. Topbar restyle and project-name binding

- [x] 4.1 Update `internal/server/web/index.html`: change the
  `.project-name` span to a `.brand` element with class `t-brand`,
  bound via `x-text="project_name"`. Keep the `.status-dot` in the
  right zone. Set the topbar's structural classes so the `.t-brand`
  typography applies. Done when the served `index.html` contains
  `class="brand t-brand"` (or similar) and an `x-text="project_name"`
  binding on the brand element.
- [x] 4.2 Update `internal/server/web/app.js`: add `project_name: ''`
  to the Alpine component's data; assign
  `this.project_name = data.project_name || 'Ezida'` inside
  `load()` after the JSON decode. Done when `grep -n
  "project_name" internal/server/web/app.js` returns at least two
  matches (declaration + assignment) and the file still passes a
  manual sanity read.
- [x] 4.3 Style `.status-dot` per Redacto: 8px circle,
  `var(--rounded-full)`, `--accent` family fill when `.on`,
  `--text-faint` family when `.off`. No hex literals in the rule.
  Done when the served page's status dot renders with the token-
  driven palette.

## 5. Column glass panels and card chrome

- [x] 5.1 Rename `.columns` to `.board` in `index.html` and
  `style.css` (or add `.board` as a parallel class). Wire the
  outer wrapper as `<div class="board">` so the drag-scroll
  handler in section 9 has a stable target. Done when the served
  page contains `class="board"` on the columns wrapper.
- [x] 5.2 Style `.column` as a 296px-wide glass panel: width
  `var(--space-list-width, 296px)` (define if needed),
  `background: color-mix(in oklab, var(--surface) 75%,
  transparent)`, `backdrop-filter: blur(14px) saturate(140%)`,
  `border: 1px solid var(--border)`, `border-radius:
  var(--rounded-xl)`, `padding: var(--space-sm)`. Done when a
  `.column` element's computed `backdrop-filter` is non-empty and
  contains both `blur` and `saturate`.
- [x] 5.3 Style `.card` per Redacto chrome: `background:
  var(--surface)`, `border: 1px solid var(--border)`,
  `border-radius: var(--rounded-lg)`, `padding: 10px 12px`,
  `box-shadow: var(--shadow-card-idle)`, `transition: transform
  120ms, box-shadow 120ms`. On `:hover` (excluding mid-drag-scroll
  state), `transform: translateY(-1px)`, `box-shadow:
  var(--shadow-card-hover)`. Done when a card's hover state
  produces a translated transform and a multi-layer shadow.
- [x] 5.4 Style `.column-header` with `.t-list-title` (uppercase,
  tracked), and `.column-count` with `.t-mono-counter` plus a
  small `var(--surface)` background, `var(--rounded-md)` corners,
  1px `var(--border)`. Done when the column header renders the
  uppercase Geist 600 / 12px / 0.08em treatment.

## 6. Tag chips and priorities (visual only)

- [x] 6.1 Style `.tag` chips on the card surface per Redacto: pill
  shape (`var(--rounded-full)`), `.t-tag` typography, faint
  background `color-mix(in oklab, var(--text) 3%, transparent)`,
  `var(--text-faint)` text color, 1px subtle border. Behavior
  unchanged — readonly on the card body. Done when card tags
  render as rounded-full pills with the Redacto tag typography.
- [x] 6.2 Rewrite `.priority-low`, `.priority-medium`,
  `.priority-high` to use token-driven border / background
  combinations only (no hex literals in component rules). Each
  priority MUST produce a visually distinguishable card border or
  badge fill using the existing palette. Done when the three
  priority selectors exist in `@layer components` and reference
  only tokens.

## 7. Modal restyle

- [x] 7.1 Restyle `.modal-overlay` with backdrop `color-mix(in
  oklab, var(--bg-base) 10%, transparent)` and no blur. Done when
  the overlay renders as a low-tint scrim over the board.
- [x] 7.2 Restyle `.modal` with `background: var(--surface)`,
  `border-radius: var(--rounded-xl)`, `box-shadow:
  var(--shadow-popover)`, `padding: var(--space-3xl)`, max-width
  ~520px. Header `h2` carries `.t-list-title`. Inputs and textareas
  use `.t-body-md` typography, `border-radius: var(--rounded-md)`,
  1px `var(--border)` resting, 1px `var(--accent)` on focus,
  `outline: none`. Done when the modal's `border-radius` is 14px
  and its shadow resolves to multiple layers consistent with
  `--shadow-popover`. Behavior unchanged (V3 fields and Save/Cancel
  flow preserved).
- [x] 7.3 Restyle modal tag chips with the same Redacto chip skin
  as card tags but keeping V3's editable `×` remove button and tag
  input. Done when modal tag chips visually match the card tag
  treatment.

## 8. Loading and empty-column states

- [x] 8.1 Style `.loading` per Redacto: `.t-body-md` typography,
  `color: var(--text-muted)`, centered horizontally and vertically
  in the board surface. Done when the loading placeholder renders
  with the muted-text token and the body-md type.
- [x] 8.2 Style `.empty` (the per-column empty placeholder) per
  Redacto: `.t-body-md` typography, italic, `color:
  var(--text-faint)`, centered inside the column body, no border,
  no fill. Done when `.empty` renders as faint italic centered
  text.

## 9. Drag-scroll affordance

- [x] 9.1 Add a `.is-scrolling .card { pointer-events: none }` rule
  in `@layer components` so cards do not intercept clicks mid-
  drag-scroll. Done when the rule exists in `style.css`.
- [x] 9.2 Add a `setupDragScroll()` method to the Alpine component
  in `app.js`. Call it from `$nextTick` inside `load()` after the
  first successful fetch, guarded by an `_dragScrollMounted` flag
  so it only attaches once. The handler:
  - Resolves `const board = this.$el.querySelector('.board')`.
  - On `pointerdown` with `button === 0` that does NOT
    `closest('.card, .column-header, button, input, textarea,
    select, .modal, .modal-overlay')`, captures the pointer,
    records `startX` and `startScroll`, and adds
    `body.is-scrolling`.
  - On `pointermove` while active, sets `board.scrollLeft =
    startScroll - (e.clientX - startX)`.
  - On `pointerup` / `pointercancel`, releases capture and removes
    `body.is-scrolling`.
  Done when `grep -n "setupDragScroll\|is-scrolling" app.js`
  returns at least three matches (declaration, body-class add,
  body-class remove) and the file parses (manual read).
- [x] 9.3 Verify the drag-scroll does not interfere with Sortable
  card drags — Sortable's own pointer listener still fires because
  the drag-scroll handler bails on `closest('.card')`. Automated
  proof via Chrome MCP smoke in the final gate.

## 10. Server: project_name field

- [x] 10.1 In `internal/server/server.go`, inside `runWithContext`,
  compute `projectName` from the resolved board path using
  `filepath.Abs` then `filepath.Base(filepath.Dir(abs))`, with
  fallback to `"Ezida"` when the result is empty, `"."`, or the
  platform separator. Pass the value into the `serverState`
  constructor. Done when `serverState` carries a `projectName
  string` field and `runWithContext` populates it.
- [x] 10.2 In `internal/server/handlers.go`, add `ProjectName
  string \`json:"project_name"\`` as the last field of
  `boardResponse`. In `handleBoard`, set `resp.ProjectName =
  s.projectName`. Done when `go vet ./...` passes and `go build
  ./...` succeeds.
- [x] 10.3 In `internal/server/server_test.go` (or a sibling test
  file), add or update at least two test cases for `/api/board`:
  one asserting `project_name` equals the parent-directory basename
  of the temp board path, and one asserting the `"Ezida"` fallback
  for a synthesized empty/`"."` basename (the fallback may be
  unit-tested by extracting the resolver into a small helper
  function and asserting directly — pick whichever path is
  cleaner). Done when `go test ./internal/server/...` passes with
  the new assertions in place.

## 11. Verify gate and smoke

- [x] 11.1 Run `go test ./...` and confirm the entire suite passes
  (the configured verify gate). Done when the command exits 0.
- [x] 11.2 Run `go vet ./...` and confirm clean output. Done when
  the command exits 0 with no diagnostics.
- [x] 11.3 Automated proof via Chrome MCP smoke (orchestrator-run):
  start `ezida serve --no-open --port=7777` against a temp board
  named e.g. `redacto-smoke/kanban.toml`, then:
  - `curl -s http://127.0.0.1:7777/api/board | jq -r .project_name`
    returns `redacto-smoke`.
  - `curl -sI http://127.0.0.1:7777/static/vendor/fonts/Geist-Regular.woff2`
    returns `200` with `Content-Type: font/woff2`.
  - Chrome MCP loads `http://127.0.0.1:7777/`, captures a screenshot
    at 1280×800, and confirms: brand reads `REDACTO-SMOKE` (uppercase
    by CSS), columns render as glass panels, cards have the new
    shadow ramp, the modal opens with Redacto chrome on click, the
    drag-scroll affordance scrolls the board horizontally on
    pointer-drag of empty surface, and the status dot is round.
  Done when the orchestrator confirms each sub-check.
- [x] 11.4 Sanity grep: `grep -nE '#[0-9a-fA-F]{3,6}'
  internal/server/web/style.css | grep -v '@layer tokens' | grep -v
  '@font-face'` returns no matches outside the tokens / font-face
  blocks (visual review of remaining hits if any). Done when the
  grep is clean or the remaining hits are inside the `:root` token
  declaration block.
