## MODIFIED Requirements

### Requirement: Filter matches title, description, tags, and card ID case-insensitively

The filter SHALL perform a case-insensitive substring match against
each card's title, description, tag values, and ID. The popover SHALL
expose four independent scope toggles — Title, Description, Tags, and
ID — that gate which fields the query is tested against. Every
keystroke in the filter input MUST update the rendered set of visible
cards. Whitespace-only queries MUST be treated as an empty filter
(every card visible). When all four scope toggles are off and the
query is non-empty, no card MUST match.

#### Scenario: Title substring match

- **WHEN** the board contains a card with title `Refactor auth flow`
  and the user types `auth` into the filter input
- **THEN** that card MUST remain visible
- **AND** cards whose title, description, tags, and id contain no
  `auth` substring MUST be hidden

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

#### Scenario: ID substring match

- **WHEN** the board contains a card with id `f20wbo` and the user
  types `f20wbo` into the filter input
- **THEN** that card MUST remain visible
- **AND** cards whose id, title, description, and tags contain no
  `f20wbo` substring MUST be hidden

#### Scenario: ID partial substring match

- **WHEN** the board contains a card with id `f20wbo` and the user
  types `f20` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: ID match is case-insensitive

- **WHEN** the board contains a card with id `F20WBO` and the user
  types `f20` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: ID scope can be turned off

- **WHEN** the user types a card's id into the filter input
- **AND** the ID scope pill in the popover is toggled off
- **AND** the Title, Description, and Tags scope pills are also off,
  OR none of those fields contain the id substring
- **THEN** that card MUST NOT be matched by the filter

#### Scenario: Empty filter shows everything

- **WHEN** the filter input is empty
- **THEN** every card on the board MUST be rendered as visible
- **AND** no `No matches` placeholder MUST be rendered

#### Scenario: Whitespace-only filter shows everything

- **WHEN** the filter input contains only spaces
- **THEN** every card on the board MUST be rendered as visible

## ADDED Requirements

### Requirement: Filter popover exposes an ID scope pill

The filter popover SHALL render an ID scope pill alongside the
existing Title, Description, and Tags pills. The pill MUST toggle the
`inId` scope flag in the filter state, MUST render an `aria-pressed`
attribute reflecting that flag, and MUST default to enabled on first
page load.

#### Scenario: ID pill is present in the popover

- **WHEN** the user clicks the Filter button to open the popover
- **THEN** the popover MUST contain a clickable pill labelled `ID`
- **AND** the pill MUST carry an `aria-pressed` attribute set to `true`
  on first load

#### Scenario: Clicking the ID pill toggles its state

- **WHEN** the ID pill is enabled and the user clicks it
- **THEN** the pill MUST visually reflect the disabled state
- **AND** `aria-pressed` MUST become `false`
- **AND** subsequent matches MUST stop testing the `card.id` field
