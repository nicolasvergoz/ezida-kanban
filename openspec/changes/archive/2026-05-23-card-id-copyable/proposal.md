## Why

Card IDs are needed to drive `ezida` CLI commands (`ezida get <id>`, `ezida move <id>`) and to reference specific cards when working with AI assistants. Today the ID is only visible after opening the detail modal, which is friction for what is otherwise a one-glance lookup. Surfacing the ID on the card itself — and making it copyable everywhere it appears — removes that friction.

## What Changes

- Render each card's ID as a small grey monospace label above the card title in the web viewer.
- Card-level ID label uses a smaller font size than the modal-header ID so it stays visually subordinate to the card title.
- Clicking the ID (on the card AND in the detail modal) copies it to the clipboard. Click on the card-level ID must not also open the modal — propagation stops at the ID.
- Card-level ID click does not trigger a Sortable drag.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `viewer-ui`: card rendering grows a visible monospace ID above the title; both card-level and modal ID labels gain click-to-copy-to-clipboard behaviour.

## Impact

- `internal/server/web/index.html`: add `.card-id` span inside the `.card` template; bind `@click` on both `.card-id` and existing `.modal-id`.
- `internal/server/web/app.js`: add `copyId(id, evt)` method on the Alpine root component (uses `navigator.clipboard.writeText`, stops propagation, falls back to `document.execCommand('copy')` when the Clipboard API is unavailable, e.g. non-secure-context).
- `internal/server/web/style.css`: add `.card-id` rule (mono font, smaller than `.modal-id`'s 11px, faint colour, `cursor: pointer`); ensure `.modal-id` also gets `cursor: pointer`.
- No CLI changes. No server changes. No schema changes.
