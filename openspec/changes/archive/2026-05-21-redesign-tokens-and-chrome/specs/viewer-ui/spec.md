## ADDED Requirements

### Requirement: Design tokens are CSS custom properties on `:root`

The embedded stylesheet SHALL define the Redacto design token set as
CSS custom properties on `:root`, covering at minimum: a base color
ramp (`--bg-base`, `--surface`, `--surface-2`, `--border`,
`--border-strong`, `--text`, `--text-muted`, `--text-faint`), the
semantic accent / danger palette (`--accent`, `--accent-soft`,
`--danger`), a spacing scale (`--space-xxs` through `--space-4xl`
covering 2/4/6/8/10/12/14/18/28 px), a rounded scale (`--rounded-xs`
through `--rounded-full`), and an elevation set (`--shadow-card-idle`,
`--shadow-card-hover`, `--shadow-popover`). Every CSS rule that
styles a UI component MUST reference these tokens via `var(--…)`
rather than hard-coding hex literals; hex literals MUST appear only
inside the `:root` token declaration block (and `@font-face`
sources).

#### Scenario: Root exposes the token set

- **WHEN** `getComputedStyle(document.documentElement).getPropertyValue('--bg-base')`
  is evaluated against the served page
- **THEN** the returned value is a non-empty color string

#### Scenario: Components consume tokens, not hex

- **WHEN** the served `style.css` is inspected
- **THEN** the only CSS rules containing hex literals (`#xxxxxx`)
  are the `:root` token declaration block and any `@font-face`
  declarations
- **AND** every component selector references colors via
  `var(--…)` exclusively

### Requirement: Typography utility classes exist for every type ramp

The stylesheet SHALL provide CSS utility classes corresponding to
each entry in the Redacto type ramp, at minimum: `.t-brand`,
`.t-list-title`, `.t-card-text`, `.t-body-md`, `.t-button`,
`.t-tag`, `.t-mono-counter`, `.t-mono-label`. Each class MUST set
`font-family`, `font-size`, `font-weight`, `line-height`, and (where
the ramp specifies one) `letter-spacing` and `text-transform`.
Components SHALL apply typography by composing these classes onto
their markup; ad-hoc `font-size` / `font-weight` declarations on
component selectors SHALL be avoided.

#### Scenario: Brand class drives the topbar

- **WHEN** the page loads and the topbar brand element is inspected
- **THEN** the element carries the `.t-brand` class
- **AND** its computed `font-family` includes `Geist`
- **AND** its computed `text-transform` equals `uppercase`

#### Scenario: Mono counter class drives column counts

- **WHEN** a column header's card-count badge is inspected
- **THEN** the badge carries the `.t-mono-counter` class
- **AND** its computed `font-family` includes `Geist Mono`

### Requirement: Geist and Geist Mono fonts are vendored

The page SHALL load `Geist` (weights 300, 400, 500, 600, 700, 800)
and `Geist Mono` (weights 400, 500) via `@font-face` rules sourced
from `/static/vendor/fonts/*.woff2`. The files MUST be present in
`internal/server/web/vendor/fonts/` and served through the embedded
`/static/*` route. `@font-face` rules MUST set `font-display: swap`
so the browser falls back to system sans-serif while loading. The
page MUST NOT load any font from a remote URL at runtime.

#### Scenario: Font reachable via static route

- **WHEN** `GET /static/vendor/fonts/Geist-Regular.woff2` is called
  against the running server
- **THEN** the response status MUST be `200`
- **AND** the response `Content-Type` MUST be `font/woff2`

#### Scenario: No external font URL

- **WHEN** the served `style.css` is inspected
- **THEN** every `@font-face` `src` URL MUST start with
  `/static/vendor/fonts/`

### Requirement: Topbar brand binds to the server-provided project name

The topbar's brand element SHALL render the value of the
`project_name` field returned by `GET /api/board`. The brand text
MUST be uppercase (driven by CSS `text-transform`, not pre-cased
content). Until `/api/board` returns successfully, the brand MAY
display a placeholder; after a successful load, the brand MUST
reflect `project_name` verbatim (modulo letter-casing applied by
CSS).

#### Scenario: Brand renders the project name

- **WHEN** the page loads against a server whose `/api/board`
  returns `"project_name": "my-project"`
- **THEN** the topbar's brand element's text content (lower-cased
  for comparison) equals `my-project`

#### Scenario: Brand renders the "Ezida" fallback

- **WHEN** the page loads against a server whose `/api/board`
  returns `"project_name": "Ezida"`
- **THEN** the topbar's brand element's text content equals
  `Ezida` (modulo CSS-applied casing)

### Requirement: Page surfaces include bg-base, grain, and top-shade

The page SHALL render against a warm off-white base color (`--bg-base`
in light theme) and SHALL layer on top of it a low-opacity SVG noise
grain (effective opacity `0.04`) and a top-shade gradient (a
linear-gradient from `10% bg-base` at the top down to `transparent`
across the first 64px of the viewport). The grain and the top-shade
MUST NOT be interactive (they MUST NOT receive pointer events).

#### Scenario: Body background uses bg-base

- **WHEN** the page loads
- **THEN** the computed background-color of `body` resolves to the
  same value as `var(--bg-base)`

#### Scenario: Grain layer is non-interactive

- **WHEN** the page loads and the grain-overlay element is inspected
- **THEN** its computed `pointer-events` equals `none`

### Requirement: Columns render as glass panels

Each `.column` element SHALL render as a 296px-wide glass panel: 75%
opacity of `--surface` plus `backdrop-filter: blur(14px)
saturate(140%)`, with `--rounded-xl` (14px) corners and a 1px
`--border` outline. Column padding MUST be 6px around its inner
cards container. Columns MUST NOT grow or shrink horizontally.

#### Scenario: Column has the glass surface treatment

- **WHEN** the page loads and a `.column` element is inspected
- **THEN** its computed `backdrop-filter` contains both `blur` and
  `saturate`
- **AND** its computed `border-radius` equals 14px
- **AND** its computed `width` equals 296px

### Requirement: Cards have Redacto chrome with idle and hover shadow ramps

Each `.card` element SHALL render with `--surface` fill, 1px
`--border` outline, `--rounded-lg` (10px) corners, `10px 12px`
padding, and the `--shadow-card-idle` drop shadow (a single soft
2px-blur `rgba(0,0,0,0.05)` drop). On `:hover`, the card MUST apply
`--shadow-card-hover` (a three-layer stack: `0 1px 0` rim + `0 1px
3px` close + `0 4px 12px -2px` distant) and `translateY(-1px)`. The
hover treatment MUST NOT change the card's outline color away from
the resting border token unless the card is in an active drag
state.

#### Scenario: Card idle shadow is shallow

- **WHEN** a `.card` element is inspected at rest
- **THEN** its computed `box-shadow` is non-empty
- **AND** the shadow magnitude is consistent with the idle ramp
  (single soft 2px-blur drop)

#### Scenario: Card hover lifts by 1px and deepens shadow

- **WHEN** a `.card` element is hovered (pointer over)
- **THEN** its computed `transform` includes `translateY(-1px)` (or
  equivalent matrix)
- **AND** its computed `box-shadow` resolves to multiple shadow
  layers consistent with the hover ramp

### Requirement: Drag-scroll the empty board surface

The page SHALL enable horizontal drag-scroll of the `.board`
element when the user performs a primary-button `pointerdown` on
the board surface that is NOT inside a `.card`, `.column-header`,
`button`, form control, or the `.modal`. During an active drag,
`body` MUST carry the class `is-scrolling` and `.card` children
MUST have `pointer-events: none` so the drag is not hijacked by a
child click. The page MUST NOT map the mouse wheel to horizontal
scroll.

#### Scenario: Drag on empty surface scrolls horizontally

- **WHEN** the user performs `pointerdown` on the `.board`
  background (not over a card or column header), moves the pointer
  100px to the left, and releases
- **THEN** the `.board` element's `scrollLeft` increases by ~100px
- **AND** `body.is-scrolling` was added during the drag and
  removed on `pointerup`

#### Scenario: Pointerdown on a card does not initiate drag-scroll

- **WHEN** the user performs `pointerdown` directly on a `.card`
- **THEN** `body.is-scrolling` MUST NOT be added by the drag-
  scroll handler (existing card click / Sortable drag behavior is
  preserved)

#### Scenario: Pointerdown on a button does not initiate drag-scroll

- **WHEN** the user performs `pointerdown` on any `button` element
  inside the board
- **THEN** `body.is-scrolling` MUST NOT be added by the drag-
  scroll handler

## MODIFIED Requirements

### Requirement: Empty columns display a placeholder

When a column has zero cards, the rendered `.column` SHALL display
a placeholder element styled with the Redacto empty-state treatment:
faint italic text using `var(--text-faint)`, `.t-body-md`
typography, centered inside the column body, with no border or fill.
The element MUST carry the class `.empty` so existing tests still
match.

#### Scenario: Empty column

- **WHEN** the board contains a column `backlog` with no cards
- **THEN** the corresponding `.column` element contains exactly
  one `.empty` placeholder element
- **AND** the column-count badge reads `0`
- **AND** the placeholder's computed color resolves to
  `var(--text-faint)`

### Requirement: Priorities map to distinguishable visual styles

The page SHALL apply a CSS class `priority-<value>` to each card
whose `priority` field is set. Each value present in
`[board].priorities` MUST produce a visually distinguishable
treatment driven by design tokens (no hex literals) — implementations
SHALL use combinations of token-referenced border colors, badge
fills, or text emphasis from the existing palette. The treatment
MUST NOT introduce a new color outside the ramp defined under
`:root`.

#### Scenario: All three priorities present

- **WHEN** the board contains three cards with priorities `low`,
  `medium`, `high` respectively
- **THEN** each rendered card carries the matching `priority-*`
  class
- **AND** the computed CSS produces a different visual state for
  each (asserted via differing computed `border-left-color`,
  `background-color`, badge presence, or equivalent token-driven
  property)

#### Scenario: Card without priority

- **WHEN** a card has no `priority` field
- **THEN** the rendered `.card` MUST NOT carry any `priority-*`
  class

### Requirement: Top bar is present and minimal

The rendered page SHALL include a `<header>` element with class
`topbar` containing two zones: a left zone with the brand element
(rendering the server-provided `project_name`, styled via
`.t-brand`), and a right zone with the SSE status dot (skinned per
Redacto). The topbar MUST be 64px tall, MUST sit above the board
visually via the top-shade gradient, and MUST be visually distinct
from the board surface. Filter button and theme toggle are NOT
present in this phase (they ship in later UI redesign phases).

#### Scenario: Topbar present

- **WHEN** the page loads
- **THEN** the DOM contains exactly one `header.topbar` element
- **AND** the topbar's height equals 64px

#### Scenario: Topbar brand uses the project name

- **WHEN** the page loads against a server whose `/api/board`
  returns `"project_name": "alpha"`
- **THEN** the brand element's text content (lower-cased) equals
  `alpha`

### Requirement: Topbar shows connection status

The topbar SHALL render a small dot element in its right zone whose
class reflects the live SSE connection state: `on` when the
EventSource is open, `off` when it is closed or in retry. The dot
MUST use design tokens (`--accent` family for `on`, `--text-faint`
family for `off`) and MUST be visually distinguishable in the two
states. The dot MUST be a circle (`--rounded-full`) of fixed size
(approximately 8px).

#### Scenario: Dot reflects open connection

- **WHEN** the EventSource is open
- **THEN** the topbar dot's class list MUST contain `on`
- **AND** its computed background-color resolves to the accent
  family token

#### Scenario: Dot reflects closed connection

- **WHEN** the EventSource is in the closed state (server killed,
  network dropped)
- **THEN** the topbar dot's class list MUST contain `off`

### Requirement: Page degrades safely on board-load failure

The page SHALL keep its loading state visible (no broken board) when
`/api/board` returns a non-2xx response on initial load, and MUST
log the error to the browser console for developer visibility. The
loading placeholder MUST be styled with the Redacto loading treatment
(`.t-body-md` typography, `var(--text-muted)` color, centered). The
page MUST NOT show an alert or toast in this phase.

#### Scenario: Server returns 500

- **WHEN** the page loads against a server whose `kanban.toml` is
  missing (returns 500)
- **THEN** the rendered DOM still contains the topbar
- **AND** the `.loading` placeholder remains visible
- **AND** the placeholder's computed color resolves to
  `var(--text-muted)`
- **AND** the browser console contains an error message naming the
  failed fetch

### Requirement: Clicking a card opens an edit modal

The page SHALL open a modal dialog when the user clicks a `.card`
element. The modal MUST pre-fill its inputs with the clicked card's
current values for title, description, tags, and priority, and MUST
display the card's `id`, `column`, `created_at`, and `updated_at`
as read-only metadata. The modal surface SHALL be styled with the
Redacto chrome: `var(--surface)` fill, `var(--rounded-xl)` (14px)
corners, `var(--shadow-popover)` floating shadow, and a backdrop
overlay using `var(--bg-base)` at 10% opacity (no blur). Form
fields (label-wraps-input, textarea, select, tag chip list) retain
their V3 structure — Trello-style click-to-edit is out of scope
for this phase.

#### Scenario: Click opens modal with current values

- **WHEN** the user clicks a card whose title is `Refactor auth`
  and tags are `["security"]`
- **THEN** the DOM MUST contain a visible `.modal` element
- **AND** the modal's title input value MUST equal `Refactor auth`
- **AND** the modal's tag chip list MUST contain a chip with text
  `security`

#### Scenario: Modal chrome uses Redacto tokens

- **WHEN** the modal is open
- **THEN** the `.modal` element's computed `border-radius` equals
  14px
- **AND** its computed `box-shadow` resolves to the popover ramp
  (multi-layer shadow consistent with floating elevation)

#### Scenario: Click does not open modal during a drag

- **WHEN** the user starts a drag from a card and drops it elsewhere
- **THEN** the modal MUST NOT open as a result of the drop
