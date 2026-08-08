# Add epics: schema, colors, terminal columns, CLI

## Why

A board of 22 cards has no way to say "these six belong to the same chantier". Tags come close but give no rollup, no ordering, and no place to describe the chantier itself. Ezida needs a grouping primitive that survives its own constraints: a hand-editable TOML file shared between a human and Claude, where silent data loss is worse than a loud failure.

This change lays the whole foundation in the CLI and on disk. It touches no pixel — the viewer keeps rendering exactly what it renders today.

## What Changes

**Epic = a card.** A card may point at another card via `epic`. No new entity, no second id space, no extra TOML section. One level only: a card that carries `epic` may not itself be cited as an epic, which makes cycles structurally impossible rather than something to detect.

**Links are documentary.** Nothing is ever blocked, refused, or warned about. A card can move anywhere regardless of what it points at.

**Colors.** A parent card carries `color = '#8b5cf6'`. The file only ever stores a hex; the named set (`violet`, `emerald`, `orange`, `blue`, `pink`, `lime`, `cyan`, `fuchsia`) lives in the binary. A new epic gets the least-used palette color, so deletions never cause a collision the way a modulo counter would. The set is ordered by chromatic distance, not by hue, so two epics created back to back are visually far apart.

**Terminal columns, encoded in the column name.** A `*` suffix in `[board].columns` marks a column whose cards count as done — `columns = ['backlog', 'todo', 'ongoing', 'done*']`. The marker exists **only serialized**: in memory, in the CLI, and on the wire a column is a name plus a boolean. This is what feeds an epic's progress counter.

A separate `done_columns` list was the obvious alternative and was rejected: it can desync from `columns` on any hand edit, git conflict resolution, or stale-file copy, producing a perfectly valid file where every epic silently reads `0/N`. Encoding the flag in the name makes that state impossible to write, and removes any propagation work from `columns rename` and `columns rm`.

**BREAKING — `schema_version` moves to 2.** `board.Load` refuses any other version by design. Staying on v1 would let an older binary rewrite `kanban.toml` and silently strip every `epic` and `color`; failing loudly is the correct trade for a git-tracked file. A new `ezida migrate` command upgrades v1 → v2, writes a `.v1.bak` backup, picks the terminal column (a column named `done` if present, otherwise the last declared one) and reports its choice.

**CLI surface**
- `ezida add --epic <id> --color <name|hex>`
- `ezida edit --epic <id> | --no-epic | --color <name|hex> | --no-color`
- `ezida rm <parent>` orphans its children and names them in the output
- `ezida list --epic <id>`
- `ezida get <id>` reports the epic on a child, the children and progress on a parent
- `ezida columns done <name>` / `ezida columns undone <name>`; `ezida columns` marks terminal columns in its listing
- `ezida colors` lists the palette and which epic holds each color
- `ezida migrate`

**Explicitly out of scope.** No viewer change, no HTTP API change, no `ezida export` change. The chip, the parent card, the color picker and the epic filter scope all land in later changes.

## Capabilities

### New Capabilities
- `card-epics`: the epic relation (`epic`, `color`), the one-level nesting rule, the named color palette and its assignment rule, the derived children/progress values, and `ezida colors`. Per-command flag behavior lives in the command capabilities below, so no requirement is claimed twice.
- `schema-migration`: `ezida migrate`, the v1 → v2 upgrade, the backup file, the terminal-column choice it makes, and the error message an out-of-date binary produces.

### Modified Capabilities
- `board-storage`: `Card` gains `Epic` and `Color`; `[board].columns` gains the `*` suffix codec applied in `Load`/`Save`; validation gains rules 11–17; `SupportedSchemaVersion` becomes 2.
- `board-config`: `ezida edit` accepts `--epic`/`--no-epic`/`--color`/`--no-color`; `ezida columns` gains `done`/`undone`; `ezida columns rename` preserves the terminal marker across a rename.
- `card-writing`: `ezida add` accepts `--epic`/`--color`; `ezida rm` orphans children instead of refusing.
- `card-reading`: `ezida init` writes a terminal marker and its explanatory comment; `ezida board` reports which columns are terminal; `ezida list` gains `--epic`; `ezida get` reports epic, children and progress.

## Impact

**Code**
- `internal/board/`: `board.go` (Card fields, column codec in Load/Save), `validation.go` (rules 11–17), new `colors.go` (palette, assignment), `columns.go` (terminal flag propagation).
- `internal/commands/`: `add.go`, `edit.go`, `rm.go`, `list.go`, `board.go`, `columns.go`, new `migrate.go`, new `colors.go`.
- `internal/output/`: `json.go` (`ListCard`, `GetCard`, `BoardEnvelope` gain fields), `text.go` (rendering).
- `internal/skill/SKILL.md`: the embedded skill must teach Claude the new flags, or Claude will keep writing v1-shaped cards.

**Untouched by design**: `internal/server/` and `internal/server/web/`.

**Known intermediate inconsistency**: after this change, `ezida get --json` exposes `epic` while `ezida export` does not — `output.ExportCard` and `server.cardResponse` are parallel structs kept in shape-sync by convention, and both move together in the wire change. This is accepted as the cost of keeping this change frontend-free.

**Data**: every existing `kanban.toml` in the wild requires `ezida migrate` before the upgraded binary will read it. Older binaries stop reading migrated files, with a message naming the fix.

**Docs**: `docs/usage.md` (CLI reference, new commands, migration section), `README.md` if the pitch mentions the schema.
