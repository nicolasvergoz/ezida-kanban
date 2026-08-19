## MODIFIED Requirements

### Requirement: `ezida export --json` emits the full board envelope

The CLI SHALL provide an `export` subcommand that, when invoked with
`--json`, writes the same JSON envelope to stdout as the viewer's
`GET /api/board` endpoint.

Shape parity is the whole point of this command: `output.ExportCard`
and `server.cardResponse` are parallel structs, and any field added to
one MUST be added to the other in the same change. The envelope
therefore carries `done_columns` at the top level, and `epic` and
`color` per card with `omitempty` semantics.

Like `GET /api/board`, the export MUST NOT carry denormalized relation
data — no `epic_title`, no `epic_color`, no `children`, or `progress`.
Column names MUST be emitted bare; the `*` terminal marker MUST NEVER
appear in the output.

The envelope carries a top-level `version` string field set to the
build-time `server.Version` constant, so a static snapshot records the
binary that produced it. The field is always present, even for local
builds (it renders `"dev"`).

#### Scenario: Export shape matches /api/board

- **WHEN** `ezida export --json` is run from a project root
- **THEN** stdout contains a JSON object with keys `schema_version`,
  `project_name`, `version`, `columns`, `done_columns`, `priorities`,
  `cards_per_column`, and `cards`, in the same shape as the viewer's
  `boardResponse`

#### Scenario: project_name from parent dir

- **WHEN** `ezida export --json` is run inside `/some/path/my-project/`
  with a `kanban.toml`
- **THEN** the emitted `project_name` is `"my-project"`

#### Scenario: version reflects the build constant

- **WHEN** `ezida export --json` is run with a binary built with
  `server.Version=v0.4.0-beta`
- **THEN** the emitted `version` equals `"v0.4.0-beta"`

#### Scenario: version renders "dev" for a local build

- **WHEN** `ezida export --json` is run with a binary built without an
  ldflags override
- **THEN** the emitted `version` equals `"dev"`