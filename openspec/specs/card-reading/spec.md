# Card Reading Specification

## Purpose

CLI surface for read-only commands: `ezida init`, `ezida board`, `ezida list`, `ezida get`. Establishes the global CLI conventions (flags, exit codes, output formats) every other phase reuses.

## Requirements

### Requirement: Global CLI surface

`ezida` SHALL be invokable as `ezida <command> [args] [flags]`. The
following flags MUST be available on the root command and inherited by
every subcommand:

- `--json` — emit JSON to stdout instead of human text.
- `--no-color` — force plain text output regardless of TTY detection.
- `--help` / `-h` — print usage and exit `0`.
- `--version` — print the binary's semver and exit `0`.

#### Scenario: Unknown command exits with user error

- **WHEN** `ezida wat` is invoked
- **THEN** the process exits with code `1`
- **AND** stderr contains a message naming the unknown command

#### Scenario: `--json` flag propagates to subcommands

- **WHEN** `ezida board --json` is invoked against a valid board
- **THEN** stdout is parseable as JSON
- **AND** stderr is empty

### Requirement: Exit code convention

Every command SHALL exit with one of three codes:

- `0`: success.
- `1`: user error (invalid input, target not found, validation failure).
- `2`: system error (filesystem unreachable, permission denied, internal
  failure).

#### Scenario: Validation failure yields exit 1

- **WHEN** `ezida list` is run in a directory whose `kanban.toml`
  references an undefined column
- **THEN** the process exits with code `1`

#### Scenario: Missing file yields exit 1

- **WHEN** `ezida board` is run in a directory with no `kanban.toml`
- **THEN** the process exits with code `1`
- **AND** stderr contains a message suggesting `ezida init`

#### Scenario: Permission denied yields exit 2

- **WHEN** `ezida board` is run against a `kanban.toml` whose mode is
  `0000`
- **THEN** the process exits with code `2`

### Requirement: Color and TTY handling

Text-mode output SHALL colorize only when stdout is a TTY and the
`NO_COLOR` environment variable is unset and `--no-color` is not passed.
JSON output SHALL never contain ANSI escape sequences.

#### Scenario: Piped stdout disables color

- **WHEN** `ezida list` is run with stdout piped to another process
- **THEN** stdout MUST NOT contain ANSI escape sequences

#### Scenario: NO_COLOR disables color in TTY

- **WHEN** `ezida list` is run with stdout attached to a TTY and
  `NO_COLOR=1` set
- **THEN** stdout MUST NOT contain ANSI escape sequences

### Requirement: Error envelope

When a command fails, it SHALL write the error to **stderr** and exit
non-zero per the exit-code rule. The shape MUST be:

- Text mode: `Error: <human sentence>` followed by a newline.
- JSON mode: `{"error":{"code":"<UPPER_SNAKE>","message":"<sentence>","details":{...}}}` followed by a newline.

Error codes MUST be drawn from a stable enumeration. P2 introduces the
codes: `BOARD_NOT_FOUND`, `CARD_NOT_FOUND`, `INVALID_FILTER`,
`SCHEMA_VERSION_MISMATCH`, `VALIDATION_FAILED`, `IO_ERROR`,
`ALREADY_INITIALIZED`.

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

### Requirement: `ezida init` creates a new board

`ezida init` SHALL write a fresh `kanban.toml` at the working directory
with `schema_version = 2`, the columns from `--columns` (or the defaults
`["todo", "ongoing", "done"]`), the priorities from `--priorities` (or
the defaults `["low", "medium", "high"]`), and an empty `[[cards]]`
section.

Exactly one column MUST be written as terminal, chosen by the same rule
`ezida migrate` uses: a column named `done` when present, otherwise the
last column in the list. The file MUST carry a comment above the
`columns` key explaining the `*` suffix, so the marker is
self-documenting on first read.

A user MAY pass a `*` suffix inside `--columns` to choose the terminal
columns explicitly; doing so MUST suppress the automatic choice.

#### Scenario: Fresh init with defaults

- **WHEN** `ezida init` is run in an empty directory
- **THEN** `kanban.toml` exists
- **AND** the file parses through `board.Load` without error
- **AND** `[board].columns` equals `["todo", "ongoing", "done*"]`
- **AND** `[board].priorities` equals `["low", "medium", "high"]`
- **AND** `schema_version` equals `2`

#### Scenario: Init with custom columns and priorities

- **WHEN** `ezida init --columns="backlog,wip,done" --priorities="low,high"` is run
- **THEN** the resulting `[board].columns` equals
  `["backlog", "wip", "done*"]`
- **AND** `[board].priorities` equals `["low", "high"]`

#### Scenario: Init falls back to the last column

- **WHEN** `ezida init --columns="todo,wip,shipped"` is run
- **THEN** the resulting `[board].columns` equals
  `["todo", "wip", "shipped*"]`

#### Scenario: Init honors an explicit terminal marker

- **WHEN** `ezida init --columns="todo,shipped*,wont-fix*"` is run
- **THEN** the resulting `[board].columns` equals
  `["todo", "shipped*", "wont-fix*"]`

#### Scenario: Init writes the explanatory comment

- **WHEN** `ezida init` is run in an empty directory
- **THEN** the resulting file MUST contain a comment above the `columns`
  key describing the `*` suffix

#### Scenario: Init refuses to overwrite

- **WHEN** `ezida init` is run in a directory where `kanban.toml`
  already exists
- **THEN** the process exits with code `1`
- **AND** stderr's error code (in JSON mode) is `ALREADY_INITIALIZED`
- **AND** the existing `kanban.toml` is byte-unchanged

#### Scenario: Init with `--force` overwrites

- **WHEN** `ezida init --force` is run in a directory where
  `kanban.toml` already exists
- **THEN** the process exits with code `0`
- **AND** `kanban.toml` reflects the new defaults (or flag values)

### Requirement: `ezida board` reports structure and per-column counts

`ezida board` SHALL load `kanban.toml`, then emit the board's
schema version, columns (preserving display order from `[board].columns`),
priorities (preserving order), and the number of cards per column.

Column names MUST be reported in their decoded bare form. Terminal
status MUST be reported as structured data in JSON mode and as a visual
marker in text mode — the `*` suffix MUST NOT leak into either output.

JSON output MUST follow:
```json
{
  "schema_version": 2,
  "columns": ["todo", "ongoing", "done"],
  "done_columns": ["done"],
  "priorities": ["low", "medium", "high"],
  "cards_per_column": {"todo": 3, "ongoing": 1, "done": 7}
}
```

Text output MUST follow:
```
schema 2
columns:    todo (3) → ongoing (1) → done ✓ (7)
priorities: low < medium < high
```

#### Scenario: JSON output for a populated board

- **WHEN** `ezida board --json` is invoked against a board with
  3 `todo`, 1 `ongoing`, 7 `done`
- **THEN** stdout's `cards_per_column` equals
  `{"todo":3,"ongoing":1,"done":7}`
- **AND** `columns` is the array `["todo","ongoing","done"]`

#### Scenario: Text output preserves column order

- **WHEN** `ezida board` is invoked against a board whose
  `[board].columns` is `["wip","done","backlog"]`
- **THEN** stdout's `columns:` line lists `wip`, then `done`, then
  `backlog` in that order

#### Scenario: Terminal columns are reported in JSON

- **WHEN** `ezida board --json` is invoked against a board whose
  `[board].columns` is `["todo", "shipped*", "wont-fix*"]`
- **THEN** `columns` MUST equal `["todo","shipped","wont-fix"]`
- **AND** `done_columns` MUST equal `["shipped","wont-fix"]`

#### Scenario: The suffix never leaks into output

- **WHEN** `ezida board` is invoked in either mode against a board with
  a terminal column
- **THEN** no emitted column name MUST contain a `*` character

#### Scenario: A board with no terminal columns

- **WHEN** `ezida board --json` is invoked against a board where no
  column carries the marker
- **THEN** `done_columns` MUST be an empty array
- **AND** the process exits with code `0`

### Requirement: `ezida list` and its filters

`ezida list` SHALL print every card in the board by default, preserving
file order. Filters:

- `--column=<name>`: keep only cards whose `column` matches exactly.
- `--title-contains=<substr>`: keep only cards whose `title` contains
  `<substr>` (case-insensitive).
- `--tag=<tag>`: keep only cards that have `<tag>` in their `tags` array.
- `--priority=<priority>`: keep only cards whose `priority` matches
  exactly. Cards without a `priority` are excluded by this filter.
- `--epic=<id>`: keep only the card whose `id` equals `<id>` and the
  cards whose `epic` equals `<id>`. The parent is included so that
  scoping to an epic never hides the epic itself. An id matching no card
  is rejected with `INVALID_FILTER`.

Multiple filters MUST be AND-combined.

Two flags select the archive:

- `--include-archived`: append archived cards to the live results.
- `--archived-only`: return archived cards instead of live ones.

The two MUST NOT be combined; supplying both MUST be rejected with
`MUTUALLY_EXCLUSIVE_FLAGS` and exit code `1`. When neither is supplied, the
archive MUST NOT be read and the output MUST be byte-identical to the output
the same invocation produced before archiving existed.

With `--include-archived`, live cards MUST be listed first in board file order,
followed by archived cards in archive file order. Results MUST NOT be
interleaved by date: file order is the board's only ordering concept.

Filters apply to both sets. With `--archived-only`, the validity check for
`--column`, `--priority` and `--epic` MUST widen to accept any value present
among archived cards, because an archived card can reference a column,
priority or epic the board no longer declares.

JSON output MUST follow:
```json
{
  "cards": [
    {
      "id": "a3f2k9",
      "title": "Refactor auth",
      "column": "todo",
      "priority": "high",
      "tags": ["security"],
      "epic": "rl4m9x",
      "created_at": "2026-05-20T14:30:00Z",
      "updated_at": "2026-05-20T14:30:00Z"
    }
  ]
}
```
The `description` field MUST NOT appear in `list --json` output. The
`epic` and `color` fields MUST be omitted when unset. An `archived_at` field
MUST be present on archived cards and MUST be **absent** — not null, not a zero
timestamp — on live cards.

Text output MUST be an aligned table with a header row:
```
ID      COLUMN   PRI   TITLE              TAGS
a3f2k9  todo     high  Refactor auth      security,tech-debt
b7m1p4  todo     -     Update README      -
```
Missing priority is rendered as `-`. Empty tags are rendered as `-`. An
`ARCHIVED` column MUST be appended when, and only when, one of the two archive
flags is supplied; its value is the archive date as `YYYY-MM-DD`, or `-` for a
live card.

#### Scenario: No filters returns every card

- **WHEN** `ezida list --json` is invoked against a board with 11 cards
- **THEN** the `cards` array length equals 11
- **AND** the IDs appear in the same order as in `kanban.toml`

#### Scenario: AND-combined filters

- **WHEN** `ezida list --column=todo --tag=security` is invoked
- **THEN** every returned card has `column = "todo"` AND `"security"` in
  `tags`

#### Scenario: Case-insensitive title substring

- **WHEN** `ezida list --title-contains=AUTH` is invoked against a board
  with a card titled `"Refactor auth module"`
- **THEN** that card appears in the output

#### Scenario: Description omitted in list JSON

- **WHEN** `ezida list --json` is invoked
- **THEN** no card object in `cards` contains a `description` key

#### Scenario: Unknown column filter is a user error

- **WHEN** `ezida list --column=ghost` is invoked against a board whose
  `columns` does not include `ghost`
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_FILTER`

#### Scenario: Epic filter includes the parent

- **WHEN** `ezida list --epic=rl4m9x --json` is invoked against a board
  where card `rl4m9x` exists and three cards carry `epic = "rl4m9x"`
- **THEN** the `cards` array length equals 4
- **AND** it MUST include card `rl4m9x`

#### Scenario: Unknown epic filter is a user error

- **WHEN** `ezida list --epic=zzzzzz` is invoked and no card `zzzzzz`
  exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_FILTER`

#### Scenario: Epic filter combines with other filters

- **WHEN** `ezida list --epic=rl4m9x --column=backlog` is invoked
- **THEN** every returned card MUST be in `backlog` AND MUST either
  carry `epic = "rl4m9x"` or be card `rl4m9x` itself

#### Scenario: Default output ignores the archive entirely

- **WHEN** `ezida list` is invoked against a board that has an archive file
- **THEN** stdout MUST be byte-identical to the output of the same invocation
  against the same board with the archive file removed

#### Scenario: Live cards precede archived cards

- **WHEN** `ezida list --include-archived --json` is invoked
- **THEN** every live card MUST appear before every archived card
- **AND** the live cards MUST be in board file order
- **AND** the archived cards MUST be in archive file order

#### Scenario: `archived_at` is absent for live cards

- **WHEN** `ezida list --include-archived --json` is invoked
- **THEN** no live card object MUST contain an `archived_at` key
- **AND** every archived card object MUST contain one

#### Scenario: Both archive flags is a user error

- **WHEN** `ezida list --include-archived --archived-only` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `MUTUALLY_EXCLUSIVE_FLAGS`

#### Scenario: `--archived-only` accepts a column the board dropped

- **WHEN** `ezida list --archived-only --column=review` is invoked and `review`
  exists only among archived cards
- **THEN** the process exits with code `0`
- **AND** the matching archived cards are returned

### Requirement: `ezida get` reports a single card with full description

`ezida get <id>` SHALL look up the card by exact ID and print its full
detail.

When the card carries an `epic`, the output MUST report the parent's id
and title. When the card is referenced as the `epic` of other cards, the
output MUST instead report its children with their columns, plus the
derived done/total counts. A card MUST NEVER report both, because
one-level nesting makes that state unrepresentable.

JSON output MUST follow:
```json
{
  "card": {
    "id": "a3f2k9",
    "title": "Refactor auth",
    "column": "todo",
    "priority": "high",
    "tags": ["security"],
    "description": "Move from session-based to JWT.\nCheck token expiry handling.\n",
    "epic": {"id": "rl4m9x", "title": "Card relations"},
    "created_at": "2026-05-20T14:30:00Z",
    "updated_at": "2026-05-20T14:30:00Z"
  }
}
```

For a parent card, `epic` is omitted and the card object instead carries:
```json
{
  "color": "#8b5cf6",
  "children": [{"id": "f20wbo", "title": "Card dependencies", "column": "backlog"}],
  "progress": {"done": 1, "total": 3}
}
```

Text output MUST be a key:value block:
```
ID:         a3f2k9
Title:      Refactor auth module
Column:     todo
Priority:   high
Tags:       security, tech-debt
Created:    2026-05-20T14:30:00Z
Updated:    2026-05-20T14:30:00Z

Description:
Move from session-based to JWT.
Check token expiry handling.
```

#### Scenario: Get returns full card with description

- **WHEN** `ezida get a3f2k9 --json` is invoked and card `a3f2k9` exists
  with a multi-line description
- **THEN** the `card.description` field equals the file's description
  byte-for-byte (after TOML unescaping)

#### Scenario: A child reports its parent

- **WHEN** `ezida get f20wbo --json` is invoked on a card carrying
  `epic = "rl4m9x"`
- **THEN** `card.epic.id` MUST equal `"rl4m9x"`
- **AND** `card.epic.title` MUST equal that card's title
- **AND** `card.children` and `card.progress` MUST be absent

#### Scenario: A parent reports its children and progress

- **WHEN** `ezida get rl4m9x --json` is invoked on a card referenced by
  three others, one of which sits in a terminal column
- **THEN** `card.children` MUST list the three cards in board file order
  with their ids, titles, and columns
- **AND** `card.progress` MUST equal `{"done": 1, "total": 3}`
- **AND** `card.epic` MUST be absent

#### Scenario: An unrelated card reports neither

- **WHEN** `ezida get b7m1p4 --json` is invoked on a card that carries no
  `epic` and is referenced by none
- **THEN** `card.epic`, `card.children`, and `card.progress` MUST all be
  absent

#### Scenario: Text mode reports the relation

- **WHEN** `ezida get rl4m9x` is invoked on a parent card in text mode
- **THEN** stdout MUST include a line naming each child card
- **AND** stdout MUST include the done/total counts

#### Scenario: Missing card is a user error

- **WHEN** `ezida get zzzzzz` is invoked and no card with that ID exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `CARD_NOT_FOUND`
- **AND** the text error names the searched ID

#### Scenario: Missing priority renders as dash in text mode

- **WHEN** `ezida get b7m1p4` is invoked on a card without a `priority`
- **THEN** the text output's `Priority:` line equals `Priority:   -`
- **AND** in JSON mode the `card.priority` field is omitted (not `null`)
