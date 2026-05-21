# viewer-ui

### Requirement: Embedded page renders the board read-only

`GET /` SHALL return an HTML document that, when executed by a
JavaScript-enabled browser, fetches `/api/board` once on load and
renders columns and cards according to the response. The page MUST
function without any user interaction beyond a successful page
load.

#### Scenario: Page loads and renders columns

- **WHEN** a browser navigates to `http://127.0.0.1:<port>/` with
  a server backing a board that has columns `todo`, `doing`,
  `done` and 3 cards distributed across them
- **THEN** the page DOM contains exactly 3 `.column` elements
  in the order of the `columns` array
- **AND** each `.column` header shows the column name and a
  card count matching the response

#### Scenario: Cards display title, priority, and tags

- **WHEN** the board contains a card with `title="Refactor auth"`,
  `priority="high"`, `tags=["security","tech-debt"]`
- **THEN** the rendered `.card` element contains the literal text
  `Refactor auth`
- **AND** the `.card` carries a class `priority-high`
- **AND** the rendered tags include both `security` and `tech-debt`
  inside `.tag` elements

#### Scenario: Updated-at tooltip

- **WHEN** the user hovers a card
- **THEN** the browser surfaces a tooltip whose text contains the
  card's `updated_at` value (via the `title` attribute)

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
padding, and the `--shadow-card-idle` drop shadow (a subtle 1–2 px
blur `rgba(0,0,0,~0.03–0.05)` single drop). On `:hover`, the card
MUST apply `--shadow-card-hover` (a stacked drop — at least one
close and one distant layer at low opacity to suggest lift) and
`translateY(-1px)`. The hover treatment MUST NOT change the card's
outline color away from the resting border token in any state.

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

### Requirement: Vendored Alpine.js is served from embedded FS

The page SHALL reference `/static/vendor/alpine.min.js` as a
`<script defer>`. The asset MUST be present in
`internal/server/web/vendor/alpine.min.js` and MUST be served
verbatim through the existing `/static/*` route. The page MUST NOT
load Alpine (or any other JS) from a remote URL at runtime.

#### Scenario: Alpine reachable via static route

- **WHEN** `GET /static/vendor/alpine.min.js` is called against the
  running server
- **THEN** the response status is `200`
- **AND** the body's first bytes match the embedded file

#### Scenario: No external script tags

- **WHEN** the page source is inspected
- **THEN** no `<script>` tag has a `src` attribute pointing outside
  `/static/`

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

### Requirement: Cards are draggable across and within columns

The embedded page SHALL initialize Sortable.js on every column's card list so that cards can be dragged between columns and reordered within a column. The card's body (no separate handle) MUST be the drag affordance. Each `<li.card>` MUST carry `data-card-id="<id>"`; each `<ul.cards>` MUST carry `data-column="<column-name>"` so the drop handler can read them without DOM traversal.

#### Scenario: Drag card to another column

- **WHEN** the user drags a card from `todo` and drops it on `done`
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with body `{"column":"done","position":<int>}`
- **AND** the card visually appears in the `done` column at the dropped slot before the request resolves

#### Scenario: Drag card within the same column

- **WHEN** the user drags a card from position 0 of `todo` and drops it at position 2
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with body `{"column":"todo","position":2}`
- **AND** the card visually appears at the new slot before the request resolves

### Requirement: Drop failure resets the UI from the server

If the move request returns a non-2xx response, the page SHALL refetch `/api/board` and re-render so the displayed state matches disk. The page MUST NOT attempt a manual revert (server is source of truth per ADR 0002 §D3).

#### Scenario: Server rejects the move

- **WHEN** the move endpoint returns `404 CARD_NOT_FOUND` (e.g. the card was deleted via CLI between page load and drop)
- **THEN** the page MUST refetch `/api/board`
- **AND** the rendered DOM MUST reflect the refetched state (the dropped card is no longer present)
- **AND** the browser console MUST contain an error log describing the failure

### Requirement: Sortable.js is vendored, not loaded from CDN

The page SHALL reference `/static/vendor/sortable.min.js` as a `<script defer>`. The file MUST be present in `internal/server/web/vendor/sortable.min.js` and MUST be served verbatim through the embedded `/static/*` route. The page MUST NOT load Sortable from a remote URL at runtime.

#### Scenario: Sortable reachable via static route

- **WHEN** `GET /static/vendor/sortable.min.js` is called against the running server
- **THEN** the response status MUST be `200`
- **AND** the body's first bytes MUST match the embedded file

#### Scenario: No external sortable script

- **WHEN** the page source of `GET /` is inspected
- **THEN** no `<script>` tag has a `src` attribute pointing outside `/static/`

### Requirement: Clicking a card opens a detail modal

The page SHALL open a modal dialog when the user clicks a `.card`
element. The modal MUST display the card's current values for title,
description, priority, and tags as rendered text (NOT as form inputs),
and MUST display the card's `id`, `column`, `created_at`, and
`updated_at` as read-only metadata. The modal MUST NOT contain a
global Save button or a global Cancel button.

#### Scenario: Click opens modal with values rendered as text

- **WHEN** the user clicks a card whose title is `Refactor auth`,
  description is `Use OAuth flow`, priority is `high`, and tags are
  `["security"]`
- **THEN** the DOM MUST contain a visible `.modal` element
- **AND** the modal MUST render `Refactor auth` as text inside a
  `.field` element (NOT as the value of an `<input>`)
- **AND** the modal MUST render `Use OAuth flow` as text inside a
  `.field--multiline` element (NOT as the value of a `<textarea>`)
- **AND** the modal MUST render `high` as text inside a `.field`
  element (NOT as the selected value of a `<select>`)
- **AND** the modal MUST contain a chip element with text `security`
- **AND** the modal MUST NOT contain a `<button type="submit">` Save
  element
- **AND** the modal MUST NOT contain a Cancel button

#### Scenario: Empty description shows a placeholder hint

- **WHEN** the user clicks a card whose description is the empty
  string
- **THEN** the modal's description `.field--multiline` MUST render
  visible placeholder text indicating the field is addable (e.g.
  `Add a description`)

#### Scenario: Missing priority renders as `no priority`

- **WHEN** the user clicks a card whose `priority` field is empty
- **THEN** the modal's priority `.field` MUST render the literal text
  `no priority`

#### Scenario: Click does not open modal during a drag

- **WHEN** the user starts a drag from a card and drops it elsewhere
- **THEN** the modal MUST NOT open as a result of the drop

### Requirement: Fields enter inline edit mode on click

The modal MUST switch a rendered field into an inline editor when the
user clicks it. Clicking a rendered editable field (`.field` for
title, priority; `.field--multiline` for description) SHALL replace it
with the corresponding inline editor: a single-line `<input>` for
title, a multi-line `<textarea>` for description, and a `<select>` for
priority. The editor MUST receive focus on the next event tick. Only
one field MAY be in edit mode at any moment; clicking a second field
while one is in edit mode MUST first commit (blur) the active one
before entering edit on the new field.

The tag chip editor is exempt from the click-to-edit gate — chips and
the tag-add input are always live whenever the modal is open.

#### Scenario: Click title enters title edit mode

- **WHEN** the modal is open showing a card whose title is `T1` and
  the user clicks the title `.field`
- **THEN** the title `.field` rendered span MUST be hidden
- **AND** an `<input>` element with value `T1` MUST be visible and
  focused
- **AND** no other field's editor MUST be visible

#### Scenario: Click description enters description edit mode

- **WHEN** the modal is open showing a card with description
  `Long body...` and the user clicks the description
  `.field--multiline`
- **THEN** a `<textarea>` element with value `Long body...` MUST be
  visible and focused

#### Scenario: Click priority enters priority edit mode

- **WHEN** the modal is open showing a card with priority `medium`
  and the user clicks the priority `.field`
- **THEN** a `<select>` element with `medium` as the selected option
  MUST be visible and focused
- **AND** the `<select>` MUST include one `<option value="">` whose
  visible text is `no priority` plus one `<option>` per value in
  `[board].priorities`

#### Scenario: Clicking a second field commits the first

- **WHEN** the user is editing the title field (title editor visible)
  with a changed value, and then clicks the description rendered
  `.field--multiline`
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with the
  changed title in the body
- **AND** the description editor MUST then be visible and focused
- **AND** the title editor MUST NOT be visible

### Requirement: Field commit on blur or Enter sends a single-key PATCH

The modal MUST commit an edited field independently of all other
fields. Committing a field — defined as blur on its editor, `Enter`
in the title input, `Cmd/Ctrl+Enter` in the description textarea, or
`change`/blur on the priority `<select>` — SHALL issue
`PATCH /api/cards/<id>` with a JSON body containing exactly one key
(the field that was edited) and its new value. On a 2xx response, the field MUST return to rendered text
mode with the value from the server's response. On a non-2xx response,
the field MUST remain in edit mode with the editor visible and its
current value preserved, AND the server's `error.message` (or a
fallback `HTTP <status>` string) MUST be displayed inline in a
`.field-error` element placed directly under the field.

#### Scenario: Blur on title commits title via PATCH

- **WHEN** the user edits the title input value to `New title` and the
  input loses focus (blur)
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"title":"New title"}`
- **AND** the request body MUST NOT contain any other top-level key
- **AND** on 2xx the title rendered `.field` MUST display `New title`
- **AND** the title editor MUST NOT be visible

#### Scenario: Enter in title commits title via PATCH

- **WHEN** focus is in the title input with value `New title` and the
  user presses `Enter`
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"title":"New title"}`

#### Scenario: Cmd/Ctrl+Enter in description commits description via PATCH

- **WHEN** focus is in the description textarea with value
  `New body` and the user presses `Cmd+Enter` (macOS) or `Ctrl+Enter`
  (elsewhere)
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"description":"New body"}`
- **AND** plain `Enter` in the textarea MUST NOT trigger a PATCH (it
  inserts a newline)

#### Scenario: Priority `<select>` change commits via PATCH

- **WHEN** the user changes the priority `<select>` from `high` to
  `low`
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"priority":"low"}`

#### Scenario: Clearing priority via the dropdown

- **WHEN** the user opens a card whose priority is `high`, clicks the
  priority field, and selects `no priority`
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"priority":""}`
- **AND** after the response the priority `.field` MUST render the
  literal text `no priority`

#### Scenario: Server error keeps field in edit mode with inline message

- **WHEN** the user clears the title in the inline input and the
  editor loses focus, and the server returns 400 `MISSING_TITLE` with
  message `title is required`
- **THEN** the title editor MUST remain visible
- **AND** the title rendered `.field` MUST NOT be visible
- **AND** a `.field-error` element directly under the title field
  MUST display the text `title is required`
- **AND** the user MUST be able to retype a non-empty value and
  re-blur to issue a fresh PATCH

#### Scenario: Saving state while PATCH is in flight

- **WHEN** the user commits a field and the PATCH request is still
  in flight
- **THEN** the field's editor MUST carry a `field--saving` class (or
  equivalent CSS marker) for the duration of the request
- **AND** no spinner element MUST be added to the DOM (CSS-only
  affordance)

### Requirement: Escape reverts the active field or closes the modal

The modal SHALL respond to the `Escape` key with context-sensitive
behavior:

- If any field is currently in edit mode, `Escape` MUST revert that
  field: the editor's in-flight value is discarded, the field returns
  to rendered text mode with the last-saved value, and no PATCH is
  issued. The modal MUST stay open.
- If no field is in edit mode, `Escape` MUST close the modal. The
  on-disk `kanban.toml` MUST be byte-unchanged as a result.

#### Scenario: Esc in title editor reverts the title field

- **WHEN** the title is in edit mode with in-flight value `Half-typed`
  (different from the saved value `Saved title`) and the user presses
  `Esc`
- **THEN** the title editor MUST NOT be visible
- **AND** the title rendered `.field` MUST display `Saved title`
- **AND** no PATCH request MUST have fired
- **AND** the modal MUST remain visible

#### Scenario: Esc with no active field closes the modal

- **WHEN** the modal is open with all fields in rendered mode and the
  user presses `Esc`
- **THEN** the `.modal` element MUST NOT be visible
- **AND** no PATCH request MUST have fired
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Esc in description editor reverts only that field

- **WHEN** the description is in edit mode with in-flight value
  `draft body` (different from the saved value `original body`), and
  the title field is in rendered mode showing `Saved title`, and the
  user presses `Esc`
- **THEN** the description editor MUST NOT be visible
- **AND** the description rendered field MUST display `original body`
- **AND** the title rendered field MUST still display `Saved title`
  (unaffected)
- **AND** the modal MUST remain visible

### Requirement: Tag chip add and remove each commit a PATCH

The modal SHALL display each existing tag as a removable chip element
and SHALL provide a text input for adding new tags. The chip list and
tag-add input are always live whenever the modal is open (no
click-to-edit gate). Pressing Enter or comma in the input MUST add
the trimmed, deduplicated value to the tag list AND immediately issue
`PATCH /api/cards/<id>` with body `{"tags": <new array>}`. Clicking a
chip's remove button (`×`) MUST remove that chip from the tag list
AND immediately issue `PATCH /api/cards/<id>` with body
`{"tags": <new array>}`. The tag input MUST be cleared on add. On a
non-2xx response the chip list MUST revert to the pre-action state
(by re-reading from the locally cached card or refetching) AND a
`.field-error` element MUST display the server error message under
the tag field.

#### Scenario: Add a tag via Enter fires a PATCH

- **WHEN** the user opens a card with tags `["a"]`, types `b` in the
  tag input, and presses Enter
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"tags":["a","b"]}`
- **AND** the request body MUST NOT contain any other top-level key
- **AND** on 2xx the modal MUST display two chips: `a` and `b`
- **AND** the tag input MUST be empty

#### Scenario: Add a tag via comma fires a PATCH

- **WHEN** the user opens a card with tags `[]`, types `urgent,` in
  the tag input
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"tags":["urgent"]}`

#### Scenario: Remove a tag via the chip's button fires a PATCH

- **WHEN** the user opens a card with tags `["a","b"]` and clicks the
  `×` button on the `a` chip
- **THEN** a `PATCH /api/cards/<id>` request MUST fire with body
  `{"tags":["b"]}`
- **AND** on 2xx the modal MUST display one chip: `b`

#### Scenario: Duplicate tag input is deduplicated and does NOT fire a PATCH

- **WHEN** the user opens a card with tags `["a"]` and types `a` then
  Enter
- **THEN** the chip list MUST still show exactly one `a` chip
- **AND** no `PATCH` request MUST have fired
- **AND** the tag input MUST be empty

#### Scenario: Tag PATCH rejection shows inline error

- **WHEN** the user types an invalid tag (e.g. a tag containing only
  whitespace), Enter, and the server returns 400 `INVALID_TAG`
- **THEN** a `.field-error` element under the tag field MUST display
  the server error message
- **AND** the chip list MUST reflect the pre-add state

### Requirement: Modal-overlay close affordances

Clicking the `.modal-overlay` outside the `.modal` element SHALL
close the modal. There MUST NOT be a confirmation prompt — there is
no aggregated draft to discard. Any field that is mid-commit
(`saving` state) when the modal closes MAY complete its in-flight
PATCH; the modal MUST NOT block on or cancel it.

#### Scenario: Overlay click closes the modal

- **WHEN** the modal is open with all fields in rendered mode and the
  user clicks on the `.modal-overlay` outside the inner `.modal`
- **THEN** the `.modal` element MUST NOT be visible
- **AND** the page MUST NOT call `window.confirm` or otherwise prompt

#### Scenario: Click inside the modal does NOT close it

- **WHEN** the modal is open and the user clicks anywhere inside the
  inner `.modal` element (e.g. on the header, on a field, on a chip)
- **THEN** the `.modal` element MUST remain visible

### Requirement: Page subscribes to `/api/events` on load

The page SHALL open an `EventSource` connection to `/api/events` after the initial board load completes. On receiving an `event: board-changed`, the page SHALL refetch `/api/board` and re-render. The browser's native auto-reconnect SHALL handle dropped connections.

#### Scenario: External change triggers a refetch

- **WHEN** the page is open and the watcher fires (e.g. due to a CLI command in another terminal)
- **THEN** the page MUST issue a fresh `GET /api/board` request within 1 s of the event
- **AND** the rendered DOM MUST reflect the new board state

#### Scenario: EventSource auto-reconnects after a server restart

- **WHEN** the server is restarted (process exits and starts again on the same port)
- **THEN** the page's `connected` indicator MUST eventually return to the connected state without a user-initiated reload

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

### Requirement: Open edit modal closes on external change

If the edit modal is open at the moment an external change event arrives, the page SHALL close the modal without prompting and discard any unsaved draft. The page MUST NOT show a confirmation dialog before discarding. The page MAY suppress this close behavior for board-changed events triggered by the viewer's own writes (writes the client itself just issued via PATCH/POST/DELETE) so per-field commits in the redesigned modal do not interrupt the user mid-edit.

#### Scenario: Modal open when external change fires

- **WHEN** the user has the modal open with unsaved edits and an external CLI write arrives (no in-flight client write within the prior ~1500 ms)
- **THEN** the modal MUST close
- **AND** the rendered card MUST display the values from the refetched board (not the discarded draft)

#### Scenario: Modal stays open when own write fires board-changed

- **WHEN** the user has the modal open and just issued a PATCH against the open card (per-field commit), and the resulting board-changed SSE event arrives within ~1500 ms of that PATCH
- **THEN** the modal MUST remain open
- **AND** the modal's rendered fields MUST reflect the post-write board state

#### Scenario: No prompt before discard

- **WHEN** the modal closes due to an external change
- **THEN** the page MUST NOT have called `window.confirm` or otherwise blocked on user input

### Requirement: Topbar exposes a 3-state theme toggle

The rendered topbar SHALL include a theme toggle composed of exactly
three buttons representing the user's choice: Light, System, and Dark.
Each button MUST carry a stable `data-theme-choice` attribute whose
value is `"light"`, `"system"`, or `"dark"` respectively, and an icon
(sun, monitor, moon). The currently-selected button MUST carry
`aria-pressed="true"`; the other two MUST carry `aria-pressed="false"`.
The default selected button SHALL be the one whose `data-theme-choice`
equals `"system"` when no prior preference is stored.

#### Scenario: Three toggle buttons present in the topbar

- **WHEN** the page loads with no `ezida.theme` value in `localStorage`
- **THEN** the topbar contains exactly three elements matching
  `[data-theme-choice]`
- **AND** their `data-theme-choice` values in DOM order are `"light"`,
  `"system"`, `"dark"`
- **AND** the button with `data-theme-choice="system"` has
  `aria-pressed="true"`
- **AND** the other two buttons have `aria-pressed="false"`

#### Scenario: Clicking a segment changes the active state

- **WHEN** the user clicks the `data-theme-choice="dark"` button
- **THEN** that button MUST carry `aria-pressed="true"`
- **AND** the two other buttons MUST carry `aria-pressed="false"`

#### Scenario: System default derives from OS preference

- **WHEN** the page loads with no stored preference AND the browser's
  `prefers-color-scheme` is `dark`
- **THEN** `document.documentElement.getAttribute("data-theme")` MUST
  equal `"dark"`
- **AND** the System segment MUST be the one with `aria-pressed="true"`

#### Scenario: System mode reacts live to OS theme change

- **WHEN** the toggle is set to System AND the OS-level
  `prefers-color-scheme` flips from `light` to `dark` during the
  session (e.g. macOS automatic dusk transition)
- **THEN** `document.documentElement.getAttribute("data-theme")` MUST
  update to `"dark"` without a page reload
- **AND** the System segment MUST remain the one with
  `aria-pressed="true"` (the user's choice did not change)

### Requirement: Dark color tokens override light values via `[data-theme="dark"]`

The stylesheet at `/static/style.css` SHALL contain a rule block whose
selector is `[data-theme="dark"]` and which reassigns the same CSS
custom properties defined under `:root` (at minimum `--bg-base`,
`--surface`, `--surface-2`, `--border`, `--border-strong`, `--text`,
`--text-muted`, `--text-faint`) to dark-theme values per
`refs/design.md` §"Colors". No component selector outside of
`:root` / `[data-theme="dark"]` MAY embed a hex literal that depends
on a specific theme.

#### Scenario: Stylesheet exposes the dark selector

- **WHEN** `GET /static/style.css` is fetched from the running server
- **THEN** the response body contains the literal substring
  `[data-theme="dark"]`

#### Scenario: Body background differs between light and dark

- **WHEN** the page is rendered with `data-theme="light"` (or no
  attribute) AND the computed `background-color` of `<body>` is read
- **THEN** the computed value reflects the light `--bg-base` (warm
  off-white `#fbfaf8` per design.md)
- **AND** when the same page is then switched to `data-theme="dark"`,
  the computed `background-color` of `<body>` MUST resolve to a value
  derived from the dark `--bg-base` (`#25282e` per design.md), so the
  two computed values MUST differ

### Requirement: Theme preference persists across reloads

The user's explicit choice from the toggle SHALL be written to
`localStorage` under the key `"ezida.theme"` with one of the literal
string values `"light"`, `"system"`, or `"dark"`. On subsequent page
loads, the stored value MUST drive the initial toggle state and the
initial `data-theme` attribute. The page MUST NOT throw if
`localStorage` is unavailable (e.g. private browsing, blocked by
policy) — the in-memory choice still drives the UI for the current
session but is not persisted.

#### Scenario: Choosing Dark persists across reload

- **WHEN** the user clicks the `data-theme-choice="dark"` button
- **THEN** `localStorage.getItem("ezida.theme")` MUST equal `"dark"`
- **AND** after a full page reload, the Dark segment MUST be the
  active one
- **AND** `document.documentElement.getAttribute("data-theme")` MUST
  equal `"dark"` on the reloaded page

#### Scenario: Choosing System persists across reload

- **WHEN** the user clicks the `data-theme-choice="system"` button
- **THEN** `localStorage.getItem("ezida.theme")` MUST equal `"system"`
- **AND** after reload, the System segment MUST be the active one
- **AND** the effective `data-theme` on `<html>` MUST resolve from
  the current `prefers-color-scheme`

#### Scenario: Invalid stored value falls back to System

- **WHEN** the page loads with `localStorage["ezida.theme"]` set to a
  value other than `"light"`, `"system"`, or `"dark"` (e.g. `"foo"`
  or stale corrupted state)
- **THEN** the System segment MUST be the active one
- **AND** the effective `data-theme` MUST be derived from
  `prefers-color-scheme`

#### Scenario: localStorage is blocked

- **WHEN** the page loads in an environment where `localStorage` reads
  and writes throw (private browsing, blocked storage)
- **THEN** the page MUST render without an uncaught exception in the
  console
- **AND** the System segment MUST be active by default
- **AND** clicking any toggle segment MUST update the active state
  and the `data-theme` attribute for the session, even though no
  value is persisted

### Requirement: Topbar exposes a Filter button that toggles a popover

The topbar SHALL render a Filter button in its right zone. Clicking
the button MUST toggle a popover anchored to the button. The popover
MUST close on Escape and on any click outside its bounds. Closing the
popover MUST NOT clear the filter text.

#### Scenario: Click opens the popover

- **WHEN** the page is loaded and the user clicks the Filter button
- **THEN** the DOM MUST contain a visible filter popover element
- **AND** the popover MUST contain an `<input>` element with focus

#### Scenario: Click on the button while popover is open closes it

- **WHEN** the popover is open and the user clicks the Filter button
  again
- **THEN** the popover MUST be hidden
- **AND** the filter text MUST be unchanged

#### Scenario: Escape closes the popover

- **WHEN** the popover is open and the user presses Escape
- **THEN** the popover MUST be hidden
- **AND** the filter text MUST be unchanged

#### Scenario: Outside click closes the popover

- **WHEN** the popover is open and the user clicks any element
  outside the popover and outside the Filter button
- **THEN** the popover MUST be hidden
- **AND** the filter text MUST be unchanged

#### Scenario: Closing the popover preserves the filter

- **WHEN** the user has typed `auth` into the filter input and
  presses Escape to close the popover
- **THEN** the popover MUST be hidden
- **AND** the filter state MUST still be `auth`
- **AND** non-matching cards MUST remain hidden in their columns

### Requirement: Filter matches title, description, and tags case-insensitively

The filter SHALL perform a case-insensitive substring match against
each card's concatenated title, description, and tag values. Every
keystroke in the filter input MUST update the rendered set of visible
cards. Whitespace-only queries MUST be treated as an empty filter
(every card visible).

#### Scenario: Title substring match

- **WHEN** the board contains a card with title `Refactor auth flow`
  and the user types `auth` into the filter input
- **THEN** that card MUST remain visible
- **AND** cards whose title, description, and tags contain no `auth`
  substring MUST be hidden

#### Scenario: Case-insensitive match

- **WHEN** the board contains a card with title `Refactor AUTH flow`
  and the user types `auth` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: Description substring match

- **WHEN** the board contains a card with title `Cleanup` and
  description `replace the legacy auth call with the new one` and
  the user types `auth` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: Tag substring match

- **WHEN** the board contains a card with tags `["security",
  "tech-debt"]` and the user types `secur` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: Empty filter shows everything

- **WHEN** the filter input is empty
- **THEN** every card on the board MUST be rendered as visible
- **AND** no `No matches` placeholder MUST be rendered

#### Scenario: Whitespace-only filter shows everything

- **WHEN** the filter input contains only spaces
- **THEN** every card on the board MUST be rendered as visible

### Requirement: Filter state is transient and not persisted

The filter text and the popover open state SHALL exist only in the
Alpine component state. The page MUST NOT write the filter text to
`localStorage`, `sessionStorage`, cookies, or the URL. A page reload
MUST reset both the filter text and the popover open state to their
defaults.

#### Scenario: Reload clears the filter

- **WHEN** the user has typed `auth` into the filter input and then
  reloads the page
- **THEN** the filter input MUST be empty after reload
- **AND** every card on the board MUST be rendered as visible

#### Scenario: No localStorage write

- **WHEN** the user types into the filter input
- **THEN** no `localStorage` entry related to the filter (e.g. a key
  matching `*filter*` or `*query*`) MUST be created

### Requirement: Non-matching cards are hidden; columns with zero matches show a `No matches` placeholder

When the filter is non-empty, cards that do not match SHALL be
excluded from the rendered column body (not just visually hidden,
but removed from the DOM so they cannot be clicked or dragged).
Columns that have at least one total card but zero matching cards
SHALL render a `No matches` placeholder inside the column body. The
column `list-count` badge MUST continue to display the total card
count for the column (NOT the filtered count).

#### Scenario: Non-matching cards are removed from the column DOM

- **WHEN** the board contains a `todo` column with cards titled
  `Refactor auth`, `Write docs`, `Fix bug` and the user types
  `auth` into the filter input
- **THEN** the rendered `todo` column DOM MUST contain exactly one
  card element (the `Refactor auth` card)
- **AND** the `Write docs` and `Fix bug` card elements MUST NOT be
  present in the DOM

#### Scenario: Column with cards but zero matches shows `No matches`

- **WHEN** the `done` column contains 4 cards, none of whose title,
  description, or tags contain `xyz`, and the user types `xyz` into
  the filter input
- **THEN** the rendered `done` column body MUST contain exactly one
  `.no-matches` placeholder element
- **AND** the placeholder's text content MUST contain the literal
  string `No matches`
- **AND** no card elements MUST be present in the column body

#### Scenario: Column list-count badge shows total, not filtered

- **WHEN** the `todo` column contains 3 cards and the user types a
  filter that matches only 1 of them
- **THEN** the `todo` column header's `list-count` badge MUST
  display `3` (not `1`)

#### Scenario: Hidden cards cannot be clicked into the modal

- **WHEN** a card is hidden by the filter
- **THEN** clicking the position where the card would have been
  rendered MUST NOT open the edit modal (the card is not in the DOM)

#### Scenario: Empty column placeholder unchanged when filter is empty

- **WHEN** a column has zero total cards and the filter is empty
- **THEN** the column body MUST render the existing empty
  placeholder (the V1 `.empty` placeholder)
- **AND** the column body MUST NOT render a `.no-matches` placeholder

### Requirement: Filter button shows active state and mono-counter badge when filter is non-empty

When the filter text is non-empty, the Filter button SHALL render in
its active state (surface fill) and SHALL display a mono-counter
badge whose text content is the total count of matching cards across
the entire board. When the filter text is empty, the active state
and the badge MUST NOT be rendered.

#### Scenario: Active state appears when filter is non-empty

- **WHEN** the user types any non-empty value into the filter input
- **THEN** the Filter button element MUST carry a CSS class
  indicating active state (e.g. `state-active`)

#### Scenario: Mono-counter badge shows total board-wide match count

- **WHEN** the board contains 12 cards total across all columns,
  and 4 of them match the current filter text
- **THEN** the Filter button MUST render a badge element with
  mono-counter typography
- **AND** the badge's text content MUST be `4`

#### Scenario: Match count updates on every keystroke

- **WHEN** the user types one additional character into the filter
  input such that the number of matching cards changes from 4 to 1
- **THEN** the Filter button badge's text content MUST update to
  `1`

#### Scenario: Clearing the filter removes the active state and badge

- **WHEN** the filter input is non-empty and the user clears it
  (either by editing the input to empty or by clicking the
  `Clear filter` inline link)
- **THEN** the Filter button MUST NOT carry the active-state class
- **AND** the badge element MUST NOT be rendered (or MUST be hidden
  such that its text content is not visible)

#### Scenario: Clear filter link is visible only when filter is non-empty

- **WHEN** the filter input is empty
- **THEN** the popover MUST NOT render a visible `Clear filter`
  link

- **WHEN** the filter input is non-empty
- **THEN** the popover MUST render a visible `Clear filter` link
  below the input

#### Scenario: Clear filter link empties the filter

- **WHEN** the filter input contains `auth` and the user clicks the
  `Clear filter` link
- **THEN** the filter input MUST become empty
- **AND** every card on the board MUST be rendered as visible
- **AND** the popover MUST remain open

### Requirement: Each column footer renders an "Add a card" idle button

Every `.column` SHALL render a `.column-footer` element directly
after the column's `<ul.cards>`. When the composer is idle, the
footer MUST display a single button with class `button-ghost
composer-open`, full-width, with visible text `+ Add a card`
rendered in the muted-text token from the UI-1 token system. The
button MUST be present even when the column is empty (in which
case it appears beneath the V1 `.empty` placeholder).

#### Scenario: Idle footer present on every column

- **WHEN** the page loads against a board with columns
  `["todo","doing","done"]` and no filter is active
- **THEN** the DOM MUST contain exactly three `.column-footer`
  elements (one per column)
- **AND** each `.column-footer` MUST contain a visible
  `.composer-open` button whose text content includes `Add a card`

#### Scenario: Footer present on an empty column

- **WHEN** a column has zero cards
- **THEN** the column's `.column-footer` MUST be present
- **AND** the `.composer-open` button MUST be focusable and
  clickable

### Requirement: Clicking "Add a card" opens an inline composer

Clicking the `.composer-open` button SHALL hide the button and
render a `.composer` form in its place, containing a focused
`<textarea>` (placeholder text MAY read `Enter a title…`), a
primary `Add` submit button, and a ghost `Cancel` button. The
textarea MUST receive keyboard focus on the same tick the
composer becomes visible (Alpine `$nextTick`).

#### Scenario: Click opens composer with focused textarea

- **WHEN** the user clicks `.composer-open` in the `todo` column
- **THEN** the column's `.composer-open` MUST be hidden
  (`x-show="false"` or equivalent)
- **AND** the column's `.composer` element MUST be visible
- **AND** the textarea inside `.composer` MUST be the document's
  active element

#### Scenario: Composers in different columns are independent

- **WHEN** the user opens the composer in `todo`, then clicks
  `.composer-open` in `doing`
- **THEN** both composers MUST be visible (composer state lives on
  the per-column Alpine sub-scope, not on the root component)

### Requirement: Composer submit posts to `POST /api/cards`

A composer submit MUST send a card-create request to the server.
Submitting the composer — clicking `Add`, pressing `Enter` in the
textarea, or submitting the form by any other means — SHALL issue
`POST /api/cards` with a JSON body containing the column name and
the trimmed title. On 2xx, the composer MUST reset to its idle
state (button visible, textarea hidden, draft cleared). On non-2xx,
the composer MUST remain open with the server's `error.message`
(or a fallback `HTTP <status>` string) displayed inside the
composer surface.

The board re-render after a successful create is handled by the
existing `board-changed` SSE listener — the composer MUST NOT
mutate the local `cards` array directly on success.

#### Scenario: Successful submit closes the composer

- **WHEN** the user types `Draft v1` into the `todo` composer and
  clicks `Add`, and the server returns 201
- **THEN** a `POST /api/cards` request MUST have fired with a body
  containing `"column":"todo"` and `"title":"Draft v1"`
- **AND** the composer MUST return to its idle state (button
  visible, textarea hidden)
- **AND** within 500 ms of the SSE `board-changed` event, the
  column MUST contain a new `.card` element whose title is
  `Draft v1`

#### Scenario: Submit with whitespace-only draft cancels silently

- **WHEN** the user types `   ` (whitespace only) into the
  composer and presses `Enter`
- **THEN** NO `POST /api/cards` request MUST fire
- **AND** the composer MUST return to its idle state
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Server validation error keeps the composer open

- **WHEN** the user types `x` and clicks `Add`, and the server
  returns `400 MISSING_TITLE` (e.g. because of a race that cleared
  the title — pathological case, but the path MUST handle it)
- **THEN** the composer MUST remain visible
- **AND** the textarea's value MUST be preserved
- **AND** a visible error message MUST appear inside the composer,
  containing the server's `error.message`

### Requirement: Composer keyboard handling

The composer textarea SHALL respond to keyboard input as follows:

- `Enter` (no modifier) MUST submit the composer.
- `Shift+Enter` MUST insert a newline (default textarea behaviour
  preserved).
- `Escape` MUST cancel the composer (return to idle, discard the
  draft).

#### Scenario: Enter submits

- **WHEN** focus is in the composer textarea, the draft is `Hello`,
  and the user presses `Enter`
- **THEN** a `POST /api/cards` request MUST fire

#### Scenario: Shift+Enter inserts a newline

- **WHEN** focus is in the composer textarea and the user presses
  `Shift+Enter`
- **THEN** NO `POST /api/cards` request MUST fire
- **AND** the textarea's value MUST contain a newline character

#### Scenario: Escape cancels

- **WHEN** focus is in the composer textarea, the draft is
  non-empty, and the user presses `Escape`
- **THEN** NO `POST /api/cards` request MUST fire
- **AND** the composer MUST return to its idle state

### Requirement: Composer blur cancels when focus leaves the surface

The composer MUST cancel itself when focus leaves the composer
surface. When the textarea loses focus and the related target is
**outside** the composer surface (e.g. focus moves to another part of
the page), the composer SHALL cancel and return to idle. When the
related target is inside the composer (e.g. the user clicks the
`Add` or `Cancel` button), the composer MUST NOT cancel on blur.

#### Scenario: Click outside cancels

- **WHEN** the composer is open with a non-empty draft and the
  user clicks elsewhere on the page (outside the composer)
- **THEN** the composer MUST return to its idle state
- **AND** NO `POST /api/cards` request MUST fire

#### Scenario: Click on Add button does not blur-cancel

- **WHEN** the composer is open and the user clicks the `Add`
  button
- **THEN** the composer MUST submit (not cancel)

### Requirement: Cancel button discards the draft

Clicking the composer's `Cancel` button SHALL return the composer
to its idle state without firing any HTTP request.

#### Scenario: Cancel discards the draft

- **WHEN** the composer is open with draft `Draft v1` and the user
  clicks `Cancel`
- **THEN** the composer MUST return to its idle state
- **AND** NO `POST /api/cards` request MUST fire
- **AND** opening the composer again MUST show an empty textarea

### Requirement: Each card carries a hover-revealed delete button

Every `.card` SHALL contain a `<button class="card-delete">`
positioned absolutely in its top-right corner, 22 px round, with
the literal character `×` as its visible glyph and
`aria-label="Delete card"`. The button MUST be hidden by default
(opacity 0 and `pointer-events: none`) and MUST become visible and
interactive on `.card:hover`. On the button's own `:hover`, its
background and glyph MUST shift to the danger-tinted tokens
(`--danger-bg` / `--danger-fg`) from the UI-1 token system.

#### Scenario: Delete button present on every card

- **WHEN** the board renders against a fixture with three cards
- **THEN** each `.card` element MUST contain exactly one
  `.card-delete` button
- **AND** each `.card-delete` button MUST have
  `aria-label="Delete card"`

#### Scenario: Delete button hidden until card hover

- **WHEN** the page renders and no card is hovered
- **THEN** every `.card-delete` button's computed style MUST have
  `opacity: 0` and `pointer-events: none`

#### Scenario: Delete button visible on card hover

- **WHEN** the user hovers a `.card` element
- **THEN** that card's `.card-delete` button's computed style MUST
  have `opacity: 1` and `pointer-events: auto`

### Requirement: Clicking the delete button issues `DELETE /api/cards/:id`

Clicking the `.card-delete` button SHALL issue a `DELETE
/api/cards/<id>` request, stop event propagation so the card's
click-to-open-modal handler does NOT fire, and rely on the
existing SSE listener to refetch the board on success. No
confirmation dialog, no undo, no optimistic mutation.

On a 404 response (the card was already deleted elsewhere — CLI
race, manual edit, etc.) the page MUST refetch `/api/board` to
recover from the drift.

#### Scenario: Click deletes without opening the modal

- **WHEN** the user hovers a card and clicks its `.card-delete`
  button
- **THEN** a `DELETE /api/cards/<id>` request MUST fire
- **AND** the V3 edit modal MUST NOT open (propagation stopped)

#### Scenario: 404 triggers a refetch

- **WHEN** the user clicks `.card-delete` on a card whose `id` no
  longer exists on disk (e.g. deleted by the CLI between renders)
- **THEN** the server returns `404 CARD_NOT_FOUND`
- **AND** the client MUST issue a follow-up `GET /api/board` to
  reconcile its local state

### Requirement: Delete button is immune to drag-end mouseup

The page MUST suppress card delete requests that originate from a
Sortable drag-end whose mouseup happens to land on the `.card-delete`
region. A pointer drag started on a card that releases over
`.card-delete` MUST NOT trigger a delete. Implementation: the root
component sets a transient `_dragJustEnded` flag inside Sortable's
`onEnd` and clears it on the next tick; `deleteCard` checks the flag
and returns early when set.

#### Scenario: Drag-end over delete button does not delete

- **WHEN** the user starts a drag on a card, moves the pointer,
  and releases the mouse with the pointer over the `.card-delete`
  button
- **THEN** NO `DELETE /api/cards/<id>` request MUST fire
- **AND** the card MUST still exist in the rendered board after
  the drop completes

### Requirement: Dashed Add-list placeholder + composer

The page SHALL render a dashed-border placeholder element with class
`add-list-placeholder` and visible label "Add list" (or equivalent
short call-to-action) at the end of the column strip, after the last
column. Click on the placeholder MUST swap it for an inline composer
(class `list-composer`) consisting of a text `<input>`, an Add
button, and a Cancel button. Composer state (`composingList`,
`listDraft`, `listError`) MUST live on the `board()` Alpine component
(ADR 0003 §D10). Enter in the input or click on Add MUST issue
`POST /api/columns` with the trimmed input value. Escape or Cancel
MUST collapse the composer back to the placeholder without a
network request. On 2xx response, the composer MUST collapse and
the SSE refetch MUST update the rendered columns. On non-2xx
response, the composer MUST remain open and display the server's
`error.message` (or "HTTP <status>" fallback) inline below the
input.

#### Scenario: Placeholder rendered after the last column

- **WHEN** the page renders a board with 3 columns
- **THEN** the DOM MUST contain exactly one
  `.add-list-placeholder` element
- **AND** the placeholder MUST be the last child of the column
  strip container (after every `.column` element)

#### Scenario: Click placeholder opens the composer

- **WHEN** the user clicks the `.add-list-placeholder`
- **THEN** the placeholder MUST be hidden
- **AND** a `.list-composer` element MUST be visible in its place
- **AND** the composer's text input MUST have keyboard focus

#### Scenario: Enter submits the composer

- **WHEN** the user types `review` in the composer's input and
  presses Enter
- **THEN** a `POST /api/columns` request MUST fire with body
  `{"name":"review"}`

#### Scenario: Successful submit collapses the composer

- **WHEN** the server returns 201 with `{"columns":[...]}`
- **THEN** the composer MUST collapse back to the placeholder
- **AND** the input MUST be empty
- **AND** any prior error message MUST be cleared

#### Scenario: Server error keeps the composer open

- **WHEN** the user submits a name that the server refuses with
  `COLUMN_ALREADY_EXISTS`
- **THEN** the composer MUST remain visible
- **AND** the composer MUST display the server's `error.message`
  inline (e.g. below the input)
- **AND** the input value MUST be preserved so the user can edit
  it

#### Scenario: Escape cancels without a request

- **WHEN** the composer is open with some text in the input and the
  user presses Escape
- **THEN** the composer MUST collapse back to the placeholder
- **AND** no network request MUST fire

#### Scenario: Cancel button cancels without a request

- **WHEN** the composer is open and the user clicks Cancel
- **THEN** the composer MUST collapse back to the placeholder
- **AND** no network request MUST fire

#### Scenario: Empty submit shows inline error and stays open

- **WHEN** the user submits an empty (or whitespace-only) value
- **THEN** the composer MUST remain open
- **AND** the composer MUST display an inline error message
- **AND** no network request MUST fire (client-side guard)

### Requirement: List header exposes a 3-dots menu with Delete action

Each rendered `.list-header` SHALL contain a button (class
`list-menu-btn`) on its right side. Clicking the button MUST toggle
the corresponding column's menu open (and close any other open
menu). The menu (class `list-menu`) MUST contain at least one
action: a "Delete list" button styled in the danger color (class
`menu-item danger`). Clicking Delete MUST issue
`DELETE /api/columns/:name` for the corresponding column. The menu
MUST close on click-outside or Escape. On a 2xx response, the menu
MUST close and the SSE refetch MUST update the rendered columns. On
a non-2xx response, the menu MUST remain open and display the
server's `error.message` inline inside the menu.

#### Scenario: 3-dots button rendered in every list header

- **WHEN** the page renders a board with 3 columns
- **THEN** every `.list-header` MUST contain exactly one
  `.list-menu-btn` element

#### Scenario: Click opens the menu

- **WHEN** the user clicks the `.list-menu-btn` for the `todo`
  column
- **THEN** the corresponding `.list-menu` MUST be visible
- **AND** the menu MUST contain a `.menu-item.danger` button labeled
  "Delete list" (or equivalent short danger action)

#### Scenario: Successful delete closes the menu

- **WHEN** the user clicks Delete on an empty column and the server
  returns 200
- **THEN** the menu MUST close
- **AND** the SSE refetch MUST update the column strip to no longer
  show the deleted column

#### Scenario: COLUMN_HAS_CARDS surfaces inline

- **WHEN** the user clicks Delete on a column with cards and the
  server returns `COLUMN_HAS_CARDS`
- **THEN** the menu MUST remain visible
- **AND** the menu MUST display the server's `error.message` (or a
  derived message containing the card count) inline
- **AND** the column MUST remain in the rendered DOM unchanged

#### Scenario: CANNOT_DELETE_LAST_COLUMN surfaces inline

- **WHEN** the user clicks Delete on the only remaining column and
  the server returns `CANNOT_DELETE_LAST_COLUMN`
- **THEN** the menu MUST remain visible
- **AND** the menu MUST display the server's `error.message` inline
- **AND** the column MUST remain in the rendered DOM unchanged

#### Scenario: Click outside the menu closes it

- **WHEN** the menu is open and the user clicks anywhere outside
  the menu and its trigger button
- **THEN** the menu MUST close
- **AND** any inline error MUST be cleared

#### Scenario: Escape closes the menu

- **WHEN** the menu is open and the user presses Escape
- **THEN** the menu MUST close

#### Scenario: Only one menu open at a time

- **WHEN** the menu for column A is open and the user clicks the
  3-dots button for column B
- **THEN** the menu for column A MUST close
- **AND** the menu for column B MUST open

### Requirement: List-header title is click-to-rename

Clicking the `.column-name` span inside a list header SHALL swap
the span for an `<input>` pre-filled with the current column name.
The input MUST receive keyboard focus and the input's text MUST be
selected (so the user can type-to-replace). Enter MUST commit the
rename via `PATCH /api/columns/:name` if the trimmed value is
non-empty and differs from the current name; otherwise it MUST
revert without a network request. Escape MUST revert without a
network request. Blur MUST commit-or-revert by the same rule. On
2xx response, the input MUST swap back to a span showing the new
name (driven by the SSE refetch). On non-2xx response, the input
MUST remain visible and the server's `error.message` MUST display
inline next to the input.

#### Scenario: Click opens the rename input

- **WHEN** the user clicks the `.column-name` for column `todo`
- **THEN** the `.column-name` span MUST be hidden
- **AND** an `<input>` MUST be visible in its place with value
  `todo`
- **AND** the input MUST have keyboard focus
- **AND** the input's text MUST be selected

#### Scenario: Enter commits a changed value

- **WHEN** the user changes the value to `backlog` and presses Enter
- **THEN** a `PATCH /api/columns/todo` request MUST fire with body
  `{"name":"backlog"}`

#### Scenario: Successful rename swaps back to a span

- **WHEN** the server returns 200 with the renamed payload
- **THEN** the input MUST be hidden
- **AND** the `.column-name` span MUST be visible with the new
  value (driven by the SSE refetch)

#### Scenario: Enter with unchanged value is a no-op revert

- **WHEN** the user presses Enter without changing the value
- **THEN** no network request MUST fire
- **AND** the input MUST swap back to the span unchanged

#### Scenario: Enter with empty value is a no-op revert

- **WHEN** the user clears the input and presses Enter
- **THEN** no network request MUST fire
- **AND** the input MUST swap back to the span unchanged

#### Scenario: Escape reverts

- **WHEN** the user types a partial value and presses Escape
- **THEN** no network request MUST fire
- **AND** the input MUST swap back to the span unchanged

#### Scenario: Blur commits a changed value

- **WHEN** the user changes the value to `backlog` and the input
  loses focus (e.g. clicks elsewhere)
- **THEN** a `PATCH /api/columns/todo` request MUST fire with body
  `{"name":"backlog"}`

#### Scenario: Blur with empty or unchanged value reverts

- **WHEN** the user blurs the input with an unchanged or empty
  value
- **THEN** no network request MUST fire

#### Scenario: Server error keeps the input visible

- **WHEN** the user submits a value that the server refuses with
  `COLUMN_ALREADY_EXISTS`
- **THEN** the input MUST remain visible
- **AND** the server's `error.message` MUST display inline next to
  the input
- **AND** the input value MUST be preserved

### Requirement: List headers are drag-to-reorder

The page SHALL initialize a Sortable.js instance on the column-strip
container (`.columns`) with `handle: '.list-header'` so columns can
be reordered by dragging their headers. The instance MUST use a
`group` value distinct from the card Sortable instance's group
(established by V2) so the two instances do not interfere. On drop,
the page MUST issue `POST /api/columns/move` with the dropped
column's name and the new 0-indexed position. The drop handler MUST
read the column's name from `data-column` on the dragged
`.column` element. On a non-2xx response, the page MUST refetch
`/api/board` and re-render so the displayed state matches disk
(ADR 0002 §D3).

#### Scenario: Drag column header to reorder

- **WHEN** the user drags the `done` column's header and drops it
  before `todo` in a board with columns `["todo","ongoing","done"]`
- **THEN** a `POST /api/columns/move` request MUST fire with body
  `{"name":"done","position":0}`

#### Scenario: Server rejects the move

- **WHEN** the move endpoint returns a non-2xx response
- **THEN** the page MUST refetch `/api/board`
- **AND** the rendered column order MUST reflect the refetched
  state

#### Scenario: Card drag does not initiate column drag

- **WHEN** the user drags a card body inside a column
- **THEN** the card MUST move per the existing card Sortable
  behavior
- **AND** no `POST /api/columns/move` request MUST fire

#### Scenario: Column drag does not initiate card drag

- **WHEN** the user drags a column's header and drops it elsewhere
  in the column strip
- **THEN** a `POST /api/columns/move` request MUST fire
- **AND** no `POST /api/cards/:id/move` request MUST fire

#### Scenario: Drag handle is the list header

- **WHEN** the user presses pointer-down on a card body, the empty
  state, or the cards list
- **THEN** no column drag MUST initiate
- **AND** only pointer-down on a `.list-header` element MUST be
  able to initiate a column drag

#### Scenario: Rename input is not a drag handle

- **WHEN** the user clicks the `.column-name` to start a rename and
  the input becomes visible
- **THEN** pointer-down on the input MUST NOT initiate a column
  drag
- **AND** pointer-down on the 3-dots button MUST NOT initiate a
  column drag
