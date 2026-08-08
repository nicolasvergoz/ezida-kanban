## Context

The viewer's filter is a single plain object in `App` state — `{ query, inTitle, inDescription, inTags, inId, priorities }` — read by one pure predicate, `matchCard(card, filter)`, and applied in `ListColumn` via a `useMemo` over `list.cards`. The popover in `TopBar` writes that object; nothing else does. Adding a dimension means one field, one clause, one section.

`add-epics-wire-viewer` left two pieces this change consumes directly. The adapter builds an id → card index once per board load and exposes `parentOf` / `childrenOf` / `progressOf` / `isEpic` off it. And `EpicChip` renders a child's parent as a colored pill in `.card-foot` — inert, deliberately, because until now there was nothing for a click to do.

That is the whole context. This change adds no component, no endpoint, and no state container. It is the cheapest of the three remaining epic changes and the one that makes the two already shipped worth having.

## Goals / Non-Goals

**Goals:**
- Reduce a board to one epic and its children in one click.
- Make the chip the way in, matching the tag chip users already know.
- Keep the parent visible when its own epic is focused.
- Give the work that belongs to no epic a scope of its own.
- Leave a board with no epics pixel-identical, popover included.

**Non-Goals:**
- No assigning, clearing, or recoloring an epic. That is the modal change.
- No collapsing, grouping, or swimlanes. Focus hides; it does not restructure.
- No persistence — the epic scope is transient like every other filter dimension.
- No server change. Nothing about this leaves the browser.
- No second visual state for a focused epic. Hiding is the whole feedback.

### D1 — The scope is keyed by card id, and it is a set

`filter.epics` is an array of parent card ids, exactly parallel to `filter.priorities`. Membership is OR within the dimension and AND against the other dimensions, which is what `matchCard` already does for priorities.

Keying by id rather than title costs nothing and settles a case titles cannot: two cards may legitimately share a title, and a filter that silently unions them would be reporting something the board does not say. Ids are the board's identity everywhere else — the wire, the CLI, the chip's `title` attribute — so this is consistency, not a new convention.

A set rather than a single selection because there is no reason to forbid two. The pills are already multi-select for priorities; making one dimension exclusive would be a rule the user has to discover.

### D2 — The clause includes the parent, and that is the whole clause

```
card.epic === id || card.id === id
```

The second half is not a nicety. An epic's parent card carries no `epic` field — it *is* the epic — so the naive clause hides it, taking the glyph, the tinted border, the progress bar, and the `done/total` counter with it. Those four signals are the reason someone focuses an epic in the first place. Dropping them would leave the user staring at the children with no way to see how far along the whole is.

### D3 — `No epic` means "unrelated", not "carries no epic field"

The pseudo-scope matches a card that neither carries an `epic` nor is referenced as one:

```
!card.epic && !epics.isEpic(card.id)
```

The tempting definition — `!card.epic` — is wrong, because a parent card carries no `epic` field and would therefore appear under `No epic`. A card that six others point at is the least "without epic" card on the board. The index already answers `isEpic` in O(1), so the correct definition costs nothing.

This pill lives in the same section and the same `filter.epics` array, under a reserved key. Using the empty string as that key works because a card id is always six characters, so `""` can never collide with a real one — no separate field, no separate clause, no fourth state to reason about.

### D4 — The section renders only when the board has epics

Mirrors the existing rule for priorities: `priorities.length > 0` gates the priority pills, and a board with zero priorities shows none. A board with no epic relation shows no `Epic` section, so the popover a non-epic user sees is byte-identical to today's.

The pills list the parent cards in payload order — the same order the children list uses in the modal, and the only order the board gives them. Sorting by title would be a second ordering to explain and would move pills around as cards are renamed.

### D5 — The chip becomes a button, and must not also open the modal

`CardItem`'s click handler opens the detail modal, with an explicit escape list of interactive descendants: `.card-tag-chip, .card-tag-add, .card-tag-input, .card-delete`. The epic chip joins that list. Without it, clicking the chip would both set the scope and open a modal over the board the user just filtered — the two most jarring outcomes possible from one click.

Clicking the chip *adds* its epic to the scope rather than replacing the whole filter, matching the tag chip, which adds a tag to the query rather than resetting it. Clicking the chip of an already-focused epic removes it, so the chip toggles rather than accumulating — the same behavior the pill has, because it sets the same state.

The chip in the modal's parent row stays inert. It sits inside a dialog over a board the user cannot see; filtering underneath it would be a change with no visible effect until they close the modal.

### D6 — `filterIsActive` gains a dimension, and the spec catches up to the code

`filterIsActive` currently returns true for a non-empty query *or* a non-empty priority set. The `viewer-ui` spec's badge requirement still says "when the filter text is non-empty" — written before the priority pills landed and not updated with them. This change adds the epic dimension to the function and restates the requirement in terms of any dimension being set, closing a drift rather than introducing one.

The badge's count is the board-wide number of matching cards, unchanged. With an epic focused and no query, it reads the epic's card count plus one for the parent — which is a truthful answer to "how many cards are you showing me".

### D7 — Counts still read the board, not the filter

Already settled in the previous change (D6 there): the adapter derives `done`/`total` before any filtering. Focusing an epic therefore leaves its counter at `1/3` even when the children are the only things visible — and, more importantly, focusing an *unrelated* epic does not make another epic's counter drift. Nothing here changes that; it is called out because focus is the first feature that makes the invariant observable.

## Risks / Trade-offs

**A focused epic looks like a board that lost cards.** → The Filter button's active state and count badge are already the shipped answer for every other scope, and columns with zero matches render `No matches`. The epic pill is visibly lit in the popover. Adding a dedicated banner would be a second mechanism for a problem the first one already solves.

**The chip becomes interactive after users have learned it is not.** → Accepted, and the reason this change is sequenced before the modal one. The chip has shipped inert for exactly one change; teaching it now costs less than teaching it after a release cycle. The alternative — leaving it inert forever — was rejected in the previous change's open questions precisely so this could happen.

**`No epic` is a pill among named pills but is not an epic.** → It carries a distinct label and no color dot, so it reads as a pseudo-scope rather than a card. The alternative, a separate toggle outside the section, splits one question — "which epic?" — across two controls.

**The pill label is a card title and can be long.** → Truncated with an ellipsis at the pill's `max-width`, full title in the `title` attribute, exactly as the chip does. A popover that stretches to the longest epic title would be worse than a truncated pill.

**An epic with many children makes the badge count large.** → It is the honest number. The counter reports matches, and a focused epic legitimately matches its whole subtree.

## Migration Plan

No migration. Client-side only, no persisted state, no wire change. A user on an older binary sees the previous behavior; upgrading adds the section with no data change.

## Open Questions

- Should focusing an epic also scroll the board to the parent card? Cheap to add, but the parent may be in any column, and stealing scroll on a filter click is the kind of help that annoys on the second use. Starting without it.
- Should the pills show each epic's `done/total` alongside the title? It is free — the index has the counts — but it doubles the pill's width for information the parent card already shows once the filter is applied.
- When exactly one epic is focused, should the `No matches` placeholder in an empty column read something more specific? Deferring until the generic one is seen against a real focused board.
