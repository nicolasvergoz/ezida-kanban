# Card Writing Specification (delta)

## MODIFIED Requirements

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

When the deleted card is referenced as the `epic` of other cards, the
command SHALL clear the `epic` field on every one of them in the same
write, rather than refusing the deletion. Orphaning MUST NOT refresh
those cards' `updated_at` — losing a parent is a board-level
consequence, not an edit to the child, consistent with how column
rename propagates.

The command MUST report the orphaned cards. In text mode the count and
ids MUST appear on stderr after the success line. In JSON mode the
success envelope MUST carry them.

When the deleted card is a child, no other card is affected.

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
- **THEN** stdout equals `{"id":"a3f2k9","deleted":true,"orphaned":[]}\n`

#### Scenario: Deleting a parent orphans its children

- **WHEN** `ezida rm rl4m9x --yes` is invoked and three cards carry
  `epic = "rl4m9x"`
- **THEN** the process exits with code `0`
- **AND** card `rl4m9x` no longer appears in `kanban.toml`
- **AND** none of the three surviving cards contains an `epic` key
- **AND** each surviving card's `updated_at` is unchanged

#### Scenario: Orphaning is reported in JSON

- **WHEN** `ezida rm rl4m9x --yes --json` is invoked and cards
  `f20wbo`, `wrshlo`, `42q7t6` carry `epic = "rl4m9x"`
- **THEN** stdout's `orphaned` array MUST contain exactly those three
  ids in board file order

#### Scenario: Orphaning is reported in text mode

- **WHEN** `ezida rm rl4m9x --yes` is invoked and three cards carry
  `epic = "rl4m9x"`
- **THEN** stderr MUST name the count of orphaned cards and list their
  ids

#### Scenario: Deleting a child affects no other card

- **WHEN** `ezida rm f20wbo --yes` is invoked and `f20wbo` carries
  `epic = "rl4m9x"`
- **THEN** card `rl4m9x` MUST be byte-unchanged, including its `color`
  and `updated_at`
