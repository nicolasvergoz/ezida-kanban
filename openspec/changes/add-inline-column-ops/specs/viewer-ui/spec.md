## ADDED Requirements

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
