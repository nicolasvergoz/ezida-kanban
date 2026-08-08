# board-export Specification (delta)

## MODIFIED Requirements

### Requirement: `ezida export --json` emits the full board envelope
The CLI SHALL provide an `export` subcommand that, when invoked with `--json`, writes the same JSON envelope to stdout as the viewer's `GET /api/board` endpoint.

Shape parity is the whole point of this command: `output.ExportCard` and `server.cardResponse` are parallel structs, and any field added to one MUST be added to the other in the same change. The envelope therefore carries `done_columns` at the top level, and `epic` and `color` per card with `omitempty` semantics.

Like `GET /api/board`, the export MUST NOT carry denormalized relation data — no `epic_title`, `epic_color`, `children`, or `progress`. Column names MUST be emitted bare; the `*` terminal marker MUST NEVER appear in the output.

#### Scenario: Export shape matches /api/board
- **WHEN** `ezida export --json` is run from a project root
- **THEN** stdout contains a JSON object with keys `schema_version`, `project_name`, `columns`, `done_columns`, `priorities`, `cards_per_column`, and `cards`, in the same shape as the viewer's `boardResponse`

#### Scenario: project_name from parent dir
- **WHEN** `ezida export --json` is run inside `/some/path/my-project/` with a `kanban.toml`
- **THEN** the emitted `project_name` is `"my-project"`

#### Scenario: Empty board fields are arrays
- **WHEN** the board has no cards or no tags on a card
- **THEN** the emitted JSON renders `"cards": []` and `"tags": []`, never `null`

#### Scenario: Epic and color are exported
- **WHEN** `ezida export --json` is run against a board where card `f20wbo` has `epic = 'rl4m9x'` and card `rl4m9x` has `color = '#8b5cf6'`
- **THEN** the `f20wbo` object's `epic` equals `"rl4m9x"`
- **AND** the `rl4m9x` object's `color` equals `"#8b5cf6"`

#### Scenario: Absent epic and color omit the keys
- **WHEN** `ezida export --json` is run against a board where no card carries an `epic` or a `color`
- **THEN** no object in `cards` contains an `epic` or `color` key

#### Scenario: Terminal columns are exported bare plus a done_columns array
- **WHEN** `ezida export --json` is run against a board whose `[board].columns` is `['todo', 'done*']`
- **THEN** `columns` equals `["todo","done"]`
- **AND** `done_columns` equals `["done"]`

#### Scenario: Export and /api/board agree byte-for-byte on the same board
- **WHEN** `ezida export --json` and `GET /api/board` are run against the same `kanban.toml` containing an epic with children
- **THEN** both documents MUST contain the same set of top-level keys
- **AND** the corresponding card objects MUST contain the same set of keys
