## MODIFIED Requirements

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
