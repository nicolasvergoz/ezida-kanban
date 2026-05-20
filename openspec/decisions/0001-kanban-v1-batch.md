# 0001. Kanban v1 batch — cross-phase decisions

Date: 2026-05-20
Status: Accepted

## Context

This ADR records the cross-cutting decisions for the `kanban-v1` batch, which
builds `ezida` — a file-based Kanban CLI for software projects. The board lives
as a single `kanban.toml` at the project root, manipulated by a Go binary and
read by AI assistants through an embedded skill.

The work is split into six phases that each produce a working, testable
increment (see `refs/PROJECT_BRIEF.md` §10). Phases are strictly ordered: each
relies on the contracts of the previous one. To keep per-phase artifacts
focused, every transverse choice — stack, file format, CLI output shape,
release strategy — is decided here once and referenced from the per-phase
proposals.

Constraints from the brief drove these decisions: single static binary, no
network at runtime, no server, no database, diff-friendly file format, and
token-efficient output for AI consumers.

## Decisions

### D1. Stack — Go binary, cobra, pelletier/go-toml/v2

`ezida` is a Go 1.22+ program built around `spf13/cobra` for sub-commands and
`pelletier/go-toml/v2` for marshal/unmarshal. The rest is stdlib only
(`crypto/rand`, `time`, `os`, `io/fs`, `encoding/json`).

**Alternatives considered:**
- Rust + clap: heavier toolchain, slower install for v1's scope.
- Python: requires runtime on user machine, violates "single binary" goal.
- `BurntSushi/toml`: less actively maintained than `pelletier/go-toml/v2`.

**Affects:** all phases.

### D2. Target platforms

Release binaries: `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`.
No Windows for v1. No 32-bit.

**Alternatives considered:**
- Windows support: deferred until user demand is real; would require additional
  CI matrix entries and install path changes.

**Affects:** P1 (cross-compile sanity), P6 (release workflow + install.sh).

### D3. TOML schema v1

The on-disk format follows `refs/PROJECT_BRIEF.md` §5 exactly:
`schema_version = 1`, `[board]` table with `columns` + `priorities`,
`[[cards]]` array with the fields listed in §5. Field rules in brief §5 are
normative. Card order in the file equals card order within its column.

**Alternatives considered:**
- JSON storage: noisier diffs, no comments, no multiline strings.
- YAML: indentation sensitivity, fragile to manual edits.

**Affects:** P1 (Load/Save), P2 (board output reflects schema), P3+P4 (writes
preserve schema), P5 (SKILL.md documents schema).

### D4. Atomic writes via tmp + rename

All writes to `kanban.toml` go through a temp file in the same directory
(`.kanban.toml.tmp`) followed by `os.Rename` for atomic replacement. No file
locking. If the rename fails, the temp file is cleaned up. Partial writes
never leave a half-written `kanban.toml`.

**Alternatives considered:**
- `flock` advisory locking: cross-platform inconsistency, dev+AI concurrent
  writes are rare, atomic rename is sufficient for single-board-per-repo.
- In-place truncate-and-write: not crash-safe.

**Affects:** P1 (Save implementation), P3+P4 (every mutating command).

### D5. ID generation — 6 chars [0-9a-z], retry max 10

IDs are exactly 6 characters drawn uniformly from `[0-9a-z]` (36 alphabet,
≈2.1 billion combinations). Generation uses `crypto/rand`. On collision with
an existing card, retry up to 10 times then fail with an explicit error
(should never happen in practice but the rail must exist).

**Alternatives considered:**
- UUIDv4: 36 chars, ugly in TOML.
- ULID/KSUID: time-ordered but heavier and harder to type.
- Shorter IDs (4 chars): 1.7M space, real collision risk at scale.

**Affects:** P1 (id.go), P3 (add command).

### D6. `schema_version` mismatch is fatal in `Load()`

If the file's `schema_version` does not match the supported version,
`Load()` returns a structured error of a dedicated type (`SchemaVersionError`)
carrying both the file's version and the supported one. Every command that
loads the board surfaces this error and suggests `ezida migrate` (a future,
yet-unimplemented command). No silent upgrades.

**Alternatives considered:**
- Silent best-effort load: risks data corruption.
- Warning only: easy to miss in scripts.

**Affects:** P1 (Load), all read/write phases (error surfacing).

### D7. JSON output — envelope, snake_case, list omits description

Every command supports `--json`. The output shape:

- `ezida board --json` → `{"schema_version":1, "columns":["todo",...], "priorities":["low",...], "cards_per_column":{"todo":3,...}}`
- `ezida list --json` → `{"cards":[{...}]}`. **Cards in `list` omit the `description` field** (token-efficient — clients call `get` for full body).
- `ezida get --json` → full card object including `description`.
- Mutating commands with `--json` (add, edit, move) → the full card object
  after the operation. `rm --json` → `{"id":"a3f2k9","deleted":true}`.

All JSON keys are `snake_case`, matching TOML keys (no mental remap).
Timestamps are ISO 8601 UTC strings (`2026-05-20T14:30:00Z`).

**Alternatives considered:**
- Bare array root: not extensible — no room for future `total`, `filtered_by`
  meta fields.
- `camelCase`: forces translation layer between TOML and JSON shapes.
- Full description in `list`: token-heavy for large boards.

**Affects:** P2 (all read commands), P3 (write commands echo cards in JSON),
P4 (edit returns full card), P5 (SKILL.md documents shapes).

### D8. Error output — structured JSON on stderr, exit non-zero

Errors always go to stderr. In `--json` mode they are
`{"error":{"code":"<UPPER_SNAKE>","message":"...","details":{...}}}`.
In text mode they are a plain human sentence prefixed with `Error: `.
Both modes use the exit code rules from D10.

Error codes are stable identifiers (`COLUMN_NOT_FOUND`, `CARD_NOT_FOUND`,
`COLUMN_IN_USE`, `SCHEMA_VERSION_MISMATCH`, `INVALID_PRIORITY`, etc.) so AI
clients can branch on them without parsing English.

**Alternatives considered:**
- Plain strings only: forces clients to regex error messages.
- HTTP-style numeric codes: opaque, less self-documenting.

**Affects:** P1 (error types), P2/P3/P4 (command surfacing).

### D9. Text output formats

Default (non-`--json`) formats:

- `ezida list`: aligned table with header — columns `ID COLUMN PRI TITLE TAGS`.
- `ezida board`: sections with counts, e.g. `columns: todo (3) → ongoing (1) → done (7)` and `priorities: low < medium < high`.
- `ezida get`: key:value block with `Description:` heading then the body.
- Mutating commands (`add`, `move`, `edit`): print the affected card ID
  alone on stdout on success (Unix style, easy to pipe). `rm` in text mode
  asks `Delete card a3f2k9 "<title>"? [y/N]` interactively.

**Alternatives considered:**
- TSV by default: parsable but ugly for humans.
- ASCII art kanban for `board`: breaks with long column names and many
  columns; visual layout belongs to the future HTML viewer (out of scope v1).

**Affects:** P2 (read commands), P3 (write commands), P4 (edit).

### D10. Exit codes — three-tier convention

- `0`: success.
- `1`: user-facing error (invalid input, card not found, validation failure).
- `2`: system error (cannot read/write file, internal failure).

`SIGINT` during interactive prompt exits `130` (shell convention, default
behavior — no special handling needed).

**Alternatives considered:**
- Two-tier (0/1): collapses recoverable vs unrecoverable cases.
- Granular codes per error type: noisy and unstable.

**Affects:** all phases that exit on error.

### D11. Color output — auto with `NO_COLOR` and `--no-color`

Text mode colorizes only when stdout is a TTY. Respects the `NO_COLOR`
environment variable. A `--no-color` flag forces plain output regardless.
JSON mode never colorizes.

**Alternatives considered:**
- Always plain: less scannable in terminals.
- Always colored: breaks piping into `grep`/`less`.

**Affects:** P2 (output layer used by all text-mode commands).

### D12. Card placement — append to bottom

`add` places the new `[[cards]]` block at the end of its column section
(bottom of that column visually). `move <id> <column>` places the card at the
end of the new column. Reordering within a column is out of scope for v1
(future `ezida reorder`).

**Affects:** P1 (helper for "insert at end of column"), P3 (add/move).

### D13. `columns add --position` — 1-indexed, default append at end

`ezida columns add <name>` without `--position` appends to the right end of
`[board].columns`. With `--position=N`, N is 1-indexed (`--position=1` puts
the new column first). Out-of-range values error with
`POSITION_OUT_OF_RANGE`.

**Affects:** P4.

### D14. Refusal error — list affected cards with id and title

`columns rm <name>` and `priorities rm <name>` refuse when at least one card
references the target. The text error lists each card as `  <id>  <title>`
(two-space indent). The JSON error includes
`"details":{"column":"<name>","cards":[{"id":"a3f2k9","title":"..."}]}`.

**Affects:** P4.

### D15. `init` — refuse overwrite without `--force`; `--skill-only` overwrites silently

- `ezida init` refuses to touch an existing `kanban.toml`. `--force`
  overwrites it. No backup file is created — the user owns version control.
- `ezida init --skill-only` (and the implicit skill write in `init`)
  overwrites `.claude/skills/ezida-kanban/SKILL.md` silently. The flag's
  intent is "refresh the skill after a binary upgrade".

**Affects:** P2 (init), P5 (--skill-only flag).

### D16. Skill embedding — `go:embed` from `skill/SKILL.md`

The skill ships as a single Markdown file embedded in the binary via
`//go:embed skill/SKILL.md`. `ezida init` writes the embedded bytes to
`.claude/skills/ezida-kanban/SKILL.md` in the target project. The embedded
copy is the single source of truth — no network fetch, no version drift.

The file at `skill/SKILL.md` in this repo derives from `refs/SKILL.md` with
**two patches applied** before commit:
- Replace "installed via pip" with "installed via the install script".
- Remove the Python fallback block (the binary is self-contained, no
  `python ezida.py` fallback exists).

**Alternatives considered:**
- Fetch from GitHub at init: introduces runtime network dependency and a
  version-skew failure mode.
- Separate downloadable artifact: extra step, no benefit.

**Affects:** P5.

### D17. GitHub repository — `github.com/nicolasvergoz/ezida-kanban`

Canonical repo path for releases, install script URL, README links.

**Affects:** P6 (install.sh URL, release workflow), P5 (SKILL.md link to docs).

### D18. Release artifact naming — `ezida_vX.Y.Z_<os>_<arch>.tar.gz`

Each platform produces a gzipped tarball whose name embeds the semver version
and the `GOOS_GOARCH` pair: `ezida_v0.1.0_darwin_arm64.tar.gz`. The tarball
contains the binary named `ezida` (no version suffix inside).

**Alternatives considered:**
- Bare binaries: harder to distinguish in the releases UI.
- Version inside the binary filename: breaks the `mv ezida ~/.local/bin/`
  install pattern.

**Affects:** P6.

### D19. Integrity — `checksums.txt` SHA256, verified by install.sh

Each release attaches a `checksums.txt` file listing SHA256 of every tarball
(`sha256sum`-compatible format). `install.sh` downloads the checksums file
alongside the relevant tarball and verifies before extracting. No signing for
v1 (Apple notarization deferred — cost/complexity outside MVP scope).

**Alternatives considered:**
- No checksums: trivially MITM-able.
- Cosign signing: adds a verification dependency for end users.
- Apple notarization: 99 €/year + lengthy setup.

**Affects:** P6.

### D20. Install script — silent overwrite, PATH reminder only

`install.sh` overwrites `~/.local/bin/ezida` silently on each run (idempotent
upgrade pattern, matches `rustup`/`uv`/`bun`). It prints a reminder if
`~/.local/bin` is not in `PATH` but never modifies the user's shell
configuration files.

**Alternatives considered:**
- Confirm-on-overwrite prompt: breaks `curl ... | sh` flows.
- Auto-edit `.zshrc`/`.bashrc`: invasive, error-prone across shells.

**Affects:** P6.

### D21. Release process — tag from `main`, semver starting `v0.1.0`

Releases are cut by pushing a `v*` tag from `main` only. No release branches.
The first tag is `v0.1.0` (pre-1.0 semver — early but usable). Tag format:
`vMAJOR.MINOR.PATCH`. Pre-release suffixes (`-rc.1`) are supported but unused
for v1.

**Affects:** P6 (release workflow trigger).

## Consequences

**Positive:**
- Single artifact to install, no runtime dependencies.
- File format is diff-friendly and git-native — board changes show up in PRs.
- JSON contract is stable from P2 onward — AI skill can rely on it.
- Atomic writes + structured errors make the CLI safe for both human and
  automated use without locking complexity.
- Cross-cutting decisions live here once; per-phase artifacts focus on
  phase-specific scope.

**Negative / risks:**
- TOML comment preservation is not guaranteed by `pelletier/go-toml/v2` — any
  comments a user adds manually are stripped on the next write. Documented
  in README and `ezida init` output (brief §7.8).
- `pelletier/go-toml/v2` slice ordering must be verified during P1 (spike).
  If marshal does not preserve slice order, a thin custom serializer is
  required to honor card ordering (D3, brief §7.7). Captured as a P1 task.
- No file locking means two concurrent `ezida` processes could race: the
  last writer wins. Acceptable for v1 (single dev per repo, AI prompts are
  serial).
- No Windows support narrows the audience; revisit in v2 if requested.

**Follow-up:**
- v2 deliverables (HTML viewer, slash commands, `ezida migrate`, card
  reordering, multi-board) are explicitly out of scope (brief §11, §12).

## References

- Brief: `refs/PROJECT_BRIEF.md`
- Skill source: `refs/SKILL.md`
- Phases: `board-storage`, `card-reading`, `card-writing`, `board-config`,
  `skill-packaging`, `distribution`
