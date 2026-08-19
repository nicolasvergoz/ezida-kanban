## Context

Ezida's web viewer (`internal/server/web/app.jsx`) allows viewing and editing card details via `CardDetailModal`. An epic card renders a `Children` section listing its child cards and an overall progress bar; a child card renders an `Epic` section displaying its parent epic's colored chip and id.

Currently, clicking child rows in the `Children` section or clicking the parent epic chip in the `Epic` section does nothing. Users must close the modal and find the card on the board to view it.

## Goals / Non-Goals

**Goals:**
- Allow clicking any child in the epic's `Children` list to open that child card's detail modal.
- Allow clicking the parent epic chip in a child card's `Epic` section to open the parent epic card's detail modal.
- Maintain separate, reliable click handling for the remove (`✕`) button so detaching a child never opens its modal.
- Reset all internal modal states cleanly upon card navigation using `key={card.id}` on `<CardDetailModal>`.
- Maintain keyboard navigation (`Tab`, `Enter`) and semantic markup.
- Verify two-way modal navigation with Playwright E2E tests.

**Non-Goals:**
- Deep-linking card IDs to browser URLs or managing browser history stack (tracked separately in card `dlmeof`).
- Changing epic schema or backend endpoints.

## Decisions

### Decision 1: Split child item into sibling `.modal-epic-child-main` button and `.modal-epic-child-remove` button

Inside `<li className="modal-epic-child">`, wrap title and column metadata in a transparent `<button type="button" className="modal-epic-child-main">`, placed alongside `<button className="modal-epic-child-remove">`.

- *Rationale*: Avoids invalid nested button markup, provides semantic keyboard accessibility (`Tab` to card, `Enter` to open, `Tab` to remove button), and cleanly isolates event handlers.
- *Alternatives considered*: Making the entire `<li>` clickable with an `onClick` handler — rejected because nested button clicks require tricky event cancellation and have inferior accessibility semantics.

### Decision 2: Mount `<CardDetailModal key={card.id}>` and wire `onOpenCard`

Pass `onOpenCard={(id) => setOpenCardId(id)}` to `CardDetailModal` and add `key={card.id}` on the element rendered in `App`.

- *Rationale*: React's `key` prop forces a clean re-mount when `openCardId` changes. All draft text, editing flags, open dropdowns, and error messages reset automatically without needing manual synchronization effects.
- *Alternatives considered*: Resetting every state in `useEffect` — rejected as brittle and prone to state leaks across card transitions.

### Decision 3: Make the parent `EpicChip` in the modal clickable

In `CardDetailModal`, wire `onOpenCard(parent.id)` to the parent epic chip/row.

- *Rationale*: Completes two-way navigation (epic → child and child → epic).

## Risks / Trade-offs

- **[Risk]** Accidental navigation when trying to click the remove button (`✕`).
  → **Mitigation**: Sibling button layout ensures the click target for remove does not overlap the child navigation button. `e.stopPropagation()` added defensively on the remove button.
- **[Risk]** Long child titles breaking layout flexbox.
  → **Mitigation**: Add `min-width: 0` and text truncation styles on `.modal-epic-child-main` and `.modal-epic-child-title`.
