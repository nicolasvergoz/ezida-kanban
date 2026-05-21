## ADDED Requirements

### Requirement: README surfaces the Web UI

`README.md` SHALL surface the existence of the Web UI shipped via
`ezida serve` in its Quick start area, so a first-time reader can
discover the page without needing to read `docs/usage.md` or run
`ezida --help`.

#### Scenario: README mentions `ezida serve`

- **WHEN** a reader opens `README.md` at the repo root
- **THEN** the file contains the literal string `ezida serve`
- **AND** the file contains the literal string `Web UI`
- **AND** the mention appears in or near the Quick start section,
  not buried in a footer.

#### Scenario: README points to the full reference

- **WHEN** a reader scans the Web UI sub-section of `README.md`
- **THEN** the section links the reader to `docs/usage.md` for the
  full `ezida serve` reference (either by a `./docs/usage.md` link
  or by referring to the "Documentation" section that already
  points there).

### Requirement: `docs/usage.md` includes an `ezida serve` reference

`docs/usage.md` SHALL document `ezida serve` in the CLI reference
block with the same shape as other subcommands (flags table,
example), plus a list of the capabilities the Web UI exposes.

#### Scenario: usage.md has an `ezida serve` section

- **WHEN** a reader opens `docs/usage.md`
- **THEN** the file contains a heading whose text is `ezida serve`
  (e.g. `### ezida serve`)
- **AND** the section names both the `--port` and `--no-open`
  flags
- **AND** the section mentions port `7777` as the default

#### Scenario: usage.md lists Web UI capabilities

- **WHEN** a reader reads the `ezida serve` section
- **THEN** the section lists the capabilities the Web UI exposes
  today: read the board, inline create / edit / delete cards,
  drag-and-drop card move/reorder, inline column add / rename /
  delete / reorder, board filter, dark theme, and hot reload of
  `kanban.toml`.

#### Scenario: usage.md points readers to the authoritative spec

- **WHEN** a reader wants to know the exact runtime contract
- **THEN** the `ezida serve` section references
  `openspec/specs/viewer-server/spec.md` and
  `openspec/specs/viewer-ui/spec.md` as the source of truth for
  behaviour.
