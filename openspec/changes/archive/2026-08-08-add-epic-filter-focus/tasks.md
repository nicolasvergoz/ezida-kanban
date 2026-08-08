## 1. Adapter

- [x] 1.1 Expose the board's epics — the parent cards, in payload order — from the epic index built in `toUiBoard` in `internal/server/web/app.jsx`
- [x] 1.2 Derive that list from the full payload so an active filter never changes it
- [x] 1.3 Exclude a card that carries a `color` but is referenced by nobody

## 2. Filter state and predicate

- [x] 2.1 Add `epics: []` to `DEFAULT_FILTER`
- [x] 2.2 Make `filterIsActive` return true when the epic set is non-empty
- [x] 2.3 Add the epic clause to `matchCard`, matching `card.epic === id || card.id === id` so a focused epic keeps its parent visible
- [x] 2.4 Make the clause OR across the selected ids and AND against the query and priority dimensions
- [x] 2.5 Reserve the empty string as the `No epic` key and match it as "carries no `epic` and is not referenced as one"
- [x] 2.6 Leave the dimension inert when the set is empty

## 3. Popover section

- [x] 3.1 Render an `Epic` section below `Priority` in `TopBar`, one pill per epic plus a trailing `No epic` pill
- [x] 3.2 Fill each epic pill's dot from that epic's `color`, and give the `No epic` pill no dot
- [x] 3.3 Label each pill with the parent's title, truncated with an ellipsis, full title in the `title` attribute
- [x] 3.4 Toggle set membership on click and render `aria-pressed` from it
- [x] 3.5 List the pills in the adapter's epic order
- [x] 3.6 Render nothing — no heading, no pills — when the board has no epic relation
- [x] 3.7 Confirm `Clear all` resets the epic set with the rest of the filter
- [x] 3.8 Style the epic pill's dot in `styles.css`, reusing the existing `.filter-pill` / `.filter-pill-dot` rules for everything else

## 4. Clickable chip

- [x] 4.1 Make `EpicChip` toggle its epic in the filter set when rendered on a card
- [x] 4.2 Add `.card-epic-chip` to the escape list in `CardItem`'s click handler so the click does not also open the modal
- [x] 4.3 Keep the chip in the modal's parent row inert
- [x] 4.4 Give the card chip a pointer cursor and a hover state; leave the modal chip's cursor unchanged

## 5. Verification

- [x] 5.1 Confirm a focused epic shows its children and its parent, and hides everything else
- [x] 5.2 Confirm the parent's `done/total` still reads the whole board while focused
- [x] 5.3 Confirm `No epic` hides both parents and children, and shows only unrelated cards
- [x] 5.4 Confirm the Filter button goes active and the badge counts the parent alongside its children
- [x] 5.5 Confirm a board with no epics renders an unchanged popover and an unchanged card footer, by screenshot comparison
- [x] 5.6 Confirm a reload clears the epic set and writes nothing to `localStorage`
- [x] 5.7 Confirm drag, reorder, inline edit, and delete are all unaffected under an active epic focus

## 6. Docs

- [x] 6.1 Add the focus entry to the "What the Web UI lets you do" list in `docs/usage.md`, naming it next to the CLI's `ezida list --epic`
