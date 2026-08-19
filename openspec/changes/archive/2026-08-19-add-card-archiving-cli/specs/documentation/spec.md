## ADDED Requirements

### Requirement: `docs/usage.md` documents the archive verbs

`docs/usage.md` SHALL document `ezida archive` and `ezida unarchive` in the
CLI reference block with the same shape as other subcommands, and SHALL
document the two archive flags on `ezida list`.

#### Scenario: usage.md has an `ezida archive` section

- **WHEN** a reader opens `docs/usage.md`
- **THEN** the file contains a heading whose text is `ezida archive`
  (e.g. `### ezida archive`)
- **AND** the section documents the `<id>`, `column <name>`, `list` and
  `get <id>` forms
- **AND** the section names the `--yes` flag on the `column` form

#### Scenario: usage.md has an `ezida unarchive` section

- **WHEN** a reader opens `docs/usage.md`
- **THEN** the file contains a heading whose text is `ezida unarchive`
- **AND** the section names the `--column` flag

#### Scenario: usage.md documents the list flags

- **WHEN** a reader reads the `ezida list` section
- **THEN** the section names both `--include-archived` and `--archived-only`
- **AND** it states that the two cannot be combined

### Requirement: `docs/usage.md` states the archiving limitations

`docs/usage.md` SHALL record, under its known-limitations section, the two
behaviours a user can otherwise only discover by surprise: that an archive
operation writes two files without a cross-file transaction, and what a crash
between those writes leaves behind.

#### Scenario: usage.md names the two-file caveat

- **WHEN** a reader reads the known-limitations section
- **THEN** it states that archiving writes `kanban.toml` and
  `kanban.archive.toml` separately
- **AND** it states that an interrupted operation can leave a card in both
  files, never in neither
- **AND** it states that the live board wins when the two disagree

### Requirement: `docs/usage.md` documents the `archived_at` contract

The JSON contract section of `docs/usage.md` SHALL document the `archived_at`
key, including that it is absent rather than null on live cards.

#### Scenario: usage.md documents `archived_at`

- **WHEN** a reader reads the JSON contract section
- **THEN** it documents `archived_at` for `ezida list` and `ezida archive get`
- **AND** it states that the key is omitted entirely for live cards
