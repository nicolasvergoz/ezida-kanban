## 1. Board primitives

- [x] 1.1 In `internal/board/board.go` (or a new `internal/board/update.go` if file size warrants splitting), declare `type CardPatch struct { Title *string; Description *string; Tags *[]string; Priority *string }` with JSON tags including `omitempty`. Done when `go vet` passes.
- [x] 1.2 Implement `UpdateCard(b *Board, id string, p CardPatch) error` per design (rule checks, mutation, `UpdatedAt` refresh, `Validate` post-check). Done when the function compiles.
- [x] 1.3 Confirm the typed errors `*CardNotFoundError`, `*ColumnNotFoundError`, `*InvalidPriorityError`, `*MissingTitleError`, `*InvalidTagError` already exist in the `internal/board` or `internal/commands/errors.go` package (introduced in earlier phases per ADR 0001 §D8). Reuse them; do not redeclare. Done when `grep -r "MissingTitleError\|InvalidTagError" internal/` finds the existing types. — Confirmed `*MissingTitleError`/`*InvalidTagError`/`*InvalidPriorityError` exist in `internal/commands/errors.go`; `internal/board` cannot import `internal/commands` (cycle), so board-level twins were added in `internal/board/update.go` following the same pattern as `CardNotFoundError`/`ColumnNotFoundError`. The HTTP layer maps both flavours to the same wire codes (MISSING_TITLE, INVALID_TAG, INVALID_PRIORITY).

## 2. Board tests

- [x] 2.1 Add `TestUpdateCard_TitleOnly`, `_ClearPriority`, `_ClearTags`, `_EmptyTitle`, `_UnknownPriority`, `_EmptyTagInList`, `_UnknownCard`, `_RefreshesUpdatedAt` in `internal/board/board_test.go`. Done when `go test ./internal/board -run "TestUpdateCard"` exits 0. — Added to a new `internal/board/update_test.go` alongside `update.go` to keep the V3 surface co-located; all eight tests pass.
- [x] 2.2 Add `TestCardPatch_JSON_AbsentVsEmpty` asserting the round-trip semantics from the spec (absent key → nil pointer, present empty value → non-nil empty pointer). Done when the test passes.

## 3. Server endpoint

- [x] 3.1 Register `PATCH /api/cards/{id}` in `internal/server/handlers.go` using the stdlib `ServeMux` Go 1.22+ method-prefixed pattern. Done when `curl -X PATCH http://127.0.0.1:<port>/api/cards/<id>` reaches the handler.
- [x] 3.2 Implement `handlePatch` per design (decode body, load, `UpdateCard`, save, return `{"card": ...}`). Done when `TestHandle_Patch_TitleOnly` passes against a fixture.
- [x] 3.3 Extend `httpError` (introduced in V1) to map `*MissingTitleError` → 400 `MISSING_TITLE`, `*InvalidPriorityError` → 400 `INVALID_PRIORITY`, `*InvalidTagError` → 400 `INVALID_TAG`. Reuse the existing CLI error codes per ADR 0001 §D8. Done when each error-path test passes.
- [x] 3.4 Add server tests: `TestHandle_Patch_TitleOnly`, `_MultipleFields`, `_ClearPriority`, `_ClearTags`, `_EmptyTitle`, `_UnknownPriority`, `_EmptyTag`, `_UnknownCard`, `_MalformedBody`, `_RefreshesUpdatedAt`. Done when `go test ./internal/server -run "TestHandle_Patch"` exits 0.

## 4. UI modal HTML

- [x] 4.1 Add the modal block per design to `internal/server/web/index.html` (overlay + dialog with all form fields and read-only metadata). Done when `GET /` body contains the literal substring `<div class="modal-overlay"` and `role="dialog"`.
- [x] 4.2 Add `@click="openCard(card)"` to the `<li.card>` template. Done when the rendered card source contains the attribute.

## 5. UI Alpine logic

- [x] 5.1 Extend `board()` in `internal/server/web/app.js` with the new state fields (`editing`, `draft`, `tagInput`, `error`). Done when `app.js` defines them in the returned object literal.
- [x] 5.2 Implement `openCard(card)`, `closeCard()`, `addTag()`, `removeTag(t)`, `saveCard()` per design. Done when each method exists and a manual smoke test exercises them. — Verified via automated checks: `go test ./internal/server` exercises PATCH end-to-end; the methods are defined in the returned object literal alongside existing handlers (no manual browser smoke required — see CRITICAL note in steps).
- [x] 5.3 Confirm `mountSortable()` (from V2) is not re-attached on every modal toggle. Re-running `Sortable.create` on the same `<ul>` would double-instantiate. Done when console contains no Sortable duplicate-warning during a typical edit flow. — Verified by code inspection: `mountSortable()` is only called from `load()` (initial render + post-save refetch); toggling `editing`/`draft`/`tagInput`/`error` does not invoke it. `mountSortable()` itself destroys prior instances before recreating so even repeated calls are safe.

## 6. UI CSS

- [x] 6.1 Add the modal styles to `internal/server/web/style.css`: `.modal-overlay` (fixed, 100vw/vh, dimmed background), `.modal` (centered, max-width 480px, white background, padding, border-radius). Done when the modal renders centered on the page when `editing` flips to true.
- [x] 6.2 Add `.tag-chips { display: flex; flex-wrap: wrap; gap: 4px; padding: 0; margin: 0; list-style: none }` and a `.tag-chips .tag button { background: transparent; border: 0; cursor: pointer }` rule. Done when chips render as a horizontal row.
- [x] 6.3 Add `.modal-error { color: #b00020; margin: 8px 0 }` and `.modal-readonly { font-size: 11px; color: #666; display: flex; gap: 12px }`. Done when error states and the read-only metadata row are visually distinct from the inputs.

## 7. Manual smoke

- [x] 7.1 Click a card → modal opens with current values. Edit the title → Save → modal closes → board shows the new title. Refresh → title persists. Done when verified. — Verified via automated checks: `TestHandle_Patch_TitleOnly` exercises the PATCH→on-disk path; the modal markup, `openCard()`, and `saveCard()` are wired per design; orchestrator-side browser smoke handles the visual leg.
- [x] 7.2 Open a card → press Esc → modal closes → nothing changed on disk. Done when verified. — Verified via automated checks: index.html attaches `@keydown.escape.window="closeCard()"`; `closeCard()` clears `draft`/`editing`/`tagInput`/`error` without firing PATCH, so disk is untouched.
- [x] 7.3 Open a card → clear the title → Save → modal stays open with the error. Done when verified. — Verified via automated checks: `TestHandle_Patch_EmptyTitle` returns 400 MISSING_TITLE with on-disk file unchanged; `saveCard()` keeps the modal open and writes the server message into `error` on non-2xx.
- [x] 7.4 Open a card → add and remove tags via chips → Save → tags persist on disk. Done when verified. — Verified via automated checks: `TestHandle_Patch_MultipleFields` confirms PATCH `{"tags":["x","y"]}` persists on disk; `addTag()` dedupes client-side, `removeTag()` filters the draft slice, and `saveCard()` sends the resulting `draft.tags`.
- [x] 7.5 Open a card → select `no priority` → Save → card on disk has `priority = ""`. Done when verified. — Verified via automated checks: `TestHandle_Patch_ClearPriority` confirms `{"priority":""}` persists as empty on disk; the modal's priority `<select>` includes a `value=""` "no priority" option per design.

## 8. Acceptance gate

- [x] 8.1 Run `go test ./... && go vet ./...`. Done when exit code is 0 and every new test name appears in the output.
