## 1. Server: the endpoint accepts `done`

- [x] 1.1 Rename `columnRenamePayload` to `columnPatchPayload` in
  `internal/server/handlers.go` and change its fields to `Name *string`
  and `Done *bool` (design D1).
- [x] 1.2 In `handleColumnRename`, reject a body carrying neither key
  with `&InvalidBodyError{}`; keep the present-but-empty `name`
  rejection exactly as it is.
- [x] 1.3 Add the up-front membership check on `:name` before any
  mutation, returning `*board.ColumnNotFoundError` (design D3).
- [x] 1.4 Call `board.RenameColumn` only when `Name` is present, then
  apply `b.Board.SetDoneColumn(target, *p.Done)` on the post-rename name
  when `Done` is present — in that order (design D2).
- [x] 1.5 Emit the `renamed` echo only when a rename actually occurred;
  leave the rest of the response body untouched.
- [x] 1.6 Rename the handler to `handleColumnPatch` and update the mux
  registration, so the name stops claiming the route only renames.

## 2. Server tests

- [x] 2.1 Table-driven handler tests for the new bodies: `{done:true}`,
  `{done:false}`, `{name,done}` both directions, `{name}` alone
  preserving the marker, `{}`, and the already-held no-op.
- [x] 2.2 Assert the on-disk `[board].columns` after each, including the
  `*` suffix placement, and that `GET /api/board` reports the expected
  `done_columns`.
- [x] 2.3 Assert the tightened `PATCH /api/columns/ghost {"name":"ghost"}`
  now returns 400 `COLUMN_NOT_FOUND` with a byte-unchanged file.
- [x] 2.4 Confirm the existing `{"name":""}` and malformed-JSON tests
  still pass unchanged — the pointer must not weaken them.

## 3. Viewer: the staged check in the rename input

- [x] 3.1 Give `EditableText` two optional props: an `accessory` node
  rendered beside the input while editing, and a staged payload handed
  back to the commit callback (design D4).
- [x] 3.2 Change the commit rule: fire when the trimmed name is
  non-empty **and** the name or the staged terminal value differs from
  current; build the body with only the keys that changed.
- [x] 3.3 Reset the staged value on Escape and on a trimmed-empty
  commit; seed it from `list.done` when the editor opens.
- [x] 3.4 Render the check in `List`'s header as the accessory: handle
  `onMouseDown` with `preventDefault()`, expose an accessible label and
  a checked state, and make it a non-handle for the header's
  `draggable`.
- [x] 3.5 Point `renameList` at the new body shape (`{name?, done?}`)
  and keep its error surfacing intact.

## 4. Viewer: the `⋯` menu entry

- [x] 4.1 Add a terminal-column entry to `ListMenu` above `Delete list`,
  rendering the column's current state.
- [x] 4.2 Wire it to a single `PATCH /api/columns/:name` carrying only
  `{done: !list.done}`, then close the menu (design D6).
- [x] 4.3 Style both the menu entry and the accessory check in
  `styles.css`, reusing `IconCheck` and the existing muted-foreground
  tokens rather than introducing new ones.

## 5. Demo shim

- [x] 5.1 Track `done_columns` in the shim's state, defaulting to `[]`
  and seeding it from `board.json`.
- [x] 5.2 Make the `PATCH /api/columns/<old>` branch tolerate an absent
  `name`, apply `done`, carry the marker across a rename, and drop it on
  column delete (design D7).
- [x] 5.3 Reject a body with neither key the way the server does.

## 6. Browser tests

- [x] 6.1 New `e2e/terminal-column.spec.ts` with a fixture whose board
  has one terminal and one non-terminal column.
- [x] 6.2 Assert the menu path: exactly one `PATCH`, body `{done:true}`,
  no `name` key, menu closes, resting check mark appears after refetch.
- [x] 6.3 Assert the staged path: rename plus check produces **exactly
  one** request with both keys — the regression this change is most
  likely to grow.
- [x] 6.4 Assert that activating the check does not blur the input and
  does not fire a request on its own.
- [x] 6.5 Assert Escape and an emptied name each discard the staged
  value with no request.
- [x] 6.6 Assert pointer-down on the check does not start a column
  reorder.
- [x] 6.7 Confirm `plain.toml` still renders unchanged.

## 7. Verify

- [x] 7.1 `./scripts/verify.sh` green (gofmt, vet, go test, Playwright).
- [x] 7.2 `./scripts/verify.sh --visual` reviewed — the list header
  gained a control, so any baseline shift must be looked at, not
  accepted blind.
- [x] 7.3 Ask the user to confirm the two paths by hand in a real
  browser, including that column drag still works from the header.
