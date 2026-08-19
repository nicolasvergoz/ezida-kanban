## Why

`add-card-archiving-cli` (archived 2026-08-19) delivered `kanban.archive.toml`,
the archive/unarchive board operations, and the full CLI surface, but
deliberately stopped at the storage and CLI boundary. `GET /api/board` and the
rendered board are still untouched: a card archived from the CLI simply
vanishes from the viewer with no way to see it, search it, or bring it back
without leaving the browser. This change closes that gap.

## What Changes

- `GET /api/board` gains an `archived_cards` array (`omitempty`) carrying every
  archived card, so the viewer can render them without a second endpoint or a
  second SSE coordination path.
- Three new mutation routes: `POST /api/cards/{id}/archive`,
  `POST /api/cards/{id}/unarchive`, `POST /api/columns/{name}/archive` —
  the verb-sub-resource pattern already used by `POST /api/cards/{id}/move`.
- The card-creation mint path (`POST /api/cards`) already merges archived ids
  (`add-card-archiving-cli` §4.4); nothing changes there.
- The board-file watcher becomes multi-path: it now also watches
  `kanban.archive.toml`, which does not exist at server start on most boards,
  so an edit that later creates it (an external `ezida archive`) still
  triggers a `board-changed` SSE event.
- The viewer renders a virtual **Archive** column at the end of the board,
  **collapsed by default** to a narrow strip with a count, absent entirely
  when the archive is empty. Expanding it lists archived cards read-only
  (id, title, priority, archived date, original column). A card's detail
  modal gains an Archive action (with a confirm when it would cascade); an
  archived card's modal is read-only with a Restore action; the column's ⋯
  menu gains "Archive all cards".
- `ezida board --json` gains `archived_count` (`omitempty`), which the topbar
  needs to render the collapsed strip's number without paging through
  `archived_cards`.
- Not in this change: drag-and-drop into/out of the Archive column (CLAUDE.md:
  Playwright cannot drive native HTML5 DnD reliably — the modal/menu actions
  above are what the e2e suite drives; real dragging needs your manual
  confirmation).

## Capabilities

### Modified Capabilities

- `viewer-server`: `GET /api/board` response gains `archived_cards`; three new
  mutation routes; the board-file-watcher requirement widens to cover a
  second, not-yet-existing path.
- `viewer-ui`: the embedded page gains a collapsed-by-default Archive column,
  archive/restore/archive-column affordances, and the adapter logic that
  keeps a plain board's DOM pixel-identical to before this feature.
- `card-reading`: `ezida board --json` gains `archived_count`.
- `documentation`: the `ezida serve` section's Web UI capability list gains
  the archive/restore actions.

### New Capabilities

None — this change is additive to the two capabilities the CLI change already
modified, plus one existing wire field.

## Impact

- **Modified code**: `internal/server/handlers.go` (`boardResponse`, 3 new
  handlers, `httpError` gains 2 arms for the board package's existing
  `CardNotArchivedError`/`IDCollisionError`), `internal/server/watcher.go`
  (`NewWatcher` becomes variadic), `internal/server/server.go` (watcher
  construction), `internal/server/web/{app.jsx,styles.css}`,
  `internal/commands/board.go` + `internal/output/json.go`
  (`archived_count`), `docs/usage.md` (Web UI capability list).
- **Existing tests touched**: `internal/server/handlers_epics_test.go`'s
  `TestWireShape_ExportMatchesBoard` needs its `jsonTags` helper to recurse
  into anonymous embedded fields (the archived-card wire type embeds
  `cardResponse`, same trick `ArchivedCard` used for the board layer).
- **Dependencies**: none added.
- **Risk**: directory-based fsnotify watching behaves differently across
  platforms (kqueue on darwin opens every directory entry); design.md carries
  a fallback watcher strategy if the primary one proves flaky under test.
