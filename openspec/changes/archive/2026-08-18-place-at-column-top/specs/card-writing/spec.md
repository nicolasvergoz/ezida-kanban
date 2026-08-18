## MODIFIED Requirements

### Requirement: `ezida add` creates a new card

`ezida add "<title>" --column=<name>` SHALL create a new `[[cards]]`
entry, place it at the **top** of the target column's existing cards
(position 0), and write the file atomically. Required flags:

- `--column=<name>`: MUST match a value in `[board].columns`.

Optional flags:

- `--priority=<p>`: when provided, MUST match a value in
  `[board].priorities`.
- `--tags=t1,t2,...`: comma-separated; each tag is trimmed; empty
  entries are rejected with `INVALID_TAG`.
- `--description=<text>`: free-form multi-line string; defaults to empty.
- `--epic=<id>`: when provided, MUST name an existing card that does not
  itself carry an `epic`; any violation is rejected with `INVALID_EPIC`.
  When the named parent carries no `color`, the command MUST assign it
  one from the palette in the same write.
- `--color=<name|hex>`: when provided, sets the new card's own color. A
  palette name resolves to its hex; any other value MUST match
  `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`, else `INVALID_COLOR`.

The CLI MUST set `id` (via `board.NewUniqueID` against existing card
IDs), `created_at`, and `updated_at`. Both timestamps MUST equal the
current UTC time at second precision and MUST be identical at creation.

#### Scenario: Add with required flags only

- **WHEN** `ezida add "Refactor auth" --column=todo` is invoked against
  a fresh board
- **THEN** the resulting `kanban.toml` contains exactly one card
- **AND** that card's `column` equals `"todo"`
- **AND** that card's `title` equals `"Refactor auth"`
- **AND** that card's `description` equals `""`
- **AND** that card's `tags` equals `[]`
- **AND** that card's `created_at` equals its `updated_at`
- **AND** the saved card block contains neither an `epic` nor a `color` key
- **AND** stdout (text mode) contains only the new card's ID followed
  by a newline

#### Scenario: Add with all flags

- **WHEN** `ezida add "Refactor auth" --column=todo --priority=high --tags=security,tech-debt --description="JWT migration"` is invoked
- **THEN** the resulting card has `priority = "high"`, `tags = ["security","tech-debt"]`, and `description = "JWT migration"`

#### Scenario: Add JSON mode echoes the full card

- **WHEN** `ezida add "Refactor auth" --column=todo --json` is invoked
- **THEN** stdout is a JSON document whose `card` object contains the
  generated `id`, the title, the column, and the timestamps

#### Scenario: Add to an unknown column

- **WHEN** `ezida add "Something" --column=ghost` is invoked against a
  board whose columns do not include `ghost`
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `COLUMN_NOT_FOUND`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Add with unknown priority

- **WHEN** `ezida add "Something" --column=todo --priority=urgent` is
  invoked against a board whose priorities do not include `urgent`
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_PRIORITY`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Add with empty title

- **WHEN** `ezida add "" --column=todo` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `MISSING_TITLE`

#### Scenario: Add places the card at the top of the column

- **WHEN** `ezida add "New" --column=todo` is invoked against a board
  whose `[[cards]]` order is `A(todo), B(done), C(todo)`
- **THEN** the resulting card order in the file is
  `New(todo), A(todo), B(done), C(todo)`

#### Scenario: Add with malformed tag list

- **WHEN** `ezida add "Title" --column=todo --tags=,security,` is
  invoked (leading or trailing comma producing an empty tag)
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_TAG`

#### Scenario: Add under an epic colors the parent

- **WHEN** `ezida add "Card due dates" --column=backlog --epic=rl4m9x`
  is invoked and card `rl4m9x` carries no `color`
- **THEN** the new card has `epic = "rl4m9x"`
- **AND** card `rl4m9x` has a `color` drawn from the palette
- **AND** card `rl4m9x`'s `updated_at` is refreshed

#### Scenario: Add under an unknown epic

- **WHEN** `ezida add "Something" --column=todo --epic=zzzzzz` is
  invoked and no card `zzzzzz` exists
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_EPIC`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Add under a card that is itself a child

- **WHEN** `ezida add "Something" --column=todo --epic=f20wbo` is
  invoked and card `f20wbo` already carries `epic = "rl4m9x"`
- **THEN** the process exits with code `1`
- **AND** the error code is `INVALID_EPIC`
- **AND** `kanban.toml` is byte-unchanged

### Requirement: `ezida move` changes a card's column

`ezida move <id> <column>` SHALL update the card's `column` field, set
its `updated_at` to the current UTC time at second precision, and
re-place the card at the **top** of the new column's existing cards
(position 0) in `b.Cards`.

#### Scenario: Move to an existing column

- **WHEN** `ezida move a3f2k9 ongoing` is invoked on a card currently
  in `todo`
- **THEN** the card's `column` equals `"ongoing"`
- **AND** the card's `updated_at` is strictly greater than its
  `created_at`
- **AND** the card appears in `b.Cards` before every pre-existing
  `ongoing` card

#### Scenario: Move to the same column re-places at the top

- **WHEN** `ezida move a3f2k9 todo` is invoked on a card already in
  `todo`, not currently at the top of `todo`
- **THEN** the process exits with code `0`
- **AND** the card's `updated_at` is refreshed (to honor "any
  modification refreshes `updated_at`" — invoking the command counts as
  a modification request)
- **AND** the card is now the first `todo` card in `b.Cards`

#### Scenario: Move to an unknown column

- **WHEN** `ezida move a3f2k9 ghost` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `COLUMN_NOT_FOUND`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Move an unknown card

- **WHEN** `ezida move zzzzzz todo` is invoked and no card `zzzzzz`
  exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `CARD_NOT_FOUND`

#### Scenario: Move JSON mode echoes the updated card

- **WHEN** `ezida move a3f2k9 ongoing --json` is invoked
- **THEN** stdout is `{"card":{...}}` where the card's `column` is
  `"ongoing"` and `updated_at` reflects the refreshed timestamp
