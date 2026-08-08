# Focus an epic from the filter popover

## Why

`add-epics-wire-viewer` made epics visible: a child wears a colored chip, a parent shows a progress bar. But there is no way to *act* on that. On a board of forty cards, seeing that six of them belong to "Card model extensions" does not help you work on those six — you still scroll past thirty-four others.

The chip is also currently inert, which is the odd part. Tag chips are clickable, and a user who has learned that will click the epic chip and get nothing. Every render of the chip today trains that expectation and disappoints it.

Everything this needs already exists. The filter popover has priority pills; the adapter already indexes every parent card. The epic scope is one more clause in `matchCard` and one more section in a popover built for exactly this.

**Depends on `add-epics-wire-viewer`.** The adapter's epic index and the chip are both from that change.

## What Changes

**A filter scope keyed by epic.** The popover gains an `Epic` section below `Priority`, built the same way: one pill per epic on the board, each with a colored dot, multi-select, cleared by `Clear all`. The pill's key is the parent card's id, not its title — two cards may share a title, ids are unique.

**The parent passes its own filter.** The clause reads `card.epic === id || card.id === id`. Without the second half, focusing an epic hides the epic — along with the progress bar and counter that are the reason to focus it.

**A `No epic` pseudo-scope.** One extra pill, same section, matching cards that neither carry an `epic` nor are referenced as one. A parent card is *not* "without epic" — it is the epic. This is the pill for the work that belongs to nothing.

**The chip becomes the way in.** Clicking a child's epic chip activates that epic's scope. This is what makes the chip worth being a chip rather than a label, and it mirrors the tag chip, which already adds itself to the filter on click. The chip must stop the click from also opening the card modal.

**The counter counts.** The Filter button's badge and its active state currently key off the query text in the spec, though the shipped code already includes priorities. This change states the rule the code follows: the button is active when *any* dimension of the filter is set.

**Nothing renders when nothing applies.** A board with no epic relations renders no `Epic` section, exactly as a board with no priorities renders no priority pills.

**Explicitly out of scope.** No assigning, clearing, or recoloring an epic — that is the modal change. No collapsing, no swimlanes, no grouping. No persistence: the epic scope is transient like every other filter dimension.

## Capabilities

### New Capabilities
<!-- None. Every change extends an existing viewer-ui capability. -->

### Modified Capabilities
- `viewer-ui`: the filter gains an epic dimension — a new popover section, a new clause in the match predicate, a `No epic` pseudo-scope, and a clickable chip that sets it. The adapter must expose the board's epics in payload order. The Filter button's active-state rule is restated to cover every filter dimension rather than the query text alone.

## Impact

**Code**
- `internal/server/web/app.jsx`: `DEFAULT_FILTER` (a new `epics` array), `filterIsActive`, `matchCard`, `toUiBoard`'s epic index (expose the parent list), `TopBar` (the new section), `EpicChip` and `CardItem` (click handling and the modal-open guard).
- `internal/server/web/styles.css`: the epic pill's colored dot; the existing `.filter-pill` / `.filter-pill-dot` rules carry the rest.

**No server change.** `GET /api/board` already returns every card with its `epic` and `color`; the filter is entirely client-side, like every other scope.

**Inherited for free**: `site/demo/app.jsx` and `site/demo/styles.css` are symlinks to the embedded assets, so the demo picks the scope up with no edit and no fixture change — `site/demo/board.json` already contains an epic.

**No breaking change.** A board with no epics renders an unchanged popover and an unchanged card footer.

**Docs**: `docs/usage.md`'s "What the Web UI lets you do" list gains the focus entry, and the CLI's `ezida list --epic` gets its viewer counterpart named next to it.
