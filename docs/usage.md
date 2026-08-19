# Usage

End-user reference for `ezida`: CLI commands, JSON contract, embedded
skill, manual install, and known limitations. For the project pitch
and quick start, see the [README](../README.md). For contributing and
release procedure, see [development.md](./development.md).

## Manual install

If you would rather not pipe the install script to `sh`, install
manually:

1. Pick the tarball for your platform from the
   [latest release](https://github.com/nicolasvergoz/ezida-kanban/releases/latest):
   - `ezida_<version>_darwin_arm64.tar.gz`
   - `ezida_<version>_darwin_amd64.tar.gz`
   - `ezida_<version>_linux_arm64.tar.gz`
   - `ezida_<version>_linux_amd64.tar.gz`
2. Verify the SHA256 against `checksums.txt` from the same release.
3. Extract: `tar -xzf ezida_<version>_<os>_<arch>.tar.gz`
4. Move the `ezida` binary somewhere on your `PATH`
   (`~/.local/bin/`, `/usr/local/bin/`, …) and make sure it is
   executable (`chmod 0755`).

If you would rather inspect the install script before piping it to
`sh`, download it first:

```sh
curl -fsSL -o install.sh https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh
less install.sh
sh install.sh
```

## CLI reference

Every command supports `--json` for structured output and `--no-color`
to disable ANSI colors in text mode. `NO_COLOR=1` in the environment
has the same effect. JSON output is never colored.

### `ezida init`

Create `kanban.toml` in the current directory plus the embedded skill
at `.claude/skills/ezida-kanban/SKILL.md`.

| Flag             | Description                                           |
|------------------|-------------------------------------------------------|
| `--columns`      | Comma-separated column names (default: `todo,ongoing,done`). |
| `--priorities`   | Comma-separated priority names (default: `low,medium,high`). |
| `--force`        | Overwrite an existing `kanban.toml`.                  |
| `--skill-only`   | Only refresh the skill file (e.g. after a binary upgrade). |

Exactly one column is marked *terminal* — a column whose cards count as
done for epic progress. `init` picks a column named `done` when present,
otherwise the last one. Pass explicit `*` suffixes in `--columns` to
choose yourself; doing so suppresses the automatic choice.

```sh
ezida init --columns="backlog,todo,ongoing,review,done"
ezida init --columns="todo,shipped*,wont-fix*"
```

### `ezida board`

Print the board's schema version, columns, priorities, and per-column
card counts. Terminal columns are flagged: `done_columns` in JSON, a
`✓` in text mode. Column names are always reported bare — the `*`
suffix never leaves the file.

```sh
ezida board
ezida board --json
```

### `ezida list`

Print every card. Filters are AND-combined.

| Flag                  | Description                                              |
|-----------------------|----------------------------------------------------------|
| `--column=<name>`     | Keep only cards in this column.                          |
| `--title-contains=<s>`| Case-insensitive substring match on title.               |
| `--tag=<tag>`         | Keep only cards with this tag.                           |
| `--priority=<p>`      | Keep only cards with this priority.                      |
| `--epic=<id>`         | Keep the named card and every card belonging to it.      |
| `--include-archived`  | Append archived cards after the live results.            |
| `--archived-only`     | Return archived cards instead of live ones.              |

An `--epic` id matching no card is rejected with `INVALID_FILTER`. The
parent is always included, so scoping to an epic never hides the epic.

`--include-archived` and `--archived-only` cannot be combined
(`MUTUALLY_EXCLUSIVE_FLAGS`). Neither flag touches the archive file at
all when omitted, so plain `ezida list` behaves exactly as it did before
archiving existed. With `--include-archived`, live cards are listed
first in board order, then archived cards in archive order — never
interleaved by date. Text mode appends an `ARCHIVED` column (the archive
date, or `-`) only when one of the two flags is given.

```sh
ezida list --column=todo --tag=security
ezida list --epic=rl4m9x
ezida list --include-archived
ezida list --archived-only --column=done
```

### `ezida get`

Print one card with its full description. A card belonging to an epic
also reports its parent; a card that *is* an epic reports its children
and a done/total count instead. A card is never both.

```sh
ezida get a3f2k9
ezida get a3f2k9 --json
```

### `ezida add`

Create a new card, appended to the bottom of its column.

| Flag             | Description                                           |
|------------------|-------------------------------------------------------|
| `--column`       | Required. Destination column.                          |
| `--priority`     | Optional. Must exist in `[board].priorities`.          |
| `--tags`         | Optional. Comma-separated tag list.                    |
| `--description`  | Optional. Card body (may span multiple lines).         |
| `--epic`         | Optional. Id of the card this one belongs to.          |
| `--color`        | Optional. Palette name or hex value for this card.     |

`--epic` must name an existing card that does not itself belong to an
epic; anything else is rejected with `INVALID_EPIC`. When the named
parent has no color yet, it gets one from the palette in the same write.

```sh
ezida add "Refactor auth" --column=todo --priority=high --tags=security,tech-debt
ezida add "Card due dates" --column=backlog --epic=rl4m9x
```

### `ezida edit`

Update one or more fields on a card. Any combination of flags is
allowed; omitted fields are left unchanged.

| Flag             | Description                                           |
|------------------|-------------------------------------------------------|
| `--title`        | New title.                                            |
| `--description`  | New description body.                                  |
| `--priority`     | New priority (must exist in `[board].priorities`).    |
| `--tags`         | New tag list (replaces the previous list).             |
| `--column`       | Move the card to this column.                          |
| `--epic`         | Attach the card to this epic.                          |
| `--no-epic`      | Detach the card from its epic.                         |
| `--color`        | Set the card's color (palette name or hex).            |
| `--no-color`     | Clear the card's color.                                |

`--epic`/`--no-epic` and `--color`/`--no-color` are mutually exclusive.

```sh
ezida edit a3f2k9 --priority=medium --tags=security
ezida edit a3f2k9 --epic=rl4m9x
ezida edit rl4m9x --color=emerald
```

### `ezida move`

Convenience for column-only changes. The card is appended to the
bottom of the new column.

```sh
ezida move a3f2k9 ongoing
```

### `ezida rm`

Delete a card. In a TTY the command prompts for confirmation; pass
`--yes` to skip the prompt (required for non-interactive use).

Deleting a card that other cards name as their epic detaches those
children rather than refusing, and reports the ids it detached — on
stderr in text mode, in the `orphaned` array in JSON mode. Their
`updated_at` is not refreshed: losing a parent is a board-level
consequence, not an edit to the card.

```sh
ezida rm a3f2k9 --yes
```

### `ezida archive`

Move a card out of `kanban.toml` into a sibling `kanban.archive.toml`,
keeping every field it had and adding `archived_at`. Archiving an epic
takes its children with it; archiving a lone child of a live epic is
allowed and leaves the parent untouched.

```sh
ezida archive <id>                     # archive one card (and its children, if it's an epic)
ezida archive column <name>            # archive every card in a column; the column itself stays
ezida archive list                     # list archived cards (same filters as `list`)
ezida archive get <id>                 # show one archived card
```

| Flag (on `archive column`) | Description                                        |
|-----------------------------|-----------------------------------------------------|
| `--yes`                     | Skip the confirmation prompt shown when the cascade would also take cards from other columns. Required with `--json`. |

`kanban.archive.toml` is created by the first archive operation and
**removed** when the archive empties out — a board that never archives
looks exactly like one built before this feature existed. Archiving a
column leaves the column itself in `[board].columns`, which is what
turns `ezida columns rm`'s `COLUMN_IN_USE` refusal into a solvable
problem.

```sh
ezida archive a3f2k9
ezida archive column done --yes
ezida archive list --column=done
```

### `ezida unarchive`

Restore an archived card — and any archived children of it — back onto
the board.

| Flag       | Description                                                    |
|------------|-----------------------------------------------------------------|
| `--column` | Restore into this column instead of the one the card was archived from. |

Each restored card returns to its stored column when that column still
exists; otherwise it lands in the board's first column and the command
reports that it was relocated. A card whose epic parent is neither on
the board nor part of the same restore has its `epic` cleared, mirroring
`rm`'s orphaning behaviour. Restoring an id that already exists on the
board is refused with `ID_COLLISION`.

```sh
ezida unarchive a3f2k9
ezida unarchive a3f2k9 --column=todo
```

### `ezida columns`

Manage the board's columns.

```sh
ezida columns                                # list columns with counts and terminal marks
ezida columns add review --position=3       # 1-indexed; default appends to the end
ezida columns rename ongoing in-progress     # updates [board] AND every referencing card
ezida columns rm review                      # fails if any card still references it
ezida columns done shipped                   # mark a column terminal
ezida columns undone shipped                 # clear the terminal mark
```

`done` and `undone` are idempotent: toggling a column to the state it
already holds exits `0` and leaves `kanban.toml` byte-unchanged. Both
take a bare name — a `*` in the argument is not a column, and is
rejected as `COLUMN_NOT_FOUND`. A rename target containing `*` is
rejected with `INVALID_COLUMN_NAME`; the marker survives the rename
regardless (`done*` renamed to `shipped` yields `shipped*`).

### `ezida priorities`

Manage the board's priorities. Same shape as `columns`.

```sh
ezida priorities add urgent
ezida priorities rename medium normal
ezida priorities rm urgent
```

### `ezida colors`

List the epic color palette, each entry's hex value, and the card
holding it (or `free`). Never mutates the board. A color in use that is
not part of the palette is listed as an extra entry with no name.

```sh
ezida colors
ezida colors --json
```

### `ezida migrate`

Upgrade `kanban.toml` from `schema_version = 1` to `2`. See
[Migration](#migration).

```sh
ezida migrate
```

### `ezida serve`

Launch the Web UI: an HTTP server on `127.0.0.1` that renders the
current `kanban.toml` as an interactive Kanban board in the
browser. The page is read **and** write — every mutation goes
through the same code path as the CLI, and the page hot-reloads
on every change to `kanban.toml` (yours or the CLI's) via Server-
Sent Events.

| Flag        | Description                                                     |
|-------------|-----------------------------------------------------------------|
| `--port`    | Starting HTTP port (default `7777`). If busy, the server tries the next 10 ports in sequence and uses the first free one. Exits with `PORT_UNAVAILABLE` if all 11 are taken. |
| `--no-open` | Do not launch the default browser on startup.                   |

```sh
ezida serve
ezida serve --port=9000 --no-open
```

The server binds loopback-only — it never listens on `0.0.0.0` or
any public interface. It blocks until `SIGINT` or `SIGTERM`, then
drains in-flight requests within 5 seconds.

**What the Web UI lets you do:**
- Read the board as a column grid with card counts.
- Click any card to open a detail modal with click-to-edit fields
  (title, description, priority, tags) — each field commits via a
  single PATCH on blur.
- Add a new card inline at the top of a column without leaving
  the page.
- Delete a card from the modal.
- Drag cards to reorder within a column or move between columns.
- Add a new column, rename a column inline, delete an empty
  column, or drag columns to reorder them.
- Filter visible cards by title, tag, or priority.
- See which epic a card belongs to: a colored chip in the card
  footer, named after the parent card.
- Spot an epic at a glance: a four-square glyph before its title, a
  border tinted with its color, and a `done/total` progress bar.
- See which columns count as done — a check mark in the column
  header — so the progress numbers are readable.
- Read the full relation in the card modal: the parent a card belongs
  to, or the children an epic holds with their columns.
- Attach a card to an epic, reassign it, or detach it — from the
  modal's Epic section, searching the board's cards by title or id.
  The same picker adds a child from the epic's own modal, and each
  child row can be removed from it.
- Recolor an epic from the eight-color palette, or clear its color.
  A hand-edited hex outside the palette shows up as a ninth swatch
  rather than disappearing.
- Focus one epic: click its chip on any card, or pick it in the
  filter popover's Epic section — the viewer's counterpart to
  `ezida list --epic`. The parent stays visible with its progress
  bar. A `No epic` pill shows the work that belongs to nothing.
- Toggle between light and dark themes.
- Archive a card or an entire column's cards, and restore an archived
  card — from a collapsed-by-default Archive section at the end of the
  board, which appears only once something has been archived and
  shows each archived card's original column and archive date.

The authoritative behavioural contract lives in
[`openspec/specs/viewer-server/spec.md`](../openspec/specs/viewer-server/spec.md)
and
[`openspec/specs/viewer-ui/spec.md`](../openspec/specs/viewer-ui/spec.md);
consult those if you need exact scenarios (port-fallback edge
cases, hot-reload semantics, accessibility contract, …).

## Epics

An epic is just a card. A card belongs to an epic by naming it:

```toml
[[cards]]
id = "f20wbo"
epic = "rl4m9x"
```

No separate entity, no second id space, no extra TOML section. The
consequences:

- **One level, always.** A card carrying an `epic` may not itself be
  cited as an epic. Attempting it fails with `INVALID_EPIC` and a
  message saying why. Cycles are therefore unrepresentable rather than
  something to detect.
- **The link is documentary.** Nothing is ever blocked, refused, or
  warned about because of it. A child moves anywhere; a parent moves
  anywhere; either can be edited or deleted independently.
- **Deleting a parent detaches its children** instead of refusing, and
  names the cards it detached.
- **Derived values are never stored.** `children`, `done` and `total`
  are computed at read time, so a hand edit cannot falsify them.

### Colors

A parent carries a hex color, assigned from an ordered palette the
first time it acquires a child:

| Name | Hex | | Name | Hex |
|---------|-----------|---|---------|-----------|
| violet | `#8b5cf6` | | pink | `#ec4899` |
| emerald | `#10b981` | | lime | `#84cc16` |
| orange | `#f97316` | | cyan | `#06b6d4` |
| blue | `#3b82f6` | | fuchsia | `#d946ef` |

Assignment picks the **least-used** color, breaking ties by palette
order — so deleting an epic frees its color for the next one, which a
modulo-of-count strategy would get wrong. The order is by chromatic
distance rather than hue, so two epics created back to back stay
distinguishable at chip size, and no entry collides with a default
priority color.

The file only ever stores hex. The names are a CLI convenience:
`--color=emerald` writes `#10b981`. Run `ezida colors` to see which
card holds which entry.

### Terminal columns

A column whose cards count as *done* for epic progress is marked with a
`*` suffix on its name in `[board].columns`:

```toml
columns = ['backlog', 'todo', 'ongoing', 'done*']
```

The marker exists **only in the file**. In memory, in the CLI, and on
the wire, a column is a name plus a boolean — no command accepts the
suffix as an argument and no output emits it. Use
`ezida columns done|undone <name>` to toggle it.

The obvious alternative, a separate `done_columns = ['done']` array,
was rejected: it is a second reference to a name that already lives in
`columns`, so a hand edit, a git conflict resolution, or a stale file
copy can desync the two and produce a perfectly valid file in which
every epic silently reports `0/N`. Encoding the flag in the name makes
that state impossible to write. `*` is consequently reserved: a column
name containing it is rejected.

A board with no terminal column is legal, and reports `0/N` for every
epic. That is a truthful reading of the configuration, not an error.

## Migration

`schema_version = 2` added `epic`, `color` and the terminal-column
suffix. The binary refuses any other version outright — a lenient read
would let an older `ezida` round-trip a v2 file and silently strip
every new field on write, which for a git-tracked file is worse than a
loud failure.

Upgrading an existing board:

```sh
ezida migrate
```

It backs the file up to `kanban.toml.v1.bak`, sets `schema_version = 2`,
marks one column terminal — a column named `done` if present, otherwise
the last declared one — and reports which it chose. Rolling back is
restoring the backup and reinstalling the previous binary; a v1 board
carries no epic data to lose in either direction.

After migrating, refresh the embedded skill so assistants learn the new
commands:

```sh
ezida init --skill-only
```

## JSON contract

Every command supports `--json`. Keys are `snake_case`; timestamps are
ISO 8601 UTC strings. Errors always go to stderr; the exit code is
`0` on success, `1` on user error, `2` on system error.

### `ezida board --json`

```json
{
  "schema_version": 2,
  "columns": ["todo", "ongoing", "done"],
  "done_columns": ["done"],
  "priorities": ["low", "medium", "high"],
  "cards_per_column": {"todo": 3, "ongoing": 1, "done": 7}
}
```

With an archive present, the payload additionally carries
`"archived_count": 4` — the number of cards in `kanban.archive.toml`.
The key is omitted entirely (not `0`) for a board that has never
archived anything.

### `ezida list --json`

The `description` field is omitted from `list` output (token-efficient
— call `get` for the full body).

```json
{
  "cards": [
    {
      "id": "a3f2k9",
      "title": "Refactor auth",
      "column": "todo",
      "priority": "high",
      "tags": ["security"],
      "epic": "rl4m9x",
      "created_at": "2026-05-20T14:30:00Z",
      "updated_at": "2026-05-20T14:30:00Z"
    }
  ]
}
```

`epic` and `color` are omitted when unset. With `--include-archived` or
`--archived-only`, each archived card additionally carries
`"archived_at": "2026-08-19T10:00:00Z"`. The key is **omitted entirely**
— never `null`, never a zero timestamp — on every live card, which is
what keeps plain `ezida list --json` output unchanged for anyone not
using archiving.

### `ezida archive get --json`

Same envelope as `ezida get --json`, plus `archived_at`. An archived
card is not resolved against the live board's epic graph — restore it
first to see `epic`/`children`/`progress` — so those keys are always
absent here.

```json
{
  "card": {
    "id": "a3f2k9",
    "title": "Refactor auth",
    "column": "todo",
    "tags": ["security"],
    "description": "Move from session-based to JWT.\n",
    "created_at": "2026-05-20T14:30:00Z",
    "updated_at": "2026-05-20T14:30:00Z",
    "archived_at": "2026-08-19T10:00:00Z"
  }
}
```

### `ezida get --json`

```json
{
  "card": {
    "id": "a3f2k9",
    "title": "Refactor auth",
    "column": "todo",
    "priority": "high",
    "tags": ["security"],
    "description": "Move from session-based to JWT.\nCheck token expiry handling.\n",
    "epic": {"id": "rl4m9x", "title": "Card relations"},
    "created_at": "2026-05-20T14:30:00Z",
    "updated_at": "2026-05-20T14:30:00Z"
  }
}
```

A card that *is* an epic carries `color`, `children` and `progress`
instead of `epic`:

```json
{
  "card": {
    "id": "rl4m9x",
    "color": "#8b5cf6",
    "children": [{"id": "f20wbo", "title": "Card dependencies", "column": "backlog"}],
    "progress": {"done": 1, "total": 3}
  }
}
```

### `ezida colors --json`

```json
{
  "colors": [
    {"name": "violet", "hex": "#8b5cf6", "held_by": {"id": "rl4m9x", "title": "Card relations"}},
    {"name": "emerald", "hex": "#10b981", "held_by": null}
  ]
}
```

A color in use that is not part of the palette appears as an extra
entry with a `null` name.

### Error envelope

In `--json` mode, errors are emitted to stderr as:

```json
{"error":{"code":"CARD_NOT_FOUND","message":"no card with id zzzzzz","details":{"id":"zzzzzz"}}}
```

`code` is a stable `UPPER_SNAKE_CASE` identifier — clients should
branch on `code`, never on the English `message`. The full list of
codes lives in [`openspec/specs/`](../openspec/specs/).

## The embedded skill

`ezida init` writes a Markdown skill file to
`.claude/skills/ezida-kanban/SKILL.md` in the target repository. The
file is embedded into the binary via `go:embed`, so the install does
not touch the network and there is no version drift between the skill
and the CLI.

AI assistants that understand the
[Claude Code skill format](https://docs.claude.com/en/docs/claude-code/skills)
discover the file automatically when they enter the project directory.
The skill teaches the assistant:
- The JSON envelopes documented above.
- The exit-code convention (`0` / `1` / `2`).
- The TOML schema (`schema_version`, `[board]`, `[[cards]]`).
- The epic relation, its one-level nesting rule, and the color palette.
- The terminal-column `*` suffix, and that the CLI only ever speaks
  bare names.
- That every mutation must go through the CLI — assistants never
  rewrite `kanban.toml` directly.

To refresh the skill after upgrading the binary, run:

```sh
ezida init --skill-only
```

The skill file is overwritten silently; the `kanban.toml` is left
untouched.

## Known limitations

- **TOML comments are not preserved across writes.** Any comments you
  add manually to `kanban.toml` are stripped the next time `ezida`
  writes the file. The TOML library this project depends on does not
  round-trip comments.
- **No Windows support.** Builds target macOS and Linux on amd64 and
  arm64 only. Windows is not on the v1 roadmap.
- **Single board per repo.** `ezida` always reads and writes the
  `kanban.toml` in the current working directory. Multi-board layouts
  (e.g. one board per workstream) are out of scope for v1.
- **No real-time collaboration.** Concurrent writers race; the last
  writer wins. The atomic `tmp + rename` strategy keeps the file
  consistent on disk, but two simultaneous `ezida add` invocations
  may drop one of the cards.
- **Archiving is not atomic across `kanban.toml` and
  `kanban.archive.toml`.** Each file is written atomically on its own,
  but there is no transaction spanning both. A crash between the two
  writes can leave a card present in **both** files — never in neither
  — and the next read silently heals it: the live board always wins,
  and the duplicate is dropped from the archive results.
- **A card whose id is literally `column` shadows the `archive column`
  subcommand.** Card ids are always exactly six characters of
  `[0-9a-z]`, and `column` is the only one of `archive`'s subcommand
  names (`column`, `list`, `get`) that is also six characters — so it is
  the only one a real id can ever collide with. When it happens, cobra
  resolves `ezida archive column` to the subcommand before
  `ezida archive <id>` ever sees it; the odds are 1 in 36⁶ (≈1 in
  2.2 billion), and the viewer's archive action offers a way through
  regardless.
