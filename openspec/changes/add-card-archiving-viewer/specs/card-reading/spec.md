## MODIFIED Requirements

### Requirement: `ezida board` reports structure and per-column counts

`ezida board` SHALL load `kanban.toml`, then emit the board's
schema version, columns (preserving display order from `[board].columns`),
priorities (preserving order), and the number of cards per column.

Column names MUST be reported in their decoded bare form. Terminal
status MUST be reported as structured data in JSON mode and as a visual
marker in text mode — the `*` suffix MUST NOT leak into either output.

JSON output SHALL additionally carry `archived_count`, the number of
cards in the sibling archive file, with `omitempty` semantics: the key
is entirely absent when the count is zero (no archive file, or an
archive with no cards), which is what keeps `ezida board --json` output
unchanged for a board that has never archived anything. Text mode is
unaffected by this requirement.

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

#### Scenario: A board with no archive omits archived_count

- **WHEN** `ezida board --json` is invoked against a board with no
  `kanban.archive.toml` on disk
- **THEN** stdout, decoded as a generic JSON object, MUST NOT contain
  an `archived_count` key

#### Scenario: A populated archive reports its count

- **WHEN** `ezida board --json` is invoked against a board whose
  archive holds 4 cards
- **THEN** stdout's `archived_count` equals `4`
