## ADDED Requirements

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
