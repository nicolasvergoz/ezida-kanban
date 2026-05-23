## ADDED Requirements

### Requirement: `ezida export --json` emits the full board envelope
The CLI SHALL provide an `export` subcommand that, when invoked with `--json`, writes the same JSON envelope to stdout as the viewer's `GET /api/board` endpoint.

#### Scenario: Export shape matches /api/board
- **WHEN** `ezida export --json` is run from a project root
- **THEN** stdout contains a JSON object with keys `schema_version`, `project_name`, `columns`, `priorities`, `cards_per_column`, and `cards`, in the same shape as the viewer's `boardResponse`

#### Scenario: project_name from parent dir
- **WHEN** `ezida export --json` is run inside `/some/path/my-project/` with a `kanban.toml`
- **THEN** the emitted `project_name` is `"my-project"`

#### Scenario: Empty board fields are arrays
- **WHEN** the board has no cards or no tags on a card
- **THEN** the emitted JSON renders `"cards": []` and `"tags": []`, never `null`

### Requirement: Exit codes mirror other read commands
The command SHALL exit non-zero on any error loading the board (missing file, invalid TOML, validation error) using the same error envelope format as `ezida board` and `ezida list`.

#### Scenario: Missing kanban.toml
- **WHEN** `ezida export --json` is run from a directory without `kanban.toml`
- **THEN** the command exits non-zero and emits a JSON error envelope to stdout
