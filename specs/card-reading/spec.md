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
with `schema_version = 1`, the columns from `--columns` (or the defaults
`["todo", "ongoing", "done"]`), the priorities from `--priorities` (or
the defaults `["low", "medium", "high"]`), and an empty `[[cards]]`
section.

#### Scenario: Fresh init with defaults

- **WHEN** `ezida init` is run in an empty directory
- **THEN** `kanban.toml` exists
- **AND** the file parses through `board.Load` without error
- **AND** `[board].columns` equals `["todo", "ongoing", "done"]`
- **AND** `[board].priorities` equals `["low", "medium", "high"]`

#### Scenario: Init with custom columns and priorities

- **WHEN** `ezida init --columns="backlog,wip,done" --priorities="low,high"` is run
- **THEN** the resulting `[board].columns` equals
  `["backlog", "wip", "done"]`
- **AND** `[board].priorities` equals `["low", "high"]`

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

JSON output MUST follow:
```json
{
  "schema_version": 1,
  "columns": ["todo", "ongoing", "done"],
  "priorities": ["low", "medium", "high"],
  "cards_per_column": {"todo": 3, "ongoing": 1, "done": 7}
}
```

Text output MUST follow:
```
schema 1
columns:    todo (3) → ongoing (1) → done (7)
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

### Requirement: `ezida list` and its filters

`ezida list` SHALL print every card in the board by default, preserving
file order. Filters:

- `--column=<name>`: keep only cards whose `column` matches exactly.
- `--title-contains=<substr>`: keep only cards whose `title` contains
  `<substr>` (case-insensitive).
- `--tag=<tag>`: keep only cards that have `<tag>` in their `tags` array.
- `--priority=<priority>`: keep only cards whose `priority` matches
  exactly. Cards without a `priority` are excluded by this filter.

Multiple filters MUST be AND-combined.

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
      "created_at": "2026-05-20T14:30:00Z",
      "updated_at": "2026-05-20T14:30:00Z"
    }
  ]
}
```
The `description` field MUST NOT appear in `list --json` output.

Text output MUST be an aligned table with a header row:
```
ID      COLUMN   PRI   TITLE              TAGS
a3f2k9  todo     high  Refactor auth      security,tech-debt
b7m1p4  todo     -     Update README      -
```
Missing priority is rendered as `-`. Empty tags are rendered as `-`.

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

### Requirement: `ezida get` reports a single card with full description

`ezida get <id>` SHALL look up the card by exact ID and print its full
detail.

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
    "created_at": "2026-05-20T14:30:00Z",
    "updated_at": "2026-05-20T14:30:00Z"
  }
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

#### Scenario: Missing card is a user error

- **WHEN** `ezida get zzzzzz` is invoked and no card with that ID exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `CARD_NOT_FOUND`
- **AND** the text error names the searched ID

#### Scenario: Missing priority renders as dash in text mode

- **WHEN** `ezida get b7m1p4` is invoked on a card without a `priority`
- **THEN** the text output's `Priority:` line equals `Priority:   -`
- **AND** in JSON mode the `card.priority` field is omitted (not `null`)
