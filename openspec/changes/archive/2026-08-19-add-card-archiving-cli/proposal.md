## Why

Finished cards pile up in the terminal column and never leave `kanban.toml`.
The file grows without bound, and the human who has to read it — the primary
audience for a file-based board — loses the ability to scan it. Deleting the
cards is the only exit today, and it destroys the record.

Archiving moves a card out of the active board into a sibling file while
keeping it readable, searchable and restorable. This change delivers the
storage format and the complete CLI; the viewer surface follows in a second
change.

## What Changes

- New sibling file `kanban.archive.toml`, holding archived cards. Each keeps
  every field it had, retains its original `column` name, and gains
  `archived_at`. The file is created by the first archive operation and
  **deleted** when the last card is restored, so a board that never archives
  is byte-identical to one built before this feature.
- New `ezida archive <id>` — moves a card out of the board. When the card is an
  epic, its children are archived with it (cascade).
- New `ezida archive column <name>` — archives every card in a column, leaving
  the column itself in place. This is what unblocks deleting a non-empty
  column.
- New `ezida archive list` / `ezida archive get <id>` — read the archive with
  the same filters and envelopes as `ezida list` / `ezida get`.
- New `ezida unarchive <id>` — restores a card (and its archived children) to
  its original column, or to the board's first column when that column no
  longer exists.
- `ezida list` gains `--include-archived` and `--archived-only`.
- ID generation now considers archived IDs, so a new card can never collide
  with one waiting in the archive.
- `columns rm`'s refusal message now points at `ezida archive column`.
- Not in this change: any viewer or HTTP surface. `GET /api/board` and the
  rendered board are untouched until the follow-up change.

## Capabilities

### New Capabilities

- `card-archiving`: the `kanban.archive.toml` format and its validator, the
  archive/unarchive operations including epic cascade and column archiving,
  the cross-file write ordering, and the `ezida archive` / `ezida unarchive`
  CLI surface.

### Modified Capabilities

- `board-storage`: ID uniqueness is no longer scoped to a single file — a
  generated ID MUST avoid archived IDs as well as live ones.
- `card-reading`: `ezida list` gains `--include-archived` and
  `--archived-only`, a conditional `ARCHIVED` text column, and an
  `archived_at` key in the JSON envelope.
- `board-config`: the `COLUMN_IN_USE` / `COLUMN_HAS_CARDS` refusal message
  names `ezida archive column <name>` as a remedy.
- `documentation`: `docs/usage.md` gains CLI reference sections for the new
  verbs, the new flags, the `archived_at` JSON contract, and a stated
  limitation about two-file atomicity.
- `skill-packaging`: the embedded `SKILL.md` lists the new verbs.

## Impact

- **New code**: `internal/board/archive.go`, `archive_validation.go`,
  `archive_ops.go`; `internal/commands/archive.go`, `unarchive.go`,
  `archive_helpers.go`.
- **Modified code**: `internal/commands/{init.go,add.go,list.go,get.go,errors.go}`,
  `internal/board/columns.go` (message only), `internal/output/json.go`
  (`ListCard`/`GetCard` gain `archived_at`), `cmd/ezida/main.go` (two new
  commands), `internal/skill/SKILL.md` **and** `.claude/skills/ezida-kanban/SKILL.md`
  (byte-identical, enforced by `skill_test.go`), `docs/usage.md`.
- **Existing test touched**: `internal/commands/errors_test.go` asserts the
  `COLUMN_IN_USE` message literal, which this change rewords.
- **Dependencies**: none added. `go-toml` v2 already flattens anonymous
  embedded structs, which is what lets the archived-card type reuse `Card`.
- **Risk**: there is no cross-file transaction. A crash between the two writes
  can duplicate a card; the design makes duplication the only possible failure
  mode and resolves it deterministically on the next read.
