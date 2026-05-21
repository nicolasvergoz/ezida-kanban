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

### Requirement: Clicking a card opens an edit modal

The page SHALL open a modal dialog when the user clicks a `.card` element. The modal MUST pre-fill its inputs with the clicked card's current values for title, description, tags, and priority, and MUST display the card's `id`, `column`, `created_at`, and `updated_at` as read-only metadata.

#### Scenario: Click opens modal with current values

- **WHEN** the user clicks a card whose title is `Refactor auth` and tags are `["security"]`
- **THEN** the DOM MUST contain a visible `.modal` element
- **AND** the modal's title input value MUST equal `Refactor auth`
- **AND** the modal's tag chip list MUST contain a chip with text `security`

#### Scenario: Click does not open modal during a drag

- **WHEN** the user starts a drag from a card and drops it elsewhere
- **THEN** the modal MUST NOT open as a result of the drop

### Requirement: Modal saves via `PATCH /api/cards/:id`

The Save action (clicking the Save button or submitting the form) SHALL issue `PATCH /api/cards/<id>` with a JSON body containing the current title, description, tags, and priority values from the modal inputs. On a 2xx response, the modal MUST close and the page MUST refetch `/api/board` to display the updated state. On a non-2xx response, the modal MUST remain open and display the server's `error.message` (or a fallback "HTTP <status>" string).

#### Scenario: Successful save

- **WHEN** the user edits the title and clicks Save and the server returns 200
- **THEN** the modal MUST close
- **AND** the page MUST refetch and re-render the board with the new title

#### Scenario: Server rejects with validation error

- **WHEN** the user clears the title and clicks Save and the server returns 400 `MISSING_TITLE`
- **THEN** the modal MUST remain open
- **AND** the modal MUST display the error message

#### Scenario: Cancel discards changes

- **WHEN** the user types in the title field and clicks Cancel
- **THEN** the modal MUST close
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged
- **AND** the rendered card title in the board MUST be the original value

### Requirement: Priority dropdown includes a "no priority" option

The modal SHALL populate its priority `<select>` with one `<option value="">` whose visible text is `no priority` plus one `<option>` per value in `[board].priorities`. Selecting `no priority` and saving MUST clear the card's priority on disk.

#### Scenario: All priorities listed

- **WHEN** the board's `[board].priorities` is `["low","medium","high"]` and the modal opens for any card
- **THEN** the priority `<select>` MUST contain exactly 4 `<option>` elements (one for `""`, three for the values)

#### Scenario: Clearing priority via the dropdown

- **WHEN** the user opens a card whose priority is `high`, selects `no priority`, and clicks Save
- **THEN** a PATCH request MUST fire with body containing `"priority":""`
- **AND** after the response the card in the board MUST no longer show a priority badge

### Requirement: Tags are edited as chips

The modal SHALL display each existing tag as a removable chip element and SHALL provide a text input for adding new tags. Pressing Enter or comma in the input MUST add the trimmed value as a new chip (deduplicated client-side) and clear the input. Clicking a chip's remove button (`×`) MUST remove that chip from the draft tags. Save MUST send the resulting tag array.

#### Scenario: Add a tag via Enter

- **WHEN** the user opens a card with tags `["a"]`, types `b` in the tag input, and presses Enter
- **THEN** the modal MUST display two chips: `a` and `b`
- **AND** the tag input MUST be empty

#### Scenario: Remove a tag via the chip's button

- **WHEN** the user opens a card with tags `["a","b"]` and clicks the `×` on the `a` chip
- **THEN** the modal MUST display one chip: `b`

#### Scenario: Duplicate tag input is deduplicated

- **WHEN** the user opens a card with tags `["a"]` and types `a` then Enter
- **THEN** the chip list MUST still show exactly one `a` chip

### Requirement: Keyboard shortcuts in the modal

The modal SHALL respond to:

- `Esc` (anywhere in the modal or its overlay) MUST close the modal without saving.
- `Enter` while focus is in the title input MUST trigger Save.
- `Cmd+Enter` or `Ctrl+Enter` while focus is in the description textarea MUST trigger Save.

#### Scenario: Esc closes the modal

- **WHEN** the modal is open and the user presses `Esc`
- **THEN** the modal MUST close
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Enter in title saves

- **WHEN** focus is in the title input and the user presses `Enter`
- **THEN** a PATCH request MUST fire

#### Scenario: Cmd/Ctrl+Enter in description saves

- **WHEN** focus is in the description textarea and the user presses `Cmd+Enter` (on macOS) or `Ctrl+Enter` (elsewhere)
- **THEN** a PATCH request MUST fire
