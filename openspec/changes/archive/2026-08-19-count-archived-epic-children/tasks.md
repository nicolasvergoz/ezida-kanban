Groups 1–4 verify with `./scripts/verify.sh --go`. Group 5 needs the browser.
No file-format or wire changes anywhere in this change — the archive already
records each card's `column`, and `/api/board` already ships `archived_cards`.

## 1. Board layer: archive-aware epic derivation

- [x] 1.1 Add `ArchivedChildrenOf(a *Archive, id string) []ArchivedCard` to `internal/board/epics.go` — archive file order, nil archive yields nothing. Leave `ChildrenOf(b, id)` live-only: the two return different types and callers must tell them apart.
- [x] 1.2 Change `EpicProgress(b *Board, id string)` to `EpicProgress(b *Board, a *Archive, id string) (done, total int)`. A live child counts toward `done` on `b.Board.IsDoneColumn(c.Column)`; an archived child on `b.Board.IsDoneColumn(c.Column)` too — its `Column` is the one the archive recorded. A nil archive reproduces the previous behaviour exactly.
- [x] 1.3 Change `IsEpic(b *Board, id string)` to `IsEpic(b *Board, a *Archive, id string) bool` — true when any live *or* archived card names id as its epic; nil archive reduces to live-only
- [x] 1.4 Tests in `internal/board/epics_test.go`: `TestEpicProgress_CountsArchivedChildren` (3 done children, 2 archived → still `3/3`), `_ArchivedFromNonTerminalColumnCountsOnlyTotal`, `_NilArchiveMatchesPreviousBehaviour`, `_ArchivedChildStopsCountingWhenColumnDeleted` (archive from a terminal column, then remove it from `b.Board.Columns` → `0/1`), `TestIsEpic_TrueWithOnlyArchivedChildren`, `TestArchivedChildrenOf_ArchiveFileOrder`

## 2. `ezida get` reports archived children

- [x] 2.1 Add `Archived bool \`json:"archived,omitempty"\`` to `output.ChildRef` — `omitempty` suppresses `false`, so a live child's entry stays byte-identical
- [x] 2.2 In `runGet` (`internal/commands/get.go`), load the archive via the existing `loadArchive` helper and pass it to `board.EpicProgress`
- [x] 2.3 Build the JSON `children` slice from live children then `board.ArchivedChildrenOf`, marking the latter `Archived: true`; gate the whole `children`/`progress` block on `len(live)+len(archived) > 0` rather than `len(children) > 0`
- [x] 2.4 In text mode, append the archived children to the `Children:` block with an `(archived)` suffix on the line, keeping the existing `  %s %s  %-10s %s` alignment and the done marker (an archived child from a terminal column still gets its `✓`)
- [x] 2.5 Tests in `internal/commands/epics_test.go` (or a new `get_archive_test.go`): `TestGet_ListsAndCountsArchivedChildren`, `TestGet_MarksArchivedChildInJSON` (decode to `map[string]any`; assert the key is present on the archived entry and **absent** on live ones), `TestGet_MarksArchivedChildInText`, `TestGet_NoArchiveOutputUnchanged` (byte-identical stdout vs. the same board with no archive file), `TestGet_EpicWithOnlyArchivedChildrenStillReportsThem`

## 3. Viewer: one index over live + archived

- [x] 3.1 In `buildEpicIndex` (`app.jsx`), read each card's column through a `columnOf(c) => c.archivedColumn ?? c.column` accessor instead of `c.column` directly, so an archived UI card resolves its done-ness from the column the archive recorded
- [x] 3.2 In `toUiBoard`, build a single index over `[...allCards, ...archivedCards]` and use it for both `epics` and the Archive section — replacing the two indexes the previous change ended up with (live-only `epics` plus the archive-scoped `archiveEpics` added when the epic chip turned out missing). Delete `archiveEpics`; `board.archive.epics` and `board.epics` become the same object.
- [x] 3.3 Confirm by reading the call sites that nothing else changes: archived cards still never enter `board.lists`, are still excluded from `filteredCount`, and the Archive section still renders them read-only. Counting and placement stay separate.
- [x] 3.4 Syntax-check with the vendored Babel under Node (`Babel.transform(src, {presets:['react']})`) — there is no JSX linter in this repo, so this is the only pre-browser check available

## 4. Nesting guard follows the same rule *(self-contained — droppable, see design.md)*

- [x] 4.1 Change `CheckEpicTarget(b *Board, childID, epicID string)` to `CheckEpicTarget(b *Board, a *Archive, childID, epicID string)`; its final rule calls the now archive-aware `IsEpic`
- [x] 4.2 Thread the archive through `board.UpdateCard` (`internal/board/update.go:119` is `CheckEpicTarget`'s only board-layer caller) → `UpdateCard(b *Board, a *Archive, id string, patch CardPatch) error`
- [x] 4.3 Update the three commands-layer call sites: `add.go:92` (already has an `archive` in scope from the id-minting change), `edit.go:170`, and whichever `UpdateCard` caller `edit.go` uses
- [x] 4.4 Update the server's `PATCH /api/cards/{id}` handler (`handlePatch`) to load the archive and pass it to `board.UpdateCard`
- [x] 4.5 Tests: `TestCheckEpicTarget_RefusesParentForCardWithOnlyArchivedChildren`, `TestCheckEpicTarget_NilArchiveMatchesPreviousBehaviour`, and an end-to-end `TestEdit_RefusesEpicWhenChildrenAreArchived` asserting `INVALID_EPIC` and a byte-unchanged `kanban.toml`
- [x] 4.6 Verify the trap is actually closed: archive a child, attach its parent to another epic (must now be refused), and confirm that without the fix the subsequent `unarchive` would have failed with `VALIDATION_FAILED`

## 5. e2e: the counter survives archiving

- [x] 5.1 Extend `e2e/fixtures/archived.archive.toml` so at least one archived card is a child of a **live** epic in `archived.toml` (today its archived epic/child pair is self-contained, which does not exercise a live parent counting an archived child)
- [x] 5.2 Add to `e2e/archive-render.spec.ts`: a live epic whose child is archived still renders its glyph, tint and bar, and its counter includes the archived child
- [x] 5.3 Add to `e2e/archive-actions.spec.ts`: read an epic's counter, archive one of its done children through the modal, and assert the counter is **unchanged** — the regression this whole change exists to prevent
- [x] 5.4 Confirm the `plain.toml` "no archive" describe block still passes untouched (no epic chrome, no `[data-archive]`)

## 6. Documentation

- [x] 6.1 In `docs/usage.md`'s epics section, state that archived children count toward `done`/`total`, and name the rule (the column recorded in the archive, checked against the board's terminal columns at read time)
- [x] 6.2 Update the "Derived values are never stored" bullet so it stays accurate — the values are still derived, now over a wider set
- [x] 6.3 Add the deleted-column caveat to the known-limitations section: removing or un-marking the column an archived child came from stops it counting toward `done` while it keeps counting toward `total`, reachable via `archive column done` → `columns rm done`

## 7. Final gate

- [x] 7.1 `gofmt -l .` clean, `go vet ./...` clean
- [x] 7.2 `./scripts/verify.sh` green (full run including Playwright)
- [x] 7.3 Manual smoke on the real board: pick a live epic with done children, note its counter, `ezida archive <a done child id>`, confirm `ezida get <epic id>` and the viewer both still report the same `done/total`
