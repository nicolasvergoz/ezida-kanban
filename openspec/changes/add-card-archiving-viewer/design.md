## Context

`add-card-archiving-cli` delivered the storage layer and the CLI: cards move
between `kanban.toml` and a sibling `kanban.archive.toml` via
`board.ArchiveCard` / `board.ArchiveColumn` / `board.UnarchiveCard`, all pure
in-memory operations the caller persists. The commands package already wraps
them in `runArchive` / `runArchiveColumn` / `runUnarchive`, with the exact
JSON envelope shapes `{id, archived, cascaded}` and
`{id, unarchived, cascaded, orphaned, column, relocated}`. Nothing in
`internal/server` or `internal/server/web` knows any of this exists yet —
`GET /api/board` still reads only `board.Load`.

Three properties of the existing server carry directly into this change:

- **`boardResponse` / `output.ExportEnvelope` are parity-tested.**
  `TestWireShape_ExportMatchesBoard` (`handlers_epics_test.go:242`) compares
  the two structs' `json` tags via reflection and fails the build if they
  diverge — any new wire field on one side needs the other, or the test
  needs to know why not.
- **The watcher is single-file and fails fast if that file is missing**
  (`watcher.go:40`, `NewWatcher(path)` returns an error from `fsw.Add` when
  the path does not exist). The archive file usually does not exist at
  server start.
- **Mutations are load → mutate → save → `httpError` on failure**, uniformly,
  and every mutation that touches the board file already causes the watcher
  to fire and the SSE broker to broadcast — no route in this change needs to
  broadcast anything itself.

This change is server + viewer only. No board-layer or CLI code changes; the
board package's `ArchivePathFor`, `LoadArchive`, `ExistingIDs`,
`ReconcileArchive`, `ArchiveCard`, `ArchiveColumn`, `UnarchiveCard`,
`CardNotArchivedError`, `IDCollisionError` are consumed as-is.

## Goals / Non-Goals

**Goals:**

- Make an archived card visible and restorable from the browser without
  leaving it — the CLI-only state from the prior change was a dead end for
  anyone driving the board through the Web UI.
- Keep a plain board (nothing ever archived) byte-identical on the wire and
  pixel-identical in the DOM — the same non-goal-as-goal the CLI change held
  for `kanban.toml` and JSON output.
- Make the Archive section usable at scale: it can hold hundreds of cards
  without the board becoming unscannable, hence collapsed by default.
- Give the e2e suite non-drag affordances for every archive action, since
  CLAUDE.md rules out asserting real HTML5 DnD.

**Non-Goals:**

- Drag-and-drop into or out of the Archive column. The modal/menu actions are
  the actual UI; drag support (if ever added) is a follow-up, and dragging
  must be confirmed by hand regardless of what Playwright can assert.
- Editing an archived card's fields. Restore first — same rule the CLI change
  established for `archive get`.
- A visual regression baseline for the new section. Baselines are
  machine-specific and this change already carries enough surface; deferred.
- Any change to `internal/board` or `internal/commands`. Everything here
  calls the existing pure operations exactly as the CLI commands do.

## Decisions

### `GET /api/board` gains `archived_cards`, not a second endpoint

`handleBoard` (`handlers.go:625`) additionally calls
`board.LoadArchive(board.ArchivePathFor(s.boardPath))`, then
`board.ReconcileArchive(b, archive)` before building the response — the same
reconciliation the CLI's read paths perform, so a duplicate left by a crash
between an archive operation's two writes is hidden here exactly as it is
from `ezida list --include-archived`.

```go
type archivedCardResponse struct {
    cardResponse
    ArchivedAt time.Time `json:"archived_at"`
}
```

`boardResponse` gains `ArchivedCards []archivedCardResponse
\`json:"archived_cards,omitempty"\``, declared as a `var`, never
`make(...)` — unlike `cards`, which is deliberately non-nil so it always
renders `[]`. Here the *absence* of the key is the contract: `omitempty` on a
nil slice drops the key entirely, which is what keeps a plain board's
response byte-identical to before this feature, the same test style
`epics-render.spec.ts` already applies to `epic`/`color`.

*Alternative rejected:* a second `GET /api/archive` endpoint. It would need
its own SSE coordination (or double the refetch on every `board-changed`),
and splits the "one payload, client resolves relations" convention CLAUDE.md
documents for this codebase. One extra field is cheaper than a second fetch
path, and the CLI already established the pattern (`ezida export`'s
`ArchivedCards` field) — the server should not disagree with it.

Wire parity extends the same way the board layer did it: `archivedCardResponse`
embeds `cardResponse` anonymously, and the export-side counterpart
`output.ArchivedExportCard` embeds `output.ExportCard` anonymously, so
`encoding/json` flattens both without a second hand-written field list.
`TestWireShape_ExportMatchesBoard` gets a third case,
`{"archived card", archivedCardResponse{}, output.ArchivedExportCard{}}` —
but that only holds if `jsonTags` recurses into anonymous fields, since a
field with no explicit tag currently returns `""` and gets skipped. This is a
real change to a shared test helper (`jsonTags`, `handlers_epics_test.go:265`)
and is called out explicitly for review, the same way the board-layer
`ArchivedCard` embedding needed a purpose-built reflection guard
(`TestArchivedCard_EmbedsCardVerbatim`) rather than trusting a shallow
comparison.

### `ezida board --json` gains `archived_count`, mirroring `omitempty`

The topbar's collapsed Archive strip needs a card count before the section is
ever expanded, and `archived_cards` already carries that count implicitly —
but making the client `.length` a payload it already has is wasteful only if
it always fetches the whole array first. Since `GET /api/board` returns the
full array unconditionally (v1 has no pagination), the client already has the
count for free from `archived_cards.length` — **so this decision is really
about the CLI's `ezida board --json`**, which the original CLI change's open
questions deferred exactly to this follow-up. `output.BoardEnvelope` gains
`ArchivedCount int \`json:"archived_count,omitempty"\``; `runBoard`
(`commands/board.go:27`) computes it via `loadArchive` + `len(a.Cards)`, and
the JSON contract in `docs/usage.md` gets a matching entry. `omitempty`
suppresses zero, so a board that has never archived produces unchanged
`ezida board --json` output.

### Three mutation routes, one per CLI verb, no new board-layer error types

```
POST /api/cards/{id}/archive
POST /api/cards/{id}/unarchive     body: {"column": "<name>"} (optional)
POST /api/columns/{name}/archive
```

`POST /api/cards/{id}/move` is the existing precedent for a verb
sub-resource. Each handler follows the load → mutate → save → respond shape
every other mutation handler already uses, calling `board.ArchiveCard` /
`board.ArchiveColumn` / `board.UnarchiveCard` directly — no new board-layer
functions, no new commands-layer functions, because the pure operations
already take a loaded `*board.Board` and `*board.Archive` and the HTTP
handler is exactly that loader.

Two new arms in `httpError`'s `errors.As` chain (`handlers.go:672`), for the
board package's *existing* `CardNotArchivedError` and `IDCollisionError`
(both added by the CLI change, unused by any server code until now):

| Board error | HTTP status | Code |
|---|---|---|
| `*board.CardNotArchivedError` | 404 | `CARD_NOT_ARCHIVED` |
| `*board.IDCollisionError` | 409 | `ID_COLLISION` |

`*board.CardNotFoundError` and `*board.ColumnNotFoundError` already have
arms and are reused verbatim for the archive-card and archive-column routes.

**409 is a new status class for this codebase** (which otherwise uses only
400/404/500). Carried over unchanged from the CLI design's same call: the
request is well-formed and the resource exists, the conflict is with server
state, and inventing a 400-shaped workaround would be less honest about what
actually went wrong.

`POST /api/columns/{name}/archive` has **no interactive prompt** — HTTP has
no TTY to prompt against — so it always behaves as the CLI's
`--yes` path: it archives the full cascade unconditionally and reports what
it took. Confirmation, when the viewer wants one, happens **client-side**,
before the request is even sent (see the viewer-ui decision below). This
mirrors the existing split: the server never asks questions, only the
CLI does, because the CLI owns a terminal and the browser owns a modal.

### Watcher becomes directory-armed and multi-path

`NewWatcher(path string)` (`watcher.go:40`) becomes
`NewWatcher(paths ...string) (*Watcher, error)`, arming one fsnotify watch per
**distinct parent directory** of the given paths (typically one directory,
since board and archive are siblings) and filtering delivered events by
basename against the set of watched files. This is necessary because the
archive file usually does not exist at server start, and a per-file
`fsw.Add` on a missing path fails immediately — exactly the fail-fast
behavior the *board* file legitimately wants, but not the archive file. A
directory watch survives the file not existing yet: fsnotify delivers a
`Create` event for the archive path the moment `ezida archive` (or the
viewer's own new routes) first writes it.

The existing per-file Rename/Create/Remove re-arm block
(`watcher.go:101-109`) is **deleted** — a directory watch is not disturbed by
the atomic temp+rename under it, so there is nothing to re-arm. The basename
filter (`names map[string]struct{}`) is what keeps `.kanban.toml.tmp.*` and
unrelated files in the same directory from firing spurious events; without
it, every temp file rename would coalesce into a false-positive debounce
window, and — worse — a completely unrelated file write in the project root
would trigger a client refetch.

The "board missing at startup fails fast" contract does not disappear: it
moves to an explicit `os.Stat(boardPath)` in `runWithContext`
(`server.go`, before watcher construction) rather than living inside
`NewWatcher`. The caller becomes `NewWatcher(boardPath,
board.ArchivePathFor(boardPath))`.

*Alternative rejected, kept as the documented fallback:* leave the board
watcher as a single-file watch and add a **second, lazily-armed** watcher for
the archive path that retries `fsw.Add(archivePath)` on every board-file
event (the archive file is only ever created by an operation that also
rewrites the board, so a board event is a reliable retry trigger). This is
more code and a slight staleness window (an external `ezida archive` run
while the retry hasn't fired yet is missed until the *next* board write), but
avoids directory-level fsnotify semantics entirely. **Flagged as the plan B**
if the directory-watch approach proves flaky in testing — see Risks.

### Viewer: collapsed-by-default virtual column, not a real column

The Archive section is not an entry of `board.lists` — `toUiBoard`
(`app.jsx:21`) returns a sibling field instead:

```js
archive: archivedCards.length
  ? { cards: archivedCards, collapsed: /* local UI state, see below */ }
  : null,
```

`Board` (`app.jsx:767`) renders `{board.archive && <ArchiveColumn .../>}`
after `board.lists.map(...)`, before the `.add-list` composer. DOM marker
`data-archive="true"` on the section root, **not** `data-column="archive"` —
a real user column literally named `archive` must remain addressable by
`[data-column="archive"]` without ambiguity; this is why the two markers are
kept disjoint by construction rather than by a naming convention that a user
could accidentally collide with.

Collapse state is a `useState` inside `App` (not inside `toUiBoard`, which is
a pure adapter re-run on every fetch) — `archiveExpanded: boolean`, default
`false`, toggled by clicking the collapsed strip, never persisted to
`localStorage`. Collapsing is what makes the section viable at hundreds of
cards: the DOM cost of rendering the full list is paid only once the user
opts in, and the strip itself is O(1) regardless of archive size.

`archived_cards` from the wire, once present, is always fully fetched (v1 has
no pagination) — collapse only controls **rendering**, not the network call.
This is consistent with `archived_count` on `ezida board --json`: the CLI
gets a cheap count without a full list fetch, the viewer already has the full
list from `/api/board` so it just reads `.length`.

**Read-only, not a second `CardItem` variant with a flag threaded through
every prop.** `CardItem` (`app.jsx:1038`) gains a `readOnly` boolean; when
set, `onRemove`/`onToggleTag`/drag handlers are not wired and the click
target that normally opens the editable modal is swapped for one that opens
a read-only variant instead. One component, one set of CSS selectors, rather
than a parallel `ArchivedCardItem` that would drift from `CardItem` over
time.

**Epic index stays live-only.** `buildEpicIndex` (`app.jsx:63`) is still
built exclusively from `board.lists`' cards. An archived child does not
inflate a live parent's progress denominator, and an archived parent's chip
resolves to nothing (matching the board layer: an archived lone child keeps
an `epic` string that no longer resolves, and the CLI change's rule 11
exclusion made that a valid, expected state — the viewer must not treat it as
an error).

### Non-drag affordances, one per action, matching where a user looks

- **Archive one card** — a new button in `CardDetailModal`'s
  `.modal-actions` (`app.jsx` near its existing delete-confirm button),
  `window.confirm` first only when the card is an epic with children
  (mirroring the existing delete confirm precisely).
- **Restore one card** — a read-only `CardDetailModal` variant (reusing the
  same component, `readOnly` prop) whose single action is Restore.
- **Archive a whole column** — a third `ListMenu` (`app.jsx:1003`) entry,
  "Archive all cards", between "Terminal column" and "Delete list", hidden
  when the column has no cards. `window.confirm` first only when the server
  would report a cascade — which the client cannot know before asking, so
  the flow is: send the request, and if the server-computed `cascaded` array
  is non-empty **and** the user has not already confirmed, surface a
  post-hoc notice rather than blocking the request on a pre-flight guess.
  *Alternative considered:* a pre-flight dry-run endpoint to know the cascade
  size before asking. Rejected as unwarranted complexity for a board-scale
  feature — the CLI's own `--yes`-gated interactive prompt already accepts
  that column archiving can surprise the user once in a while, and the
  viewer's confirm-after-the-fact is strictly less disruptive than a second
  round-trip before every column archive.

This lands on a deliberately different confirmation shape from the CLI's
(which knows the cascade size *before* asking, because it computes
`ArchiveColumn` locally before deciding to save). The viewer cannot cheaply
replicate that without a second endpoint, and duplicating `ArchiveColumn`'s
logic in JavaScript would be a second implementation of the same cascade rule
to keep in sync. Documented as a real UX asymmetry, not an oversight.

### CSS: one new banner-commented section, flat hyphenated names

`.list-archive`, `.list-archive.collapsed`, `.card-archived-at`,
`.card-archived-col`, `.card-restore` — following the existing convention
(`.list-*`, `.card-*`, bare adjective state classes like `.collapsed`).

## Risks / Trade-offs

- **Directory-level fsnotify behaves differently across platforms** — kqueue
  on darwin opens every directory entry to implement a directory watch, which
  can pressure the fd limit in a large project root; inotify on Linux does
  not have this problem. → Run `go test ./internal/server/...` repeatedly
  during implementation before trusting it; fall back to the lazily-armed
  second-watcher design (documented above) if it proves flaky. This is the
  single riskiest step in the change.

- **The viewer's column-archive confirmation cannot know the cascade size in
  advance**, unlike the CLI's. → Accepted; a post-hoc notice after a
  server-reported cascade is the chosen trade-off, not a silent gap — stated
  above rather than treated as parity the client owes the CLI.

- **`jsonTags`, a shared test helper, must learn to recurse into anonymous
  struct fields** to make the new wire-parity case meaningful rather than
  vacuously true. → A small, deliberate change to existing test
  infrastructure; called out for review rather than folded in silently.

- **No visual regression baseline for the new section in this change.** →
  Deferred; the section's structural behavior (present/absent, expand/
  collapse, selector shape) is covered by e2e assertions instead.

## Migration Plan

None required. Additive: no new wire field is ever populated for a board that
has not archived anything, so every existing client of `/api/board` or
`ezida board --json` is unaffected. Rollback is reverting this change's
commits; the CLI-only archive/unarchive workflow from the prior change keeps
working exactly as before.

## Open Questions

None outstanding — the two open questions the CLI change deferred here
(`archived_count` on `ezida board --json`, and the follow-up change itself)
are both resolved by this proposal.
