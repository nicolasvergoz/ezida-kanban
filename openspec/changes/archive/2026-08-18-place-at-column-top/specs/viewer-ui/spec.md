## MODIFIED Requirements

### Requirement: Cards are draggable across and within columns

The embedded page SHALL use the HTML5 Drag-and-Drop API on every
card and column so that cards can be dragged between columns and
reordered within a column. The card body (no separate handle) MUST
be the drag affordance (`draggable="true"`). On drop, the page MUST
issue `POST /api/cards/{id}/move` with `{ column, position }`
derived from the drop target column's name and the insertion index
relative to the column's current card list.

Dropping directly onto the column body — blank space below the last
card, or an empty column, as opposed to dropping onto another card —
MUST resolve to position `0` (the top of the column), not the
column's current card count.

#### Scenario: Drag card to another column

- **WHEN** the user drags a card from `todo` and drops it on `done`
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with
  body `{"column":"done","position":<int>}`
- **AND** the card visually appears in the `done` column at the
  dropped slot before the request resolves

#### Scenario: Drag card within the same column

- **WHEN** the user drags a card from position 0 of `todo` and
  drops it at position 2
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with
  body `{"column":"todo","position":2}`
- **AND** the card visually appears at the new slot before the
  request resolves

#### Scenario: Drop indicator above or below the hovered card

- **WHEN** the user drags a card over another card and the cursor
  is in the upper half of that card
- **THEN** a 2px accent line appears above the hovered card

- **WHEN** the cursor is in the lower half
- **THEN** the 2px accent line appears below the hovered card

#### Scenario: Drop on the column body places the card at the top

- **WHEN** the user drags a card from `todo` and drops it on the
  `done` column's blank space below its last card (not on another
  card)
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with
  body `{"column":"done","position":0}`
- **AND** the card visually appears at the top of the `done` column
  before the request resolves

#### Scenario: Drop on an empty column places the card at the top

- **WHEN** the user drags a card and drops it on a column with no
  existing cards
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with
  body `{"column":"<target>","position":0}`
