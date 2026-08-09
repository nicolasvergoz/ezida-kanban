## ADDED Requirements

### Requirement: The rename input exposes a terminal-column check

While a list header is in rename mode, the page SHALL render a
clickable terminal-column check beside the input, reflecting whether
the column is currently terminal. Activating it MUST stage the intent
locally; it MUST NOT issue a request of its own.

The control MUST act on pointer-down and MUST prevent the default
focus shift, so activating it never blurs the rename input. Blur is
what commits the rename, so a control that let focus leave would send
the rename before the staged marker could ride along — the request
would be split in two, or the marker lost entirely.

The control MUST carry an accessible label naming what it toggles, and
MUST expose its checked state to assistive technology. Pointer-down on
it MUST NOT initiate a column drag, on the same grounds as the rename
input itself.

While the check is displayed, the resting terminal marker in that same
header MUST NOT be rendered. Both report one fact; showing them side by
side reads as two independent settings, one of which cannot be changed.
The resting marker MUST return when the editor closes, whichever way it
closed.

#### Scenario: The resting marker stands down while the editor is open

- **WHEN** the user opens the rename input on a column listed in
  `done_columns`
- **THEN** that header MUST render the staged check
- **AND** that header MUST NOT render the resting terminal marker

#### Scenario: The resting marker returns when the editor closes

- **WHEN** the user opens the rename input on a terminal column and then
  presses Escape
- **THEN** that header MUST render the resting terminal marker again

#### Scenario: The check reflects the current state on open

- **WHEN** the user opens the rename input on a column listed in
  `done_columns`
- **THEN** the check MUST render in its checked state

#### Scenario: The check reflects a non-terminal column

- **WHEN** the user opens the rename input on a column absent from
  `done_columns`
- **THEN** the check MUST render in its unchecked state

#### Scenario: Activating the check fires no request

- **WHEN** the user activates the check
- **THEN** no network request MUST fire
- **AND** the check MUST show the toggled state

#### Scenario: Activating the check keeps the input focused

- **WHEN** the user activates the check while the rename input holds
  keyboard focus
- **THEN** the rename input MUST still hold keyboard focus
- **AND** no `PATCH /api/columns/:name` request MUST fire

#### Scenario: The check is not a drag handle

- **WHEN** the user presses pointer-down on the check
- **THEN** no column drag MUST initiate

#### Scenario: The check is labelled and exposes its state

- **WHEN** the check is rendered
- **THEN** it MUST expose an accessible label naming the
  terminal-column setting
- **AND** it MUST expose whether it is currently checked

### Requirement: The list menu exposes a terminal-column toggle

The `⋯` list menu SHALL offer a terminal-column entry alongside the
existing `Delete list` entry, so the marker is reachable without
entering a rename. The entry MUST show the column's current state.

Activating it MUST issue a single `PATCH /api/columns/:name` with body
`{"done": <negated current value>}` and MUST close the menu. It MUST
NOT send a `name` key, so a stale client name can never be written back
to the board as a rename.

#### Scenario: The menu offers the entry

- **WHEN** the user opens the `⋯` menu on a list header
- **THEN** a terminal-column entry MUST be present
- **AND** the `Delete list` entry MUST still be present

#### Scenario: Marking a column terminal from the menu

- **WHEN** the user activates the entry on the column `shipped`, which
  is absent from `done_columns`
- **THEN** exactly one `PATCH /api/columns/shipped` request MUST fire
  with body `{"done":true}`
- **AND** the request body MUST NOT contain a `name` key
- **AND** the menu MUST close

#### Scenario: Clearing the marker from the menu

- **WHEN** the user activates the entry on the column `done`, which is
  present in `done_columns`
- **THEN** exactly one `PATCH /api/columns/done` request MUST fire with
  body `{"done":false}`

#### Scenario: The entry reflects the current state

- **WHEN** the menu is opened on a column present in `done_columns`
- **THEN** the entry MUST render in its checked state
- **AND** on a column absent from `done_columns` it MUST render
  unchecked

#### Scenario: The header marker follows the toggle

- **WHEN** the user marks a non-terminal column terminal from the menu
  and the board refetches
- **THEN** that list header MUST render the resting check mark

## MODIFIED Requirements

### Requirement: List-header title is click-to-rename

Clicking the `.column-name` span inside a list header SHALL swap
the span for an `<input>` pre-filled with the current column name.
The input MUST receive keyboard focus and the input's text MUST be
selected (so the user can type-to-replace).

The edit carries two values: the name in the input, and the staged
terminal-column value seeded from the column's current state. Enter
MUST commit via a single `PATCH /api/columns/:name` when the trimmed
name is non-empty **and** at least one of the two values differs from
the column's current state; otherwise it MUST revert without a network
request. The request body MUST carry `name` only when the name changed
and `done` only when the staged terminal value changed, so a commit
never asserts a value the user did not touch.

Escape MUST revert both values without a network request. Blur MUST
commit-or-revert by the same rule. A trimmed-empty name MUST revert the
whole edit, staged terminal value included — the input is in an invalid
state and there is no committing half of it.

On 2xx response, the input MUST swap back to a span showing the new
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
- **AND** the request body MUST NOT contain a `done` key

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

#### Scenario: A staged marker alone commits on Enter

- **WHEN** the user opens the rename on `shipped`, activates the
  terminal check without editing the name, and presses Enter
- **THEN** exactly one `PATCH /api/columns/shipped` request MUST fire
  with body `{"done":true}`
- **AND** the request body MUST NOT contain a `name` key

#### Scenario: A rename and a staged marker commit as one request

- **WHEN** the user opens the rename on `review`, changes the name to
  `shipped`, activates the terminal check, and presses Enter
- **THEN** exactly one `PATCH /api/columns/review` request MUST fire
- **AND** its body MUST equal `{"name":"shipped","done":true}`

#### Scenario: Escape discards a staged marker

- **WHEN** the user activates the terminal check and then presses
  Escape
- **THEN** no network request MUST fire
- **AND** the column's terminal state MUST be unchanged after the
  input swaps back

#### Scenario: An emptied name discards the staged marker too

- **WHEN** the user activates the terminal check, clears the input,
  and presses Enter
- **THEN** no network request MUST fire
- **AND** the column's terminal state MUST be unchanged

#### Scenario: Activating the check twice commits nothing

- **WHEN** the user activates the terminal check twice, returning it
  to its original state, and presses Enter without editing the name
- **THEN** no network request MUST fire
