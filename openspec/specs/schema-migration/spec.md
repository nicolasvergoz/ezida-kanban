# Schema Migration Specification

## Purpose

`ezida migrate`, the v1 → v2 upgrade path, the backup file it writes, the terminal-column choice it makes and reports, and the error message an out-of-date binary produces.

## Requirements

### Requirement: `ezida migrate` upgrades a board to the supported schema version

`ezida migrate` SHALL read `kanban.toml`, upgrade it from schema version 1 to schema version 2, and write the result atomically through the same path as every other mutating command.

The command MUST bypass the version check that `board.Load` enforces — it is the only command permitted to read a file whose `schema_version` does not equal `SupportedSchemaVersion`. It MUST still reject a file that is not valid TOML, and it MUST still run `Validate` against the upgraded board before writing.

The upgrade MUST NOT alter any card's `id`, `title`, `column`, `description`, `tags`, `priority`, `created_at`, or `updated_at`. A v1 board carries no epic data, so no card is modified in content — only `schema_version` and `[board].columns` change.

#### Scenario: Successful upgrade

- **WHEN** `ezida migrate` is invoked against a valid `schema_version = 1` board with 22 cards
- **THEN** the process exits with code `0`
- **AND** the resulting file's `schema_version` equals `2`
- **AND** all 22 cards' `id`, `title`, `column`, `description`, `tags`, `priority`, `created_at`, and `updated_at` are byte-unchanged

#### Scenario: Migrate refuses an already-current board

- **WHEN** `ezida migrate` is invoked against a `schema_version = 2` board
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `MIGRATION_NOT_NEEDED`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Migrate refuses an invalid board

- **WHEN** `ezida migrate` is invoked against a `schema_version = 1` board whose cards violate a validation rule
- **THEN** the process exits with code `1`
- **AND** the error code is `VALIDATION_FAILED`
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Migrate refuses a future version

- **WHEN** `ezida migrate` is invoked against a file whose `schema_version` is `3`
- **THEN** the process exits with code `1`
- **AND** the error code is `SCHEMA_VERSION_MISMATCH`

### Requirement: `ezida migrate` writes a backup before replacing the file

Before writing the upgraded board, the command SHALL copy the original file to `kanban.toml.v1.bak` in the same directory. If a file at that path already exists it MUST be overwritten — the backup always reflects the most recent migration attempt.

If the backup cannot be written, the command MUST abort with `IO_ERROR` and leave `kanban.toml` untouched.

#### Scenario: Backup contains the pre-migration bytes

- **WHEN** `ezida migrate` succeeds
- **THEN** `kanban.toml.v1.bak` MUST exist
- **AND** its contents MUST be byte-identical to `kanban.toml` before the command ran

#### Scenario: Backup failure aborts the migration

- **WHEN** the backup file cannot be created
- **THEN** the process exits with code `1`
- **AND** the error code is `IO_ERROR`
- **AND** `kanban.toml` is byte-unchanged

### Requirement: `ezida migrate` selects and reports a terminal column

Because schema version 2 introduces terminal columns and a v1 board declares none, the command SHALL pick exactly one column to mark as terminal:

1. If a column named `done` (case-sensitive, after trim) exists in `[board].columns`, that column is marked.
2. Otherwise, the last column in `[board].columns` is marked.

The command MUST report which column it marked and why, so the choice is never silent. If the board declares no columns, `Validate` already fails and the command aborts before this step.

#### Scenario: A column named done is preferred

- **WHEN** `ezida migrate` runs against a board whose columns are `["backlog", "done", "archive"]`
- **THEN** the resulting `[board].columns` MUST mark `done` as terminal
- **AND** `archive` MUST NOT be marked

#### Scenario: Falls back to the last column

- **WHEN** `ezida migrate` runs against a board whose columns are `["todo", "wip", "shipped"]`
- **THEN** the resulting `[board].columns` MUST mark `shipped` as terminal

#### Scenario: The choice is reported

- **WHEN** `ezida migrate` succeeds in text mode
- **THEN** stdout MUST name the column that was marked terminal

#### Scenario: JSON mode reports the choice structurally

- **WHEN** `ezida migrate --json` succeeds
- **THEN** stdout MUST be a JSON document reporting at minimum the source version, the target version, and the name of the column marked terminal

### Requirement: A version mismatch names the remedy

When `board.Load` rejects a file because its `schema_version` is lower than the binary's supported version, the CLI error message SHALL name `ezida migrate` as the fix. When the file's version is higher than the binary's, the message SHALL instead direct the user to upgrade `ezida`, because no downgrade path exists.

#### Scenario: Older file suggests migrate

- **WHEN** any read or write command is invoked against a `schema_version = 1` board by a binary supporting version 2
- **THEN** the process exits with code `1`
- **AND** the error code is `SCHEMA_VERSION_MISMATCH`
- **AND** the stderr message MUST contain the literal `ezida migrate`

#### Scenario: Newer file suggests upgrading the binary

- **WHEN** any command is invoked against a `schema_version = 3` board by a binary supporting version 2
- **THEN** the process exits with code `1`
- **AND** the error code is `SCHEMA_VERSION_MISMATCH`
- **AND** the stderr message MUST NOT suggest `ezida migrate`
