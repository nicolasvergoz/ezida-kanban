## Context

`add-card-archiving-viewer` built the viewer's epic index over live cards only,
with a comment saying that folding in archived cards would let an archived
child "inflate a live parent's denominator". The Go side never even had the
option: `board.EpicProgress(b, id)` and `board.ChildrenOf(b, id)` take only a
`*Board`, so `ezida get` has never seen the archive at all.

That reasoning protected the denominator and broke the numerator. Both move
together — an archived done child leaves `done` *and* `total` — so excluding
them does not keep the ratio honest, it just discards evidence of completed
work. The concrete failure the user hit: archive four finished children of a
five-child epic and it reads `0/1`.

The pieces already on disk make this cheap:

- The archive records each card's `column` — the column it occupied when
  archived — and `board.LoadArchive` returns a missing file as an empty
  archive, so every read path can be archive-aware without a nil dance.
- `GET /api/board` already ships `archived_cards` (each with its `column`)
  alongside `done_columns`, so the viewer needs **no wire change** — only a
  different index construction.
- `BoardConfig.IsDoneColumn(name)` is a pure name lookup, so it works
  unchanged on a column name that came out of the archive.

## Goals / Non-Goals

**Goals:**

- Make archiving finished work leave an epic's progress unchanged, on every
  surface that reports it.
- Keep a board with no archive byte-identical in output and pixel-identical in
  the DOM, the same guarantee both prior archive changes held.
- List archived children wherever they are counted, so a `total` always
  matches the visible list.

**Non-Goals:**

- `ezida list --epic=<id>`. It is a filter over a card set, already governed by
  `--include-archived` / `--archived-only`; conflating it with progress
  derivation would make one flag mean two things.
- Freezing done-ness into the archive file (a `was_done` field). Rejected
  below.
- Any change to `GET /api/board`, `ezida export`, or the archive file format.
- Restoring a card correctly — already covered; `epic` survives archiving
  untouched, which is why it comes back on restore regardless of what any of
  this displays.

## Decisions

### Done-ness is resolved at read time, from the stored column

An archived child counts toward `done` when the `column` recorded in the
archive is a terminal column **now**. This is the same rule a live card
follows, applied to the column the archive happens to have frozen.

It keeps the project's existing invariant — *"Derived values are never stored.
`children`, `done` and `total` are computed at read time, so a hand edit
cannot falsify them"* (`docs/usage.md`) — intact and unqualified.

*Alternative rejected: a stored `was_done` bool on `ArchivedCard`.* It would be
historically exact forever and immune to the caveat below, and the archive file
is arguably a historical record where frozen facts belong (it already stores
`column` and `archived_at` that way). It was still rejected: it adds a field to
a format one change old, it makes `done` the one derived value in the system
that *is* persisted, and `TestArchivedCard_EmbedsCardVerbatim` — the reflection
guard asserting the archive's tag set is exactly `tags(Card) ∪ {archived_at}` —
would have to be loosened, weakening the check that keeps the archive in step
with `Card` for free.

*Alternative rejected: "archived implies done".* Simple, and right for the
common case, but flatly wrong when a card is archived from `backlog` —
abandoned work would inflate progress, which is a worse error than the
understatement being fixed.

**The caveat this accepts**, called out because the workflow that triggers it is
one the previous change shipped and documented: `ezida archive column done`
followed by `ezida columns rm done` — the sequence archiving was built to
enable — deletes the column those archived cards recorded, so they silently
stop counting toward `done` while still counting toward `total`. An epic can
therefore *drop* from `4/4` to `0/4` by way of a column deletion. It is
consistent with what happens to live cards when a terminal marker is cleared,
it is documented in both the spec and `docs/usage.md`, and the remedy is to
re-create or re-mark the column. If it proves annoying in practice, the
`was_done` alternative above is the escape hatch, and it is additive.

### Go: widen the two epic helpers rather than add parallel ones

```go
// ChildrenOf returns the live children of id, in board file order.
func ChildrenOf(b *Board, id string) []Card

// ArchivedChildrenOf returns the archived children of id, in archive
// file order. A nil archive yields nothing.
func ArchivedChildrenOf(a *Archive, id string) []ArchivedCard

// EpicProgress counts the children of id — live and archived — and how
// many count as done. A nil archive reduces it to the live-only
// behaviour it had before.
func EpicProgress(b *Board, a *Archive, id string) (done, total int)

// IsEpic reports whether id is referenced as the epic of at least one
// card, live or archived. A nil archive reduces it to live-only.
func IsEpic(b *Board, a *Archive, id string) bool
```

`EpicProgress` and `IsEpic` change signature rather than gaining `…WithArchive`
twins. There are few call sites (`get.go` ×3 for progress, plus `IsEpic`'s
users), and every one of them *should* be archive-aware — a parallel API would
leave the old name as a working footgun that silently under-reports. Accepting
a nil `*Archive` keeps the "no archive" path a single explicit argument rather
than a second function.

`ChildrenOf` stays live-only and gains a sibling instead: the two return
different types (`[]Card` vs `[]ArchivedCard`), and callers genuinely need to
tell them apart — `ezida get` has to mark the archived ones.

### The nesting guard follows, because it asks the same question

`IsEpic` has exactly one Go caller: `CheckEpicTarget`, whose last rule is
*"the card being edited has children of its own, and epic nesting is limited
to one level"*. Once a card with only archived children counts as an epic,
leaving that guard live-only opens a reachable trap:

1. Card `P` has one child `C`. Archive `C` — `P` now has no *live* children.
2. `ezida edit P --epic=G` succeeds: the live-only guard sees no children.
3. `ezida unarchive C` restores `C.epic = P` while `P.epic = G` — two levels.
   `Save` runs `Validate`, rule 13 fires, and the restore is refused.

Nothing is corrupted (both files are left untouched), but the user is left
with a card that cannot be restored until they detach `P`, and no hint of why.
Refusing at step 2 is the same refusal, delivered where it is actionable.

So `IsEpic` and `CheckEpicTarget` both take the archive, which ripples into
`board.UpdateCard` (its only board-layer caller) and from there to
`commands/edit.go` and the server's `PATCH /api/cards/{id}`. That is the
largest mechanical part of this change — roughly a dozen call sites — and it is
the one piece a reviewer could reasonably cut: **task group 4 is self-contained
and can be dropped**, leaving progress counting correct and the nesting hazard
as a documented follow-up. It is included because the hazard is created by this
change rather than merely exposed by it.

### `ezida get` loads the archive and marks archived children

`runGet` gains a `loadArchive` call (the existing commands-layer helper, which
already turns a missing file into an empty archive). `output.ChildRef` gains:

```go
Archived bool `json:"archived,omitempty"`
```

`omitempty` on a `bool` suppresses `false`, so a live child's entry is
byte-identical to before — the same trick `archived_at`'s pointer plays on
`ListCard`, for the same reason.

Text mode already prints a per-child line with a done marker
(`  ✓ f20wbo  done       Card due dates`). An archived child gets an additional
`(archived)` suffix rather than a new column, so the existing alignment holds.

Ordering is live children then archived children, matching
`list --include-archived` — the one ordering convention the codebase already
committed to for mixed sets.

### Viewer: one index, built over live + archived, with per-card done lookup

`buildEpicIndex(cards, doneSet)` currently reads `c.column` off every card.
Archived UI cards carry `archivedColumn`, not `column`, so the index gains a
tiny accessor rather than a second implementation:

```js
const columnOf = (c) => c.archivedColumn ?? c.column;
```

`toUiBoard` then builds **one** index over `[...allCards, ...archivedCards]`
and uses it for both the live board and the Archive section, replacing the two
indexes the previous change ended up with (`epics` live-only, plus the
archive-scoped `archiveEpics` added when the epic chip turned out to be
missing). One index is now correct for both, because counting archived children
is exactly what the live surface wants too.

`isEpic` follows automatically: a parent whose children are all archived is in
`kids`, so it keeps its glyph, tint and bar.

What does *not* change: archived cards still never appear in `board.lists`, are
still excluded from `filteredCount`, and the Archive section still renders them
read-only. Counting and placement stay separate concerns — that distinction is
what made the original exclusion look reasonable, and it survives intact.

## Risks / Trade-offs

- **Deleting an archived child's column silently lowers `done`.** → Accepted
  and documented in three places (spec, `docs/usage.md` known limitations, and
  the rejected-alternative note above). Reachable via the shipped
  `archive column` → `columns rm` workflow, so it is called out rather than
  buried.

- **`EpicProgress` / `IsEpic` signature changes ripple into tests.** →
  Deliberate: a compile error at every call site is the point, since a
  silently-live-only reading is the bug being fixed. Existing tests on
  archive-free boards pass `nil` and keep their expectations.

- **Threading the archive through `UpdateCard` reaches the HTTP PATCH
  handler**, the widest blast radius here and the only part not strictly
  required to fix the counting. → Isolated in task group 4 so it can be cut
  wholesale; the rest of the change stands without it.

- **The viewer collapses two indexes into one.** → The archive-scoped index
  added mid-implementation last change becomes redundant; leaving both would
  mean two answers to "is this an epic". The e2e assertions added for the epic
  chip on archived cards guard the behaviour that index was introduced for.

- **An epic's counter can now change without any card moving**, if a column's
  terminal marker is toggled while children sit archived. → Already true for
  live cards; the archive just widens the set it applies to.

## Migration Plan

None. No file format changes, no wire changes, no new flags. A board with no
archive derives exactly as before — asserted explicitly on both surfaces.
Rollback is reverting the commit; nothing on disk needs undoing.

## Open Questions

None. The one genuine product decision — which rule decides whether an
archived child counts as done — was settled before this design: stored column,
resolved at read time, with the deleted-column caveat accepted and documented.
