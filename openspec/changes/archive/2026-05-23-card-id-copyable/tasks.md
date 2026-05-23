## 1. CSS

- [x] 1.1 Add `.card-id` rule in `internal/server/web/style.css` (mono family via `'Geist Mono', ui-monospace, monospace`, `font-size` smaller than `.modal-id`'s 11px, faint colour via `var(--text-faint)`, `cursor: pointer`, small bottom margin so it sits above `.card-title`).
- [x] 1.2 Add `cursor: pointer` to the existing `.modal-id` rule in the same file.

## 2. Markup

- [x] 2.1 In `internal/server/web/index.html`, inside the `<li class="card" ...>` template, insert a `<span class="card-id t-mono-label" x-text="card.id" @click.stop="copyId(card.id, $event)"></span>` BEFORE the `.card-title` div.
- [x] 2.2 In the same file, wire the existing `.modal-id` span with `@click="copyId(openCardData.id, $event)"`.

## 3. Behaviour

- [x] 3.1 Add a `copyId(id, evt)` method to the Alpine root component in `internal/server/web/app.js` (near `openCard` / `deleteCard`). Implementation: `evt.stopPropagation()`; if `navigator.clipboard && navigator.clipboard.writeText`, call `navigator.clipboard.writeText(id).catch(() => fallback(id))`; otherwise call `fallback(id)`. Define `fallback(id)` inline (textarea + `document.execCommand('copy')`, then remove the textarea). Errors are swallowed.

## 4. Verification

- [x] 4.1 Run `go build ./...` to confirm the embedded FS still compiles cleanly.
- [x] 4.2 Headless verification: `go build` clean; grep confirms `.card-id` at style.css:592 with `font-size: 9px` (< modal's 11px); `.modal-id` rule has `cursor: pointer`; index.html:112 has card-level span with `@click.stop="copyId(card.id, $event)"` and `@mousedown.stop` guard; index.html:231 wires modal `@click="copyId(openCardData.id, $event)"`; `copyId` method present at app.js:533 with clipboard + execCommand fallback. Visual / real-clipboard verification deferred to the user (run the viewer, click an ID on a card, click the ID in the modal, paste).
