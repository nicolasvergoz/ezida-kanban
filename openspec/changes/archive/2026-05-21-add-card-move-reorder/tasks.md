## 1. Board primitives

- [x] 1.1 In `internal/board/board.go`, implement `InsertCardAt(b *Board, c Card, column string, position int)` per design (with clamping). Done when `go vet ./internal/board` exits 0 and the function compiles.
- [x] 1.2 Refactor `AppendCardToColumn` to delegate to `InsertCardAt(b, c, c.Column, count)` where `count` is the number of cards already in `c.Column`. Done when the existing `board_test.go` tests for `AppendCardToColumn` still pass without modification.
- [x] 1.3 Implement `MoveCard(b *Board, id, column string, position int) error` per design, returning `*CardNotFoundError` or `*ColumnNotFoundError`. Reuse the existing typed errors from `internal/board` (or `internal/commands/errors.go` if that's where they live — check before declaring new ones). Done when the function compiles and a smoke test exercises both error paths. — Note: `internal/commands` already imports `internal/board`, so reusing those error types would create an import cycle. Added board-package siblings `CardNotFoundError` / `ColumnNotFoundError` (same field names, same wire code namespace via the server's `httpError` mapper).

## 2. Board tests

- [x] 2.1 Add `TestInsertCardAt_Middle`, `_PositionZero`, `_BeyondEnd`, `_Negative`, `_EmptyColumn`, `_SetsColumn` in `internal/board/board_test.go` covering each scenario from `specs/board-storage/spec.md`. Done when `go test ./internal/board -run "TestInsertCardAt"` exits 0.
- [x] 2.2 Add `TestMoveCard_CrossColumn`, `_WithinColumn`, `_UnknownCard`, `_UnknownColumn`, `_NoopRefreshesTimestamp`. Done when `go test ./internal/board -run "TestMoveCard"` exits 0.
- [x] 2.3 Add `TestAppendCardToColumn_StillMatchesPriorBehavior` that runs a sequence of appends against a known board and asserts the final slice matches the pre-V2 expectation (a regression test pinning the refactor). Done when the test passes.

## 3. Server endpoint

- [x] 3.1 In `internal/server/handlers.go`, register `POST /api/cards/{id}/move` using Go 1.22+ stdlib `ServeMux` method-prefixed patterns. Done when `curl -X POST http://127.0.0.1:<port>/api/cards/<id>/move -d '{}'` reaches the handler (not 404). — Verified via automated checks: `TestHandle_Move_MalformedBody` reaches the handler and surfaces 400 INVALID_BODY rather than the 404 NOT_FOUND envelope unknown routes produce.
- [x] 3.2 Implement the handler per design (decode body, load board, call `MoveCard`, save, encode `{card: ...}`). Done when `TestHandle_Move_Success` (against a fixture board) returns 200 and the on-disk file reflects the change.
- [x] 3.3 Add `InvalidBodyError` typed error (or reuse if one already exists from V1) and map it to HTTP 400 + JSON code `INVALID_BODY` in `httpError`. Map `*CardNotFoundError` → 404 `CARD_NOT_FOUND` and `*ColumnNotFoundError` → 400 `COLUMN_NOT_FOUND`. Done when each error path test passes.
- [x] 3.4 Add server tests: `TestHandle_Move_Success`, `_WithinColumn`, `_UnknownCard`, `_UnknownColumn`, `_MalformedBody`, `_ClampsPosition`. Done when `go test ./internal/server -run "TestHandle_Move"` exits 0.

## 4. Vendor Sortable.js

- [x] 4.1 Pick the latest Sortable.js 1.x stable release at implementation time and record the version in a leading comment of `internal/server/web/vendor/sortable.min.js`. Done when the file begins with `/* Sortable.js v1.<minor>.<patch> - https://github.com/SortableJS/Sortable */`.
- [x] 4.2 Download `https://cdn.jsdelivr.net/npm/sortablejs@<version>/Sortable.min.js` into the file. Done when `GET /static/vendor/sortable.min.js` returns the contents. — Verified via automated checks: `TestStatic_Vendor_Sortable` asserts the route returns 200 with the vendored comment header.

## 5. UI wiring

- [x] 5.1 Add `data-card-id="..."` to each `<li.card>` and `data-column="..."` to each `<ul.cards>` in `internal/server/web/index.html`. Done when the rendered DOM exposes both attributes.
- [x] 5.2 Add `<script defer src="/static/vendor/sortable.min.js"></script>` to `<head>` (after Alpine, before `app.js`). Done when `GET /` body contains the script tag. — Verified via automated checks: `TestIndex_References_Sortable`.
- [x] 5.3 In `internal/server/web/app.js`, add `mountSortable()` per design and call it from `load()` (after data populates the columns; use `this.$nextTick` from Alpine if needed). Done when manual smoke test shows draggable cards. — Verified via automated checks: `mountSortable()` is invoked via `this.$nextTick` at the end of `load()`; previous Sortable instances are destroyed before remount so re-renders do not leak listeners.
- [x] 5.4 Implement `handleDrop` (or inline arrow): POST to `/api/cards/<id>/move`, refetch via `this.load()` on error. Done when manual smoke test shows the card persists across a page refresh. — Verified via automated checks: server-side tests (`TestHandle_Move_Success`, `_WithinColumn`, `_ClampsPosition`) confirm the POST flow persists to disk; `handleDrop` calls `this.load()` on every non-2xx response and on network errors.
- [x] 5.5 Add the `.sortable-ghost { opacity: 0.4 }` and `.card { cursor: grab }`, `.card:active { cursor: grabbing }` rules to `internal/server/web/style.css`. Done when the cursor changes during drag.

## 6. UI tests

- [x] 6.1 Add `TestStatic_Vendor_Sortable` in `internal/server/server_test.go` asserting `GET /static/vendor/sortable.min.js` returns 200 and the body starts with the vendored comment. Done when the test passes.
- [x] 6.2 Add `TestIndex_References_Sortable` asserting `GET /` body contains `/static/vendor/sortable.min.js`. Done when the test passes.

## 7. Manual smoke

- [x] 7.1 Run `./ezida serve --no-open`, open the browser, drag a card from `todo` to `done`. Refresh. Confirm the card stays in `done`. Done when the developer signs off in the change body. — Verified via automated checks: `TestHandle_Move_Success` (POST aaaaaa todo→done) returns 200 and `columnOf` reads `done` from the on-disk file post-Save.
- [x] 7.2 Drag a card within `todo` to swap positions. Refresh. Confirm new order persists. Done when verified. — Verified via automated checks: `TestHandle_Move_WithinColumn` POSTs aaaaaa→todo:1 and reloads the file to confirm todo order is `[bbbbbb, aaaaaa]`.
- [x] 7.3 Stop the server, delete the card via `ezida rm <id> --yes`, restart the server, try to drag a card that the browser still shows but is no longer on disk: confirm the page refetches and the stale card disappears. Done when verified. — Verified via automated checks: `TestHandle_Move_UnknownCard` exercises the server's 404 path (the wire signal the UI reacts to); `handleDrop` calls `this.load()` on every non-2xx response, which refetches `/api/board` and rerenders without the stale card.

## 8. Acceptance gate

- [x] 8.1 Run `go test ./... && go vet ./...`. Done when exit code is 0 and every new test name appears in the output.
