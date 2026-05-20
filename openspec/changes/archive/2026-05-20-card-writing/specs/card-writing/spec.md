## ADDED Requirements

### Requirement: `ezida add` creates a new card

`ezida add "<title>" --column=<name>` SHALL create a new `[[cards]]`
entry, place it at the end of the target column's existing cards, and
write the file atomically. Required flags:

- `--column=<name>`: MUST match a value in `[board].columns`.

Optional flags:

- `--priority=<p>`: when provided, MUST match a value in
  `[board].priorities`.
- `--tags=t1,t2,...`: comma-separated; each tag is trimmed; empty
  entries are rejected with `INVALID_TAG`.
- `--description=<text>`: free-form multi-line string; defaults to empty.

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

#### Scenario: Add appends at the bottom of the column

- **WHEN** `ezida add "New"  --column=todo` is invoked against a board
  whose `[[cards]]` order is `A(todo), B(done), C(todo)`
- **THEN** the resulting card order in the file is
  `A(todo), B(done), C(todo), New(todo)`

#### Scenario: Add with malformed tag list

- **WHEN** `ezida add "Title" --column=todo --tags=,security,` is
  invoked (leading or trailing comma producing an empty tag)
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_TAG`

### Requirement: `ezida move` changes a card's column

`ezida move <id> <column>` SHALL update the card's `column` field, set
its `updated_at` to the current UTC time at second precision, and
re-place the card at the end of the new column's existing cards in
`b.Cards`.

#### Scenario: Move to an existing column

- **WHEN** `ezida move a3f2k9 ongoing` is invoked on a card currently
  in `todo`
- **THEN** the card's `column` equals `"ongoing"`
- **AND** the card's `updated_at` is strictly greater than its
  `created_at`
- **AND** the card appears in `b.Cards` at a position after the last
  pre-existing `ongoing` card

#### Scenario: Move to the same column is a no-op write

- **WHEN** `ezida move a3f2k9 todo` is invoked on a card already in
  `todo`
- **THEN** the process exits with code `0`
- **AND** the card's `updated_at` is refreshed (to honor "any
  modification refreshes `updated_at`" — invoking the command counts as
  a modification request)
- **AND** the card's position within the column is unchanged

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

### Requirement: `ezida rm` deletes a card with interactive safety

`ezida rm <id>` SHALL delete the card identified by `<id>`. Safety
rules:

- If stdout AND stdin are TTYs and `--yes` is NOT passed, the command
  MUST print the prompt `Delete card <id> "<title>"? [y/N] ` to stderr
  and read one line from stdin. Only an answer of `y` or `Y` (with
  optional surrounding whitespace) proceeds; anything else aborts with
  exit code `0` and a message `aborted` on stderr.
- If `--yes` is passed, the command MUST proceed without prompting.
- If invoked with `--json`, the command MUST require `--yes` and exit
  `1` with code `INTERACTIVE_REQUIRED` otherwise. JSON output is for
  scripts; prompts in JSON mode are forbidden.
- If invoked with stdin redirected (non-TTY) and `--yes` is NOT
  passed, the command MUST exit `1` with code `INTERACTIVE_REQUIRED`.

#### Scenario: Remove with `--yes`

- **WHEN** `ezida rm a3f2k9 --yes` is invoked against an existing card
- **THEN** the process exits with code `0`
- **AND** the card no longer appears in `kanban.toml`
- **AND** stdout (text mode) contains `removed a3f2k9` followed by a
  newline

#### Scenario: Interactive accept

- **WHEN** `ezida rm a3f2k9` is invoked in a TTY context and the user
  types `y` then enter
- **THEN** the card is removed and the process exits with code `0`

#### Scenario: Interactive reject

- **WHEN** `ezida rm a3f2k9` is invoked in a TTY context and the user
  types `n` then enter (or just presses enter)
- **THEN** the card is NOT removed
- **AND** the process exits with code `0`
- **AND** stderr contains `aborted` followed by a newline

#### Scenario: JSON mode without `--yes`

- **WHEN** `ezida rm a3f2k9 --json` is invoked (regardless of TTY)
- **THEN** the process exits with code `1`
- **AND** the error code is `INTERACTIVE_REQUIRED`
- **AND** the card is NOT removed

#### Scenario: Non-TTY without `--yes`

- **WHEN** `ezida rm a3f2k9` is invoked with stdin redirected from a
  file or pipe and `--yes` is not passed
- **THEN** the process exits with code `1`
- **AND** the error code is `INTERACTIVE_REQUIRED`

#### Scenario: Remove an unknown card

- **WHEN** `ezida rm zzzzzz --yes` is invoked and no card `zzzzzz`
  exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `CARD_NOT_FOUND`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: JSON success envelope

- **WHEN** `ezida rm a3f2k9 --yes --json` is invoked and succeeds
- **THEN** stdout equals `{"id":"a3f2k9","deleted":true}\n`

### Requirement: Mutating commands always re-validate before writing

Every command that mutates the board (`add`, `move`, `rm`, and any
future write command) SHALL call `board.Validate` after applying the
mutation in memory and before invoking `board.Save`. The save MUST be
refused if validation fails.

#### Scenario: Validation failure prevents write

- **WHEN** a hypothetical bug causes a mutating command to produce a
  card whose `column` is empty
- **THEN** `board.Save` MUST return a `*board.ValidationError` and the
  on-disk `kanban.toml` MUST remain byte-unchanged

#### Scenario: Round-trip add → move → rm

- **WHEN** `ezida add "T1" --column=todo` then
  `ezida move <id> ongoing` then `ezida rm <id> --yes` are executed in
  sequence
- **THEN** the final `kanban.toml` is byte-identical to its
  pre-sequence state (except for content unrelated to that card)
- **AND** every intermediate file state passes `board.Validate`

## MODIFIED Requirements

### Requirement: Error envelope

When a command fails, it SHALL write the error to **stderr** and exit
non-zero per the exit-code rule. The shape MUST be:

- Text mode: `Error: <human sentence>` followed by a newline.
- JSON mode: `{"error":{"code":"<UPPER_SNAKE>","message":"<sentence>","details":{...}}}` followed by a newline.

Error codes MUST be drawn from a stable enumeration. P2 introduced the
codes: `BOARD_NOT_FOUND`, `CARD_NOT_FOUND`, `INVALID_FILTER`,
`SCHEMA_VERSION_MISMATCH`, `VALIDATION_FAILED`, `IO_ERROR`,
`ALREADY_INITIALIZED`.

P3 extends the enumeration with: `COLUMN_NOT_FOUND`, `INVALID_PRIORITY`,
`MISSING_TITLE`, `INVALID_TAG`, `INTERACTIVE_REQUIRED`.

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

#### Scenario: New code surfaced via output.Fail

- **WHEN** any P3 mutating command returns a typed error of
  `*ColumnNotFoundError`, `*InvalidPriorityError`, `*MissingTitleError`,
  `*InvalidTagError`, or `*InteractiveRequiredError`
- **THEN** `output.Fail` MUST emit the corresponding code from the P3
  enumeration extension
