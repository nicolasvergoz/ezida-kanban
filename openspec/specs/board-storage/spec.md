# Board Storage Specification

## Purpose

Persist a Kanban board to disk as a single TOML file with atomic writes, schema-version checks, ID generation, and a nine-rule validator. This is the foundation every other phase builds on.

## Requirements

### Requirement: File schema and on-disk format

The system SHALL persist a Kanban board as a single UTF-8 encoded
`kanban.toml` file using TOML v1.0. The schema MUST follow
`refs/PROJECT_BRIEF.md` §5: a top-level `schema_version` integer, a `[board]`
table with `columns` and `priorities` string arrays, and zero or more
`[[cards]]` array-of-table entries with the fields `id`, `title`, `column`,
`description`, `created_at`, `updated_at`, `tags`, and optional `priority`.

#### Scenario: Round-trip preserves all fields

- **WHEN** a valid `kanban.toml` fixture is loaded and then saved without
  modification
- **THEN** the resulting file MUST contain the same `schema_version`,
  the same `[board]` arrays in the same order, the same `[[cards]]` blocks
  in the same order, and the same field values for every card

#### Scenario: Card order is preserved across writes

- **WHEN** a board with three cards `[a, b, c]` in the same column is
  loaded, an unrelated field on card `b` is mutated, and the board is
  saved
- **THEN** the saved file's `[[cards]]` blocks MUST appear in the order
  `[a, b, c]`

#### Scenario: Column order is preserved across writes

- **WHEN** a board with columns `["todo", "ongoing", "done"]` is loaded
  and saved
- **THEN** the saved `[board].columns` array MUST be exactly
  `["todo", "ongoing", "done"]`

### Requirement: Schema version compatibility check

`Load` SHALL refuse to return a `*Board` when the file's `schema_version`
does not equal the version supported by the binary. It MUST return an error
of type `SchemaVersionError` carrying both the file's version and the
supported one.

#### Scenario: Mismatched schema version

- **WHEN** a file with `schema_version = 2` is loaded by a binary that
  supports version 1
- **THEN** `Load` MUST return a non-nil error of type `SchemaVersionError`
- **AND** the error MUST report file version `2` and supported version `1`
- **AND** no `*Board` value is returned

#### Scenario: Matching schema version

- **WHEN** a file with `schema_version = 1` is loaded by a binary that
  supports version 1
- **THEN** `Load` MUST return a populated `*Board` and a nil error
  (assuming the rest of the file passes validation)

### Requirement: Atomic persistence

`Save` SHALL write the board to disk atomically. It MUST write the new
contents to a temporary file in the same directory and rename the temp file
over the target. The target file MUST NEVER exist in a half-written state.

#### Scenario: Successful save replaces the file

- **WHEN** `Save("kanban.toml", b)` is called and succeeds
- **THEN** `kanban.toml` MUST contain the marshaled board
- **AND** no `.kanban.toml.tmp` file MUST remain in the directory

#### Scenario: Failed marshal leaves original file untouched

- **WHEN** marshaling fails before the rename step
- **THEN** the original `kanban.toml` MUST be unchanged
- **AND** the temp file MUST be removed if it was created

### Requirement: ID format and generation

Card IDs SHALL be exactly six characters drawn uniformly from the alphabet
`[0-9a-z]`. The package MUST expose `NewID() string` for unconditional
generation and `NewUniqueID(existing []string) (string, error)` that retries
up to ten times against the provided set and returns `ErrIDExhausted` on
exhaustion.

#### Scenario: NewID format

- **WHEN** `NewID()` is called
- **THEN** the returned string MUST match the regular expression
  `^[0-9a-z]{6}$`

#### Scenario: NewUniqueID avoids collisions

- **WHEN** `NewUniqueID(existing)` is called with a non-empty `existing`
  slice
- **THEN** the returned ID MUST NOT appear in `existing`

#### Scenario: NewUniqueID gives up after ten attempts

- **WHEN** `NewUniqueID` is invoked against a synthetic `existing` set
  that covers all 36⁶ values
- **THEN** the function MUST return `ErrIDExhausted`

### Requirement: Validation enforces the nine business rules

`Validate(b *Board)` SHALL return a non-nil `*ValidationError` when any of
the nine rules below is violated, and `nil` otherwise. The error MUST
enumerate all violations found in a single pass (no early return on the
first failure).

The nine rules:
1. `schema_version` equals the supported version.
2. `[board].columns` is non-empty and contains no duplicates.
3. `[board].priorities` is non-empty and contains no duplicates.
4. Every card's `id` matches `^[0-9a-z]{6}$`.
5. Card IDs are unique across the board.
6. Every card's `title` is non-empty.
7. Every card's `column` exists in `[board].columns`.
8. Every card's `priority`, when present, exists in `[board].priorities`.
9. `created_at` and `updated_at` are non-zero timestamps and
   `updated_at >= created_at`.

#### Scenario: Valid board passes

- **WHEN** `Validate` is called on a board that satisfies all nine rules
- **THEN** it MUST return `nil`

#### Scenario: Duplicate card IDs are reported

- **WHEN** `Validate` is called on a board whose cards include two entries
  with `id = "a3f2k9"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 5 and reference both offending cards

#### Scenario: Card references unknown column

- **WHEN** `Validate` is called on a board whose card has
  `column = "wip"` but `[board].columns = ["todo", "done"]`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 7 and name the offending card and
  the missing column

#### Scenario: Card references unknown priority

- **WHEN** `Validate` is called on a board whose card has
  `priority = "urgent"` but `[board].priorities = ["low", "high"]`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 8

#### Scenario: Multiple violations in one pass

- **WHEN** `Validate` is called on a board that violates rules 6 and 7
- **THEN** it MUST return one `*ValidationError` whose details list both
  violations

### Requirement: Default placement appends to the end of a column

The package SHALL expose a helper `AppendCardToColumn(b *Board, c Card)`
that places the card at the position immediately after the last card whose
`column` matches `c.Column`, or at the end of `b.Cards` if no such card
exists. This codifies the "append to bottom" behavior (ADR §D12) so write
phases inherit a single implementation.

#### Scenario: Card appended after existing same-column cards

- **WHEN** a board has cards `[A(todo), B(done), C(todo)]` (in this slice
  order) and a new card `D(todo)` is appended via
  `AppendCardToColumn`
- **THEN** `b.Cards` MUST equal `[A(todo), B(done), C(todo), D(todo)]`

#### Scenario: First card in a column appends to end of slice

- **WHEN** a board has cards `[A(todo)]` and a new card `B(done)` is
  appended via `AppendCardToColumn`
- **THEN** `b.Cards` MUST equal `[A(todo), B(done)]`

### Requirement: Structured error types for downstream consumers

Failures returned by this package MUST be one of: `SchemaVersionError`,
`*ValidationError`, `ErrIDExhausted`, or an `error` wrapping a stdlib I/O
error from the underlying filesystem. Each typed error MUST carry enough
context (file version, list of violations, etc.) for a CLI layer to map
the failure to a stable error code per ADR §D8.

#### Scenario: ValidationError lists all violations

- **WHEN** validation fails with three rule violations
- **THEN** the returned `*ValidationError` MUST expose a slice of three
  violation entries, each naming the rule number and the offending element

#### Scenario: I/O failure is surfaced without wrapping into a typed error

- **WHEN** `Load` is called against a path that does not exist
- **THEN** the returned error MUST satisfy `errors.Is(err, fs.ErrNotExist)`
