## Why

V3 closed the editing loop (click → modal → save), but card
**creation** and **deletion** still live exclusively in the CLI
(`ezida add`, `ezida rm`). The viewer is no longer "read-only with
edit-in-place" — it is the primary surface for most users — and the
asymmetry is jarring: a user can rename and re-prioritise a card with
two clicks but has to drop to a terminal to add a sibling.

Phase 4 of the UI redesign batch (ADR 0003 §D13) closes that gap with
two Redacto-native affordances:

- **Inline composer at the foot of each column** — the column footer
  ends with an "Add a card" ghost button. Clicking it swaps in a
  textarea + Add / Cancel buttons (ADR 0003 §D10). Submitting POSTs
  to a new `/api/cards` endpoint; the SSE pipeline (V5) refetches.
- **Hover-revealed × button on each card** — invisible until the
  card is hovered, tints to the danger colour on its own hover.
  Click DELETEs via a new `/api/cards/:id` endpoint with no
  confirmation dialog (ADR 0003 §D13 — the hover-only affordance
  *is* the friction; accidental card clicks open the modal, not
  delete).

The V3 click-to-edit modal is preserved untouched in this phase —
it is the right shape for the read-mostly detail surface (ADR 0003
§D3) and will be deepened in UI-5 (Trello-style click-to-edit
fields *inside* the modal). UI-4 is strictly the column-foot
composer + the hover delete.

After this phase the viewer reaches CLI parity for the
common-path verbs: read (V1), move (V2), edit (V3), create + delete
(UI-4). The remaining CLI-only surfaces (`ezida init`, column
operations) are intentionally out of scope — UI-6 covers columns.

## What Changes

- `internal/board/board.go` (or a new `internal/board/delete.go`):
  - Add `DeleteCard(b *Board, id string) error` returning
    `*CardNotFoundError` if no card matches `id`, otherwise
    removing the card from `b.Cards` in place. The helper does
    NOT persist — callers run `board.Save`. Mirrors the shape of
    the existing `MoveCard` / `UpdateCard` board-level primitives.
  - (Optional, design.md picks the cleaner shape) add
    `NewCardForColumn(b *Board, column, title, description, priority string, tags []string) (Card, error)`
    that runs the validation chain in the order
    `MISSING_TITLE → INVALID_TAG → COLUMN_NOT_FOUND → INVALID_PRIORITY`,
    generates a unique ID via `NewUniqueID`, sets
    `CreatedAt = UpdatedAt = time.Now().UTC().Truncate(time.Second)`,
    and returns the populated `Card` **without inserting**. The
    HTTP handler then calls `AppendCardToColumn` and `Save`. If
    the optional helper is skipped, the handler does the same work
    inline against existing primitives.

- `internal/server/handlers.go`:
  - Register `POST /api/cards` (no path parameter — the body
    carries the destination column). Body shape (snake_case per
    ADR 0002 §D7):
    ```json
    {
      "column":      "<name>",
      "title":       "<text>",
      "description": "<optional text>",
      "priority":    "<optional value>",
      "tags":        ["<optional>", "..."]
    }
    ```
    Validation order (matches `NewCardForColumn`): trim-empty
    title → `MISSING_TITLE` (400); any tag whose trimmed value is
    empty → `INVALID_TAG` (400); unknown column → `COLUMN_NOT_FOUND`
    (404 — see design.md for the deliberate departure from V2's
    `COLUMN_NOT_FOUND=400`); non-empty priority not in
    `[board].priorities` → `INVALID_PRIORITY` (400); malformed JSON
    → `INVALID_BODY` (400). On success: generate ID via
    `board.NewUniqueID`, append via `board.AppendCardToColumn`,
    persist via `board.Save`, respond `201 Created` with body
    `{"card": {...}}` using `cardToResponse`. `ErrIDExhausted` (the
    only pre-existing board error not otherwise mapped) surfaces as
    `IO_ERROR` 500 via the catch-all in `httpError`.
  - Register `DELETE /api/cards/{id}`. Empty body. Calls
    `board.DeleteCard`, then `board.Save`. Returns `200 OK` with
    `{"deleted": "<id>"}` on success (200-with-body for parity with
    other endpoints — easier client parsing than juggling 204).
    Unknown `id` → 404 `CARD_NOT_FOUND` via the existing `httpError`
    mapping; on-disk file is byte-unchanged.

- `internal/server/web/index.html`:
  - Each `.column` template's `<ul.cards>` is followed by a
    `<div class="column-footer">` Alpine sub-scope (`x-data` block
    keyed on the column name) hosting:
    - the idle "+ Add a card" button (Redacto `.button-ghost`,
      full-width, muted text);
    - the composer surface (textarea + "Add" primary + "Cancel"
      ghost + error line), rendered conditionally via Alpine
      `x-show`.
  - Each `<li.card>` gains an absolutely-positioned 22 px round
    `<button class="card-delete">` rendered top-right, hidden via
    CSS until the parent `.card` is hovered.

- `internal/server/web/app.js`:
  - Add the column-scoped composer state (see design.md state
    machine) — `composing`, `draft`, `error`, `submitting` —
    living on the per-column Alpine sub-component, not on
    `board()`. Methods: `openComposer()`, `cancelComposer(force)`,
    `submitComposer()`.
  - Add `deleteCard(id, evt)` on `board()`: stop propagation,
    skip-if-during-drag guard (see design.md), DELETE, refetch on
    404 to recover from external state drift. SSE handles the
    success refetch.

- `internal/server/web/style.css`:
  - Composer surface (`.composer`) with the Redacto accent border
    on focus (`refs/design.md` §"Composers"), and the active state
    of the "Add a card" button.
  - `.card-delete` hover affordance: 22 px circle, top-right
    absolute, hidden by default, opacity transitions to 1 on
    `.card:hover`, danger tint on its own `:hover` (uses the
    `--danger-*` token already minted in UI-1).

- `internal/server/server_test.go` and new
  `internal/server/handlers_create_delete_test.go` (or extended
  existing file): cover every error path on both endpoints plus
  the on-disk byte-unchanged guarantee where applicable.

- `internal/board/delete_test.go` (or extended `board_test.go`)
  covers `DeleteCard` happy + 404 paths.

References:
- ADR 0001 §D7 (JSON envelope), §D8 (error namespace).
- ADR 0002 §D7 (snake_case wire), §D9 (SSE refetch — UI-4 leans
  on it instead of mutating local state on POST/DELETE).
- ADR 0003 §D9 (reuse existing error codes — no new codes minted
  in this phase), §D10 (composer is Alpine sub-component on column
  scope), §D13 (no delete confirm).
- `refs/design.md` §"Card" (hover delete), §"Composers", §"List
  footer".

## Capabilities

### New Capabilities

(none — every change extends an existing capability)

### Modified Capabilities

- `viewer-server`: adds `POST /api/cards` (create) and
  `DELETE /api/cards/:id` (delete) with the validation /
  persistence semantics described above. No new error codes.
- `viewer-ui`: adds the column-foot inline composer (idle button,
  composing state, submit, cancel, blur-cancel, Esc-cancel,
  Enter-submit, Shift+Enter newline) and the hover-revealed card
  delete button (visibility rules, click handler, drag immunity,
  no confirmation).
- `board-storage`: adds `DeleteCard(b, id) error` returning
  `*CardNotFoundError` on miss; leaves the board unmutated on
  failure. Existing helpers (`NewUniqueID`, `AppendCardToColumn`)
  are reused by the create handler without change.

## Impact

- New code in `internal/board/` (~30 LOC for `DeleteCard`; +~50 LOC
  if `NewCardForColumn` is added; +tests).
- New code in `internal/server/handlers.go` (~100 LOC for the two
  handlers + their validation; route registration in `routes()`).
- New HTML markup in `index.html` (~30 lines: footer composer per
  column + delete button per card).
- New Alpine state on the existing per-column sub-template
  (`composing`, `draft`, `error`, `submitting`) plus three methods.
- New CSS (~30 lines) for the composer surface and hover delete
  affordance — both keyed to UI-1 tokens, no new colour literals.
- No new vendored assets, no new Go dependencies.
- No new error codes (all reused from ADR 0001 §D8 / ADR 0003 §D9).
- CLI behaviour unchanged.
- Modal markup, modal state, and modal CSS are NOT touched in this
  phase — UI-5 owns the modal redesign.
