## MODIFIED Requirements

### Requirement: `ezida edit` performs partial updates on a card

`ezida edit <id>` SHALL update one or more fields of an existing card.
At least one of `--title`, `--description`, `--priority`, `--tags`,
`--column`, `--epic`, `--no-epic`, `--color`, `--no-color` MUST be
passed; otherwise the command MUST exit `1` with code
`NOTHING_TO_EDIT`.

Behavior of each flag:

- `--title <string>`: sets the new title. Empty string rejected with
  `MISSING_TITLE`.
- `--description <string>`: sets the new description. Empty string is
  legal (clears the description).
- `--priority <p>`: sets the new priority. Empty string clears the
  field. Non-empty MUST match a value in `[board].priorities`,
  otherwise `INVALID_PRIORITY`.
- `--tags <csv>`: REPLACES the full tag list. Same parsing rules as
  `add` (`INVALID_TAG` on empty entries).
- `--column <name>`: changes the card's column. MUST match a value in
  `[board].columns`. The card MUST be re-placed at the **top** of the
  new column's existing cards (position 0, same logic as `move`),
  regardless of which other flags are combined with `--column` in the
  same invocation.
- `--epic <id>`: sets the card's parent epic. The id MUST match an
  existing card, MUST NOT equal the card's own id, and MUST NOT name a
  card that itself carries an `epic`; any violation exits `1` with
  `INVALID_EPIC`. When the named parent carries no `color`, the command
  MUST assign it one from the palette in the same write.
- `--no-epic`: clears the card's `epic` field. Mutually exclusive with
  `--epic`; passing both exits `1` with `INVALID_EPIC`.
- `--color <name|hex>`: sets the card's color. A value matching a
  palette name resolves to that entry's hex; otherwise the value MUST
  match `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`, else `INVALID_COLOR`.
- `--no-color`: clears the card's `color` field. Mutually exclusive with
  `--color`; passing both exits `1` with `INVALID_COLOR`.

In all cases, `updated_at` MUST be refreshed to the current UTC time at
second precision. `created_at` MUST be untouched. When `--epic`
triggers a color assignment on the parent, the parent's `updated_at`
MUST also be refreshed.

#### Scenario: Edit a single field

- **WHEN** `ezida edit a3f2k9 --title="New title"` is invoked on an
  existing card
- **THEN** the card's `title` equals `"New title"`
- **AND** every other field is byte-unchanged except `updated_at`,
  which is refreshed

#### Scenario: Edit multiple fields atomically

- **WHEN** `ezida edit a3f2k9 --title="New" --priority=low --tags=a,b`
  is invoked
- **THEN** all three fields are updated in a single save
- **AND** `updated_at` reflects a single moment

#### Scenario: Edit with no flags

- **WHEN** `ezida edit a3f2k9` is invoked with no field flags
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `NOTHING_TO_EDIT`

#### Scenario: Edit clears the priority

- **WHEN** `ezida edit a3f2k9 --priority=""` is invoked on a card with
  `priority = "high"`
- **THEN** the resulting card has no `priority` field in the saved TOML
- **AND** in JSON output the `card.priority` field is omitted

#### Scenario: Edit changes column places the card at the top

- **WHEN** `ezida edit a3f2k9 --column=ongoing` is invoked on a card
  currently in `todo`, when the board contains cards in the file order
  `[a3f2k9(todo), X(ongoing), Y(ongoing)]`
- **THEN** the resulting file order is `[a3f2k9(ongoing), X(ongoing), Y(ongoing)]`

#### Scenario: Edit changes column and other fields together, still places at the top

- **WHEN** `ezida edit a3f2k9 --column=ongoing --priority=high` is
  invoked on a card currently in `todo`, when the board contains cards
  in the file order `[a3f2k9(todo), X(ongoing), Y(ongoing)]`
- **THEN** the resulting file order is `[a3f2k9(ongoing), X(ongoing), Y(ongoing)]`
- **AND** `a3f2k9`'s `priority` equals `"high"`

#### Scenario: Edit JSON mode echoes the full card

- **WHEN** `ezida edit a3f2k9 --title=New --json` is invoked
- **THEN** stdout is `{"card":{...}}` containing the updated card

#### Scenario: Assigning an epic colors the parent

- **WHEN** `ezida edit f20wbo --epic=rl4m9x` is invoked and card
  `rl4m9x` carries no `color`
- **THEN** card `f20wbo` has `epic = "rl4m9x"`
- **AND** card `rl4m9x` has a `color` drawn from the palette
- **AND** both cards' `updated_at` are refreshed in a single save

#### Scenario: Assigning an epic that does not exist

- **WHEN** `ezida edit f20wbo --epic=zzzzzz` is invoked and no card
  `zzzzzz` exists
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_EPIC`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Assigning a card as its own epic

- **WHEN** `ezida edit f20wbo --epic=f20wbo` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_EPIC`

#### Scenario: Assigning an epic that is itself a child

- **WHEN** `ezida edit a3f2k9 --epic=f20wbo` is invoked and card
  `f20wbo` already carries `epic = "rl4m9x"`
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_EPIC`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Clearing an epic

- **WHEN** `ezida edit f20wbo --no-epic` is invoked on a card carrying
  `epic = "rl4m9x"`
- **THEN** the saved card MUST NOT contain an `epic` key
- **AND** card `rl4m9x` keeps its `color`

#### Scenario: Setting a color by palette name

- **WHEN** `ezida edit rl4m9x --color=emerald` is invoked
- **THEN** the saved card's `color` equals `"#10b981"`

#### Scenario: Setting a color by hex

- **WHEN** `ezida edit rl4m9x --color='#7c3aed'` is invoked
- **THEN** the saved card's `color` equals `"#7c3aed"`

#### Scenario: Malformed color rejected

- **WHEN** `ezida edit rl4m9x --color=chartreuse` is invoked and
  `chartreuse` is not a palette name
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_COLOR`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Contradictory epic flags rejected

- **WHEN** `ezida edit f20wbo --epic=rl4m9x --no-epic` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_EPIC`
