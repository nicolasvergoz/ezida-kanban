## Why

When reviewing an epic or an epic child card in the viewer detail modal, users cannot quickly jump between the parent epic and its child cards. Navigating requires closing the modal, locating the related card across columns or applying a filter, and opening it. Making related cards clickable in the modal enables fluid two-way navigation (epic → child and child → epic).

## What Changes

- **Epic detail modal (Children section)**: Make each child item in the children list clickable to open that child card's detail modal, without triggering the remove/detach action.
- **Card detail modal (Epic section)**: Make the parent epic chip clickable to open the parent epic card's detail modal.
- **Modal lifecycle**: Ensure clean state re-initialization when switching between cards within the modal (e.g. using `key={card.id}`).
- **Visual styling & accessibility**: Style child rows with pointer cursor and subtle hover state, separate keyboard focus for the child card row button and the remove button.

## Capabilities

### New Capabilities
<!-- None -->

### Modified Capabilities
- `viewer-ui`: Support clicking child cards and parent epic chips in the card detail modal to navigate directly to their respective card detail views.

## Impact

- `internal/server/web/app.jsx`: Pass `onOpenCard` to `CardDetailModal`, wrap child row contents in an accessible button, add `onClick` to parent `EpicChip` in modal, ensure clean component keying.
- `internal/server/web/styles.css`: Add styles for `.modal-epic-child-main` button and hover states.
- `e2e/epic-editing.spec.ts`: Add tests verifying navigation from epic to child and from child to epic within the modal.
