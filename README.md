# ezida-kanban

> File-based Kanban for software projects. One binary, one TOML file,
> no server, no database.

`ezida` keeps a project's Kanban board as a single `kanban.toml` at the
repository root. The board is edited by a small Go CLI and read by AI
assistants through an embedded skill, so humans and agents share the
same source of truth without a separate service.

Supported platforms: macOS (arm64, amd64), Linux (arm64, amd64).

## Install

### One-liner

```sh
curl -sSL https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh | sh
```

The script detects your OS and architecture, downloads the matching
tarball plus `checksums.txt` from the latest GitHub Release, verifies
the SHA256, and installs `ezida` to `~/.local/bin/ezida` (mode `0755`).
If `~/.local/bin` is not on your `PATH`, it prints a one-line reminder.

Pin a specific version with `EZIDA_VERSION`:

```sh
curl -sSL https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh | EZIDA_VERSION=v0.1.0 sh
```

If you would rather inspect the script before piping it to `sh`,
download it first:

```sh
curl -fsSL -o install.sh https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh
less install.sh
sh install.sh
```

### Manual download

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

## Quick start

```sh
cd my-project/
ezida init
# or with custom values:
ezida init --columns="backlog,todo,ongoing,review,done" --priorities="low,medium,high,urgent"
```

`ezida init` creates two files:
- `kanban.toml` at the project root.
- `.claude/skills/ezida-kanban/SKILL.md` — the embedded skill for AI
  assistants.

Commit both. From then on:

```sh
ezida list --column=todo
ezida add "Refactor auth" --column=todo --priority=high --tags=security
ezida move a3f2k9 ongoing
ezida edit a3f2k9 --priority=medium
ezida rm a3f2k9 --yes
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

```sh
ezida init --columns="backlog,todo,ongoing,review,done"
```

### `ezida board`

Print the board's schema version, columns, priorities, and per-column
card counts.

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

```sh
ezida list --column=todo --tag=security
```

### `ezida get`

Print one card with its full description.

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

```sh
ezida add "Refactor auth" --column=todo --priority=high --tags=security,tech-debt
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

```sh
ezida edit a3f2k9 --priority=medium --tags=security
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

```sh
ezida rm a3f2k9 --yes
```

### `ezida columns`

Manage the board's columns.

```sh
ezida columns add review --position=3       # 1-indexed; default appends to the end
ezida columns rename ongoing in-progress     # updates [board] AND every referencing card
ezida columns rm review                      # fails if any card still references it
```

### `ezida priorities`

Manage the board's priorities. Same shape as `columns`.

```sh
ezida priorities add urgent
ezida priorities rename medium normal
ezida priorities rm urgent
```

## JSON contract

Every command supports `--json`. Keys are `snake_case`; timestamps are
ISO 8601 UTC strings. Errors always go to stderr; the exit code is
`0` on success, `1` on user error, `2` on system error.

### `ezida board --json`

```json
{
  "schema_version": 1,
  "columns": ["todo", "ongoing", "done"],
  "priorities": ["low", "medium", "high"],
  "cards_per_column": {"todo": 3, "ongoing": 1, "done": 7}
}
```

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
      "created_at": "2026-05-20T14:30:00Z",
      "updated_at": "2026-05-20T14:30:00Z"
    }
  ]
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
    "created_at": "2026-05-20T14:30:00Z",
    "updated_at": "2026-05-20T14:30:00Z"
  }
}
```

### Error envelope

In `--json` mode, errors are emitted to stderr as:

```json
{"error":{"code":"CARD_NOT_FOUND","message":"no card with id zzzzzz","details":{"id":"zzzzzz"}}}
```

`code` is a stable `UPPER_SNAKE_CASE` identifier — clients should
branch on `code`, never on the English `message`. The full list of
codes lives in [`openspec/specs/`](./openspec/specs/).

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

## Contributing

The project's specs and change history live under
[`openspec/`](./openspec/). Each phase of v1 was developed as an
OpenSpec change with proposal, design, and per-capability spec deltas.

To run the test suite locally:

```sh
go test ./...
go vet ./...
shellcheck -s sh scripts/install.sh
```

Development uses the OpenSpec workflow. The relevant slash commands
in Claude Code are:

- `/opsx:new` — start a new change.
- `/opsx:propose` — create the change with all artifacts in one step.
- `/opsx:apply` — implement the change's tasks.

See [`openspec/changes/`](./openspec/changes/) for change templates
and the archived history.

## License

[MIT](./LICENSE).
