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

### Requirement: Empty columns display a placeholder

When a column has zero cards, the rendered `.column` SHALL display
a placeholder element with visible text indicating emptiness (the
literal string in v1 may be as short as `empty`).

#### Scenario: Empty column

- **WHEN** the board contains a column `backlog` with no cards
- **THEN** the corresponding `.column` element contains exactly
  one `.empty` placeholder element
- **AND** the column-count badge reads `0`

### Requirement: Priorities map to distinguishable visual styles

The page SHALL apply a CSS class `priority-<value>` to each card
whose `priority` field is set. Each value present in
`[board].priorities` MUST produce a visually distinguishable
treatment (any combination of border, background, badge, color).
The v1 palette MAY be grayscale.

#### Scenario: All three priorities present

- **WHEN** the board contains three cards with priorities `low`,
  `medium`, `high` respectively
- **THEN** each rendered card carries the matching `priority-*`
  class
- **AND** the computed CSS produces a different visual state for
  each (asserted via differing computed `border-left-color` or
  equivalent property)

#### Scenario: Card without priority

- **WHEN** a card has no `priority` field
- **THEN** the rendered `.card` MUST NOT carry any `priority-*`
  class

### Requirement: Top bar is present and minimal

The rendered page SHALL include a `<header>` element with class
`topbar` containing at least the application name. The bar MUST be
visually distinct from the board area (a border or background
suffices). Connection status indicator and project-directory name
are deferred and MUST NOT be present in v1.

#### Scenario: Topbar present

- **WHEN** the page loads
- **THEN** the DOM contains exactly one `header.topbar` element
- **AND** the topbar's text content includes `Ezida`

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
`/api/board` returns a non-2xx response on initial load, and MUST log
the error to the browser console for developer visibility. The page
MUST NOT show an alert or toast in v1.

#### Scenario: Server returns 500

- **WHEN** the page loads against a server whose `kanban.toml` is
  missing (returns 500)
- **THEN** the rendered DOM still contains the topbar
- **AND** the `.loading` placeholder remains visible
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
