## Why

P4 closes the CLI surface. P5 turns `ezida` into a tool that AI
assistants can pick up automatically: the binary now embeds the
canonical `SKILL.md`, `ezida init` writes both `kanban.toml` AND
`.claude/skills/ezida-kanban/SKILL.md`, and a new `--skill-only` flag
refreshes the skill alone after a binary upgrade. The skill text in
`refs/SKILL.md` is the source — it gets two surgical patches at
import time to align with the Go binary instead of the original Python
prototype it was drafted against.

## What Changes

- Add `skill/SKILL.md` to the repo, derived from `refs/SKILL.md` with
  the two patches required by ADR §D16:
  - Replace `"installed via pip"` with the install-script wording.
  - Remove the `python <skill-directory>/ezida.py` fallback block (the
    Go binary is self-contained, no Python fallback exists).
  This file is the single source of truth for the embedded skill.
- Embed `skill/SKILL.md` into the binary at build time via
  `//go:embed`, exposed as a `[]byte` in a small new package
  `internal/skill`.
- Modify `ezida init` (defined in P2) so it ALSO writes the embedded
  skill bytes to `.claude/skills/ezida-kanban/SKILL.md` under the
  current working directory, creating intermediate directories as
  needed. Skill write overwrites silently per ADR §D15.
- Add a new flag `--skill-only` to `ezida init`:
  - When set, the command SKIPS the `kanban.toml` step entirely
    (neither creates nor reports on it) and writes only the skill
    file.
  - When set, the `--force` flag is ignored for `kanban.toml`
    purposes; the skill always overwrites silently.
- Update the `init` JSON envelope so callers can distinguish the two
  outcomes:
  - Full init: `{"initialized":true,"path":"kanban.toml","skill_path":".claude/skills/ezida-kanban/SKILL.md"}`.
  - Skill-only: `{"skill_only":true,"skill_path":".claude/skills/ezida-kanban/SKILL.md"}`.
- Add tests that confirm the embedded bytes match `skill/SKILL.md` on
  disk and that the init output matches the documented JSON shapes.
- Note the TOML-comment limitation in the `init` text output's
  trailing message (brief §7.8, §10-P2 acceptance criterion).

## Capabilities

### New Capabilities
- `skill-packaging`: the `skill/SKILL.md` source-of-truth file, the
  `//go:embed` mechanism, the skill-write behavior of `ezida init`,
  the `--skill-only` flag, and the extended init JSON envelope.

### Modified Capabilities
- `card-reading`: the `init` command now writes the skill file in
  addition to `kanban.toml`, and exposes a new flag (`--skill-only`)
  plus an extended JSON envelope. The "refuses to overwrite without
  `--force`" rule for `kanban.toml` is unchanged.

## Impact

- New code: `internal/skill/skill.go` (the `go:embed` package),
  `skill/SKILL.md` (the patched canonical skill), additions to
  `internal/commands/init.go`.
- No new external dependency — `embed` is stdlib.
- Existing P2 spec scenarios for `init` continue to hold; new
  scenarios cover the skill-write side and the `--skill-only` flag.
- The skill file is committed to the repo so reviewers can read it
  in PRs; users never need to download it separately because it is
  baked into the binary.
- After this change, a `git clone` + `ezida init` + opening Claude
  Code in the project is enough for the AI to discover the skill on
  the next session.
