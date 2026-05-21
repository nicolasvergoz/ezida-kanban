## Why

After V2 the viewer supports drag and reorder but the only way to
edit a card's title, description, tags, or priority is the CLI. V3
closes that loop: click a card → modal opens → edit fields → save.
The CLI's `ezida edit` already covers the same surface; the HTTP
endpoint reuses the same field-level semantics so both surfaces
stay in sync via `kanban.toml`.

The visual design of the modal stays minimal per the user's
"design later" instruction — structural form elements, no theming.

## What Changes

- `internal/board/board.go`:
  - Add `UpdateCard(b *Board, id string, patch CardPatch) error`
    that applies the patch in place (present fields replace,
    absent fields untouched per ADR 0002 §D8), refreshes
    `UpdatedAt`, and re-validates the board. Returns
    `*CardNotFoundError`, `*ColumnNotFoundError`,
    `*InvalidPriorityError`, `*MissingTitleError`, or
    `*InvalidTagError` on rule violations.
  - Add `CardPatch` struct with optional fields:
    `Title *string`, `Description *string`, `Tags *[]string`,
    `Priority *string`. Pointers distinguish "missing" from
    "set to empty/zero".
- `internal/server/handlers.go`:
  - Add `PATCH /api/cards/:id` accepting body whose keys are a
    subset of `{title, description, tags, priority}`. Builds a
    `CardPatch` from the request JSON (key presence drives the
    pointer), calls `board.UpdateCard`, saves, returns
    `{"card": {...}}`. Error mapping: unknown id → 404
    `CARD_NOT_FOUND`; empty title → 400 `MISSING_TITLE`;
    unknown priority → 400 `INVALID_PRIORITY`; bad tag → 400
    `INVALID_TAG`; malformed body → 400 `INVALID_BODY`.
- `internal/server/web/index.html`:
  - Add the modal Alpine template (overlay + dialog) with form
    inputs for title, description, tags, priority, plus read-only
    rows for id/column/created_at/updated_at.
- `internal/server/web/app.js`:
  - Augment `board()` Alpine component with modal state
    (`editing`, `draft`, `error`), `openCard(card)`,
    `closeCard()`, `saveCard()`, `addTag()`, `removeTag(tag)`.
  - Wire `@click` on each card to `openCard(card)`.
  - Wire `@keydown.escape` on the modal to `closeCard()`.
  - Wire `@keydown.meta.enter` and `@keydown.ctrl.enter` on the
    description textarea to `saveCard()`.
  - Wire `@keydown.enter` on the title input to `saveCard()`.
- `internal/server/web/style.css`:
  - Add the modal overlay/dialog styles (centered, dimmed
    background) and the chip-style tag input (input row + chip
    list, no decoration beyond a 1 px border).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `board-storage`: adds `UpdateCard` + `CardPatch` helpers.
- `viewer-server`: adds `PATCH /api/cards/:id` endpoint with
  partial-update semantics per ADR 0002 §D8.
- `viewer-ui`: adds the edit modal, tag chip input, priority
  dropdown, keyboard shortcuts, and the "click a card to edit"
  interaction.

## Impact

- New code in `internal/board/board.go` (~50 LOC for `UpdateCard`
  + `CardPatch`).
- New code in `internal/server/handlers.go` (~50 LOC for the PATCH
  handler + body decoding).
- New code in `internal/server/web/app.js` (~70 LOC for modal
  state + handlers).
- New HTML markup for the modal (~40 lines).
- New CSS rules for the modal/chips (~30 lines).
- No new vendored assets, no new Go dependencies.
- No new error codes (all reused from `internal/commands/errors.go`
  per ADR 0001 §D8).
- CLI behavior unchanged.
