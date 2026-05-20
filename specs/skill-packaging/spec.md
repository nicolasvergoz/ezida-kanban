# Skill Packaging Specification

## Purpose

Embed the canonical SKILL.md into the binary via `go:embed` and have `ezida init` write it to `.claude/skills/ezida-kanban/SKILL.md` so Claude Code picks it up automatically.

## Requirements

### Requirement: Canonical embedded skill file

The repo SHALL contain `skill/SKILL.md` derived from `refs/SKILL.md`
with exactly these two patches:

1. The substring `installed via pip` is replaced with
   `installed via the install script`.
2. The fallback block that begins with
   `Otherwise, invoke the embedded Python script from this skill's directory:`
   and includes the `python <skill-directory>/ezida.py <command> [args]`
   code block MUST be removed entirely (the surrounding paragraphs are
   adjusted so the section flows without it).

No other byte-level changes are allowed without an explicit ADR
amendment. `refs/SKILL.md` remains the human reference; `skill/SKILL.md`
is the binary's source of truth.

#### Scenario: Patched file omits the Python fallback

- **WHEN** the file `skill/SKILL.md` is read
- **THEN** it MUST NOT contain the substring `python <skill-directory>/ezida.py`
- **AND** it MUST NOT contain the substring `installed via pip`

#### Scenario: Patched file documents the install script

- **WHEN** the file `skill/SKILL.md` is read
- **THEN** it MUST contain the substring `installed via the install script`

### Requirement: Skill is embedded into the binary

A package `internal/skill` SHALL expose `var Bytes []byte` declared via
`//go:embed skill/SKILL.md`. The build MUST fail if `skill/SKILL.md` is
missing or unreadable.

#### Scenario: Embedded bytes match the source file

- **WHEN** `len(skill.Bytes)` is compared to
  `len(<contents of skill/SKILL.md>)`
- **THEN** the two values are equal
- **AND** `bytes.Equal(skill.Bytes, <contents of skill/SKILL.md>)` is
  `true`

#### Scenario: Missing source fails the build

- **WHEN** `skill/SKILL.md` is renamed away and `go build ./...` is run
- **THEN** the build exits non-zero with a `//go:embed` error message

### Requirement: `ezida init` writes the embedded skill

The `ezida init` command (introduced in `card-reading`) SHALL, on a
successful run, write the embedded skill bytes to
`.claude/skills/ezida-kanban/SKILL.md` under the current working
directory. Missing parent directories MUST be created with mode `0755`.
The skill file MUST be overwritten silently if it already exists.

When `init` succeeds, both `kanban.toml` AND the skill file MUST exist.
If the skill write fails (filesystem error), the command MUST exit `2`
with code `IO_ERROR`, but `kanban.toml` MAY have been written already —
the temp-file pattern protects only the board file's atomicity, not the
two-file pair.

#### Scenario: Fresh init writes both files

- **WHEN** `ezida init` is run in an empty directory
- **THEN** `kanban.toml` exists at the working directory
- **AND** `.claude/skills/ezida-kanban/SKILL.md` exists
- **AND** the skill file's bytes equal `internal/skill.Bytes`

#### Scenario: Skill overwrite is silent

- **WHEN** `ezida init --force` is run in a directory where
  `.claude/skills/ezida-kanban/SKILL.md` already exists with stale
  content
- **THEN** the file's new content equals the embedded bytes
- **AND** the previous content is discarded
- **AND** no backup file is created

#### Scenario: Init JSON envelope mentions both paths

- **WHEN** `ezida init --json` is run successfully
- **THEN** stdout is the JSON object
  `{"initialized":true,"path":"kanban.toml","skill_path":".claude/skills/ezida-kanban/SKILL.md"}`

### Requirement: `ezida init --skill-only` refreshes the skill alone

`ezida init` SHALL accept a `--skill-only` flag. When set:

- The command MUST NOT create, overwrite, or touch `kanban.toml` in any
  way (the file's presence or absence is irrelevant to the operation).
- The command MUST write `.claude/skills/ezida-kanban/SKILL.md` with
  the embedded bytes, creating parent directories with mode `0755` and
  overwriting any existing file silently.
- The `--force` flag has no effect on the skill-write path (it is
  already silent) and MUST NOT be required.
- Text mode prints `wrote .claude/skills/ezida-kanban/SKILL.md`.
- JSON mode prints
  `{"skill_only":true,"skill_path":".claude/skills/ezida-kanban/SKILL.md"}`.

#### Scenario: Skill-only does not touch kanban.toml

- **WHEN** `ezida init --skill-only` is run in a directory containing
  a pre-existing `kanban.toml` with custom content
- **THEN** `kanban.toml` is byte-unchanged after the command
- **AND** `.claude/skills/ezida-kanban/SKILL.md` reflects the embedded
  bytes

#### Scenario: Skill-only without an existing kanban.toml succeeds

- **WHEN** `ezida init --skill-only` is run in a directory with no
  `kanban.toml`
- **THEN** the process exits with code `0`
- **AND** `.claude/skills/ezida-kanban/SKILL.md` exists
- **AND** `kanban.toml` does NOT exist after the command

#### Scenario: Skill-only JSON envelope

- **WHEN** `ezida init --skill-only --json` is run successfully
- **THEN** stdout equals
  `{"skill_only":true,"skill_path":".claude/skills/ezida-kanban/SKILL.md"}\n`

#### Scenario: Skill-only is silent about kanban.toml

- **WHEN** `ezida init --skill-only` is run (with or without
  pre-existing `kanban.toml`)
- **THEN** text mode stdout MUST NOT mention `kanban.toml`
- **AND** the process MUST NOT exit with `ALREADY_INITIALIZED`

## MODIFIED Requirements

### Requirement: `ezida init` creates a new board

`ezida init` SHALL write a fresh `kanban.toml` at the working directory
with `schema_version = 1`, the columns from `--columns` (or the defaults
`["todo", "ongoing", "done"]`), the priorities from `--priorities` (or
the defaults `["low", "medium", "high"]`), and an empty `[[cards]]`
section.

Unless `--skill-only` is passed, the command SHALL ALSO write
`.claude/skills/ezida-kanban/SKILL.md` (overwriting silently) per the
`skill-packaging` capability. When `--skill-only` is passed, the
`kanban.toml` step is skipped entirely (see `skill-packaging` spec).

The text-mode success output now reads:
```
initialized kanban.toml
wrote .claude/skills/ezida-kanban/SKILL.md
note: TOML comments are not preserved across ezida writes
```
The "note" line surfaces brief §7.8 once at init time so the user knows
the limitation up front. JSON mode does NOT include the note (consumers
that need it read README or the spec).

#### Scenario: Fresh init with defaults

- **WHEN** `ezida init` is run in an empty directory
- **THEN** `kanban.toml` exists
- **AND** `.claude/skills/ezida-kanban/SKILL.md` exists
- **AND** the file parses through `board.Load` without error
- **AND** `[board].columns` equals `["todo", "ongoing", "done"]`
- **AND** `[board].priorities` equals `["low", "medium", "high"]`

#### Scenario: Init with custom columns and priorities

- **WHEN** `ezida init --columns="backlog,wip,done" --priorities="low,high"` is run
- **THEN** the resulting `[board].columns` equals
  `["backlog", "wip", "done"]`
- **AND** `[board].priorities` equals `["low", "high"]`
- **AND** the skill file is also written

#### Scenario: Init refuses to overwrite

- **WHEN** `ezida init` is run in a directory where `kanban.toml`
  already exists
- **THEN** the process exits with code `1`
- **AND** stderr's error code (in JSON mode) is `ALREADY_INITIALIZED`
- **AND** the existing `kanban.toml` is byte-unchanged
- **AND** the skill file is NOT written (because the run aborted)

#### Scenario: Init with `--force` overwrites

- **WHEN** `ezida init --force` is run in a directory where
  `kanban.toml` already exists
- **THEN** the process exits with code `0`
- **AND** `kanban.toml` reflects the new defaults (or flag values)
- **AND** the skill file is written (or overwritten silently)

#### Scenario: Init text output mentions the TOML-comment note

- **WHEN** `ezida init` is run successfully (without `--skill-only`)
- **THEN** stdout's last line equals
  `note: TOML comments are not preserved across ezida writes`
