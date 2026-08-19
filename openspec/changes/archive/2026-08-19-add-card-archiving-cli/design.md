## Context

`kanban.toml` is the whole database. Cards accumulate in the terminal column and
never leave, so the file grows monotonically and stops being scannable by the
human it was designed for. The only exit today is `ezida rm`, which destroys the
record.

Three properties of the existing code shape every decision below:

- **`Load` and `Save` both run `Validate`** (`board.go:164`, `board.go:198`).
  A board that references a card which is no longer present cannot be persisted
  at all — rule 11 makes `Save` refuse. Anything that removes cards must reckon
  with what points at them.
- **There is no `position` field.** Ordering is slice order, which is block
  order on disk. Extraction is a pure splice with no renumbering.
- **`Save` is atomic per file** (temp + rename in the same directory) but there
  is no cross-file transaction anywhere in the codebase, and adding one is out
  of proportion to a single-user file-based board.

This change delivers storage plus the CLI. The viewer and HTTP surface follow in
a separate change, so every decision here is verifiable with
`./scripts/verify.sh --go`.

## Goals / Non-Goals

**Goals:**

- Move cards out of `kanban.toml` without losing anything about them.
- Keep both files readable and hand-editable — the point of a file-based board.
- Make a board that never archives byte-identical to one built before this
  feature: no new file, no new keys, no changed output.
- Make archive and unarchive exact inverses, provable by a byte-identity test.
- Give the CLI enough surface to find an archived card without opening the file.

**Non-Goals:**

- Any viewer or HTTP change. `GET /api/board` and the rendered board are
  untouched until the follow-up change.
- Editing an archived card. The archive is append-and-restore; editing means
  restoring first.
- Permanent purge of an archived card. Deleting from `kanban.archive.toml` by
  hand works and needs no command yet.
- Rotation, sharding or compaction of the archive file. One file until it hurts.
- A cross-file transaction, a lock file, or a write-ahead log.

## Decisions

### The archived-card type embeds `Card` anonymously

```go
type Archive struct {
    SchemaVersion int            `toml:"schema_version"`
    Cards         []ArchivedCard `toml:"cards"`
}
type ArchivedCard struct {
    Card
    ArchivedAt time.Time `toml:"archived_at"`
}
```

go-toml v2.3.1 flattens untagged anonymous embedded structs on **both** sides —
marshal at `marshaler.go:801`, unmarshal at `unmarshaler.go:1440` — so this
produces exactly the flat `[[cards]]` block wanted, with `archived_at` last.

*Alternative rejected:* duplicating `Card`'s ten fields into `ArchivedCard`.
This repo already carries one pair of hand-synchronised parallel structs
(`output.ExportCard` / `server.cardResponse`) and documents the maintenance
burden in CLAUDE.md; embedding means a field added to `Card` reaches the archive
for free, with no second place to forget.

The guard that replaces the convention is a reflection test:
`ArchivedCard` field 0 must be anonymous and of type `Card`, and the recursive
toml-tag set must equal `tags(Card) ∪ {archived_at}`. A shadowing field fails it.

The archive reuses `SupportedSchemaVersion`. A second version constant would
imply a second migration story for a file that has never had a v1.

### Validation is a separate, deliberately relaxed rule set

An archive cannot satisfy the board's rule 7 (`column` ∈ `[board].columns`) —
its whole purpose is to outlive the columns it references — nor rule 11 (`epic`
names an existing card), because a lone archived child keeps a parent reference
that no longer resolves. Running `Validate` over an archive would therefore
reject correct data.

`ValidateArchive` keeps rules **1, 4, 5, 6, 9, 12, 14**, drops **2, 3, 7, 8, 10,
11, 13, 15, 16, 17**, and adds **18** (`archived_at` non-zero, not before
`created_at`) and **19** (`column` non-empty). Rule numbers keep their board
meanings, because they are a stable vocabulary across the specs.

*Alternative rejected:* a `relaxed bool` parameter on `Validate`. Rule 7 and 11
are not "relaxed", they are meaningless in this context, and a flag would make
every existing call site read as though it had a choice.

The two dropped rules cost something real: **restoring is where the invariants
come back**, so `UnarchiveCard` has to re-establish them (clear unresolvable
`epic`, relocate a card whose column is gone) before `Save` gets a chance to
refuse. That work is the price of not validating on the way in, and it is the
right trade — refusing to *archive* a card because its parent will not follow it
would make the feature unusable on any board that uses epics.

### Write the destination before the source

There is no cross-file transaction, so the only choice is which failure mode to
have. Ordering the writes so the **destination gains before the source loses** —
archive file first when archiving, board file first when unarchiving — means a
crash between the two writes always yields a *duplicate*, never a disappearance.

Duplicates are then resolved deterministically on read: **the live board wins**.
`ReconcileArchive` drops from the archive, in memory, any card whose id is on the
board. Reads never rewrite; the next write persists the reconciled state.

*Alternative rejected:* a `.bak` sibling, the precedent set by `migrate.go:73`.
`migrate` backs up because it is a one-way whole-file format change with no
inverse. Archive and unarchive *are* each other's inverse, and dropping a `.bak`
on every archive would litter the repo for no recoverable benefit.

*Alternative rejected:* a lock file. It defends against concurrent writers, which
is a different problem the codebase already declines to solve everywhere else.

### ID uniqueness widens to span both files

`NewUniqueID(existing)` only sees what it is handed, and rule 5 is per-file. Feed
it only live ids and a new card can take an id sitting in the archive; the later
restore then breaks rule 5 and the board becomes unloadable.

There are exactly two mint sites — `add.go:99-106` and the server's
`handleCreate` — and both must pass `ExistingIDs(b, a)`. The cost is one extra
file read per card creation, which on a board with no archive is a single ENOENT.

`UnarchiveCard` refusing with `ID_COLLISION` is the backstop for the residual
race where a third party edits the archive between the two reads. It refuses
rather than silently re-minting, because ids appear in commit messages and
issue text — a card that changes id on restore is worse than one that will not
restore until the user looks.

### Absent means empty, and empty means absent

`LoadArchive` treats a missing file as an empty archive; `SaveArchive` **deletes**
the file when the archive holds no cards. `ezida init` does not create it.

This is what makes "a board that never archives is unchanged" true at the
filesystem level rather than merely at the output level, and it is what lets the
archive→unarchive roundtrip test assert byte identity plus the absence of the
file. In the follow-up change it is also what keeps the viewer's Archive column
absent without any extra conditional.

### Archiving an epic cascades to its children

Constraint from the user, but it also falls out of rule 11: leaving children
behind with a dangling `epic` would make `Save` refuse, so the alternatives were
cascade, refuse, or orphan. Cascade keeps the group intact and restorable;
orphaning silently destroys parentage the user never asked to lose.

Within one operation: parent first then children in board file order, inserted at
the head of the archive (newest first, mirroring `PrependCardToColumn` and the
recent "cards go to the top of a column" change); one shared `archived_at`, so an
operation is identifiable in the file; `updated_at` untouched, on the same
reasoning `RenameColumn` and `DeleteCardOrphaning` already document — archiving
is not a content edit.

Restoring reverses it: the cascade is restored in reverse archive order so the
final board order matches what was archived.

### `archive column` archives cards, not the column

The column stays in `[board].columns`. Removing it would silently do a second
thing the user did not ask for, and leaving it in place is precisely what turns
`ColumnHasCardsError` into a solvable problem — hence the reworded refusal
message pointing at `ezida archive column`.

The cascade can pull cards out of *other* columns, so the command confirms
before writing whenever it would: prompt on a TTY (reusing `promptConfirm` and
the `rmIO` injection from `rm.go`), `INTERACTIVE_REQUIRED` for `--json` without
`--yes`, and no prompt at all when nothing outside the column is affected.

*Alternative rejected:* refusing when an epic has children outside the column.
That makes "clear this column" fail on any board that uses epics, with no remedy
short of hand-unlinking.

### `archive list` / `archive get` reuse the existing envelopes

Both are thin wrappers over the same `runList` / `runGet` machinery, so there is
one implementation and one envelope shape each. `ListCard` and `GetCard` gain
`ArchivedAt *time.Time` — **a pointer**, because `omitempty` does not suppress a
zero `time.Time` (it is a struct) and a value field would emit
`"archived_at":"0001-01-01T00:00:00Z"` on every live card, breaking the
"unchanged for non-users" guarantee at the JSON contract level.

Filter validation widens to `board values ∪ archived values` under
`--archived-only`, otherwise `--column review` on an archived card raises a
spurious `INVALID_FILTER`.

Ordering with `--include-archived` is live-then-archived, each in file order. Not
interleaved by date: there is no `position` field, file order is the board's only
ordering concept, and a global date sort would put `list` at odds with `board`.

### Flag exclusion is hand-rolled, not cobra's

`MarkFlagsMutuallyExclusive` produces a message that does not match any prefix in
`isUsageError` (`exit.go:141`), so it would classify as `IO_ERROR` and exit 2 — a
system-error code for a user mistake. A typed `MutuallyExclusiveFlagsError`
carrying `MUTUALLY_EXCLUSIVE_FLAGS` and exit 1 keeps the taxonomy honest.

## Risks / Trade-offs

- **A crash between the two writes duplicates a card.** → Ordering makes
  duplication the only possible failure mode; `ReconcileArchive` hides it on
  every read with a fixed winner, and the next write heals the file. Stated in
  `docs/usage.md` rather than hidden.

- **`ezida archive column` is shadowed by a card whose id is literally
  `column`.** Ids are six characters of `[0-9a-z]`, so `column` is a possible id
  and cobra resolves the subcommand first. → Accepted: probability ≈ 1/2.2·10⁹,
  the failure is a `CARD_NOT_FOUND` rather than a wrong action, and the viewer
  offers a way through. Documented under known limitations. `list` and `get` are
  not six characters and cannot collide.

- **Two more file reads on hot paths.** `add` and every archive-aware read now
  touch a second file. → A board with no archive pays one ENOENT. No caching is
  introduced; the codebase reloads from disk on every command by design.

- **Restoring re-establishes invariants the archive does not enforce**, so a
  hand-edited archive can produce a restore that fails `Save`'s validation. →
  The failure is a clean refusal with the standard `VALIDATION_FAILED` envelope
  and both files unchanged, not corruption. The relaxed validator still catches
  everything that is unambiguously malformed (id shape, empty title, timestamps).

- **The archive grows without bound.** → Out of scope by choice. It is out of the
  daily read path, which was the actual complaint; rotation can be added later
  without changing the format.

- **Reworded `COLUMN_IN_USE` text breaks an asserted literal** at
  `errors_test.go:80`, and the board-layer message is mirrored into the HTTP
  error the viewer surfaces. → Both are updated in the same task; the codes are
  unchanged, so no consumer that switches on `code` is affected.

## Migration Plan

None required. The feature is additive: no schema version bump, no change to
`kanban.toml`'s format, and no new file on any board that does not archive.
Rollback is deleting `kanban.archive.toml` after restoring anything wanted from
it — an older binary simply ignores a file it does not know about.

## Open Questions

- Should a permanent purge (`ezida archive rm <id>`) exist, or is hand-editing
  the archive file the intended escape hatch? Deferred; the format makes it a
  small addition later.
- Should `ezida board --json` report an `archived_count`? Cheap and non-breaking
  with `omitempty` suppressing zero, but outside the stated CLI scope. Deferred
  to the follow-up change, where the viewer needs the number anyway.
