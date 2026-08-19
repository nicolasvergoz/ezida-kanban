Groups 1–4 verify with `./scripts/verify.sh --go` (no browser needed).
Groups 5–7 need the full `./scripts/verify.sh`. No board-layer or CLI code
changes anywhere in this change — everything calls the pure operations
`add-card-archiving-cli` already shipped.

## 1. Server read wire: `archived_cards`

- [x] 1.1 Add `archivedCardResponse{cardResponse; ArchivedAt time.Time \`json:"archived_at"\`}` to `internal/server/handlers.go`, `cardResponse` embedded anonymously
- [x] 1.2 Add `ArchivedCards []archivedCardResponse \`json:"archived_cards,omitempty"\`` to `boardResponse`; declare it `var`, never `make(...)`
- [x] 1.3 In `handleBoard`, after `board.Load`, call `board.LoadArchive(board.ArchivePathFor(s.boardPath))` then `board.ReconcileArchive(b, archive)` before building the response; map the reconciled `archive.Cards` through `cardToResponse` plus `ArchivedAt`
- [x] 1.4 Add `output.ArchivedExportCard{ExportCard; ArchivedAt time.Time \`json:"archived_at"\`}` to `internal/output/json.go`, and `ArchivedCards []ArchivedExportCard \`json:"archived_cards,omitempty"\`` to `ExportEnvelope`
- [x] 1.5 Populate `ExportEnvelope.ArchivedCards` in `runExport` (`internal/commands/export.go`) from the archive, mirroring how `handleBoard` populates its side
- [x] 1.6 Teach `jsonTags` (`internal/server/handlers_epics_test.go:265`) to recurse into anonymous embedded fields (a field with `Anonymous == true` and no direct `json` tag contributes its own type's tags instead of being skipped)
- [x] 1.7 Add a third case to `TestWireShape_ExportMatchesBoard`: `{"archived card", archivedCardResponse{}, output.ArchivedExportCard{}}`
- [x] 1.8 Tests: `TestHandle_Board_OmitsArchivedCardsKeyWhenNoArchive` (decode to `map[string]any`, assert key absent), `TestHandle_Board_IncludesArchivedCards`, `TestHandle_Board_ReconcilesDuplicateAgainstLiveBoard` (seed the same id in both files, assert it's in `cards` and absent from `archived_cards`)

## 2. `ezida board --json` gains `archived_count`

- [x] 2.1 Add `ArchivedCount int \`json:"archived_count,omitempty"\`` to `output.BoardEnvelope`
- [x] 2.2 In `runBoard` (`internal/commands/board.go`), load the archive via the existing `loadArchive` helper and set `ArchivedCount: len(archive.Cards)`
- [x] 2.3 Tests: `TestBoard_OmitsArchivedCountWhenNoArchive`, `TestBoard_ReportsArchivedCount`

## 3. Server mutation routes

- [x] 3.1 Register `POST /api/cards/{id}/archive`, `POST /api/cards/{id}/unarchive`, `POST /api/columns/{name}/archive` in `routes` (`handlers.go:44`)
- [x] 3.2 Implement `handleCardArchive`: load board+archive, call `board.ArchiveCard`, save archive then board (destination-before-source, same order the CLI's `mutateArchiveAndSave` uses), respond `200 {"archived":id,"cascaded":[...]}`
- [x] 3.3 Implement `handleCardUnarchive`: decode optional `{"column":"..."}` body, call `board.UnarchiveCard`, save board then archive, respond `200 {"card":{...},"cascaded":[...],"orphaned":[...],"relocated":bool}`
- [x] 3.4 Implement `handleColumnArchive`: load board+archive, call `board.ArchiveColumn`, save (skip both saves when both returned slices are empty, matching the CLI's empty-column no-op), respond `200 {"archived":[...],"cascaded":[...]}`
- [x] 3.5 Add two `httpError` arms (`handlers.go:672`) for `*board.CardNotArchivedError` (404 `CARD_NOT_ARCHIVED`) and `*board.IDCollisionError` (409 `ID_COLLISION`); confirm `*board.CardNotFoundError` / `*board.ColumnNotFoundError` arms already cover the other two routes — **correction found during implementation:** `*board.ColumnNotFoundError`'s existing shared arm is 400, not 404; the viewer-server delta spec's unknown-column scenario was wrong and has been corrected to 400 to match every other route on this code (only `POST /api/cards`'s create-time check is the deliberate 404 departure)
- [x] 3.6 Tests in `internal/server/handlers_archive_test.go` (new): happy path per route (body shape + both files correct on disk afterward), `TestHandle_CardArchive_404OnUnknownID`, `TestHandle_CardUnarchive_400OnUnknownColumn`, `TestHandle_CardUnarchive_409OnIDCollision`, `TestHandle_ColumnArchive_LeavesColumnInPlace`, `TestHandle_ColumnArchive_EmptyColumnWritesNothing`, `TestHandle_ArchiveRoutes_BroadcastBoardChanged` (following `handlers_events_test.go`'s pattern — a subscribed client receives `board-changed` within 500 ms of each route succeeding)

## 4. Watcher covers the archive path

- [x] 4.1 Change `NewWatcher(path string)` to `NewWatcher(paths ...string) (*Watcher, error)` in `internal/server/watcher.go`: arm one fsnotify watch per distinct parent directory of the given paths; store `names map[string]struct{}` of watched basenames
- [x] 4.2 In `Run`'s event loop, filter on `if _, ok := w.names[filepath.Base(ev.Name)]; !ok { continue }` before the existing `Op` check
- [x] 4.3 Delete the per-file Rename/Create/Remove re-arm block (`watcher.go:101-109`) — a directory watch is not disturbed by an atomic temp+rename underneath it
- [x] 4.4 In `internal/server/server.go`, add an explicit `os.Stat(boardPath)` fail-fast check in `runWithContext` before constructing the watcher (preserves the "missing board file fails before binding a port" contract that used to live inside `NewWatcher`); change the constructor call to `NewWatcher(boardPath, board.ArchivePathFor(boardPath))`
- [x] 4.5 Tests in `internal/server/watcher_test.go`: `TestWatcher_FiresOnSecondFileCreatedAfterBoot`, `TestWatcher_IgnoresUnrelatedFilesInSameDir` (write `noise.txt`, assert no event within 3× the debounce), `TestWatcher_MissingSecondPathIsNotFatal`, plus confirm existing single-path tests still pass unmodified (variadic call sites with one argument)
- [x] 4.6 Ran `go test ./internal/server/... -run Watcher -count=5` — stable on darwin, no flakiness across 5 runs. Kept the directory-armed design; the lazily-armed fallback was not needed.

## 5. Viewer: adapter and read-only render

- [x] 5.1 In `toUiBoard` (`app.jsx:21`), map `server.archived_cards` (default `[]`) into UI-shaped cards carrying `archivedAt` and `archivedColumn` (the card's own `column` before any UI remapping), and return a sibling `archive: archivedCards.length ? { cards: archivedCards } : null` alongside `lists`
- [x] 5.2 In `Board` (`app.jsx:767`), render `{board.archive && <ArchiveColumn archive={board.archive} .../>}` after `board.lists.map(...)`, before `.add-list`
- [x] 5.3 New `ArchiveColumn` component: root `<section className="list list-archive" data-archive="true">`; collapsed state via `useState` in `App` (not in the pure adapter), default collapsed, strip shows an icon + `archive.cards.length`; expanding renders the cards
- [x] 5.4 Give `CardItem` (`app.jsx:1038`) a `readOnly` prop: when set, do not wire `onRemove`/`onToggleTag`/drag handlers, and the click target opens a read-only detail view instead of the editable modal; add `.card-archived-at` and `.card-archived-col` elements shown only when `readOnly`. **Deviation from the plan:** the *detail view* opened from a readOnly card is a separate `ArchivedCardDetailModal` component, not a `readOnly` branch inside `CardDetailModal` — that component has ~30 pieces of local state and props (`list`, `allLists`) that assume a live card throughout, and with no JSX syntax-checker available in this environment, disabling each edit affordance individually was judged more likely to introduce a subtle bug than a small, separately-verifiable component. `CardItem` itself does get the `readOnly` prop exactly as planned.
- [x] 5.5 Keep `buildEpicIndex` (`app.jsx:63`) built from `board.lists` only — do not fold archived cards into it (true by construction: `toUiBoard` never adds archived cards to `allCards`). **Amended after user testing:** the first implementation also passed `EMPTY_EPIC_INDEX` to the archived `CardItem`s, so an archived card displayed no epic chip at all — the `epic` field was stored and restored correctly, but was invisible while archived. Fixed by building a *second*, archive-scoped index over live + archived cards, used only to render the Archive section and the archived-card detail view. The live index that drives epic progress bars is unchanged, so this preserves the original intent (an archived child must not inflate a live parent's progress) while making the relation visible. Covered by two new tests in `archive-render.spec.ts`.
- [x] 5.6 Exclude archived cards from the topbar's `filteredCount` computation in `App` (true by construction: it only iterates `board.lists`)
- [x] 5.7 CSS in `styles.css`: new banner-commented section — `.list-archive`, `.list-archive.collapsed`, `.card-archived-at`, `.card-archived-col`, plus `.list-archive-strip`, `.list-archive-count`, `.list-archive-collapse`, `.card-archive-meta`, `.card-tags-readonly` (no `.card-restore` needed — restore lives in `ArchivedCardDetailModal`'s `.modal-actions`, reusing existing modal-action styling)

Verified without a browser extension in this environment: `Babel.transform(source, {presets:['react']})` using the project's own vendored `babel.min.js` under Node succeeds (syntactically valid JSX/JS), and a manual `ezida serve` smoke-boot confirmed `archived_cards` is correctly absent from `/api/board` on a fresh board. Full DOM-level verification is deferred to the real-browser Playwright suite in group 7, per CLAUDE.md.

## 6. Viewer: mutation affordances

- [x] 6.1 Add `archiveCard(id)`, `unarchiveCard(id)`, `archiveColumn(listId)` to `App`, following the existing `try { await apiSend(...) } catch { reportFailure } finally { fetchBoard() }` shape every other mutation uses. `unarchiveCard` additionally returns a success boolean (see 6.3) — every other mutation here is fire-and-close, but restore is the one archive action most likely to genuinely fail (`ID_COLLISION`), so its caller needs to know whether to close.
- [x] 6.2 `CardDetailModal`: add an Archive button (`IconArchive`) in `.modal-actions` beside the existing delete button; `window.confirm` first only when `epicChildCount > 0` (passed down from `App`, computed via `board.epics.childrenOf(card.id).length`), mirroring the existing delete-confirm pattern
- [x] 6.3 `ArchivedCardDetailModal` (new, separate component — see 5.4's deviation note) for an archived card: every field rendered inert, single primary action "Restore" calling `unarchiveCard`; on failure the error is shown inline and the modal stays open (`unarchiveCard` returns `false`); on success the caller closes it
- [x] 6.4 `ListMenu`: add "Archive all cards" between "Terminal column" and "Delete list"; hidden via `cardCount > 0` guard; calls `onArchiveColumn` (→ `archiveColumn`), then if the response's `cascaded` is non-empty, an `alert()` post-hoc notice (no pre-flight dry run — see design.md)
- [x] 6.5 Verified manually by the user: dragging a card onto the collapsed Archive strip does nothing at all (no drop handler is wired), which is the intended out-of-scope behaviour. No drag handling was added.

## 7. e2e fixtures and specs

- [x] 7.1 In `e2e/fixtures.ts`, `startServer` also copies `<fixture-basename>.archive.toml` to `<dir>/kanban.archive.toml` when that sibling fixture file exists; add `readArchive(): string | null` to the `Board` type (`null` when the file is absent, so a spec can assert deletion)
- [x] 7.2 Add `liveCardIds(page)` / `archivedCardIds(page)` helpers alongside the existing `visibleCardIds`; audited `epic-focus.spec.ts` / `epics-render.spec.ts` (the only current callers) — both use `epics.toml`/`plain.toml`, neither of which has an archive sibling, so unaffected
- [x] 7.3 New fixtures: `e2e/fixtures/archived.toml` + `archived.archive.toml` (one standalone archived card, one archived epic + its archived child, one archived card whose stored `column` is NOT in `archived.toml` — the relocated-restore path), and a **separate** `e2e/fixtures/archive-source.toml` with no archive sibling of its own — **found during implementation:** reusing `archived.toml`'s stem for a "starts with nothing archived" spec silently pulled in `archived.archive.toml` via `startServer`'s stem-pairing convention, so a distinct board fixture was needed. `plain.toml` still gets no archive sibling — that absence is the original guard.
- [x] 7.4 New spec `e2e/archive-render.spec.ts` (fixture `archived.toml` + its archive sibling): the section renders exactly the archived ids; collapsed by default with the correct count; expands on click; `.card-archived-at` present; the stored-column chip on the relocated-path card shows the deleted column name; a real column named `archive` stays addressable by `[data-column="archive"]`, disjoint from `[data-archive]`
- [x] 7.5 New spec `e2e/archive-actions.spec.ts` (fixture `archive-source.toml`, no archive sibling at boot): open a card's modal → Archive → the section *appears* → the card left its column → `board.readArchive()` non-null; then open the archived card → Restore → the section *disappears entirely* → `board.readArchive()` is `null`
- [x] 7.6 Epic cascade via the modal in `archive-actions.spec.ts`, using `page.once("dialog", d => d.accept())` for the `window.confirm`; asserts both parent and child move together, plus a companion test that declining leaves the board byte-unchanged
- [x] 7.7 `ListMenu` → "Archive all cards" → "Delete list" succeeds on a column that previously had cards; a separate test covers a cascade reaching outside the column, asserted via the post-hoc `alert()` dialog message
- [x] 7.8 `test.describe("a board with no archive")` on `plain.toml`: `.list-archive` count `0`, `[data-archive]` count `0`, and the `/api/board` response (via `page.evaluate(() => fetch(...))`) has no `archived_cards` key
- [x] 7.9 No new visual baseline added — the pre-existing `themes.spec.ts` visual test still (correctly) skips without `PW_VISUAL=1`

Full `npx playwright test` run: **87 passed, 1 skipped** (the pre-existing visual baseline, gated on `PW_VISUAL`), zero regressions in the pre-existing suite. Two bugs were caught and fixed by the real run, not by inspection: the fixture-stem collision above, and a test that checked `archivedCardIds()` without first expanding the (collapsed-by-default) Archive section.

## 8. Documentation

- [x] 8.1 Extend the `ezida serve` section's Web UI capability list in `docs/usage.md` with archiving/restoring a card or a column's cards from the collapsed Archive section
- [x] 8.2 Document `archived_count` in the `### ezida board --json` JSON contract example, noting it is omitted for a board that has never archived
- [x] 8.3 Checked: `docs/usage.md` documents no individual `/api/*` routes anywhere (`grep -n "/api/"` finds nothing) — the `viewer-server`/`viewer-ui` specs remain the source of truth per the existing "points readers to the authoritative spec" scenario, unchanged

## 9. Final gate

- [x] 9.1 `gofmt -l .` clean, `go vet ./...` clean
- [x] 9.2 `./scripts/verify.sh` green (full run: gofmt, go vet, go test all 6 packages, shellcheck, full Playwright — 87 passed, 1 skipped as expected)
- [x] 9.3 Manual smoke scenario covered end-to-end by `archive-actions.spec.ts`'s first test, in a real Chromium instance via Playwright (no Chrome extension was available in this environment to drive it interactively): nothing archived → no section → archive from the modal → section appears collapsed with count 1 → expand → restore → section disappears → `kanban.archive.toml` gone. Confirmed also via a standalone `ezida serve` boot showing `archived_cards` correctly absent from `/api/board` on a fresh board (Group 5).
- [x] 9.4 Confirmed by the user: real HTML5 drag-and-drop into/out of the Archive column does nothing in either the collapsed or expanded state, and the modal Archive/Restore buttons work. This matches the design's explicit non-goal ("Drag-and-drop into or out of the Archive column") — archived cards carry `draggable={false}` and the section wires no `onDragOver`/`onDrop`. Drag support remains a possible follow-up change, not a defect here.
