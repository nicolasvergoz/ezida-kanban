## Why

`ezida` is a single-binary Go CLI that reads and writes a project's Kanban
board stored as `kanban.toml` at the repo root. Before any CLI surface can
exist, the program needs a typed in-memory model, a deterministic TOML
round-trip, and the validation rules that protect the file from ever ending
up in an inconsistent state. This change builds that foundation as a pure
library — no CLI, no flags, no stdout — so every later phase can rely on it.

## What Changes

- Introduce a Go module (`go.mod`) targeting Go 1.22+ at the repo root.
- Add `internal/board` package with:
  - `Board` and `Card` structs carrying TOML tags that match the schema
    in `refs/PROJECT_BRIEF.md` §5.
  - `Load(path string) (*Board, error)` that parses `kanban.toml` and runs
    `Validate` before returning. Surfaces a typed `SchemaVersionError` when
    the file's `schema_version` does not match the supported version.
  - `Save(path string, b *Board) error` that writes via the temp-file +
    `os.Rename` atomic pattern.
  - `NewID() string` that draws 6 characters from `[0-9a-z]` using
    `crypto/rand` and `NewUniqueID(existing []string) (string, error)`
    that retries up to 10 times before failing with `ErrIDExhausted`.
  - `Validate(b *Board) error` covering the 9 business rules from brief §7:
    schema version, ID format/uniqueness, referential integrity (column +
    priority), non-empty title, presence of required columns/priorities,
    and absence of duplicates in columns/priorities.
  - Structured error types (`SchemaVersionError`, `ValidationError`,
    `ErrIDExhausted`) so future CLI phases can map to error codes per ADR D8.
- Add `internal/board/board_test.go` covering: TOML round-trip against a
  fixture, ID format/regex, validation positive cases (a valid board), and
  validation negative cases (one test per rule).
- Add a spike task that verifies `pelletier/go-toml/v2` preserves the order
  of `[[cards]]` slices across marshal cycles; if it does not, the package
  adds a minimal serializer step that re-orders blocks before write.

## Capabilities

### New Capabilities
- `board-storage`: the file format, in-memory model, validation rules, ID
  generation, and atomic persistence of `kanban.toml`. All later phases
  consume this capability; none of them re-implement file I/O or
  validation.

### Modified Capabilities
None.

## Impact

- New code: `internal/board/` (4 files), `go.mod`, `go.sum`.
- New test fixture under `internal/board/testdata/`.
- New dependency: `github.com/pelletier/go-toml/v2` (per ADR §D1).
- No CLI yet — package is consumed by `internal/commands/` in P2 onward.
- No user-visible behavior change: this phase ships a library, not a tool.
