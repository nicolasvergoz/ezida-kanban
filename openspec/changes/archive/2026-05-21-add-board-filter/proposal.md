## Why

Boards grow. Once a developer accumulates a few dozen cards across
columns, scanning by eye stops working — there is no built-in way to
find the card mentioning "auth" or carrying the tag `security` short
of Cmd+F'ing the rendered DOM. Phase 3 of the UI redesign batch
(ADR 0003 §D8) adds a small, transient, client-side filter so the
viewer can answer "where is the card about X?" in one keystroke,
without server changes and without persisting a query that the user
already forgot they set.

This phase is also the last "look-and-feel" increment in ADR 0003
§D14's two-group plan — after this, the batch shifts into endpoint
work. The filter is intentionally narrow: no advanced operators, no
column-scoped filter, no saved filters. It is a `Ctrl+F` for the
board, scoped to card text and tags.

## What Changes

- Add a **Filter button** to the topbar's right zone (before the
  theme toggle slot reserved by UI-2). The button renders an icon +
  the label `Filter` and, when the filter is non-empty, a
  `mono-counter` badge with the count of matching cards across the
  whole board.
- Clicking the Filter button opens a **280px popover** anchored to
  the button: surface fill, 14px rounded, 14px padding. The popover
  contains an auto-focused text `<input>`. Every keystroke updates a
  client-side, board-wide filter applied to each card's title +
  description + tags via case-insensitive substring match.
- The Filter button gets a `state=active` style (surface fill) while
  the filter is non-empty, matching the active state of other topbar
  affordances.
- When the filter is non-empty, **non-matching cards are hidden**
  inside each column (display-only — the underlying `cards` array is
  unchanged). Columns with zero matches render a faint italic
  `No matches` placeholder inside the column body.
- Column `list-count` badges **continue to show the total card count**
  for that column, not the filtered count (per `refs/design.md`
  §"List" — the count "lives in this column", it is not a result
  counter).
- An **"Clear filter"** inline link appears below the input when the
  filter is non-empty; clicking it empties the filter (popover stays
  open).
- The popover closes on **outside click** or **Escape**. Closing the
  popover does **NOT** clear the filter (per ADR 0003 §D8).
- Filter state is **transient** — it lives only in Alpine component
  state. A page reload clears it. Nothing is written to
  `localStorage`.
- Coexists with existing behaviors: card click → modal (V3) still
  works on visible cards; hidden cards are `display:none` so they
  cannot be clicked. Sortable drag-drop (V2) still works on visible
  cards because the filter is purely cosmetic — the underlying
  `cards` array is not mutated by the filter.
- **ZERO server-side change.** `/api/board` is unchanged; no new
  endpoints; no new error codes.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `viewer-ui`: adds the topbar Filter button, the filter popover,
  the client-side substring matcher, the hidden-card and
  "No matches" rendering rules, and the mono-counter badge on the
  Filter button when the filter is active.

## Impact

- `internal/server/web/index.html` — topbar gains the Filter button
  and popover container; each column body template gains a
  conditional "No matches" placeholder.
- `internal/server/web/app.js` — `board()` gains `filter` (string)
  and `filterOpen` (bool) state, a `filterMatches(card)` helper, and
  a `filteredCardsByColumn(name)` accessor. Existing
  `cardsByColumn(name)` is left in place; templates switch to the
  filtered accessor.
- `internal/server/web/style.css` — popover surface styles, active
  button state, mono-counter badge.
- `internal/server/server_test.go` — assertions that the rendered
  HTML contains the Filter button and the popover container.
- No Go source under `internal/board/`, `internal/api/`, or the SSE
  pipeline is touched. No new dependencies. No build step (ADR 0003
  §D2).

References: ADR 0003 §D2 (stack frozen), §D5 (token system — popover
surface, button states), §D8 (filter is transient).
