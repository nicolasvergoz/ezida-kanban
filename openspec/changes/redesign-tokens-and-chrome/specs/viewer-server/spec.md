## MODIFIED Requirements

### Requirement: `GET /api/board` returns the current board

`GET /api/board` SHALL load `kanban.toml` from the current working
directory at request time and respond with a JSON object containing
`schema_version`, `columns`, `priorities`, `cards_per_column`,
`cards`, and `project_name`. The `cards` array MUST include the
full `description` field for every card. Response `Content-Type`
MUST be `application/json`.

The top-level `project_name` field is a string set at server start
to `filepath.Base(filepath.Dir(<resolved boardPath>))` — i.e. the
parent-directory name of the resolved `kanban.toml` path. It MUST
fall back to the literal string `"Ezida"` when the computed
basename is empty, equal to `"."`, or equal to the platform path
separator. The value MUST NOT change for the lifetime of the
process (it is not re-evaluated when the board file changes).

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
