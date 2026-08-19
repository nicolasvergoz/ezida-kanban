## MODIFIED Requirements

### Requirement: The detail modal reports and edits the epic relation

The card-detail modal SHALL report a card's epic relation and SHALL offer the controls that change it.

The modal MUST render an `Epic` section on **every** card. For a card carrying an `epic`, that section names the parent — the parent's colored chip and its id — alongside a reassign control and a detach control. The parent epic chip SHALL be clickable to navigate directly to that parent epic card's detail modal. For a card carrying none, it renders an attach affordance in the same place. The always-present section is what makes a first relation reachable from a board that has none; the board surface itself remains unchanged for a board without epics.

For a card referenced as an epic, the modal MUST additionally render a `Children` section listing each child with its title and column, plus the progress bar, the `done/total` counter, an add-a-child control, and a per-row remove control. Each child row (title and column) SHALL be clickable to navigate directly to that child card's detail modal. Activating the per-row remove control SHALL detach the child from the epic without triggering navigation to the child card. A card MUST NEVER render both an assigned parent and a children list, because one-level nesting makes that state unrepresentable — and the attach affordance MUST NOT be offered on a card that has children.

Every relation write MUST be a single `PATCH /api/cards/{childId}` setting `epic` to the target id or to the empty string. The parent card MUST NEVER be written by the client to establish or break a relation.

#### Scenario: A child shows its parent with controls

- **WHEN** the modal opens on a card carrying `epic: "rl4m9x"`
- **THEN** it renders a section naming card `rl4m9x`, with that card's chip and id
- **AND** that section exposes a reassign control and a detach control
- **AND** it renders no children section

#### Scenario: A parent shows its children and progress

- **WHEN** the modal opens on a card referenced by three others, one of which is in a terminal column
- **THEN** it renders a list of those three cards with their titles and columns
- **AND** it renders a progress bar and the counter `1/3`
- **AND** it renders no assigned-parent row

#### Scenario: Children are listed in board order

- **WHEN** an epic's children appear in the `/api/board` `cards` array in the order `[c, a, b]`
- **THEN** the modal lists them in that order

#### Scenario: An unrelated card offers the attach affordance

- **WHEN** the modal opens on a card that carries no `epic` and is referenced by none
- **THEN** the modal MUST render the `Epic` section with an attach affordance
- **AND** it MUST render no children section

#### Scenario: Attaching commits one PATCH on the child

- **WHEN** the user attaches the open card to epic `rl4m9x`
- **THEN** exactly one `PATCH /api/cards/<open card id>` with body `{"epic":"rl4m9x"}` is issued
- **AND** no request is issued against `rl4m9x`
- **AND** the board is refetched and the modal shows `rl4m9x` as the parent

#### Scenario: Detaching clears the field

- **WHEN** the user activates detach on a card carrying an epic
- **THEN** exactly one `PATCH` with body `{"epic":""}` is issued for that card
- **AND** after the refetch the modal shows the attach affordance

#### Scenario: Removing a child from the parent side patches the child

- **WHEN** the user activates the remove control on the row for child `aaaaa1`
- **THEN** exactly one `PATCH /api/cards/aaaaa1` with body `{"epic":""}` is issued
- **AND** the child disappears from the list after the refetch
- **AND** the counter and the progress bar recompute

#### Scenario: A parent is never offered an epic of its own

- **WHEN** the modal opens on a card that has at least one child
- **THEN** it MUST NOT render an attach affordance for that card

#### Scenario: Clicking a child in the children list opens the child card detail modal

- **WHEN** the modal is open on an epic card
- **AND** the user clicks on a child item in the children list
- **THEN** the modal switches to display the detail view of that child card

#### Scenario: Activating remove on a child does not navigate to the child card

- **WHEN** the modal is open on an epic card
- **AND** the user clicks the remove button on a child row
- **THEN** the child is removed from the epic
- **AND** the modal remains on the parent epic card

#### Scenario: Clicking parent epic chip opens the parent epic card detail modal

- **WHEN** the modal is open on a child card that belongs to a parent epic
- **AND** the user clicks the parent epic chip in the modal's Epic section
- **THEN** the modal switches to display the detail view of that parent epic card
