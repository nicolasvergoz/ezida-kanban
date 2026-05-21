## 1. Board primitive — `DeleteCard`

- [x] 1.1 In `internal/board/` (new `delete.go` or appended to
  `board.go` — pick whichever keeps the file under ~300 LOC),
  declare `func DeleteCard(b *Board, id string) error`. Algorithm
  per design.md §D7: locate index, return `*CardNotFoundError` on
  miss, slice-splice on hit, no mutation on failure paths. Done
  when `go vet ./internal/board` is clean.
- [x] 1.2 Confirm `*CardNotFoundError` is already exported by
  `internal/board` (it is — defined in `board.go` next to
  `*ColumnNotFoundError`). Do NOT redeclare. Done when `grep -n
  "type CardNotFoundError" internal/board/` returns exactly one
  hit.

## 2. Board tests

- [x] 2.1 Add `TestDeleteCard_Success`,
  `TestDeleteCard_PreservesOrder`,
  `TestDeleteCard_UnknownIDReturnsNotFound`,
  `TestDeleteCard_DoesNotMutateOnMiss`,
  `TestDeleteCard_SingleCardBoardEndsEmpty`,
  `TestDeleteCard_DoesNotTouchBoardConfig` in
  `internal/board/delete_test.go` (or appended to
  `board_test.go`). Each test MUST exercise the scenario named in
  the spec delta. Done when `go test ./internal/board -run
  "TestDeleteCard"` exits 0 and every named test appears in the
  output.

## 3. Server endpoint — `DELETE /api/cards/{id}`

- [x] 3.1 Register `DELETE /api/cards/{id}` in
  `serverState.routes()` (in `internal/server/handlers.go`),
  alongside the existing `POST /api/cards/{id}/move` and `PATCH
  /api/cards/{id}` routes. Done when `curl -X DELETE
  http://127.0.0.1:<port>/api/cards/abcdef` reaches the handler
  (returns 404 or 200 depending on the fixture, NOT
  method-not-allowed).
- [x] 3.2 Implement `handleDelete(w, r)` per design.md §D7: read
  path param, `board.Load`, `board.DeleteCard`, `board.Save`,
  respond `200` with `{"deleted":"<id>"}`. Reuse the existing
  `httpError` mapping for `*CardNotFoundError` (already returns
  404 `CARD_NOT_FOUND`). Done when a unit test hitting the
  handler with an existing card observes status 200 and the
  expected JSON body.
- [x] 3.3 Add server tests:
  - `TestHandle_Delete_Success` (200, JSON body, on-disk card
    removed, surviving cards' order preserved).
  - `TestHandle_Delete_UnknownCard` (404 `CARD_NOT_FOUND`,
    `details.id` echoes back, on-disk file byte-unchanged).
  - `TestHandle_Delete_NonDeleteMethodRejected` (POST /api/cards/<id>
    without `/move` returns 405 or 404 per the v1 latitude).
  - `TestHandle_Delete_BroadcastsBoardChanged` (SSE client
    subscribed before the DELETE receives `board-changed` within
    500 ms of the response). Reuse the harness from V5's SSE
    tests.
  Done when `go test ./internal/server -run "TestHandle_Delete"`
  exits 0.

## 4. Server endpoint — `POST /api/cards`

- [x] 4.1 Register `POST /api/cards` in `serverState.routes()`.
  Use the stdlib mux method-prefix pattern (`mux.HandleFunc("POST
  /api/cards", s.handleCreate)`). Done when `curl -X POST
  http://127.0.0.1:<port>/api/cards -d '{}'` reaches the handler
  (returns 400, not 404).
- [x] 4.2 Implement `handleCreate(w, r)` per design.md §D7
  validation order:
  1. Decode body → on error,
     `writeErrorJSON(w, 400, "INVALID_BODY", err.Error(), nil)`.
  2. `strings.TrimSpace(title) == ""` → 400 `MISSING_TITLE`.
  3. Every `tags[i]` trim-non-empty → 400 `INVALID_TAG` with
     `details.tag`.
  4. `slices.Contains(b.Board.Columns, column)` → on false, 404
     `COLUMN_NOT_FOUND` with `details.column` (deliberate
     departure from V2's 400 — see design.md §D2).
  5. `priority != "" && !slices.Contains(b.Board.Priorities,
     priority)` → 400 `INVALID_PRIORITY` with `details.priority`.
  6. `id, err := board.NewUniqueID(existingIDs)` → on err, let
     `httpError` map (`ErrIDExhausted` → 500 `IO_ERROR` via the
     catch-all).
  7. Build `board.Card`, call `board.AppendCardToColumn`, then
     `board.Save`.
  8. Respond `201` with `Content-Type: application/json` and body
     `{"card": cardToResponse(card)}`.
  Done when each step is a separate, readable block in the
  handler source.
- [x] 4.3 Define the request struct `createCardPayload` in
  `handlers.go`:
  ```go
  type createCardPayload struct {
      Column      string   `json:"column"`
      Title       string   `json:"title"`
      Description string   `json:"description"`
      Priority    string   `json:"priority"`
      Tags        []string `json:"tags"`
  }
  ```
  Snake_case wire (ADR 0002 §D7). Note that `description`,
  `priority`, and `tags` default to their zero values when absent
  from the JSON — exactly the behaviour we want. Done when the
  struct compiles and the handler decodes a full-payload curl
  hit correctly.
- [x] 4.4 Add server tests covering every scenario in the
  viewer-server delta:
  - `TestHandle_Create_Success_TitleOnly` (201, well-formed
    `card` object, on-disk append, ID matches `^[0-9a-z]{6}$`).
  - `TestHandle_Create_Success_AllFields` (201, description /
    priority / tags echoed in response, on-disk persistence).
  - `TestHandle_Create_UnknownColumn` (404 `COLUMN_NOT_FOUND`,
    `details.column`, on-disk byte-unchanged).
  - `TestHandle_Create_EmptyTitle` (400 `MISSING_TITLE`,
    on-disk byte-unchanged).
  - `TestHandle_Create_MissingTitleKey` (400 `MISSING_TITLE`).
  - `TestHandle_Create_UnknownPriority` (400 `INVALID_PRIORITY`,
    `details.priority`).
  - `TestHandle_Create_EmptyTag` (400 `INVALID_TAG`).
  - `TestHandle_Create_MalformedBody` (400 `INVALID_BODY`).
  - `TestHandle_Create_AppendsToEndOfColumn` (start with 3 cards,
    create one, assert new card is 4th in column).
  - `TestHandle_Create_CreatedAtEqualsUpdatedAt` (timestamps
    match exactly).
  - `TestHandle_Create_BroadcastsBoardChanged` (SSE client gets
    `board-changed` within 500 ms).
  Done when `go test ./internal/server -run "TestHandle_Create"`
  exits 0 and every named test appears in the output.

## 5. UI — column footer + composer markup

- [x] 5.1 In `internal/server/web/index.html`, locate the existing
  `<template x-for="col in columns">` block. Inside the `<section
  class="column">`, after the existing `<ul class="cards">`, add
  the `<div class="column-footer">` Alpine sub-scope per design.md
  §D8. The sub-scope MUST declare `composing`, `draft`, `error`,
  `submitting` and the three methods `openComposer`,
  `cancelComposer`, `submitComposer`. Done when the rendered HTML
  (`curl http://127.0.0.1:<port>/`) contains the literal substring
  `class="column-footer"` once per column and the
  `submitComposer` body references `column: col` for the closure
  capture.
- [x] 5.2 Inside the composer form, the textarea MUST carry
  `@keydown.enter.prevent="submitComposer()"`,
  `@keydown.escape="cancelComposer()"`, and the blur handler
  `@blur="if (!$event.relatedTarget || !$event.relatedTarget.closest('.composer')) cancelComposer()"`.
  The Add button is `type="submit"`; Cancel is `type="button"`.
  Done when the page source contains all three handlers verbatim.
- [x] 5.3 The composer textarea MUST auto-focus when `composing`
  becomes true. Use `x-ref="composerInput"` plus the
  `x-init="$watch('composing', v => v && $nextTick(() =>
  $refs.composerInput.focus()))"` idiom. Done when a manual
  smoke-test (or a Playwright/headless probe) confirms the
  textarea receives focus on the same tick as the composer opens.

## 6. UI — hover delete button markup + handler

- [x] 6.1 In the `<li class="card">` template (inside
  `index.html`), add `<button type="button" class="card-delete"
  aria-label="Delete card"
  @click="deleteCard(card.id, $event)">×</button>` as the last
  child of the card body. Done when the rendered HTML contains
  one `.card-delete` button per card.
- [x] 6.2 In `internal/server/web/app.js`, extend the root
  `board()` component with:
  - a transient flag `_dragJustEnded: false`,
  - `async deleteCard(id, evt)` per design.md §D5 (stop
    propagation, skip-if-drag-just-ended, DELETE, refetch on
    404 or fetch reject).
  Done when both symbols exist in the returned object literal.
- [x] 6.3 Extend `mountSortable()`'s `Sortable.create` options to
  set `_dragJustEnded` inside `onEnd`:
  ```js
  onEnd: (evt) => {
      self._dragJustEnded = true;
      setTimeout(() => { self._dragJustEnded = false; }, 0);
      self.handleDrop(evt);
  },
  ```
  Done when a drag → release-over-delete-button manual smoke
  test (or an automated equivalent) does NOT fire a DELETE
  request.

## 7. CSS — composer surface + hover delete affordance

- [x] 7.1 In `internal/server/web/style.css`, add the
  `.column-footer`, `.composer-open`, `.composer`,
  `.composer textarea`, `.composer:focus-within`,
  `.composer-actions`, and `.composer-error` rules per design.md
  §D9. Every colour MUST reference a UI-1 token (`--surface`,
  `--border`, `--accent`, `--text`, `--text-muted`,
  `--danger-fg`). NO hex literals introduced. Done when a CSS
  audit (`grep -n '#[0-9a-fA-F]\{3,6\}' internal/server/web/style.css`)
  yields no new hex values beyond the existing token block.
- [x] 7.2 Add the `.card { position: relative }` + `.card-delete`
  rules per design.md §D9: 22 px round, top-right absolute,
  hidden by default, revealed on `.card:hover`, danger-tinted on
  `.card-delete:hover`. Done when the rendered page's `.card`
  elements show no visible × until hovered (manual or headless
  visual check).

## 8. Server tests — error envelope coverage

- [x] 8.1 Confirm `httpError` still maps every error code the new
  handlers can raise (`MISSING_TITLE`, `INVALID_TAG`,
  `INVALID_PRIORITY`, `CARD_NOT_FOUND`, `COLUMN_NOT_FOUND`,
  `INVALID_BODY`, `IO_ERROR`). NO new code branches required —
  the V3 handler already exercised every reusable mapping. Done
  when a code review of `httpError` confirms no missing typed-error
  cases.
- [x] 8.2 Add `TestHandle_Create_UnknownColumn_Returns404` as a
  dedicated guard for the deliberate 404 departure (design.md §D2).
  Verifies that the create handler emits the 404 status even
  though the existing `httpError` returns 400 for the same typed
  error on the move endpoint. Done when the test passes against
  the create handler and would fail if the handler accidentally
  delegated to `httpError`.

## 9. UI tests (smoke / DOM substring)

- [x] 9.1 Extend `internal/server/server_test.go` to assert that
  `GET /` returns HTML containing `class="column-footer"` (proves
  the footer markup ships) and `class="card-delete"` (proves the
  delete button markup ships). Done when the test passes.
- [x] 9.2 Optional (skip if local headless infra is absent): a
  Playwright or equivalent smoke that opens the composer in
  `todo`, types `Smoke check`, presses Enter, and asserts the
  card appears within 1 s. Done OR explicitly noted skipped with
  rationale.

## 10. Manual smoke

- [x] 10.1 Open the page → click `+ Add a card` in `todo` → type
  `Smoke check` → press Enter → the composer closes and the new
  card appears in the column. Done when verified.
- [x] 10.2 Open the composer → type `Junk` → press Escape → the
  composer closes, no request fires, no card appears. Done when
  verified.
- [x] 10.3 Open the composer → type `Junk` → click outside the
  composer → composer closes, no request fires. Done when
  verified.
- [x] 10.4 Hover a card → the × button fades in → click it → the
  card is removed without a confirmation dialog. The modal does
  NOT open. Done when verified.
- [x] 10.5 Drag a card and release the mouse with the pointer
  over the × button → the card MUST be moved (or returned to its
  original column), NOT deleted. Done when verified.
- [x] 10.6 With the CLI in another terminal, `ezida rm <id>` while
  the page is open, then click the × on the (now stale) card →
  the client gets a 404, triggers a refetch, and the card
  disappears from the rendered board. Done when verified.

## 11. Acceptance gate

- [x] 11.1 Run `go test ./... && go vet ./...`. Done when exit
  code is 0 and every new test name (`TestDeleteCard_*`,
  `TestHandle_Create_*`, `TestHandle_Delete_*`) appears in the
  output.
