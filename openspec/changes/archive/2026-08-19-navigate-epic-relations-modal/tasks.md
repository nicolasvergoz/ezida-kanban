## 1. Component & Markup Updates

- [x] 1.1 Pass `onOpenCard` prop to `CardDetailModal` and add `key={card.id}` to the modal element in `App` (`internal/server/web/app.jsx`)
- [x] 1.2 Update the `Children` list items in `CardDetailModal` to wrap the child title and column in a `.modal-epic-child-main` button wired to `onOpenCard(c.id)`
- [x] 1.3 Update the `Epic` section in `CardDetailModal` to make the parent `EpicChip` clickable and wired to `onOpenCard(parent.id)`

## 2. Styling & Accessibility

- [x] 2.1 Add styles in `internal/server/web/styles.css` for `.modal-epic-child-main` (button reset, flex layout, hover highlight, truncation)
- [x] 2.2 Verify focus visibility and keyboard interaction (`Tab`, `Enter`) for both child navigation and remove actions

## 3. Verification & Tests

- [x] 3.1 Add Playwright E2E tests in `e2e/epic-editing.spec.ts` asserting that clicking a child card in the epic detail modal opens that child's modal
- [x] 3.2 Add Playwright E2E tests in `e2e/epic-editing.spec.ts` asserting that clicking the parent epic chip in the child detail modal opens the parent epic modal
- [x] 3.3 Run `rtk playwright test` and `rtk go test ./...` to verify full test suite passes
