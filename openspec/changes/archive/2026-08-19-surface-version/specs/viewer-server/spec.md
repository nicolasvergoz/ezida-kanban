## MODIFIED Requirements

### Requirement: `GET /api/board` returns the current board

`GET /api/board` SHALL load `kanban.toml` from the current working
directory at request time and respond with a JSON object containing
`schema_version`, `columns`, `done_columns`, `priorities`,
`priority_colors`, `cards_per_column`, `cards`, `project_name`, and
`version`. The `cards` array MUST include the full `description` field
for every card. Response `Content-Type` MUST be `application/json`.

Each card object MAY carry `epic` (the six-character id of another card
on the board) and `color` (a CSS hex string). Both fields use
`omitempty` semantics — a card with neither produces a payload
byte-identical to the pre-epic shape.

The response MUST NOT carry any denormalized relation data: no
`epic_title`, no `epic_color`, no `children`, no `progress`. The
envelope already contains every card on the board, so a client resolves
a parent by id and counts children itself. Duplicating that data would
create a second source of truth that every mutation endpoint would have
to keep correct.

The top-level `columns` array MUST contain bare column names. Terminal
status is reported separately in `done_columns`, a string array listing
the names of columns whose cards count as done. The `*` marker used in
`kanban.toml` MUST NEVER appear in any response field. `done_columns`
MUST always be present, even when empty (`[]`), and every entry MUST
also appear in `columns`.

The top-level `project_name` field is a string set at server start
to `filepath.Base(filepath.Dir(<resolved boardPath>))` — i.e. the
parent-directory name of the resolved `kanban.toml` path. It MUST
fall back to the literal string `"Ezida"` when the computed
basename is empty, equal to `"."`, or equal to the platform path
separator. The value MUST NOT change for the lifetime of the
process (it is not re-evaluated when the board file changes).

The top-level `version` field is a string set at server start from the
build-time `server.Version` constant. It MUST always be present, even
for local builds (it renders `"dev"`). The value MUST NOT change for
the lifetime of the process and MUST NOT be re-evaluated on each
request — it is a build constant, not derived from the board file.

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
- **AND** the body's `schema_version` equals `2`
- **AND** `cards_per_column` reflects the per-column count
- **AND** each card in `cards` has a `description` field (may be
  empty string)
- **AND** the body contains a top-level string field `project_name`
- **AND** the body contains a top-level string field `version`
- **AND** the body contains a top-level object field `priority_colors`
  (possibly empty)
- **AND** the body contains a top-level array field `done_columns`
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

#### Scenario: Version reflects the build constant

- **WHEN** `GET /api/board` is called against a server whose binary
  was built with `server.Version=v0.4.0-beta`
- **THEN** the response body's `version` equals `"v0.4.0-beta"`

#### Scenario: Version renders "dev" for a local build

- **WHEN** `GET /api/board` is called against a server whose binary
  was built without an ldflags override
- **THEN** the response body's `version` equals `"dev"`

#### Scenario: Version is stable across requests even when the board changes

- **WHEN** `GET /api/board` is called twice against the same running
  server with the board file rewritten in between
- **THEN** both responses contain the same `version` value

#### Scenario: Board file missing

- **WHEN** `GET /api/board` is called and no `kanban.toml` exists at
  the resolved path
- **THEN** the response status is `500`
- **AND** the body's `error.code` is `BOARD_NOT_FOUND`

#### Scenario: Board file has wrong schema version

- **WHEN** `GET /api/board` is called against a `kanban.toml` whose
  `schema_version` is not `2`
- **THEN** the response status is `500`
- **AND** the body's `error.code` is `SCHEMA_VERSION_MISMATCH`

#### Scenario: Defaults fill in low/medium/high when declared

- **WHEN** `GET /api/board` is called against a board whose
  `[board].priorities = ["low", "medium", "high"]` and
  `[board.priority_colors]` is absent
- **THEN** the response body's `priority_colors` equals
  `{"low":"#22c55e","medium":"#f59e0b","high":"#ef4444"}`

#### Scenario: User values override defaults

- **WHEN** the user-supplied `[board].priority_colors` contains
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
  `[board].priority_colors] = { urgent = "#ff0000" }`
- **THEN** the response body's `priority_colors` equals
  `{"urgent":"#ff0000"}`

#### Scenario: Epic and color are carried per card

- **WHEN** `GET /api/board` is called against a board where card
  `f20wbo` has `epic = 'rl4m9x'` and card `rl4m9x` has
  `color = '#8b5cf6'`
- **THEN** the `f20wbo` object's `epic` equals `"rl4m9x"`
- **AND** the `rl4m9x` object's `color` equals `"#8b5cf6"`

#### Scenario: Cards without epic or color omit the keys

- **WHEN** `GET /api/board` is called against a board where no card
  carries an `epic` or a `color`
- **THEN** no object in `cards` contains an `epic` key
- **AND** no object in `cards` contains a `color` key

#### Scenario: No relation data is denormalized

- **WHEN** `GET /api/board` is called against a board containing an
  epic with three children
- **THEN** no object in `cards` contains `epic_title`, `epic_color`,
  `children`, or `progress`

#### Scenario: Terminal columns are reported separately from names

- **WHEN** `GET /api/board` is called against a board whose
  `[board].columns` is `['todo', 'shipped*', 'wont-fix*']`
- **THEN** the response body's `columns` equals
  `["todo","shipped","wont-fix"]`
- **AND** `done_columns` equals `["shipped","wont-fix"]`
- **AND** no string anywhere in the response contains a `*` character

#### Scenario: A board with no terminal column reports an empty array

- **WHEN** `GET /api/board` is called against a board where no column
  carries the marker
- **THEN** `done_columns` equals `[]`
- **AND** the response status is `200`