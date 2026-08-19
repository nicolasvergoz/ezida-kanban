## MODIFIED Requirements

### Requirement: A card referenced as an epic renders as a parent

A card referenced by the `epic` field of at least one other card SHALL render three additional signals: the four-square epic glyph immediately before its title, a border tinted from its own `color`, and a progress bar with a `done/total` counter.

`total` is the count of cards whose `epic` equals this card's id, counting both live cards and archived ones. `done` is the subset of those that sit in a done column: for a live child, its `column`; for an archived child, the `column` the archive recorded for it. In both cases the column is checked against the board's current `done_columns`. Both values MUST be computed in the client from the `/api/board` payload — which already carries `archived_cards` and `done_columns` — and neither is read from the wire.

Archived children MUST count, so that archiving finished work never makes an epic report less progress than it did before. A card referenced only by archived children is still an epic and MUST render all three signals.

All three signals MUST be conditional on the card actually having children, live or archived. A card carrying a `color` but referenced by nobody MUST render exactly as it does today.

The counter MUST use the monospace face with tabular numerals so counters align down a column.

#### Scenario: A parent renders the glyph, tint, and bar

- **WHEN** three cards carry `epic: "rl4m9x"` and card `rl4m9x` carries `color: "#8b5cf6"`
- **THEN** card `rl4m9x` renders the epic glyph before its title
- **AND** its border is tinted from `#8b5cf6`
- **AND** it renders a progress bar and counter

#### Scenario: Progress counts children in terminal columns

- **WHEN** an epic has three children and exactly one sits in a column listed in `done_columns`
- **THEN** the counter reads `1/3`
- **AND** the bar is filled to one third

#### Scenario: No terminal columns yields a zero bar

- **WHEN** an epic has three children and `done_columns` is `[]`
- **THEN** the counter reads `0/3`
- **AND** the bar renders empty rather than absent

#### Scenario: A colored card with no children is unchanged

- **WHEN** a card carries a `color` but no other card references it
- **THEN** it MUST render no glyph, no tinted border, and no progress bar

#### Scenario: A parent that is also filtered out of view

- **WHEN** an epic's children are hidden by an active filter but the parent is visible
- **THEN** the counter MUST still reflect the full board, not the filtered subset

#### Scenario: Archiving a completed child does not lower the counter

- **WHEN** an epic has three children, all in a done column, and two of them are archived
- **THEN** the counter MUST read `3/3`

#### Scenario: An archived child from a non-done column counts only toward total

- **WHEN** an epic has one live child in a done column and one child archived from a column not listed in `done_columns`
- **THEN** the counter MUST read `1/2`

#### Scenario: A card referenced only by archived children is still an epic

- **WHEN** every card referencing card `rl4m9x` as its epic has been archived
- **THEN** card `rl4m9x` MUST still render the glyph, the tinted border, and the progress bar

#### Scenario: A board with no archive renders identically

- **WHEN** the `/api/board` payload carries no `archived_cards` key
- **THEN** every epic counter MUST read exactly as it did before archived children were counted
