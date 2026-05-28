## MODIFIED Requirements

### Requirement: `GET /api/board` returns the current board

`GET /api/board` SHALL load `kanban.toml` from the current working
directory at request time and respond with a JSON object containing
`schema_version`, `columns`, `priorities`, `priority_colors`,
`cards_per_column`, `cards`, and `project_name`. The `cards` array
MUST include the full `description` field for every card. Response
`Content-Type` MUST be `application/json`.

The top-level `project_name` field is a string set at server start
to `filepath.Base(filepath.Dir(<resolved boardPath>))` — i.e. the
parent-directory name of the resolved `kanban.toml` path. It MUST
fall back to the literal string `"Ezida"` when the computed
basename is empty, equal to `"."`, or equal to the platform path
separator. The value MUST NOT change for the lifetime of the
process (it is not re-evaluated when the board file changes).

The top-level `priority_colors` field is a JSON object mapping
priority name → hex color string. The server SHALL resolve it on
each request as follows:

1. Start with the user-provided `[board].priority_colors` map (may
   be empty or absent).
2. For each entry in the conventional default palette
   `{"low": "#22c55e", "medium": "#f59e0b", "high": "#ef4444"}`,
   if the priority name is declared in `[board].priorities` AND the
   user did not supply a color for it, fill in the default.
3. Drop any entry whose key is not in `[board].priorities` (defense
   in depth; validation already rejects this at load time).

The field MUST always be present, even when empty (`{}`). User
values MUST always win over defaults.

#### Scenario: Valid board

- **WHEN** `GET /api/board` is called against a server whose
  `kanban.toml` contains 2 columns and 3 cards
- **THEN** the response status is `200`
- **AND** `Content-Type` is `application/json`
- **AND** the body's `schema_version` equals `1`
- **AND** `cards_per_column` reflects the per-column count
- **AND** each card in `cards` has a `description` field (may be
  empty string)
- **AND** the body contains a top-level string field `project_name`
- **AND** the body contains a top-level object field `priority_colors`
  (possibly empty)

#### Scenario: Project name reflects parent directory

- **WHEN** `GET /api/board` is called against a server whose
  resolved board path is `/tmp/my-project/kanban.toml`
- **THEN** the response body's `project_name` equals `"my-project"`

#### Scenario: Project name falls back to "Ezida" at filesystem root

- **WHEN** `GET /api/board` is called against a server whose
  resolved board path produces an empty or `"."` parent-directory
  basename
- **THEN** the response body's `project_name` equals `"Ezida"`

#### Scenario: Project name is stable across requests

- **WHEN** `GET /api/board` is called twice against the same
  running server with a board file rewritten in between
- **THEN** both responses contain the same `project_name` value

#### Scenario: Board file missing

- **WHEN** `GET /api/board` is called and no `kanban.toml` exists at
  the resolved path
- **THEN** the response status is `500`
- **AND** the body is `{"error":{"code":"BOARD_NOT_FOUND",...}}`

#### Scenario: Board file has wrong schema version

- **WHEN** `GET /api/board` is called against a `kanban.toml` whose
  `schema_version` is not `1`
- **THEN** the response status is `500`
- **AND** the body's `error.code` is `SCHEMA_VERSION_MISMATCH`

#### Scenario: Defaults fill in low/medium/high when declared

- **WHEN** `GET /api/board` is called against a board whose
  `[board].priorities = ["low", "medium", "high"]` and
  `[board.priority_colors]` is absent
- **THEN** the response body's `priority_colors` equals
  `{"low":"#22c55e","medium":"#f59e0b","high":"#ef4444"}`

#### Scenario: User values override defaults

- **WHEN** the user-supplied `[board.priority_colors]` contains
  `high = "#000000"` and priorities include `low`, `medium`, `high`
- **THEN** the response body's `priority_colors.high` equals
  `"#000000"`
- **AND** `priority_colors.low` and `priority_colors.medium` equal
  the default green and orange

#### Scenario: Custom priority names without defaults are absent

- **WHEN** `[board].priorities = ["urgent"]` and
  `[board.priority_colors]` is absent
- **THEN** the response body's `priority_colors` equals `{}`

#### Scenario: Custom priority names with explicit colors are returned

- **WHEN** `[board].priorities = ["urgent"]` and
  `[board.priority_colors] = { urgent = "#ff0000" }`
- **THEN** the response body's `priority_colors` equals
  `{"urgent":"#ff0000"}`
