## Context

`add-epics-schema-cli` establishes the model: `Card.Epic` names another card, `Card.Color` holds a hex, `BoardConfig` knows which columns are terminal, and the CLI can read and write all of it. The viewer knows none of it.

The viewer is a single-file React app transpiled in the browser by Babel-standalone, with one adapter function — `toUiBoard` — sitting between the server envelope and the component tree. Every mutation goes back out through the same REST endpoints the CLI's write path shares. There is no client-side state store; the board is refetched on every change and on every SSE `board-changed` event.

That architecture is what makes this change small. The board payload is already complete — `GET /api/board` returns every card with every field on every request — so anything derivable from the board is derivable in the adapter.

This change is read-only by design. It is the shortest path to a viewer where an epic is visible, and it deliberately stops before the parts that need a new interactive component.

## Goals / Non-Goals

**Goals:**
- Two new wire fields, and nothing else on the wire.
- `ezida export` and `GET /api/board` back to identical shape.
- A child card shows which epic it belongs to; a parent shows that it is one and how far along it is.
- Terminal columns are visible, so the progress counter is interpretable.
- The modal reports the relation in full detail, read-only.
- A board with no epics renders byte-identically to today.

**Non-Goals:**
- No creating, reassigning, or clearing an epic from the viewer.
- No color picker.
- No epic scope in the filter popover.
- No swimlanes, no grouping, no collapsing.
- No change to drag, drop, reorder, or any mutation endpoint.

## Decisions

### D1 — The wire carries `epic` and `color`, and nothing derived

`cardResponse` gains exactly two `omitempty` string fields. `boardResponse` gains `done_columns`. The response does not carry `epic_title`, `epic_color`, `children`, or `progress`.

This is the opposite of the choice made for `ezida get --json` in the previous change, and the difference is not inconsistency. `ezida get` returns a single card with no board around it, so resolving the parent's title server-side is the only way the consumer can have it. `GET /api/board` returns the entire board on every request; a client that cannot find card `rl4m9x` in an array it already holds has a bug, not a missing field.

Denormalizing would cost on three axes: payload size grows with every relation, the same title exists in two places in one document, and every mutation endpoint acquires an obligation to keep the copies correct. None of that buys anything the adapter cannot do in one pass.

### D2 — The adapter is the only place that resolves relations

`toUiBoard` builds an `id → card` Map once per board load and derives, from it, each card's parent, each parent's children, and each parent's done/total counts. Components read those values.

The alternative — each card component scanning `board.cards` for its parent — is O(n) per card, so O(n²) per render on a board where most cards belong to an epic. On tens of cards that is invisible, but it also scatters the resolution logic across three components and makes the dangling-reference case something each of them has to handle. One pass in the adapter is both faster and the only place that needs to know a reference can dangle.

This extends an existing requirement rather than adding one: `viewer-ui` already names the adapter as the single wire↔UI boundary.

### D3 — The chip lives in `.card-foot`, first, and the color does the sorting

Three placements were considered:

- **On the id line, right-aligned.** Costs zero vertical space because that row is mostly empty today. Rejected: it truncates at roughly 18 characters, and it splits the card into two metadata zones for one property.
- **On its own line between title and footer.** Never truncates, never reflows. Rejected: 18px on every child card, for a row holding one element.
- **First element of `.card-foot`, after the priority pill.** Chosen.

The chosen placement needs no new structure — `.card-foot` is already a `flex-wrap` row — and needs no positioning rule for the priority pill, which simply stays where it is. The cost is that the chip is worth roughly two tags, so a card with three or more tags wraps to a second line. That is the existing behavior of that row with many tags; card height was already variable, so nothing new has to be handled.

Distinguishing the chip from a tag is done entirely with color: epic chips are colored, tags are neutral. That means both can stay pill-shaped, and no new shape vocabulary enters the card. The priority pill keeps its own distinct form — a 4px vertical bar, never a pill — so the three signals remain separable.

### D4 — One stored hex, both themes, via `color-mix` toward `--text`

The chip computes its background, border, and text color as mixes of the stored hex with the theme's current `--text`. On the cream ground `--text` is near-black, so the mix darkens; on the navy ground it is near-white, so the mix lightens. The same stored value stays legible in both directions with no theme-specific data in `kanban.toml`.

This is not a new technique in this codebase — `.card-tag-chip` already varies its mix ratios per theme through `[data-theme="light"]` overrides. The chip follows the same pattern with different ratios.

### D5 — Parent signals are conditional on having children, not on having a color

The glyph, the tinted border, and the progress bar all key off "is referenced by at least one card", not off "carries a `color`". A card can legitimately carry a color and no children — after its last child is reassigned, for instance — and it must render as an ordinary card.

This keeps the invariant the whole feature depends on: a board with no epic relations is pixel-identical to today's. Every added signal is conditional, so average card density is unchanged for anyone not using the feature.

### D6 — Progress reflects the board, not the filter

When a filter hides an epic's children but leaves the parent visible, the counter still reads the full board. A counter that changed with the filter would be reporting the filter, not the chantier, and `2/4` becoming `1/1` on an unrelated tag search is worse than useless.

This falls out of D2 for free: the adapter derives counts from the full payload before any filtering is applied.

### D7 — The terminal-column marker ships in this change, not with the filter

It would be defensible to defer the check mark to a later change, since nothing breaks without it. It ships here because the progress counter is uninterpretable without it: `1/3` is meaningless if the user cannot see which column feeds the numerator. Shipping the number and the legend together is what makes the feature legible on arrival.

It reuses `IconCheck`, already present in `app.jsx`, and a muted foreground token, so it introduces no new asset and no new color.

### D8 — The modal sections are built read-only, then grown

The modal renders the parent row or the children list with no controls. The next change adds the card picker and the color swatches into the same containers.

Building the presentational layer first means the layout, the truncation behavior, and the empty states are settled before the interactive component — the card-search combobox, which is the single largest piece of new UI in the whole feature — is written against them.

## Risks / Trade-offs

**The chip pushes tags to a second line on tag-heavy cards.** → Accepted. `.card-foot` already wraps and card height was already variable, so no new layout case appears. Truncation at a fixed `max-width` bounds the worst case to one wrap rather than several.

**Color alone carries the epic's identity in the compact card.** → A user who cannot distinguish two palette colors reads the chip's text, which is always present, and gets the full name from the `title` attribute and the modal. The palette is capped at eight and ordered by chromatic distance precisely so adjacent assignments stay separable.

**A dangling `epic` reference can reach the client.** → Validation forbids it on disk, but a board edited between the fetch and the render, or a hand-corrupted file the server has not yet reloaded, could produce one. The adapter resolves through a Map lookup that returns undefined, and the card renders no chip. Specified as a scenario so it is tested rather than discovered.

**`site/demo/board.json` will show no epics until regenerated.** → The demo inherits the rendering automatically through the asset symlinks, but its fixture is a snapshot. Regenerating it with `ezida export` is a task here; forgetting it means the public demo silently fails to show the new feature.

**The viewer's own test suite asserts on the envelope shape.** → Both new card fields are `omitempty`, so a fixture without epics produces an identical payload and existing assertions hold. `done_columns` is genuinely new and always present, so assertions that enumerate top-level keys need updating — expected to be a small, mechanical set in `server_test.go`.

**This change is unbuildable before `add-epics-schema-cli` lands.** → Stated in the proposal. `board.Card` has no `Epic` field to read until then.

## Migration Plan

No migration. Every field added is `omitempty` or an always-present empty array. A user upgrading from a build with change 1 but not change 2 sees the CLI understand epics while the viewer ignores them; upgrading past this change adds rendering with no data change and no restart requirement beyond the usual binary swap.

## Open Questions

- Should the chip be clickable in this change? Tags already are — clicking one adds it to the filter. Making the chip clickable would be consistent, but the epic filter scope does not exist until the focus change, so a click would have nothing to do. Leaving it inert now means introducing an affordance later on an element users have already learned is not interactive.
- Should the progress bar appear in the compact card at all, or only in the modal? The bar plus counter costs a row on every parent card. Parents are a small minority of cards, so the cost is low — but it is worth confirming against a real board before committing.
- Does the children list in the modal need each child's own priority and tags, or is title-plus-column enough? Starting minimal; the list is easy to enrich once it is in front of a real board.
