## MODIFIED Requirements

### Requirement: Clicking a card opens a detail modal

The page SHALL open a modal dialog when the user clicks a `.card`
element. The modal MUST display the card's current values for title,
description, priority, column, and tags as rendered text (NOT as form
inputs), and MUST display the card's `id`, `created_at`, and
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

#### Scenario: Column is rendered as an editable field, not read-only metadata

- **WHEN** the user clicks a card in column `todo` on a board with
  columns `backlog, todo, ongoing, done`
- **THEN** the modal MUST render `todo` as text inside a `.field`
  element (the column field), NOT inside the read-only footer
- **AND** the read-only footer MUST NOT contain the card's column
  value

## ADDED Requirements

### Requirement: Modal exposes section labels above each editable field

The modal MUST render a small uppercase monospace label above each
editable field-row (Title, Description, Priority, Column, Tags) so
users can identify what they are editing without first clicking.
Each label MUST be a sibling element with a stable class
(`.modal-label`) preceding its corresponding `.field-row`. Labels MUST
NOT be inputs and MUST NOT trap clicks intended for the field below.

#### Scenario: Each editable field has a preceding uppercase label

- **WHEN** the modal is open for any card
- **THEN** the DOM MUST contain at least five `.modal-label` elements
  with visible text matching (case-insensitive): `title`,
  `description`, `priority`, `column`, `tags`
- **AND** each `.modal-label` MUST appear in document order
  immediately before its corresponding `.field-row` element

#### Scenario: Clicking a label does NOT enter edit mode

- **WHEN** the user clicks a `.modal-label` element
- **THEN** no field MUST switch into edit mode
- **AND** no PATCH request MUST fire

### Requirement: Priority rendered cell shows a colored dot and caret

The modal's priority `.field` (rendered state) MUST display a small
colored dot whose background color matches the board's configured
`priority_colors.<id>` value for the card's current priority, followed
by the priority label, followed by a chevron / caret glyph indicating
the field is a selector. When the card's priority is empty the dot
MUST still render with a neutral muted background and the label MUST
remain `no priority`. The chip styling MUST NOT change the field's
click-to-edit behavior: clicking the `.field` MUST still enter
priority edit mode (revealing the `<select>` editor).

#### Scenario: Dot color matches `priority_colors` from `/api/board`

- **WHEN** the board configuration includes
  `[board.priority_colors] high = "#ef4444"` and the user opens a card
  whose priority is `high`
- **THEN** the priority `.field` MUST contain an element with class
  `prio-dot` whose computed `background-color` resolves to `#ef4444`

#### Scenario: Caret indicates the field is a selector

- **WHEN** the modal is open with a card of any priority
- **THEN** the priority `.field` MUST contain a caret / chevron glyph
  visible to the user (e.g. an SVG, `▾`, or equivalent)

#### Scenario: Clicking the priority chip enters edit mode

- **WHEN** the user clicks anywhere inside the priority `.field`
  (including the dot or the caret)
- **THEN** the priority `<select>` editor MUST become visible and
  focused per the existing inline-edit contract

### Requirement: Modal exposes a column-move field with click-to-edit

The modal SHALL render the card's current column as a `.field`
element (rendered state) displaying the column title plus a caret.
Clicking the field MUST switch it into a `<select>` editor listing
every column from `[board].columns`, with the card's current column
preselected. Choosing a different column MUST issue
`POST /api/cards/{id}/move` with body
`{"column": <new column name>, "position": 0}`. Choosing the same
column MUST NOT issue any network request. On a 2xx response the
field MUST return to rendered mode with the new column label. On a
non-2xx response the field MUST remain in edit mode AND a
`.field-error` element directly under the column field MUST display
the server's `error.message` (or `HTTP <status>` fallback).

#### Scenario: Click column field reveals a `<select>` of all columns

- **WHEN** the user opens a card in column `todo` on a board with
  columns `backlog, todo, ongoing, done` and clicks the column
  `.field`
- **THEN** a `<select>` element MUST be visible and focused
- **AND** the `<select>` MUST contain one `<option>` per board column
  in the same order as `[board].columns`
- **AND** the `<option>` whose value is `todo` MUST be the selected
  option

#### Scenario: Choosing a different column fires a move request

- **WHEN** the user opens a card in column `todo` and changes the
  column `<select>` to `ongoing`
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with body
  containing `"column":"ongoing"` and `"position":0`
- **AND** on 2xx the column `.field` MUST display `ongoing` in
  rendered text mode
- **AND** the `<select>` editor MUST NOT be visible

#### Scenario: Selecting the same column is a no-op

- **WHEN** the user opens a card in column `todo`, clicks the column
  `.field`, and selects `todo` again
- **THEN** no `POST /api/cards/<id>/move` request MUST fire
- **AND** no `PATCH /api/cards/<id>` request MUST fire
- **AND** the column `.field` MUST return to rendered text mode

#### Scenario: Server error keeps column field in edit mode with inline message

- **WHEN** the user changes the column `<select>` and the server
  returns 400 with message `column "archive" not configured`
- **THEN** the column editor MUST remain visible
- **AND** a `.field-error` element directly under the column field
  MUST display the text `column "archive" not configured`

### Requirement: Modal header exposes a delete-card action

The modal SHALL render a delete action (typically a small trash icon
button) in its header. Activating the button MUST first prompt the
user via `window.confirm` (or equivalent confirmation gate). On
confirmation the page MUST issue `DELETE /api/cards/{id}` and, on
2xx, MUST close the modal. The id-copy affordance on the header MUST
remain functional: clicking the rendered id span MUST copy the id to
the clipboard and surface a transient `copied` confirmation, exactly
as today.

#### Scenario: Trash button asks to confirm before deleting

- **WHEN** the user clicks the trash button in the modal header
- **THEN** the page MUST invoke `window.confirm`
- **AND** no `DELETE` request MUST fire until the user confirms

#### Scenario: Confirmed delete removes the card and closes the modal

- **WHEN** the user clicks the trash button and confirms the prompt
- **THEN** a `DELETE /api/cards/<id>` request MUST fire
- **AND** on 2xx the `.modal` element MUST NOT be visible

#### Scenario: Cancelled delete leaves the modal open

- **WHEN** the user clicks the trash button and cancels the prompt
- **THEN** no network request MUST fire
- **AND** the `.modal` element MUST remain visible

#### Scenario: Clicking the id span still copies the id

- **WHEN** the user clicks the rendered `.modal-id` element for a
  card with `id="a4zkwn"`
- **THEN** the clipboard MUST contain `a4zkwn`
- **AND** the `.modal-id-copied` indicator MUST appear transiently

### Requirement: Modal footer shows relative dates with absolute tooltip

The modal footer SHALL render exactly two items — Created and
Updated — each composed of a label and a value. Values MUST be
rendered as a human-readable relative string derived from the card's
`created_at` / `updated_at` ISO timestamp (e.g. `just now`,
`5 min ago`, `2 h ago`, `3 d ago`, `23 May 2026`). The same element
MUST carry a `title` attribute exposing the absolute date AND time of
day formatted as `YYYY-MM-DD HH:MM` (24-hour, local). The two items
MUST be visually grouped with a thin separator (rule or pipe) and
MUST NOT render the raw ISO string in visible text.

#### Scenario: Recent update renders as relative string

- **WHEN** the user opens a card whose `updated_at` is 30 minutes
  before now
- **THEN** the modal footer's Updated value MUST contain the
  substring `min ago` (e.g. `30 min ago`)
- **AND** the visible text MUST NOT contain a `T` (no ISO marker)

#### Scenario: Absolute date+time is available in the tooltip

- **WHEN** the user opens a card whose `updated_at` is
  `2026-05-23T09:45:07Z`
- **THEN** the modal footer's Updated value MUST carry a `title`
  attribute matching the regular expression
  `\d{4}-\d{2}-\d{2} \d{2}:\d{2}` (date plus hours and minutes)

#### Scenario: Footer no longer renders the column

- **WHEN** the modal is open for any card
- **THEN** the modal footer MUST contain exactly two `<span>`-or-
  equivalent metadata items: one labelled `created` and one labelled
  `updated` (case-insensitive)
- **AND** the footer MUST NOT render the card's column value
