# Board Config Specification

## Purpose

CLI surface for editing cards and administering board taxonomy: `ezida edit`, `ezida columns add/rm/rename`, `ezida priorities add/rm/rename`. Includes propagation (rename), refusal (rm with references), and CARDS_AFFECTED error class.

## Requirements

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
  `[board].columns`. The card MUST be re-placed at the end of the new
  column's existing cards (same logic as `move`).
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

#### Scenario: Edit changes column re-orders the card

- **WHEN** `ezida edit a3f2k9 --column=ongoing` is invoked on a card
  currently in `todo`, when the board contains cards in the file order
  `[a3f2k9(todo), X(ongoing), Y(ongoing)]`
- **THEN** the resulting file order is `[X(ongoing), Y(ongoing), a3f2k9(ongoing)]`

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

### Requirement: `ezida columns done|undone` toggles a column's terminal status

`ezida columns done <name>` SHALL mark the named column as terminal. `ezida columns undone <name>` SHALL clear the marker. Both operate on the bare column name — the `*` suffix is a file-format detail and MUST NEVER be accepted or required as an argument.

Both sub-commands MUST be idempotent: marking an already-terminal column, or clearing an already-plain one, exits `0` and leaves `kanban.toml` byte-unchanged.

An unknown column name MUST exit `1` with code `COLUMN_NOT_FOUND`.

`ezida columns` with no sub-command SHALL list the board's columns with their card counts and MUST indicate which columns are terminal.

#### Scenario: Mark a column terminal

- **WHEN** `ezida columns done wont-fix` is invoked against a board whose columns are `["todo", "wont-fix"]`
- **THEN** the process exits with code `0`
- **AND** the saved `[board].columns` equals `["todo", "wont-fix*"]`

#### Scenario: Clear a terminal marker

- **WHEN** `ezida columns undone done` is invoked against a board whose columns are `["todo", "done*"]`
- **THEN** the process exits with code `0`
- **AND** the saved `[board].columns` equals `["todo", "done"]`

#### Scenario: Marking is idempotent

- **WHEN** `ezida columns done done` is invoked against a board where `done` is already terminal
- **THEN** the process exits with code `0`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Suffix is rejected as an argument

- **WHEN** `ezida columns done 'done*'` is invoked against a board whose columns are `["todo", "done"]`
- **THEN** the process exits with code `1`
- **AND** the error code is `COLUMN_NOT_FOUND`

#### Scenario: Unknown column rejected

- **WHEN** `ezida columns done ghost` is invoked and `ghost` is not a column
- **THEN** the process exits with code `1`
- **AND** the error code is `COLUMN_NOT_FOUND`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Listing marks terminal columns

- **WHEN** `ezida columns` is invoked against a board whose columns are `["todo", "done*"]`
- **THEN** stdout MUST list both columns
- **AND** the `done` line MUST carry a terminal indicator that the `todo` line does not

### Requirement: `ezida columns add` inserts a new column

`ezida columns add <name>` SHALL append a new column to
`[board].columns`. With `--position=N`, the column MUST be inserted at
1-indexed position `N` (so `--position=1` becomes the first column).

#### Scenario: Append (default)

- **WHEN** `ezida columns add review` is invoked against a board whose
  columns are `["todo","ongoing","done"]`
- **THEN** the resulting columns are
  `["todo","ongoing","done","review"]`

#### Scenario: Insert at position 1

- **WHEN** `ezida columns add backlog --position=1` is invoked
- **THEN** `backlog` becomes the first column

#### Scenario: Insert at last+1 position is allowed

- **WHEN** `ezida columns add review --position=4` is invoked against
  a 3-column board
- **THEN** `review` becomes the last column

#### Scenario: Duplicate column rejected

- **WHEN** `ezida columns add todo` is invoked against a board where
  `todo` already exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `DUPLICATE`

#### Scenario: Position out of range rejected

- **WHEN** `ezida columns add review --position=0` or
  `--position=99` is invoked against a 3-column board
- **THEN** the process exits with code `1`
- **AND** the error code is `POSITION_OUT_OF_RANGE`

### Requirement: `ezida columns rename` propagates atomically

`ezida columns rename <old> <new>` SHALL update both
`[board].columns` and every card whose `column` equals `<old>` to use
`<new>`, in a single write. After the command, no card MUST reference
`<old>` and no card's `column` MUST be invalid.

Both `<old>` and `<new>` are bare names; the `*` suffix MUST NOT be
accepted as part of either argument. A column's terminal status MUST
survive the rename — renaming `done*` to `shipped` yields `shipped*`.

#### Scenario: Rename propagates to every referencing card

- **WHEN** `ezida columns rename todo backlog` is invoked against a
  board where 5 cards have `column = "todo"`
- **THEN** all 5 cards' `column` equals `"backlog"`
- **AND** `[board].columns` reflects the rename in the same order

#### Scenario: Old name unknown

- **WHEN** `ezida columns rename ghost backlog` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `COLUMN_NOT_FOUND`

#### Scenario: New name already exists

- **WHEN** `ezida columns rename todo done` is invoked where both
  exist
- **THEN** the process exits with code `1`
- **AND** the error code is `DUPLICATE`

#### Scenario: Rename of unused column still works

- **WHEN** `ezida columns rename review later` is invoked when no card
  references `review`
- **THEN** `[board].columns` updates; no card changes

#### Scenario: Terminal status survives a rename

- **WHEN** `ezida columns rename done shipped` is invoked against a
  board whose columns are `["todo", "done*"]`
- **THEN** the saved `[board].columns` equals `["todo", "shipped*"]`

#### Scenario: A rename target carrying a suffix is rejected

- **WHEN** `ezida columns rename todo 'later*'` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_COLUMN_NAME`
- **AND** `kanban.toml` is byte-unchanged

### Requirement: `ezida columns rm` refuses when in use

`ezida columns rm <name>` SHALL remove the column from
`[board].columns` ONLY when no card references it. Otherwise, the
command MUST refuse with `COLUMN_IN_USE` and a payload listing every
offending card.

The text-mode error MUST be:
```
Error: column "todo" still referenced by N cards:
  <id1>  <title1>
  <id2>  <title2>
Move or remove these cards first.
```
The JSON-mode error MUST include
`"details":{"column":"<name>","cards":[{"id":"...","title":"..."}]}`.

If removing the column would leave `[board].columns` empty, the
command MUST refuse with `LAST_COLUMN`.

#### Scenario: Remove unused column

- **WHEN** `ezida columns rm review` is invoked when no card
  references `review`
- **THEN** the process exits with code `0`
- **AND** `[board].columns` no longer contains `review`

#### Scenario: Refuse when cards reference the column

- **WHEN** `ezida columns rm todo` is invoked while 2 cards have
  `column = "todo"`
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `COLUMN_IN_USE`
- **AND** the JSON error's `details.cards` lists both `{id, title}`
  pairs
- **AND** the text-mode message lists both cards as
  `  <id>  <title>` (two-space indent per line)
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Refuse to remove the last column

- **WHEN** `ezida columns rm todo` is invoked against a board whose
  `[board].columns` is `["todo"]` and where no card references `todo`
- **THEN** the process exits with code `1`
- **AND** the error code is `LAST_COLUMN`

#### Scenario: Refuse to remove an unknown column

- **WHEN** `ezida columns rm ghost` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `COLUMN_NOT_FOUND`

### Requirement: `ezida priorities add|rename|rm` mirrors columns

The priorities sub-commands SHALL behave exactly like the columns
sub-commands, with these substitutions:

- Target list: `[board].priorities` instead of `[board].columns`.
- Card field affected: `priority` instead of `column`.
- "In use" definition: a priority is "in use" when at least one card's
  `priority` equals the target name.
- Error codes: `PRIORITY_IN_USE` instead of `COLUMN_IN_USE`,
  `LAST_PRIORITY` instead of `LAST_COLUMN`. `COLUMN_NOT_FOUND` is
  replaced by `INVALID_PRIORITY` for an unknown priority name.
- `--position` SHALL NOT be exposed for priorities — priorities are
  ordered low→high and the order is set at `init` time. (If a user
  needs to insert in the middle, `init --force` with the new list is
  the supported workaround for v1.)

#### Scenario: Priorities add appends

- **WHEN** `ezida priorities add urgent` is invoked against priorities
  `["low","medium","high"]`
- **THEN** the resulting priorities are `["low","medium","high","urgent"]`

#### Scenario: Priorities rename propagates

- **WHEN** `ezida priorities rename medium normal` is invoked while 3
  cards have `priority = "medium"`
- **THEN** all 3 cards' `priority` equals `"normal"` and
  `[board].priorities` reflects the rename

#### Scenario: Priorities rm refuses when in use

- **WHEN** `ezida priorities rm high` is invoked while cards reference
  it
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `PRIORITY_IN_USE`
- **AND** the JSON error's `details.cards` lists every offending card

#### Scenario: Refuse to remove the last priority

- **WHEN** `ezida priorities rm low` is invoked against a board whose
  priorities are `["low"]`
- **THEN** the process exits with code `1`
- **AND** the error code is `LAST_PRIORITY`

#### Scenario: Priorities expose no `--position` flag

- **WHEN** `ezida priorities add urgent --position=1` is invoked
- **THEN** cobra rejects the unknown flag and the process exits with
  code `1`

### Requirement: Error envelope

When a command fails, it SHALL write the error to **stderr** and exit
non-zero per the exit-code rule. The shape MUST be:

- Text mode: `Error: <human sentence>` followed by a newline. When the
  payload includes a list of affected cards (e.g. `COLUMN_IN_USE`,
  `PRIORITY_IN_USE`), each card MUST appear on its own line, two-space
  indented, as `  <id>  <title>`, with a closing line
  `Move or remove these cards first.`.
- JSON mode: `{"error":{"code":"<UPPER_SNAKE>","message":"<sentence>","details":{...}}}` followed by a newline.

Error codes MUST be drawn from a stable enumeration. The cumulative set
across phases is:

- P2: `BOARD_NOT_FOUND`, `CARD_NOT_FOUND`, `INVALID_FILTER`,
  `SCHEMA_VERSION_MISMATCH`, `VALIDATION_FAILED`, `IO_ERROR`,
  `ALREADY_INITIALIZED`.
- P3: `COLUMN_NOT_FOUND`, `INVALID_PRIORITY`, `MISSING_TITLE`,
  `INVALID_TAG`, `INTERACTIVE_REQUIRED`.
- P4: `COLUMN_IN_USE`, `PRIORITY_IN_USE`, `DUPLICATE`,
  `POSITION_OUT_OF_RANGE`, `LAST_COLUMN`, `LAST_PRIORITY`,
  `NOTHING_TO_EDIT`.
- P5: `INVALID_EPIC`, `INVALID_COLOR`, `INVALID_COLUMN_NAME`,
  `MIGRATION_NOT_NEEDED`.

Codes MUST NOT be removed or renamed across phases — additions only.

#### Scenario: JSON error for missing card

- **WHEN** `ezida get zzzzzz --json` is invoked and no card with that ID
  exists
- **THEN** stderr contains a JSON document whose `error.code` is
  `CARD_NOT_FOUND`
- **AND** the process exits with code `1`

#### Scenario: Text error for missing board

- **WHEN** `ezida board` is invoked in a directory with no `kanban.toml`
- **THEN** stderr begins with `Error: `
- **AND** the message names `kanban.toml` and suggests `ezida init`

#### Scenario: Refusal payload lists affected cards

- **WHEN** any P4 `rm` command returns `COLUMN_IN_USE` or
  `PRIORITY_IN_USE` with N referencing cards
- **THEN** in JSON mode `error.details.cards` MUST be an array of N
  `{id, title}` objects
- **AND** in text mode N lines MUST appear, each two-space indented as
  `  <id>  <title>`, followed by the line
  `Move or remove these cards first.`

#### Scenario: INVALID_EPIC carries the offending id

- **WHEN** `ezida edit f20wbo --epic=zzzzzz --json` is invoked and no
  card `zzzzzz` exists
- **THEN** `error.code` MUST equal `INVALID_EPIC`
- **AND** `error.details` MUST name the rejected id
