# ezida-kanban

> File-based Kanban for software projects. One binary, one TOML file,
> no server, no database.

`ezida` keeps a project's Kanban board as a single `kanban.toml` at the
repository root. The board is edited by a small Go CLI and read by AI
assistants through an embedded skill, so humans and agents share the
same source of truth without a separate service.

Supported platforms: macOS (arm64, amd64), Linux (arm64, amd64).

## Install

```sh
curl -sSL https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh | sh
```

The script detects your OS and architecture, downloads the matching
tarball plus `checksums.txt` from the latest GitHub Release, verifies
the SHA256, and installs `ezida` to `~/.local/bin/ezida` (mode `0755`).

Pin a specific version with `EZIDA_VERSION`:

```sh
curl -sSL https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh | EZIDA_VERSION=v0.1.0 sh
```

Prefer to inspect the script first, or want a tarball-only install?
See [`docs/usage.md`](./docs/usage.md#manual-install).

## Quick start

```sh
cd my-project/
ezida init
```

`ezida init` creates two files:
- `kanban.toml` at the project root.
- `.claude/skills/ezida-kanban/SKILL.md` — the embedded skill for AI
  assistants.

Commit both.
From then on, drive the board from the CLI:

```sh
ezida list --column=todo
ezida add "Refactor auth" --column=todo --priority=high --tags=security
ezida move a3f2k9 ongoing
ezida edit a3f2k9 --priority=medium
ezida rm a3f2k9 --yes
```

…or just ask your AI assistant. With the embedded skill committed,
Claude Code (and any agent that reads
[Claude Code skills](https://docs.claude.com/en/docs/claude-code/skills))
picks up `.claude/skills/ezida-kanban/SKILL.md` automatically and
runs the same commands on your behalf:

> "Add a high-priority card 'Refactor auth' to todo, tagged security."
>
> "Move card a3f2k9 to ongoing."
>
> "What's in the todo column?"

## Documentation

- [`docs/usage.md`](./docs/usage.md) — full CLI reference, JSON
  contract, embedded-skill details, manual install, known limitations.
- [`docs/development.md`](./docs/development.md) — contributing
  (tests, OpenSpec workflow) and the release procedure.

## License

[MIT](./LICENSE).
