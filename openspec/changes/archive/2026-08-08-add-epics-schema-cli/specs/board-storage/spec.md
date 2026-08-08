# Board Storage Specification (delta)

## ADDED Requirements

### Requirement: Terminal columns are encoded as a name suffix

`[board].columns` SHALL encode a column's terminal status as a `*` suffix on the column name — `columns = ['backlog', 'todo', 'ongoing', 'done*']`. A column so marked is one whose cards count as done for the purpose of epic progress.

The suffix MUST exist **only in the serialized form**. `Load` SHALL decode each entry into a bare name plus a boolean, and `Save` SHALL re-encode them. In memory, `BoardConfig` MUST expose `Columns []string` holding bare names and a companion set of terminal column names; every existing comparison against `b.Board.Columns` therefore continues to work unchanged.

Cards MUST always store the bare column name. A card's `column` field MUST NEVER contain the `*` suffix.

The rationale for encoding the flag in the name rather than in a separate `done_columns` array is that a separate array can desync from `columns` under hand editing, git conflict resolution, or a stale file copy, producing a structurally valid file in which every epic silently reports zero progress. A suffix makes that state unrepresentable, and removes any propagation obligation from column rename and delete.

`Save` MUST write, immediately above the `columns` key, a comment explaining the suffix, so a reader encountering the file for the first time can decode it without consulting documentation.

#### Scenario: Suffix decodes into a bare name and a flag

- **WHEN** a file with `columns = ['todo', 'done*']` is loaded
- **THEN** `b.Board.Columns` MUST equal `["todo", "done"]`
- **AND** `done` MUST be reported as terminal
- **AND** `todo` MUST NOT be reported as terminal

#### Scenario: Round-trip preserves the suffix

- **WHEN** a file with `columns = ['backlog', 'todo', 'ongoing', 'done*']` is loaded and saved without modification
- **THEN** the saved `[board].columns` array MUST be exactly `['backlog', 'todo', 'ongoing', 'done*']`

#### Scenario: Multiple terminal columns

- **WHEN** a file with `columns = ['todo', 'shipped*', 'wont-fix*']` is loaded
- **THEN** both `shipped` and `wont-fix` MUST be reported as terminal
- **AND** the saved file MUST preserve both suffixes

#### Scenario: No terminal column is legal

- **WHEN** a file with `columns = ['todo', 'doing', 'done']` is loaded
- **THEN** `Validate` MUST return `nil`
- **AND** no column MUST be reported as terminal

#### Scenario: Cards reference bare names

- **WHEN** a board with `columns = ['todo', 'done*']` gains a card in the terminal column and is saved
- **THEN** the card's block MUST contain `column = 'done'`
- **AND** the card's block MUST NOT contain `column = 'done*'`

#### Scenario: Save writes the explanatory comment

- **WHEN** a board is saved
- **THEN** the resulting file MUST contain a comment above the `columns` key describing the `*` suffix

## MODIFIED Requirements

### Requirement: File schema and on-disk format

The system SHALL persist a Kanban board as a single UTF-8 encoded
`kanban.toml` file using TOML v1.0. The schema MUST follow
`refs/PROJECT_BRIEF.md` §5: a top-level `schema_version` integer, a `[board]`
table with `columns` and `priorities` string arrays, an OPTIONAL
`[board.priority_colors]` inline table mapping priority name → hex color
string, and zero or more `[[cards]]` array-of-table entries with the
fields `id`, `title`, `column`, `description`, `created_at`,
`updated_at`, `tags`, and optional `priority`, `epic`, and `color`.

Entries of `columns` MAY carry a trailing `*` marking the column as terminal; the marker is a serialization detail and is not part of the column's name.

`epic`, when present, holds the six-character id of another card on the same board. `color`, when present, holds a CSS hex string. Both use `omitempty` semantics — an unset value MUST NOT be written back to disk.

#### Scenario: Round-trip preserves all fields

- **WHEN** a valid `kanban.toml` fixture is loaded and then saved without
  modification
- **THEN** the resulting file MUST contain the same `schema_version`,
  the same `[board]` arrays in the same order including any terminal-column
  markers, the same `[board.priority_colors]` entries when present, the
  same `[[cards]]` blocks in the same order, and the same field values
  for every card including `epic` and `color`

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

#### Scenario: Absent epic and color round-trip as absent

- **WHEN** a card with neither `epic` nor `color` is loaded and saved
- **THEN** the saved card block MUST contain neither key

### Requirement: Schema version compatibility check

`Load` SHALL refuse to return a `*Board` when the file's `schema_version`
does not equal the version supported by the binary. It MUST return an error
of type `SchemaVersionError` carrying both the file's version and the
supported one.

`SupportedSchemaVersion` is `2`. A file at version 1 is upgradeable through `ezida migrate`, which is the only code path permitted to read a file whose version does not match.

#### Scenario: Mismatched schema version

- **WHEN** a file with `schema_version = 3` is loaded by a binary that
  supports version 2
- **THEN** `Load` MUST return a non-nil error of type `SchemaVersionError`
- **AND** the error MUST report file version `3` and supported version `2`
- **AND** no `*Board` value is returned

#### Scenario: Matching schema version

- **WHEN** a file with `schema_version = 2` is loaded by a binary that
  supports version 2
- **THEN** `Load` MUST return a populated `*Board` and a nil error
  (assuming the rest of the file passes validation)

#### Scenario: A version 1 file is refused by Load

- **WHEN** a file with `schema_version = 1` is loaded by a binary that
  supports version 2
- **THEN** `Load` MUST return a `SchemaVersionError` reporting file version
  `1` and supported version `2`

### Requirement: Validation enforces the nine business rules

`Validate(b *Board)` SHALL return a non-nil `*ValidationError` when any of
the rules below is violated, and `nil` otherwise. The error MUST
enumerate all violations found in a single pass (no early return on the
first failure).

The rules:
1. `schema_version` equals the supported version.
2. `[board].columns` is non-empty and contains no duplicates.
3. `[board].priorities` is non-empty and contains no duplicates.
4. Every card's `id` matches `^[0-9a-z]{6}$`.
5. Card IDs are unique across the board.
6. Every card's `title` is non-empty.
7. Every card's `column` exists in `[board].columns` (compared against decoded bare names).
8. Every card's `priority`, when present, exists in `[board].priorities`.
9. `created_at` and `updated_at` are non-zero timestamps and
   `updated_at >= created_at`.
10. Every key of `[board].priority_colors`, when the map is non-empty,
    exists in `[board].priorities`; every value matches
    `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`.
11. Every card's `epic`, when non-empty, equals the `id` of some card on the board.
12. No card's `epic` equals its own `id`.
13. No card carrying a non-empty `epic` is referenced as the `epic` of another card.
14. Every card's `color`, when non-empty, matches
    `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`.
15. Any card carrying a non-empty `epic` or `color` requires `schema_version >= 2`.
16. Every decoded column name is non-empty and contains no `*` character.
17. Decoded column names are unique — `['done', 'done*']` is a duplicate.

#### Scenario: Valid board passes

- **WHEN** `Validate` is called on a board that satisfies all rules
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

#### Scenario: priority_colors key not in declared priorities

- **WHEN** `Validate` is called on a board whose
  `[board].priorities = ["low", "high"]` but
  `[board.priority_colors]` contains the key `"urgent"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 10 and name the offending key

#### Scenario: priority_colors value is not a hex color

- **WHEN** `Validate` is called on a board whose
  `[board.priority_colors]` contains `low = "red"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 10 and name the offending value

#### Scenario: Epic points at a non-existent card

- **WHEN** `Validate` is called on a board whose card has `epic = "zzzzzz"`
  and no card carries that id
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 11 and name the offending card and
  the dangling id

#### Scenario: Card references itself as its epic

- **WHEN** `Validate` is called on a board whose card `a3f2k9` has
  `epic = "a3f2k9"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 12

#### Scenario: Two-level nesting is reported

- **WHEN** `Validate` is called on a board where card `A` has `epic = "B"`
  and card `B` has `epic = "C"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 13 and name card `B`

#### Scenario: Malformed card color

- **WHEN** `Validate` is called on a board whose card has `color = "violet"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 14

#### Scenario: Column name contains a reserved character

- **WHEN** `Validate` is called on a board whose decoded columns include
  `"do*ne"` or an entry that decodes to the empty string
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 16

#### Scenario: Two columns decode to the same name

- **WHEN** `Validate` is called on a board whose `[board].columns` is
  `["todo", "done", "done*"]`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 17
