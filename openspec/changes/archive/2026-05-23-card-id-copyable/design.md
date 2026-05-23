## Context

The web viewer renders cards inside columns (`internal/server/web/index.html` board template) and shows full card detail in a modal opened by clicking a card. The modal already displays the card ID via `<span class="modal-id t-mono-label">` (CSS rule at `style.css:716`, sized 11px, faint colour). Cards on the board only show title, optional priority, optional description-presence icon, and tags — no ID.

Clicking a card invokes `openCard(card)` on the Alpine root (`app.js:532`), which opens the modal. Cards are draggable via Sortable.js; the existing `.card-delete` button stops propagation so its click does not open the modal. The same propagation guard is required for the new card-level ID label so it can copy without also opening the modal or starting a drag.

## Goals / Non-Goals

**Goals:**
- Surface card ID on the board card itself in the same visual language as the modal ID, but visually subordinate to the card title.
- One-click copy of the ID to the clipboard from both the card and the modal.

**Non-Goals:**
- No new CLI behaviour, no API surface change, no schema change.
- No ID-based deep-linking via URL.
- No toast / notification UI for copy feedback (out of scope — keep change tight; click feels instant and the system clipboard is observable elsewhere).

## Decisions

**Where to render the card-level ID label.** Put it *above* the title inside `.card`, matching the modal layout (`.modal-id` sits in `.modal-header` above the title field). Reusing the position keeps the mental model consistent.

**CSS class.** New `.card-id` rule. Do NOT reuse `.modal-id` — the card variant needs a smaller font (subordinate to a 13.5px card title vs. the modal's larger title) and different vertical rhythm. Apply `.t-mono-label` utility for the typographic baseline, then override `font-size` to be smaller than `.modal-id`'s 11px. Use the existing `--text-faint` colour token so it blends in.

**Click handling.** Add `copyId(id, evt)` to the Alpine root component (same level as `openCard`, `deleteCard`). It:
1. Calls `evt.stopPropagation()` to prevent the card click from opening the modal.
2. Uses `navigator.clipboard.writeText(id)` when available (Promise-based, modern path).
3. Falls back to a hidden `<textarea>` + `document.execCommand('copy')` when `navigator.clipboard` is missing or rejects (covers non-secure-context loads, e.g. `http://192.168.*` over LAN).
4. Swallows errors silently — no user-facing toast in scope.

Alternative considered: rely on `clipboard.writeText` only. Rejected because the viewer is commonly opened via plain `http://` on LAN addresses, which can fall outside "secure context" definitions and disable `navigator.clipboard`. The fallback is ~10 lines and removes a real failure mode.

**Drag interference.** Sortable.js initiates drags on `mousedown`. `.card-id` does not need a separate guard beyond `stopPropagation` on `click` — drags are short-circuited if the click target is inside an element that calls `stopPropagation` only on its own click handler. If field testing shows a drag still kicks off when grabbing the ID, add `@mousedown.stop` to the ID span. Verify during implementation.

**Modal ID click.** Wire the same `copyId(openCardData.id, $event)` handler on the existing `.modal-id` span. No propagation concern there (the modal overlay closes only on `.self` clicks of `.modal-overlay`), but add `cursor: pointer` so users know it is interactive.

## Risks / Trade-offs

- [Clipboard API unavailable on insecure-context loads] → Fallback path using `document.execCommand('copy')` covers it.
- [Card layout shifts when ID added above title] → Minor visual change is the intent; verify spacing matches modal feel.
- [Drag start while clicking ID] → If observed, add `@mousedown.stop`. Documented as a verification step in tasks.

## Migration Plan

Pure additive UI change. No rollout gating, no data migration. Existing kanban data already carries IDs (schema unchanged).
