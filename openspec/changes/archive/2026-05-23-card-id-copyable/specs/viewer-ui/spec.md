## ADDED Requirements

### Requirement: Card displays its ID above the title

Each `.card` element SHALL render the card's `id` value as a small
monospace label positioned above the card title. The label SHALL
use the `.t-mono-label` typographic utility for family, weight,
letter-spacing, and casing, but SHALL apply a `font-size` that is
SMALLER than `.modal-id`'s 11px so that the card-level ID stays
visually subordinate to the card title. The label colour SHALL use
the `--text-faint` token.

#### Scenario: Card-level ID renders above the title

- **WHEN** the viewer renders a card with `id="a4zkwn"`
- **THEN** the `.card` element contains a child element with class
  `card-id` whose text content is exactly `a4zkwn`
- **AND** that element precedes `.card-title` in DOM order

#### Scenario: Card-level ID is visually subordinate to the modal ID

- **WHEN** the `.card-id` and `.modal-id` elements are both rendered
- **THEN** the computed `font-size` of `.card-id` is strictly less
  than the computed `font-size` of `.modal-id`

### Requirement: Card ID elements copy to clipboard on click

The system SHALL copy a card's ID string to the system clipboard
when the user clicks either the card-level `.card-id` element or
the modal-header `.modal-id` element. The handler MUST prefer
`navigator.clipboard.writeText` and MUST fall back to a
`document.execCommand('copy')` path when the Clipboard API is
unavailable, for example on non-secure-context loads. Clicks on
`.card-id` MUST NOT also open the detail modal: the click event
MUST be stopped before reaching the card-level click handler.

#### Scenario: Card-ID click copies and does not open the modal

- **WHEN** the user clicks the `.card-id` element of a closed card
- **THEN** the card's ID is written to the system clipboard
- **AND** the detail modal does NOT open

#### Scenario: Modal-ID click copies the open card's ID

- **WHEN** the detail modal is open for a card with `id="a4zkwn"`
  and the user clicks the `.modal-id` element
- **THEN** the string `a4zkwn` is written to the system clipboard
- **AND** the modal remains open

#### Scenario: Both ID elements communicate interactivity

- **WHEN** the user hovers either `.card-id` or `.modal-id`
- **THEN** the computed `cursor` is `pointer`
