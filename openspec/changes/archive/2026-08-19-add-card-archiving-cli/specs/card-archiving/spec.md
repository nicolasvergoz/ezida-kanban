## ADDED Requirements

### Requirement: Archive file schema and on-disk format

The system SHALL persist archived cards in a single UTF-8 TOML file that is a
sibling of the board file: for a board at `<dir>/kanban.toml` the archive is
`<dir>/kanban.archive.toml`. The path MUST be derived, never configured
separately.

The file MUST contain a top-level `schema_version` integer equal to the board's
`SupportedSchemaVersion`, and an array of `[[cards]]` tables. Each archived card
MUST carry every field a live card carries, with identical TOML key names and
identical omission rules, plus one added key `archived_at` (RFC 3339, UTC,
second precision).

An archived card's `column` MUST record the column the card occupied at the
moment it was archived, whether or not that column still exists on the board.

Writes MUST be atomic using the same temporary-file-plus-rename technique as
the board file, with the temporary file created in the destination directory.

#### Scenario: Archive path is derived from the board path

- **WHEN** the board path is `kanban.toml`
- **THEN** the archive path MUST be `kanban.archive.toml`
- **AND** for a board path `/a/b/kanban.toml` the archive path MUST be
  `/a/b/kanban.archive.toml`

#### Scenario: An archived card round-trips every field

- **WHEN** a card carrying `id`, `title`, `column`, `description`,
  `created_at`, `updated_at`, `tags`, `priority`, `epic` and `color` is
  archived and the archive file is read back
- **THEN** every one of those fields MUST equal its pre-archive value
- **AND** the card block MUST additionally contain an `archived_at` key
- **AND** `archived_at` MUST be a flat key inside the `[[cards]]` table, not a
  nested table

#### Scenario: Unset optional fields stay absent

- **WHEN** a card with no `epic`, `color` or `priority` is archived
- **THEN** the written `[[cards]]` block MUST contain none of those keys

### Requirement: An empty archive leaves no file behind

The archive file SHALL be created only by an operation that archives at least
one card, and MUST be removed from disk when the last archived card is
restored. A board that has never archived anything and a board whose archive
has been fully restored MUST be indistinguishable on disk.

Every read path MUST treat a missing archive file as an empty archive rather
than an error.

#### Scenario: Reading a board with no archive file

- **WHEN** the archive is loaded for a board with no `kanban.archive.toml`
- **THEN** the operation MUST succeed
- **AND** it MUST yield an archive with zero cards at the supported schema
  version

#### Scenario: Restoring the last card deletes the file

- **WHEN** `ezida unarchive` restores the only card in the archive
- **THEN** `kanban.archive.toml` MUST NOT exist on disk afterwards

#### Scenario: `ezida init` does not create an archive

- **WHEN** `ezida init` is invoked in an empty directory
- **THEN** `kanban.archive.toml` MUST NOT be created

### Requirement: Archive validation is a relaxed subset of board validation

The system SHALL validate the archive with its own rule set before every write
and after every read, collecting all violations in one pass and reporting them
with the same `Violation{Rule, Message, CardID}` shape the board validator
uses. Rule numbers SHALL keep their board meanings.

Rules kept from board validation: **1** (schema version), **4** (id matches
`^[0-9a-z]{6}$`), **5** (ids unique within the archive file), **6** (title
non-empty after trim), **9** (timestamps non-zero and `updated_at >=
created_at`), **12** (`epic` differs from the card's own id), **14** (`color`,
when set, is valid hex).

Rules deliberately NOT applied: **7** (column membership), **8** (priority
membership), **11** (epic names an existing card), **13** (one-level nesting),
**15** (schema gate on `epic`/`color`), and every `[board]`-table rule (**2**,
**3**, **10**, **16**, **17**), because the archive has no `[board]` table.

Rules added: **18** — `archived_at` MUST be non-zero and MUST NOT precede
`created_at`; **19** — `column` MUST be non-empty.

#### Scenario: A column that no longer exists is valid in the archive

- **WHEN** the archive holds a card whose `column` names a column absent from
  the board's `[board].columns`
- **THEN** archive validation MUST pass

#### Scenario: A dangling epic reference is valid in the archive

- **WHEN** the archive holds a card whose `epic` names an id present neither in
  the archive nor on the board
- **THEN** archive validation MUST pass

#### Scenario: Duplicate ids within the archive are rejected

- **WHEN** the archive holds two cards with the same `id`
- **THEN** validation MUST fail with a rule 5 violation naming that id

#### Scenario: A missing `archived_at` is rejected

- **WHEN** an archive card has a zero `archived_at`
- **THEN** validation MUST fail with a rule 18 violation

### Requirement: Archiving a card cascades to its epic children

Archiving a card SHALL move that card and every card that names it as its
`epic` out of the board and into the archive in one operation. Because nesting
is one level deep by construction, the cascade is exactly one level.

The archived cards MUST be inserted at the head of the archive, parent first
and children in board file order. Every card in one operation MUST share a
single `archived_at` value. The operation MUST NOT modify any card's
`updated_at`, because archiving is not a content edit. The children's `epic`
field MUST be preserved, so that the group restores intact.

Archiving a card that is a child of a live epic SHALL be permitted; the
archived child keeps its `epic` value even though it no longer resolves.

#### Scenario: Archiving an epic takes its children

- **WHEN** a card that three other cards name as their `epic` is archived
- **THEN** all four cards MUST leave the board
- **AND** all four MUST appear in the archive, the parent before the children
- **AND** the three children MUST retain their `epic` value

#### Scenario: One timestamp per operation

- **WHEN** an epic and its two children are archived in one operation
- **THEN** all three cards MUST carry the same `archived_at`

#### Scenario: Archiving does not touch `updated_at`

- **WHEN** a card is archived
- **THEN** its `updated_at` MUST equal its pre-archive value

#### Scenario: Archiving a lone child is allowed

- **WHEN** a card carrying `epic = "rl4m9x"` is archived while `rl4m9x` stays
  on the board
- **THEN** the operation MUST succeed
- **AND** the archived card MUST still carry `epic = "rl4m9x"`
- **AND** card `rl4m9x` MUST remain on the board unchanged

### Requirement: Unarchiving restores a card and its archived children

Unarchiving a card SHALL move that card and every archived card naming it as
`epic` back onto the board in one all-or-nothing operation.

Each restored card MUST return to the column named by its stored `column` when
that column still exists. When it does not, the card MUST be restored into the
board's first column and the operation MUST report that it was relocated.
An explicit target column MAY be supplied and MUST take precedence; an unknown
explicit column MUST be rejected with `COLUMN_NOT_FOUND`.

A restored card whose `epic` names a card that is neither on the board nor part
of the same restore MUST have its `epic` cleared, and the operation MUST report
those cards as orphaned. This mirrors the orphaning policy of card deletion.

Restoring an id that already exists on the board MUST be refused with
`ID_COLLISION` and MUST leave both files unchanged.

Restored cards MUST be placed so that their relative order matches the order
they held when archived.

#### Scenario: Restoring an epic brings its children back

- **WHEN** an archived epic with two archived children is unarchived
- **THEN** all three cards MUST be on the board
- **AND** all three MUST be absent from the archive
- **AND** the children MUST still name the epic

#### Scenario: Archive then unarchive is an identity

- **WHEN** a card is archived and then immediately unarchived
- **THEN** the board's card list MUST equal its pre-archive state, field for
  field and in the same order
- **AND** no archive file MUST remain on disk

#### Scenario: The original column is gone

- **WHEN** an archived card whose stored `column` is `review` is unarchived
  against a board whose `[board].columns` no longer contains `review`
- **THEN** the card MUST be restored into the board's first column
- **AND** the operation MUST report that it was relocated

#### Scenario: The parent is gone

- **WHEN** an archived card carrying `epic = "rl4m9x"` is unarchived while
  `rl4m9x` is neither on the board nor being restored
- **THEN** the restored card's `epic` MUST be empty
- **AND** the operation MUST report that card as orphaned

#### Scenario: The id is already taken

- **WHEN** unarchiving a card whose id matches a card already on the board
- **THEN** the operation MUST fail with `ID_COLLISION`
- **AND** both `kanban.toml` and `kanban.archive.toml` MUST be byte-unchanged

### Requirement: Cross-file writes favour duplication over loss

An archive operation touches two files and there is no cross-file transaction.
The system SHALL therefore order the two writes so that the destination file
gains the cards before the source file loses them: archiving MUST write the
archive file first and the board file second; unarchiving MUST write the board
file first and the archive file second.

A crash between the two writes therefore leaves a card present in both files
and never absent from both. Every archive read path MUST reconcile this in
memory by dropping from the archive any card whose id also appears on the live
board — **the live board wins**. Reads MUST NOT rewrite either file; the next
write persists the reconciled state.

#### Scenario: A failed board write leaves a duplicate, not a loss

- **WHEN** the archive file is written successfully but the subsequent board
  write fails
- **THEN** the card MUST be present in the archive file
- **AND** the card MUST still be present in `kanban.toml`

#### Scenario: A duplicate is hidden on read

- **WHEN** an archive is read while it holds a card whose id is also on the
  live board
- **THEN** that card MUST NOT appear in the archive results
- **AND** neither file MUST be modified by the read

### Requirement: `ezida archive <id>` archives a single card

`ezida archive <id>` SHALL archive the named card, cascading to its epic
children, and write both files in the order defined above. An unknown id MUST
be rejected with `CARD_NOT_FOUND` and leave both files unchanged.

Text mode MUST print the archived card's id and nothing else on stdout. When
the operation cascaded, it MUST additionally report on stderr how many other
cards it took and name them. When the archived card was a child of a card that
stays on the board, it MUST note that on stderr.

JSON mode MUST emit a single-line object with the key order
`id`, `archived`, `cascaded`, where `cascaded` is an array that MUST be `[]`
rather than `null` when empty.

#### Scenario: Archiving a standalone card

- **WHEN** `ezida archive a3f2k9` is invoked
- **THEN** the process exits with code `0`
- **AND** `kanban.toml` no longer contains card `a3f2k9`
- **AND** `kanban.archive.toml` contains card `a3f2k9`
- **AND** stdout contains only `a3f2k9` followed by a newline

#### Scenario: Cascade is reported on stderr

- **WHEN** `ezida archive rl4m9x` is invoked and `rl4m9x` has three children
- **THEN** stderr names the three additional cards
- **AND** stdout still contains only `rl4m9x`

#### Scenario: JSON envelope shape

- **WHEN** `ezida archive rl4m9x --json` is invoked for an epic with three
  children
- **THEN** stdout is a single JSON line whose keys are `id`, `archived`,
  `cascaded` in that order
- **AND** `cascaded` has length 3

#### Scenario: Unknown id

- **WHEN** `ezida archive zzzzzz` is invoked and no such card exists
- **THEN** the process exits with code `1`
- **AND** the error code is `CARD_NOT_FOUND`
- **AND** both files are byte-unchanged

### Requirement: `ezida archive column <name>` empties a column

`ezida archive column <name>` SHALL archive every card whose `column` matches
`<name>`, together with the epic children of those cards wherever they live.
The column itself MUST remain in `[board].columns`, so that `ezida columns rm`
becomes possible afterwards. An unknown column MUST be rejected with
`COLUMN_NOT_FOUND`.

Because the cascade can remove cards from other columns, the command MUST
confirm before writing whenever it would do so:

- On a TTY without `--yes`, it MUST prompt, naming how many cards outside the
  column it would take, and MUST abort without writing on any answer other
  than acceptance.
- With `--json` and without `--yes`, it MUST refuse with
  `INTERACTIVE_REQUIRED`.
- When no card outside the column is affected, it MUST NOT prompt.

An empty column MUST be a successful no-op that writes nothing and creates no
archive file.

#### Scenario: Archiving a column leaves the column in place

- **WHEN** `ezida archive column done` is invoked against a column holding
  seven cards
- **THEN** all seven cards MUST be in the archive
- **AND** `[board].columns` MUST still contain `done`
- **AND** a subsequent `ezida columns rm done` MUST succeed

#### Scenario: Cascade outside the column requires confirmation

- **WHEN** `ezida archive column done` is invoked on a TTY without `--yes`, and
  an epic in `done` has a child in `todo`
- **THEN** the command MUST prompt before writing
- **AND** declining MUST leave both files byte-unchanged

#### Scenario: JSON without `--yes` refuses

- **WHEN** `ezida archive column done --json` is invoked without `--yes` and a
  cascade would leave the column
- **THEN** the process exits with code `1`
- **AND** the error code is `INTERACTIVE_REQUIRED`

#### Scenario: Empty column is a no-op

- **WHEN** `ezida archive column review` is invoked and no card is in `review`
- **THEN** the process exits with code `0`
- **AND** `kanban.toml` is byte-unchanged
- **AND** no `kanban.archive.toml` is created

### Requirement: `ezida archive list` and `ezida archive get` read the archive

`ezida archive list` SHALL list archived cards using the same filters, the same
text table and the same JSON envelope as `ezida list`, restricted to the
archive. `ezida archive get <id>` SHALL report a single archived card using the
same envelope as `ezida get`. Both MUST include the card's `archived_at`.

Because an archived card may reference a column, priority or epic that no
longer exists on the board, filter validation for these commands MUST accept
any value present in the archive in addition to the board's own values.

An id that is not in the archive MUST be rejected with `CARD_NOT_ARCHIVED`.

#### Scenario: Listing the archive

- **WHEN** `ezida archive list --json` is invoked against an archive of four
  cards
- **THEN** the `cards` array length equals 4
- **AND** every entry carries an `archived_at` key
- **AND** the entries appear in archive file order

#### Scenario: Filtering by a column the board no longer has

- **WHEN** `ezida archive list --column=review` is invoked and `review` exists
  only among archived cards
- **THEN** the process exits with code `0`
- **AND** the matching archived cards are returned

#### Scenario: Getting a card that is not archived

- **WHEN** `ezida archive get a3f2k9` is invoked while `a3f2k9` is on the live
  board
- **THEN** the process exits with code `1`
- **AND** the error code is `CARD_NOT_ARCHIVED`

### Requirement: `ezida unarchive <id>` restores a card

`ezida unarchive <id>` SHALL apply the restore behaviour defined above and
write both files in the order defined above. `--column=<name>` MUST override
the stored column.

Text mode MUST print the restored card's id on stdout, and MUST report
relocation, cascade and orphaning on stderr. JSON mode MUST emit a single-line
object with the key order `id`, `unarchived`, `cascaded`, `orphaned`, `column`,
`relocated`, where `cascaded` and `orphaned` MUST be `[]` rather than `null`
when empty.

#### Scenario: Restoring reports the destination column

- **WHEN** `ezida unarchive a3f2k9 --json` is invoked for a card whose stored
  column still exists
- **THEN** `column` equals the stored column name
- **AND** `relocated` is `false`

#### Scenario: Explicit column override

- **WHEN** `ezida unarchive a3f2k9 --column=todo` is invoked
- **THEN** the restored card's `column` equals `todo`

#### Scenario: Unknown explicit column

- **WHEN** `ezida unarchive a3f2k9 --column=ghost` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `COLUMN_NOT_FOUND`
- **AND** both files are byte-unchanged

### Requirement: Structured error types for archiving

The system SHALL expose typed errors carrying a stable code, an exit code and
structured details, following the existing CLI error convention:

- `CARD_NOT_ARCHIVED` — the named id is not in the archive; details carry `id`.
- `ID_COLLISION` — a restore would duplicate an id already on the board;
  details carry `id`.
- `MUTUALLY_EXCLUSIVE_FLAGS` — two flags that cannot be combined were both
  supplied; details carry the flag names.

All three MUST exit with code `1`, because each is a user error rather than a
system failure.

#### Scenario: Codes surface through the error envelope

- **WHEN** any of these errors reaches the top-level handler in JSON mode
- **THEN** stderr MUST be a single-line `{"error":{"code":…,"message":…,"details":…}}`
  object carrying the code named above
- **AND** the process MUST exit with code `1`
