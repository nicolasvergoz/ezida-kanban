## 1. Go core placement

- [x] 1.1 Change `board.AppendCardToColumn` (`internal/board/board.go`)
      to insert at position `0` instead of `count`; rename it to
      reflect the new "top" semantics (e.g.
      `PrependCardToColumn`) and update all call sites.
- [x] 1.2 Update `internal/board/board_test.go` (or equivalent) for
      the renamed/re-behaved helper.

## 2. CLI: `move`

- [x] 2.1 Update `internal/commands/move.go:41` to use the new
      top-placement helper.
- [x] 2.2 Update `internal/commands/move_test.go` scenarios:
      "Move to an existing column" (card ends up before pre-existing
      cards), "Move to the same column" (re-places at top, not a
      position no-op).

## 3. CLI: `edit --column`

- [x] 3.1 Update `internal/commands/edit.go:195` to use the new
      top-placement helper.
- [x] 3.2 Update `internal/commands/edit_test.go` scenario "Edit
      changes column re-orders the card" for top placement, and add
      a case combining `--column` with another flag in the same
      invocation to confirm both apply.

## 4. CLI: `add`

- [x] 4.1 Update `internal/commands/add.go:120` to use the new
      top-placement helper.
- [x] 4.2 Update `internal/commands/add_test.go` scenario "Add
      appends at the bottom of the column" → top-of-column placement.

## 5. Viewer server: `POST /api/cards`

- [x] 5.1 Update `internal/server/handlers.go:282` to use the new
      top-placement helper.
- [x] 5.2 Update the corresponding handler test scenario ("Created
      card is appended to the end of its column" → placed at the
      top) in `internal/server/handlers_test.go` (or equivalent).

## 6. Viewer: column-body drop

- [x] 6.1 Update `List.onDrop` in `internal/server/web/app.jsx:879`
      to send `position: 0` instead of `list.cards.length`.
- [x] 6.2 Confirm `CardItem.onDrop` (card-to-card drop) is unchanged
      — no code edit expected there.

## 7. Browser tests

- [x] 7.1 No automated e2e coverage for the column-body drop itself:
      per CLAUDE.md, Playwright's mouse synthesis does not reliably
      drive native HTML5 `dataTransfer`, so cross-column card
      dragging is already excluded from e2e coverage project-wide.
      Manual confirmation in the simulator substitutes (see 8.2).
- [x] 7.2 (superseded by 7.1 — no empty-column drop e2e case either)
- [x] 7.3 Confirm existing card-to-card drop tests still pass
      unmodified (no code changed on that path; existing suite is
      the check).

## 8. Verify

- [x] 8.1 Run `./scripts/verify.sh` (gofmt, vet, go test, browser
      tests) and confirm green. gofmt/vet/e2e all clean;
      `TestRun_PortFallback` failed but is a pre-existing, unrelated
      environment flake (a stray `ezida serve` process already
      occupying port 7778 on the dev machine) — confirmed by the user
      and left running rather than killed.
- [x] 8.2 Manual confirmation done at the API level, not visually in
      a real browser: the Chrome extension needed for browser
      automation was not connected. Started a throwaway `ezida serve`
      on a copy of the board and issued the exact HTTP calls a real
      drop/add would send: `POST /api/cards` (new card landed at the
      top of `todo`) and `POST /api/cards/{id}/move` with
      `position: 0` (a card at the bottom of a 21-card `done` column
      moved to the top). CLI placement is already covered by the Go
      test suite. The literal JS change (`list.cards.length` → `0`)
      was not exercised through an actual browser drag — ask the user
      to confirm real dragging when convenient.
