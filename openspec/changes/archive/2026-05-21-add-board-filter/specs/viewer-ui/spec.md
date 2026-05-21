## ADDED Requirements

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
