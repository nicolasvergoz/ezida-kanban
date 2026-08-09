## MODIFIED Requirements

### Requirement: Demo mutations are in-memory only
All viewer mutations (drag a card, edit a title, rename a column, toggle a column's terminal marker, delete a card, add a column, reorder columns) SHALL be applied to an in-memory copy of the board state inside the browser and SHALL NOT be persisted across page reloads.

The shim SHALL accept the same `PATCH /api/columns/<name>` body shape as the real server — `name` and `done` both optional, at least one present — and SHALL maintain `done_columns` across renames and deletions the way the server does, so a control offered on the demo page is never dead.

#### Scenario: Drag a card then refresh
- **WHEN** a visitor drags a card to a different column and then reloads the page
- **THEN** the card is back in its original column from `board.json`

#### Scenario: Mutation succeeds visually
- **WHEN** a visitor drags a card from column A to column B
- **THEN** the card immediately appears in column B in the UI and remains there until reload

#### Scenario: Toggling the terminal marker takes effect
- **WHEN** a visitor marks a non-terminal column terminal from the list menu
- **THEN** that list header renders the resting check mark
- **AND** no request leaves the page

#### Scenario: The terminal marker survives a demo rename
- **WHEN** a visitor renames a terminal column
- **THEN** the renamed column is still reported in `done_columns`

#### Scenario: Toggling the marker then refreshing
- **WHEN** a visitor toggles a column's terminal marker and then reloads the page
- **THEN** the column's terminal state is back to what `board.json` declares
