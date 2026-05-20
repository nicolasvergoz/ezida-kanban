## Context

`ezida` is a fresh Go project — the repo currently contains only
`refs/PROJECT_BRIEF.md`, `refs/SKILL.md`, and the OpenSpec scaffold. This
change creates the Go module and the storage layer that every later phase
depends on. All cross-phase choices (stack, JSON shape, error contract,
atomic writes, ID format) are settled in
`openspec/decisions/0001-kanban-v1-batch.md` — this design covers only the
P1-specific architecture.

## Goals / Non-Goals

**Goals:**
- Provide an in-memory model that round-trips `kanban.toml` losslessly,
  including card-within-column ordering.
- Provide an atomic `Save` so the file is never observed half-written by
  another process (developer's editor, AI assistant, git index).
- Provide typed errors that downstream phases can map to CLI error codes
  and JSON error envelopes without re-classifying.
- Provide unit tests that exercise every validation rule and the
  round-trip invariant against a known fixture.

**Non-Goals:**
- No CLI surface. `cmd/ezida/` does not exist yet.
- No mutation helpers beyond `AppendCardToColumn`. `add`, `edit`, `move`,
  `rm`, and column/priority operations are P3/P4 concerns.
- No concurrent-write coordination. Atomic rename is sufficient for v1
  (see ADR §D4 "Consequences").
- No `migrate` command. Schema version mismatch is fatal in `Load` and
  reported through `SchemaVersionError`; migration tooling is post-v1.

## Decisions

### File layout

```
go.mod
go.sum
internal/board/
  board.go        # Board, Card structs + Load / Save / AppendCardToColumn
  id.go           # NewID, NewUniqueID, ErrIDExhausted
  validation.go   # Validate, ValidationError, Violation, SchemaVersionError
  board_test.go   # round-trip, ID, validation tests
  testdata/
    valid.toml          # fixture used by the round-trip test
    valid_minimal.toml  # smallest valid board (one column, zero cards)
    invalid_*.toml      # one fixture per validation rule failure
```

Splitting `id.go` and `validation.go` out of `board.go` keeps each file
under ~150 lines and matches the test groupings.

### Struct shapes

```go
type Board struct {
    SchemaVersion int     `toml:"schema_version"`
    Board         BoardConfig `toml:"board"`
    Cards         []Card  `toml:"cards"`
}

type BoardConfig struct {
    Columns    []string `toml:"columns"`
    Priorities []string `toml:"priorities"`
}

type Card struct {
    ID          string    `toml:"id"`
    Title       string    `toml:"title"`
    Column      string    `toml:"column"`
    Description string    `toml:"description"`
    CreatedAt   time.Time `toml:"created_at"`
    UpdatedAt   time.Time `toml:"updated_at"`
    Tags        []string  `toml:"tags"`
    Priority    string    `toml:"priority,omitempty"`
}
```

Rationale: 1-to-1 mapping with the TOML field names from brief §5. The
`omitempty` on `Priority` honours the brief's "if absent = no priority"
rule.

### Atomic write pattern

```go
func Save(path string, b *Board) error {
    if err := Validate(b); err != nil { return err }
    data, err := toml.Marshal(b)
    if err != nil { return err }

    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, ".kanban.toml.tmp.*")
    if err != nil { return err }
    tmpName := tmp.Name()
    defer os.Remove(tmpName) // no-op on success after rename
    if _, err := tmp.Write(data); err != nil { tmp.Close(); return err }
    if err := tmp.Sync(); err != nil { tmp.Close(); return err }
    if err := tmp.Close(); err != nil { return err }
    return os.Rename(tmpName, path)
}
```

The temp file lives in the **same directory** as the target so `os.Rename`
is guaranteed atomic on POSIX (same filesystem). `defer os.Remove` cleans
up if any step before the rename fails; after a successful rename the temp
name no longer exists, so the deferred `Remove` is a harmless no-op.

### ID generation

```go
const idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
const idLen = 6

func NewID() string {
    var buf [idLen]byte
    _, _ = rand.Read(buf[:])
    out := make([]byte, idLen)
    for i, b := range buf {
        out[i] = idAlphabet[int(b)%len(idAlphabet)]
    }
    return string(out)
}
```

Bias from `b % 36` is negligible at this alphabet size (36/256 = 14% per
byte, distributes evenly). `crypto/rand.Read` never fails on supported
platforms in practice; we ignore the error per Go idiom for `crypto/rand`.

`NewUniqueID` calls `NewID` in a loop of at most 10 attempts and returns
`ErrIDExhausted` if every attempt collided. At 2.1B address space and a
realistic board size, the expected number of attempts is essentially 1.

### Validation strategy

`Validate` walks the board once and collects violations into a slice
before returning. This avoids the "fix one error, hit the next, repeat"
loop for human users and gives AI consumers the full picture in a single
call.

```go
type Violation struct {
    Rule    int
    Message string
    CardID  string // empty if rule is board-level
}

type ValidationError struct {
    Violations []Violation
}

func (v *ValidationError) Error() string { /* one line per violation */ }
```

`SchemaVersionError` is its own type because it is fatal at `Load` time —
no other validation can meaningfully run against a wrong-version file.

### TOML library spike

`pelletier/go-toml/v2` is the chosen library (ADR §D1). The card-order
requirement (spec rule §D3, brief §7.7) hinges on the library preserving
slice order across `Marshal(Unmarshal(...))`. Manual inspection of the
library's docs and tests suggests it does, but P1 includes an explicit
spike: a test that builds a board with 5 cards in a deterministic order,
marshals, re-unmarshals, and asserts the slice is byte-identical to the
input. If the spike fails, a minimal post-process step sorts `[[cards]]`
blocks in the marshaled bytes by their TOML key match before write (out
of scope here, but the test will signal the need before any later phase
inherits the issue).

## Risks / Trade-offs

- **TOML library slice order** → covered by the spike task. If the spike
  fails, the package gains a thin re-ordering serializer; the public API
  does not change.
- **Bias in ID alphabet** → mathematically negligible at 36⁶ (~2.1B); ID
  space is overwhelmingly larger than any plausible board. Mitigation: the
  retry loop catches any collision regardless of cause.
- **Atomic rename on non-POSIX filesystems** → out of scope (no Windows
  for v1 per ADR §D2). On macOS/Linux + ext4/APFS, `os.Rename` over an
  existing file is atomic.
- **No file locking** → documented in ADR §D4 consequences. Concurrent
  writers race; last writer wins; the user can resolve via git if a
  conflict ever surfaces.
- **Validation cost on every Save** → O(N) over cards; for boards of a
  few hundred cards this is microseconds. Acceptable.

## Migration Plan

Not applicable. This change creates a fresh package in a fresh module; no
existing data, no rollback target.

## Open Questions

- None. The spike outcome is binary and self-contained inside P1.

## Spike outcome — slice order

`TestRoundTrip_PreservesCardOrder` was added against `pelletier/go-toml/v2`
v2.3.1 (the version pinned in `go.mod`). Five cards in column `todo` with IDs
`aaaaaa`..`eeeeee` were appended to a `Board` in order, marshaled via `Save`,
re-loaded via `Load`, and the resulting `b.Cards` ID sequence matched the
input exactly. **The spike passed natively**, so no serializer-side
re-ordering pass is needed. `go-toml/v2` preserves `[[cards]]` slice order
across `Marshal`/`Unmarshal` cycles, and the package relies on that behavior
without a workaround.
