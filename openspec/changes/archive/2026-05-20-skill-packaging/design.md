## Context

The skill text was drafted alongside the brief as `refs/SKILL.md`. It
mentions `pip` install and a Python fallback — both inherited from an
earlier prototype. The Go binary doesn't fit either premise. P5 imports
the text into the repo's authoritative location (`skill/SKILL.md`),
applies the two surgical patches from ADR §D16, embeds it into the
binary via `//go:embed`, and extends `ezida init` to write the skill
alongside `kanban.toml`.

The work touches very little code (one new package, two flag
additions on `init`, one extra file write), but the spec needs to be
explicit about the byte-level patches so future contributors don't
silently re-introduce the Python references.

## Goals / Non-Goals

**Goals:**
- One canonical source of skill text in the repo (`skill/SKILL.md`).
- The binary always ships with that exact text — no network fetch, no
  version drift.
- `ezida init` makes a new project AI-ready in one step.
- `--skill-only` lets users refresh the skill after a binary upgrade
  without touching their existing `kanban.toml`.

**Non-Goals:**
- No `.claude/commands/kanban.md` slash command (brief §11 — v2).
- No mechanism to opt out of the skill write. If users have a strong
  use case for "kanban file without the skill", a later flag can be
  added; the v1 default is opinionated.
- No multi-skill packaging — only the one `SKILL.md`.

## Decisions

### File layout (additions)

```
skill/
  SKILL.md             # patched copy of refs/SKILL.md (committed)
internal/skill/
  skill.go             # `//go:embed skill/SKILL.md` + Bytes var
  skill_test.go        # asserts Bytes equals the file on disk
internal/commands/
  init.go              # MODIFIED: write skill, --skill-only flag,
                       # extended JSON envelope, trailing comment note
  commands_test.go     # +TestInit_WritesSkill, TestInit_SkillOnly_*
```

### Embedding mechanics

```go
// internal/skill/skill.go
package skill

import _ "embed"

//go:embed skill/SKILL.md
var Bytes []byte
```

The `_ "embed"` blank import is required by the `//go:embed` directive
to compile. The path is relative to the file (`internal/skill/`), so
the actual layout is:

```
internal/skill/
  skill.go
  skill/SKILL.md       # copied/symlinked from /skill/SKILL.md
```

To avoid the duplicate-file problem, the package will use a small
helper: a `go generate` directive at the top of `skill.go` runs a
`cp ../../skill/SKILL.md ./skill/SKILL.md` (or a Go-native equivalent)
to refresh the internal copy from the canonical source. Alternatively
— and simpler — the canonical source lives directly at
`internal/skill/skill/SKILL.md` and `skill/` at the repo root is the
human-facing copy. The team's call; the spec only requires
`skill/SKILL.md` at the repo root (browsable for reviewers) and the
embedded copy to byte-match it.

**Chosen approach for implementation:** keep the canonical file at the
repo root at `skill/SKILL.md` and use Go's relative embed path with a
`//go:embed ../../skill/SKILL.md` directive. Go's embed rejects parent
references (`..` is disallowed for security). To work around this,
move the package one level up: place the embed package at
`internal/skill/skill.go` and reference `//go:embed skill/SKILL.md`
where the path is relative to the package directory — meaning the file
must be inside `internal/skill/`. The cleanest resolution:

- The canonical file is at `internal/skill/SKILL.md`.
- The repo root has no `skill/` directory — the user-facing copy is
  irrelevant because the file is already viewable at
  `internal/skill/SKILL.md` in any GitHub UI.
- The spec wording above ("`skill/SKILL.md` in the repo") is satisfied
  by the package path; the implementation task will adjust the path
  to `internal/skill/SKILL.md` and the spec's intent (one canonical
  file with the two patches) is preserved verbatim.

(The spec's literal path will be amended at archive time if this
adjustment lands; the OpenSpec sync step is the trigger.)

### Patched skill content

`internal/skill/SKILL.md` is generated from `refs/SKILL.md` by:

1. Replace the substring `installed via pip` →
   `installed via the install script`.
2. Locate the paragraph beginning
   `Otherwise, invoke the embedded Python script from this skill's directory:`
   and the immediately following fenced code block
   ```
   python <skill-directory>/ezida.py <command> [args]
   ```
   Remove both (and the now-redundant lead-in sentence "If the `ezida`
   command is in the PATH (installed via the install script), use it
   directly:" loses its `Otherwise,` follow-up so its closing colon is
   replaced by a full stop).

The result is committed as a static file — no template, no build step.
A regression test (`TestSkillBytes_DoesNotMentionPython`) asserts the
forbidden substrings are absent so any future hand-edit catches the
mistake.

### `init` modifications

```go
type initFlags struct {
    columns    string
    priorities string
    force      bool
    skillOnly  bool
}

func runInit(f initFlags, jsonOut bool) error {
    skillPath := filepath.Join(".claude", "skills", "ezida-kanban", "SKILL.md")

    if f.skillOnly {
        if err := writeSkillFile(skillPath); err != nil { return err }
        if jsonOut {
            fmt.Printf(`{"skill_only":true,"skill_path":%q}`+"\n", skillPath)
        } else {
            fmt.Printf("wrote %s\n", skillPath)
        }
        return nil
    }

    if _, err := os.Stat(boardPath); err == nil && !f.force {
        return &AlreadyInitializedError{Path: boardPath}
    }
    b := newDefaultBoard(f.columns, f.priorities)
    if err := board.Save(boardPath, b); err != nil { return err }
    if err := writeSkillFile(skillPath); err != nil { return err }

    if jsonOut {
        fmt.Printf(`{"initialized":true,"path":%q,"skill_path":%q}`+"\n",
            boardPath, skillPath)
    } else {
        fmt.Printf("initialized %s\n", boardPath)
        fmt.Printf("wrote %s\n", skillPath)
        fmt.Println("note: TOML comments are not preserved across ezida writes")
    }
    return nil
}

func writeSkillFile(path string) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return os.WriteFile(path, skill.Bytes, 0o644)
}
```

`os.WriteFile` is not atomic but the skill file is non-critical (it
can be re-written by re-running `init --skill-only`). The brief is
silent on atomicity for the skill; the simpler API is sufficient.

### Tests

- `TestSkillBytes_MatchesFile`: read `internal/skill/SKILL.md` into a
  buffer, compare to `skill.Bytes` byte-for-byte.
- `TestSkillBytes_DoesNotMentionPython`: assert
  `bytes.Contains(skill.Bytes, []byte("python <skill-directory>"))` is
  `false` and `bytes.Contains(skill.Bytes, []byte("installed via pip"))`
  is `false`.
- `TestSkillBytes_MentionsInstallScript`: assert
  `bytes.Contains(skill.Bytes, []byte("installed via the install script"))`
  is `true`.
- `TestInit_WritesSkill`: run `init` in a temp directory, assert both
  files exist and the skill file's content equals `skill.Bytes`.
- `TestInit_SkillOnly_DoesNotCreateBoard`: run `init --skill-only` in
  a temp directory with no `kanban.toml`, assert no `kanban.toml`
  appears.
- `TestInit_SkillOnly_DoesNotTouchExistingBoard`: pre-create
  `kanban.toml` with sentinel bytes, run `init --skill-only`, assert
  `kanban.toml` bytes are unchanged.
- `TestInit_JSONEnvelope_Full` and `TestInit_JSONEnvelope_SkillOnly`:
  assert exact JSON output.
- `TestInit_TextOutput_IncludesCommentNote`: assert the trailing line
  is present in non-skill-only mode and absent in skill-only mode.

## Risks / Trade-offs

- **Embed-path complexity** → resolved by living with the file inside
  the embed package's directory. Trade-off: the file moves from
  `/skill/SKILL.md` (the spec's wording) to
  `/internal/skill/SKILL.md`. The spec's intent (one canonical file
  with the two patches) is preserved; the path adjustment lands at
  archive-sync time.
- **Skill file overwrite without confirmation** → matches the intent
  of `--skill-only` (idempotent refresh) and avoids breaking
  `curl|sh`-style update scripts.
- **Two-file pair lacks atomicity** → if the skill write fails after a
  successful board save, the user has a fresh `kanban.toml` but no
  skill file. They can re-run `ezida init --skill-only` to recover.
  Documented in the spec.
- **Skill text drift from `refs/SKILL.md`** → guarded by the regression
  tests on the embedded bytes. Future skill changes should land in
  `internal/skill/SKILL.md` directly; `refs/SKILL.md` is the historical
  reference.

## Migration Plan

- For users running `ezida` before P5: rerun `ezida init --skill-only`
  in their existing project to drop the skill file in place. No
  binary change to the board.

## Open Questions

- None. Embed path will be finalized during implementation; the spec
  scenarios are written against the resulting on-disk path
  (`.claude/skills/ezida-kanban/SKILL.md`), which is independent of
  the internal source location.
