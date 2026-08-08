## Context

Ezida stores a board as a single hand-editable `kanban.toml`, read and written by three actors: the CLI, the viewer's HTTP layer, and Claude through the embedded skill. That shared authorship is the dominant constraint on every decision here — a representation that is easy to desync, or that fails quietly, will desync and fail quietly.

The current model is deliberately flat: `Card` has eight fields, `BoardConfig` has three, and `Validate` walks the card slice once applying ten rules. Nothing in the codebase expresses a relation between two cards, and nothing expresses a property of a column beyond its name and its position.

`board.Load` refuses any `schema_version` other than the one compiled in. That check is the reason a breaking change is safe here: adding fields at v1 would let an older binary round-trip a v2 file and silently drop every new field on `Save`, because `go-toml` marshals from the struct and the older struct has no such fields.

This change lands the entire foundation below the wire. `internal/server/` is untouched.

## Goals / Non-Goals

**Goals:**
- A card can name another card as its epic, with exactly one level of nesting.
- The relation is documentary: nothing is ever blocked, refused, or warned about because of it.
- A column can be marked terminal, in a way that cannot desync from the column list.
- An epic carries a color, stored as a plain hex, assigned from a named palette without collisions.
- A v1 board upgrades through an explicit, backed-up, self-reporting command.
- Every derived value (children, progress) is computed at read time and never persisted.

**Non-Goals:**
- No viewer change, no `cardResponse` change, no `ezida export` change — those move together in the wire change.
- No dependency/blocking relation between cards. That is a separate feature with separate semantics.
- No multi-level hierarchy. Depth is capped at one and the cap is what makes cycles unrepresentable.
- No downgrade path from v2 to v1. The backup file is the rollback.
- No reordering of children within an epic. Board file order is the order.

## Decisions

### D1 — An epic is a card, not a new entity

`epic` is a string field on `Card` holding another card's id. Rejected alternatives:

- **A `[[epics]]` array-of-tables with its own id space.** Doubles the id namespace, requires its own CRUD surface, its own validation, its own viewer treatment, and forces every "what is this thing" call site to branch. Jira's model; Ezida is not Jira.
- **A generic `[[cards.links]]` block with a `type` field.** Extensible to dependencies and file links later, but nests an array-of-tables inside an array-of-tables, which `go-toml` serializes as separate `[[cards.links]]` sections. That breaks the one-card-one-block reading of the file, which is the property that makes it hand-editable. Worth paying only for four or more link types; we need one.
- **A tag convention (`epic:auth`).** Zero code, and genuinely covers loose grouping. Rejected because it yields no rollup, no place to describe the chantier, and no way to distinguish "the epic" from "a card tagged with the epic's name".

### D2 — One level, enforced by a reachability rule rather than a cycle check

Rule 13 states that a card carrying a non-empty `epic` may not be referenced as the `epic` of another card. This is a local, single-pass check: build the set of ids that carry an epic, then walk the cards again and flag any whose `epic` is in that set.

The consequence is that cycles are unrepresentable rather than detected. A cycle of length ≥ 2 requires every participant to both carry an epic and be referenced as one, which rule 13 forbids; a self-cycle is rule 12. No DFS, no visited set, no recursion depth. The validator stays O(n) with two passes and no allocation beyond one set.

Relaxing to N levels later means deleting rule 13 and adding a real cycle detector. That is a deliberate one-way door, chosen because the UI cost of arbitrary-depth trees is far larger than the schema cost.

### D3 — Terminal columns are encoded as a `*` suffix, decoded at Load

The natural design is `done_columns = ['done']` in `[board]`, mirroring `priority_colors`. It was implemented in an earlier draft of this change and rejected.

The failure mode: `done_columns` is a second reference to a name that lives in `columns`. Any edit that touches one and not the other — a hand edit, a git conflict resolution, a copied stale file — produces a file that passes every validation rule and in which every epic reports `0/N`. The user sees wrong numbers with no error, no warning, and nothing in the file that looks wrong. Ezida's whole premise is that the file is readable and editable by a human; a representation with a silent-desync mode violates it.

Encoding the flag as a suffix on the name makes the marker and the name the same token. It cannot point at a column that does not exist. Renaming a column carries the marker; deleting a column deletes the marker. The propagation obligations that `columns rename` and `columns rm` would otherwise acquire simply do not exist.

The price is that `[board].columns` stops being a list of literal names, so `Load` and `Save` gain an encode/decode pass. This is the first place in Ezida where the file and the in-memory model are not character-for-character equivalent, and it is worth naming as such. Two mitigations keep it contained:

1. The codec is two pure functions, `DecodeColumns([]string) ([]string, map[string]bool)` and `EncodeColumns([]string, map[string]bool) []string`, tested as a pair for round-trip identity.
2. `BoardConfig.Columns` continues to hold bare names, so every existing comparison — `Validate` rule 7, `MoveCard`, `AddColumn`, the handlers — is untouched.

Alternatives considered for the marker character: `✓` is self-documenting but non-ASCII in a name is hostile to terminals and to typing; `!` reads as negation; `~` reads as approximate. `*` is ASCII, conventionally means "starred/special", and is not a plausible character in a real column name — which is why rule 16 can reserve it outright.

A `Save`-written comment above the `columns` key carries the explanation, so the encoding is discoverable from the file itself rather than only from the docs.

### D4 — Color lives on the card, not in a `[board]` map

`color = '#8b5cf6'` sits on the parent card. The `priority_colors`-shaped alternative, `[board.epic_colors]` keyed by card id, was rejected: priorities are a *declared* closed list, so a map keyed by priority name has a natural integrity constraint and a natural lifetime. Epics are not declared — they are just cards — so a map keyed by card id acquires orphan entries on every deletion, needs its own validation rule for dangling keys, and adds a second place to look. A field on the card dies with the card.

The stored value is always a literal hex. Named palette entries (`violet`, `emerald`, …) are a convenience of the CLI and the viewer; a consumer that has never heard of the palette still reads a valid color. This is the same split as `priority_colors`, where `DefaultPriorityColors` lives in Go and hex values live in the file.

### D5 — Palette assignment picks the least-used color, not the next one

A modulo-of-count strategy (`palette[nEpics % 8]`) breaks after a deletion: delete the second of three epics, create a new one, and the count-based index hands out a color already in use. Picking the least-used color and breaking ties by palette order is the same cost — one pass building a histogram — and is stable under deletion, reordering, and hand-assigned off-palette colors.

The palette is ordered by chromatic distance rather than by hue, so consecutive assignments land far apart on the wheel. Ordering by hue would hand out violet, then fuchsia, then pink, which are indistinguishable in an 18px chip. It also excludes the three default priority colors, so a colored epic chip is never mistaken for a priority signal.

### D6 — Deleting a parent orphans its children rather than refusing

`columns rm` refuses when referenced, because `column` is a required field and removing the column would produce invalid cards. `epic` is optional, so clearing it produces a perfectly valid card. Refusing would force a user to unlink six cards by hand before deleting a parent, for no integrity benefit.

Orphaning does not refresh the children's `updated_at`, on the same reasoning `RenameColumn` uses: a board-level consequence is not an edit to the card. The command reports the affected ids so the write is never silent.

### D7 — Derived values are computed, never stored

`children`, `done`, and `total` are functions of the card slice. Storing them would create a second source of truth that every mutation must maintain and that a hand edit can falsify. They are computed on demand in the read commands. The cost is an O(n) pass per `get`, on boards of tens of cards.

### D8 — A hard version bump, with `migrate` as the only reader of an old file

`SupportedSchemaVersion` becomes 2 and `Load` keeps its strict equality check. `ezida migrate` is the single code path allowed to parse a file whose version does not match; it does so by decoding the TOML directly rather than going through `Load`, then constructs a v2 board and runs the normal `Validate` + `Save`.

An "opportunistic v2" variant — accept both 1 and 2 on read, write 2 only once a card actually carries an epic — was considered and rejected. It avoids breaking users who never touch the feature, but the benefit evaporates the moment the first epic is created, and the cost is permanent: two accepted versions means two code paths to test on every future storage change.

Because a v1 board declares no terminal column, `migrate` must invent one. It prefers a column literally named `done`, falls back to the last declared column, and reports its choice on stdout. Choosing silently would reintroduce exactly the "wrong numbers, no signal" failure that D3 exists to prevent.

### D9 — The embedded skill is part of the change, not a follow-up

`internal/skill/SKILL.md` is what teaches Claude the CLI surface. Shipping the flags without updating it means Claude keeps writing v1-shaped `ezida add` invocations and never uses epics. The skill file ships inside the binary and is refreshed by `ezida init --skill-only`, so a user who upgrades must be told to re-run it — that goes in the `migrate` output.

## Risks / Trade-offs

**Every existing board stops working until `ezida migrate` runs.** → The error names the exact command. `migrate` writes `kanban.toml.v1.bak` before touching anything, so rollback is a `mv`. The failure is loud and immediate rather than a slow corruption, which is the trade this change is buying.

**Users on an older binary can no longer read a migrated board.** → Unavoidable consequence of the bump, and the point of it. The mismatch message distinguishes the two directions: an old file says "run `ezida migrate`", a new file says "upgrade `ezida`".

**The column codec makes the file and the model non-identical for the first time.** → Contained to two pure functions with a round-trip test. The existing `roundtrip_test.go` gains fixtures with zero, one, and multiple terminal columns, plus the `['done', 'done*']` collision case.

**`*` becomes reserved in column names.** → Rule 16 rejects it explicitly with a message naming the reason, rather than silently mangling. A user who genuinely wants `done*` as a display name is out of luck; judged acceptable against the desync failure D3 avoids.

**Assigning an epic mutates two cards in one command.** → `ezida edit <child> --epic <parent>` may also write a color onto the parent. Both writes go through a single `Save`, so there is no partial state, but the JSON echo returns only the edited card. The parent's change is reported in text mode and discoverable through `ezida get`.

**Rule 13 rejects a legal-looking edit with a non-obvious reason.** → Pointing a card at a parent that is itself a child fails with `INVALID_EPIC`. The message must say *why* — that the target already belongs to an epic and nesting is one level — not just that the id was rejected.

**`ezida get --json` will expose `epic` while `ezida export` will not**, until the wire change lands. → Accepted deliberately to keep this change frontend-free. `output.ExportCard` and `server.cardResponse` are parallel structs kept in sync by convention; they move together in the next change. Noted in the proposal so it is not discovered as a bug.

## Migration Plan

1. Ship the binary. Any existing board now fails every command with `SCHEMA_VERSION_MISMATCH`, naming `ezida migrate`.
2. The user runs `ezida migrate`. It backs up to `kanban.toml.v1.bak`, upgrades `schema_version` to 2, marks one column terminal, reports the choice, and reminds the user to run `ezida init --skill-only` to refresh the embedded skill.
3. Rollback: restore `kanban.toml.v1.bak` and reinstall the previous binary. No data is lost in either direction, because a v1 board carries no epic data to lose.

## Open Questions

- Should `ezida migrate` refresh `.claude/skills/ezida-kanban/SKILL.md` itself rather than only reminding the user? It would remove a step, but `migrate` would then write outside `kanban.toml`, which no other command does.
- Should `ezida columns add` accept a `--done` flag, or is `columns add` followed by `columns done` acceptable? Two commands is more typing but keeps `add` unchanged.
- The palette caps at eight named colors. Boards with more than eight epics reuse colors by design. Worth surfacing a warning at assignment time, or is silent reuse correct?
