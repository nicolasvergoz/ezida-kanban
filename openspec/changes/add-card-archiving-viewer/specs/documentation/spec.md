## MODIFIED Requirements

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
  delete / reorder, board filter, dark theme, hot reload of
  `kanban.toml`, and archiving/restoring a card or a column's cards
  from the collapsed Archive section.

#### Scenario: usage.md points readers to the authoritative spec

- **WHEN** a reader wants to know the exact runtime contract
- **THEN** the `ezida serve` section references
  `openspec/specs/viewer-server/spec.md` and
  `openspec/specs/viewer-ui/spec.md` as the source of truth for
  behaviour.
