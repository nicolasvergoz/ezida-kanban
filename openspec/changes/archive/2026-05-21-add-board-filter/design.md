## Context

Phase 3 of the UI redesign batch (ADR 0003) adds a client-side board
filter to the viewer. The viewer is a single Alpine component
(`board()` in `internal/server/web/app.js`) rendered from the embedded
`index.html`. The component already holds `cards: []` (a flat array
of every card on the board) and a `cardsByColumn(name)` accessor that
filters by column. Phase 3 layers a second filter axis — a free-text
query — on top, and exposes it through a small popover anchored to a
new topbar button.

The visual specification lives in `refs/design.md` §"Filter Popover"
and §"TopBar". The behavioral envelope is fixed by ADR 0003 §D2
(Alpine + vanilla CSS, no build step), §D5 (token system — popover
uses `--surface`, `--rounded-xl`), and §D8 (filter is transient, not
persisted).

Relevant existing state on `board()`:

- `cards: []` — full board, never mutated by filter
- `cardsByColumn(name)` — current column accessor used by templates

Relevant existing behaviors that must coexist:

- **Card click → modal (V3).** Visible cards still open the modal on
  click. Hidden cards are `display:none` so clicks are physically
  impossible.
- **Sortable drag-drop (V2).** Cards can be dragged across/within
  columns. The filter is cosmetic; the underlying `cards` array is
  not touched, so Sortable continues to operate on visible cards
  without confusion. (Hidden cards are not draggable because they
  are not rendered with pointer surface.)
- **SSE refetch (V4).** External changes refetch `/api/board` and
  rebuild `cards`. Filter state survives the refetch (it lives on
  the component, not on the board response).

## Goals / Non-Goals

**Goals:**

- Add a topbar Filter button + 280px popover that opens on click,
  closes on Esc or outside click, and never persists state.
- Apply a case-insensitive substring match against each card's
  `title` + `description` + `tags` on every keystroke.
- Hide non-matching cards via display-only rendering — keep the
  `cards` array intact so V2 (drag) and V3 (modal) still operate
  correctly on visible cards.
- Render a `No matches` placeholder inside columns whose visible
  count is zero while the filter is active.
- Show a mono-counter badge on the Filter button with the board-wide
  match count while the filter is active.
- Preserve the column `list-count` semantics: it shows the **total**
  card count, not the filtered count (per `refs/design.md` §"List"
  §"list-count").

**Non-Goals:**

- Server-side filtering or any new endpoint. `/api/board` is
  unchanged.
- Persistence of filter text (ADR 0003 §D8). No `localStorage`, no
  cookie, no URL query param.
- Advanced query syntax: no boolean operators, no field-scoped
  searches (`tag:foo`), no regex. Substring on a flattened "haystack"
  string only.
- Per-column filtering or per-tag toggle UI. The popover is text-only.
- Highlighting the match inside the card body. Out of scope; design.md
  does not call for it.
- Keyboard global shortcut to open the popover (e.g. `Cmd+F`). Out of
  scope for this phase — the topbar button is the only entry.

## Decisions

### D1. State lives on `board()` as `filter` + `filterOpen`

The component gains two new keys:

- `filter: ''` — the current substring (raw, never normalized
  on-disk; lowercased per-match below).
- `filterOpen: false` — whether the popover is rendered.

Both default to falsy on component init and after a reload.
`filter` is never written to storage; ADR 0003 §D8 forbids it.

**Alternatives considered:**

- Hoist filter into a global Alpine store: rejected — only `board()`
  consumes it; a store adds indirection with no second consumer.
- Encode filter in the URL hash for shareability: rejected — ADR
  0003 §D8 explicitly chose "filter is a query, not a preference";
  shareable URLs imply preference semantics.

**Affects:** `app.js`, all column templates in `index.html`.

### D2. Match helper — single lowercased haystack per card

`filterMatches(card)` returns `true` when `filter` is empty, otherwise
checks `lowercaseQuery` against a per-call concatenation of
`title + " " + description + " " + tags.join(" ")`, all lowercased.
Implementation:

```js
filterMatches(card) {
  const q = this.filter.trim().toLowerCase();
  if (!q) return true;
  const hay = (
    (card.title || '') + ' ' +
    (card.description || '') + ' ' +
    (card.tags || []).join(' ')
  ).toLowerCase();
  return hay.indexOf(q) !== -1;
},
```

`indexOf` is chosen over `String.prototype.includes` purely for
parity with the rest of `app.js` (no ES2015+ features beyond what
Alpine itself requires). `card.description` is included even though
the spec only mandates title + description + tags because that is
exactly what is requested; the helper covers all three with one
join.

**Alternatives considered:**

- Build the haystack once per card on board load and memoize on the
  card object: rejected — premature; boards are O(100) cards, a
  keystroke does O(100) string concats with no measurable cost. If
  profiling later shows hot path, memoize then.
- Token-based search (split by whitespace, all-must-match): rejected —
  diverges from "substring match" semantics in the brief and ADR
  0003 §D8.

**Affects:** `app.js`.

### D3. Filtered accessor — new method, do not mutate `cardsByColumn`

Add a new method:

```js
filteredCardsByColumn(name) {
  const all = this.cardsByColumn(name);
  if (!this.filter.trim()) return all;
  const self = this;
  return all.filter(function (c) { return self.filterMatches(c); });
},
```

Column templates switch from `cardsByColumn(name)` to
`filteredCardsByColumn(name)` for rendering. The existing
`cardsByColumn(name)` is preserved (it remains the right helper for
the `list-count` badge and for any future consumer that needs the
unfiltered column list).

**Why a new method, not Alpine `x-show` per card:**

- `x-show` per card would keep all cards in the DOM with
  `display:none`, but it would also keep them as Sortable drag
  candidates — Sortable indexes the `<li>` children of the `<ul>`,
  not their visibility. A user could then drag a hidden card by
  accident (e.g. via Sortable's animation phase) or, more subtly,
  the position math on drop would include hidden indices, producing
  drops that look wrong after the filter clears.
- An `x-for` over `filteredCardsByColumn(name)` removes hidden cards
  from the DOM entirely. Sortable on remount sees only the visible
  cards. Drag indices stay consistent with what the user sees.
- Trade-off: when the filter changes, Alpine re-renders the `<ul>`
  children, which means Sortable instances on those `<ul>`s need to
  be torn down and remounted. `mountSortable()` already handles this
  pattern after every board refetch; we extend the trigger to also
  fire when the filter changes (a `$watch` on `filter` in `init()`).

**Affects:** `app.js`, `index.html` column body templates.

### D4. Column counts unchanged — show TOTAL, not filtered

The `list-count` badge on each column header continues to read
`{{ cardsByColumn(col).length }}` (or the equivalent existing
expression). It does NOT switch to the filtered length. This is
explicit in `refs/design.md` §"List" §"list-count": the count is
"the number that lives in this column", not a result counter.

The board-wide match count surfaces in exactly one place: the
mono-counter badge on the Filter button, when the filter is active
(see D7).

**Affects:** `index.html`.

### D5. Popover — Alpine-controlled, CSS-absolute under the topbar

The popover is a `<div>` sibling of the Filter button inside the
topbar's right zone, gated by `x-show="filterOpen"`. Positioning is
CSS-absolute relative to the topbar's right cluster: 100% from top,
0 from right, 8px top offset. Width 280px, padding 14px, surface
fill via `var(--surface)`, border via `var(--border)`,
`border-radius: var(--rounded-xl)` (14px), modest shadow consistent
with other popovers in the design system.

The popover contains:

- A `<label class="t-mono-label">FILTER CARDS</label>` heading
  (mono-label typography token per `refs/design.md` §"Typography").
- An auto-focused `<input type="text" x-model="filter">`. The
  `x-model` is bare (no `.debounce`) — Alpine's per-keystroke
  reactivity is the spec'd behavior.
- A conditional `<a class="clear-link" x-show="filter !== ''"
  @click="filter = ''">Clear filter</a>` shown only when the filter
  is non-empty.

The input is auto-focused via `x-ref="filterInput"` + a
`@click.stop` on the button that sets `filterOpen = true` and then
`$nextTick(() => $refs.filterInput.focus())`.

**Alternatives considered:**

- Render the popover at the document body and position via JS
  measurements: rejected — adds a measurement layer, no second
  popover in this phase needs it, and the topbar is fixed so the
  anchor doesn't move.
- Use the native `<dialog>` element: rejected — modal semantics, ESC
  is built-in but outside-click is not, and the look is hard to
  override consistently.

**Affects:** `index.html` (markup), `style.css` (positioning + box).

### D6. Closing — outside click + Escape

Two close paths:

- **Escape:** `@keydown.escape.window="filterOpen && (filterOpen = false)"`
  on the popover root. Keying on `window` ensures it works even when
  focus has drifted off the input (e.g. user tabbed to the
  Clear-filter link). Escape does NOT clear `filter` — only
  `filterOpen` flips to `false`.
- **Outside click:** `@click.outside="filterOpen = false"` on the
  popover root. Alpine's `.outside` modifier handles the listener
  lifecycle. Clicking the Filter button toggles `filterOpen`; the
  `.outside` modifier ignores clicks on the bound element's own tree.

Neither path mutates `filter`. The closed popover with a non-empty
filter is the documented "filter still active, but the query box is
hidden" state — the badge on the button is the only visible cue.

**Alternatives considered:**

- A single global listener that closes any open popover (theme,
  filter, future column menus): deferred — UI-6 introduces column
  3-dots menus that would benefit, but cross-popover dismissal
  semantics need their own design pass.
- Close on click of any card or column header: rejected — outside
  click already covers this and the design.md description ("closes
  on outside click") is the simpler rule.

**Affects:** `index.html`.

### D7. Filter button — active state + mono-counter badge

The button renders three pieces of content:

1. A magnifier icon (vendored SVG, neutral stroke).
2. The label `Filter` in button typography.
3. A `<span class="t-mono-counter filter-badge" x-show="filter !== ''">`
   whose text content is the board-wide match count, computed via
   `cards.filter(c => filterMatches(c)).length`.

The button's class list toggles `state-active` when the filter is
non-empty (`:class="{ 'state-active': filter !== '' }"`). The
`state-active` style applies a surface fill (`var(--surface)`) per
`refs/design.md` §"TopBar".

The board-wide count is derived, not stored. Alpine recomputes on
every keystroke because it depends on `filter` and `cards`. For
O(100) cards, this is free.

**Alternatives considered:**

- Show the count badge always (including zero): rejected — visually
  noisy and not what design.md describes ("when the filter has any
  text").
- Show the count of visible cards across all columns (which equals
  matches): identical numerically; we keep the wording "match count"
  for clarity in the spec.

**Affects:** `index.html`, `style.css`.

### D8. Empty-column placeholder — `No matches` only when filter active

Each column body template gains a conditional placeholder rendered
when the column has zero visible cards AND the filter is non-empty:

```html
<template x-if="filter !== '' && filteredCardsByColumn(col).length === 0">
  <div class="no-matches">No matches</div>
</template>
```

This must coexist with the existing "empty column" placeholder
(V1 spec: empty columns render an `.empty` placeholder). The two
states are mutually exclusive in practice:

- Column has 0 total cards AND no filter → render existing `.empty`.
- Column has > 0 total cards AND filter hides them all → render the
  new `.no-matches`.
- Column has 0 total cards AND a filter is active → render the
  existing `.empty` placeholder (the filter doesn't change "column
  is genuinely empty"; we use the existing empty visual rather than
  duplicate the same idea twice).

The implementation expresses this as: render `.no-matches` only when
`cardsByColumn(col).length > 0 && filteredCardsByColumn(col).length
=== 0`. The existing empty placeholder template stays unchanged and
fires when `cardsByColumn(col).length === 0`.

`No matches` is rendered with `font-style: italic` and a
`text-faint`-color token, per the design.md description ("faint
italic"). It is read-only and not interactive.

**Alternatives considered:**

- Re-purpose the existing `.empty` placeholder to also handle
  "filtered to zero": rejected — the messages differ
  ("empty" vs "no matches"), and conflating them muddles the empty
  state.

**Affects:** `index.html`, `style.css`.

### D9. Re-mounting Sortable after a filter change

Sortable.js instances are bound to the `<ul.cards>` element of each
column. When `filter` changes, the `x-for` over
`filteredCardsByColumn(col)` re-renders the children. The existing
`mountSortable()` function (called after `load()` via
`$nextTick`) tears down old instances and remounts on the new
`<ul>` references.

We add a watch in `init()`:

```js
this.$watch('filter', () => this.$nextTick(() => this.mountSortable()));
```

This ensures Sortable always sees the currently-visible cards and
drag indices match what the user sees. The teardown is cheap (O(N)
columns) and only fires per keystroke that actually changes the
filter (Alpine batches identical assignments).

**Alternatives considered:**

- Leave Sortable mounted on a stale `<ul>` and rely on Sortable's
  internal child-index recompute: rejected — Sortable does recompute,
  but the issue is mid-drag state: if a drag starts on a card that
  is now hidden, the user gets the existing `dragend` cleanup with
  no smooth recovery. Remounting eliminates the edge case.
- Disable drag while a filter is active: rejected — design.md does
  not require this and it surprises users (drag works on every
  visible card, why would the filter disable it?).

**Affects:** `app.js`.

### D10. Test surface — server tests assert markup only

The viewer is rendered by the browser, not by the server, so end-to-
end behavior tests are out of scope for this phase (the project has
no JS test runner — ADR 0002 §D5). Coverage in this phase is limited
to server-side HTML structure assertions:

- `server_test.go` checks that `GET /` body contains a Filter button
  element with the literal `Filter` label.
- `server_test.go` checks that `GET /` body contains a popover
  container element (selector class or `data-` attribute).

This matches the pattern used by V1-V5 tests: assert the embedded
markup is reachable, leave Alpine behavior to be validated by
manual run during apply phase.

**Affects:** `server_test.go`.

## Risks / Trade-offs

- **Per-keystroke recompute of board-wide match count** → Mitigation:
  Alpine memoizes derived bindings; at O(100) cards the cost is sub-
  millisecond. If a future board grows to O(10k) cards, memoize the
  count or debounce the input.
- **Sortable remount on every filter change** → Mitigation: same
  teardown path as V4 SSE refetches, which is exercised constantly
  in development. If profiling shows churn, gate the remount on
  "filter went from empty → non-empty" or vice versa, since the set
  of `<ul>` references changes only at those edges.
- **`x-model` is per-keystroke; no debounce** → Mitigation: this is
  intentional per the brief ("every keystroke updates"). If users
  report typing lag on slow machines, add `.debounce.150ms`.
- **Hidden cards skip `card.title` tooltip rendering, so accessibility
  tools may not surface them** → Acceptable: filtered cards are
  intentionally not in the user's view. Screen reader users who
  clear the filter regain access.
- **No global keyboard shortcut to open the filter** → Acceptable for
  v1. Can be layered later (e.g. `/` to focus filter input) without
  changing the state shape.
- **No "filter active" announcement to screen readers** → Acceptable;
  the badge serves as the visible cue. Live-region announcements are
  a polish item.

## Migration Plan

No migration needed.

- Server: no changes; existing deployments continue to serve the
  same `/api/board` payload.
- Client: the new behavior activates as soon as the updated
  `index.html` + `app.js` + `style.css` are embedded into the
  binary. Users see the Filter button on the next page load.
- Rollback: revert the three frontend files; the topbar returns to
  its UI-2 state. No data migration. No persistence to clean up
  (ADR 0003 §D8).

## Open Questions

None at this time. ADR 0003 settled persistence (§D8), token usage
(§D5), and stack (§D2). The brief specifies all behavioral details
the spec needs.
