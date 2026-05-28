## ADDED Requirements

### Requirement: Optional `[board.priority_colors]` maps priorities to hex colors

`BoardConfig` SHALL accept an optional TOML inline table
`[board.priority_colors]` mapping priority name → hex color string.
The mapping MUST be modeled as `map[string]string` and serialized
under the TOML key `priority_colors` with `omitempty` semantics —
an absent or empty map MUST NOT be written back to disk as an empty
table on `Save`.

Each entry has two constraints, enforced by `Validate`:

- The key MUST equal a value declared in `[board].priorities`.
- The value MUST match the regular expression
  `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$` (three- or six-digit CSS hex,
  with leading `#`).

The mapping MAY be absent, empty, or a strict subset of declared
priorities. Priorities not present in the map have no configured
color (the viewer applies its default badge skin).

#### Scenario: Round-trip preserves a non-empty mapping

- **WHEN** a `kanban.toml` with
  `[board.priority_colors]\nlow = "#22c55e"\nhigh = "#ef4444"` is
  loaded and then saved without modification
- **THEN** the saved file MUST contain the same two entries under
  `[board.priority_colors]`

#### Scenario: Absent mapping round-trips as absent

- **WHEN** a `kanban.toml` without `[board.priority_colors]` is
  loaded and then saved without modification
- **THEN** the saved file MUST NOT contain a `[board.priority_colors]`
  table

#### Scenario: Empty mapping is legal

- **WHEN** a board loads with `priority_colors` unset
- **THEN** `Validate` MUST return `nil` for that field

## MODIFIED Requirements

### Requirement: Validation enforces the nine business rules

`Validate(b *Board)` SHALL return a non-nil `*ValidationError` when any of
the ten rules below is violated, and `nil` otherwise. The error MUST
enumerate all violations found in a single pass (no early return on the
first failure).

The ten rules:
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
10. Every key of `[board].priority_colors`, when the map is non-empty,
    exists in `[board].priorities`; every value matches
    `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`.

#### Scenario: Valid board passes

- **WHEN** `Validate` is called on a board that satisfies all ten rules
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

### Requirement: File schema and on-disk format

The system SHALL persist a Kanban board as a single UTF-8 encoded
`kanban.toml` file using TOML v1.0. The schema MUST follow
`refs/PROJECT_BRIEF.md` §5: a top-level `schema_version` integer, a `[board]`
table with `columns` and `priorities` string arrays, an OPTIONAL
`[board.priority_colors]` inline table mapping priority name → hex color
string, and zero or more `[[cards]]` array-of-table entries with the
fields `id`, `title`, `column`, `description`, `created_at`,
`updated_at`, `tags`, and optional `priority`.

#### Scenario: Round-trip preserves all fields

- **WHEN** a valid `kanban.toml` fixture is loaded and then saved without
  modification
- **THEN** the resulting file MUST contain the same `schema_version`,
  the same `[board]` arrays in the same order, the same
  `[board.priority_colors]` entries when present, the same `[[cards]]`
  blocks in the same order, and the same field values for every card

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
