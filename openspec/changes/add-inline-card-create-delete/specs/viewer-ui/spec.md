## ADDED Requirements

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
