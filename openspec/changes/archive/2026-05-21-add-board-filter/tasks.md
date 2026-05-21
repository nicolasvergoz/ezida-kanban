## 1. Alpine state and helpers (`internal/server/web/app.js`)

- [x] 1.1 Add `filter: ''` and `filterOpen: false` keys to the `board()` return object, alongside the existing `editing`/`draft`/`connected` state.
- [x] 1.2 Add a `filterMatches(card)` method that returns `true` when `filter` (trimmed, lowercased) is empty, otherwise checks `String.prototype.indexOf` of the lowercased query against a single lowercased haystack built from `card.title`, `card.description`, and `card.tags.join(' ')`.
- [x] 1.3 Add a `filteredCardsByColumn(name)` method that returns `cardsByColumn(name)` when the filter is empty/whitespace, and otherwise returns the same list filtered by `filterMatches`.
- [x] 1.4 In `init()` (or wherever the component currently wires `$watch`), add `this.$watch('filter', () => this.$nextTick(() => this.mountSortable()))` so Sortable re-binds to the filtered `<ul>` children.
- [x] 1.5 Add an `openFilter()` method (or inline expression) that flips `filterOpen` to `true` and focuses the input via `$refs.filterInput` on the next tick.

## 2. Topbar markup (`internal/server/web/index.html`)

- [x] 2.1 In the topbar's right zone, add a `<button class="topbar-btn filter-btn">` element bound to `:class="{ 'state-active': filter !== '' }"` and `@click="filterOpen ? (filterOpen = false) : openFilter()"`. Content: magnifier icon SVG + the literal label `Filter` + a conditional `<span class="t-mono-counter filter-badge" x-show="filter.trim() !== ''" x-text="cards.filter(c => filterMatches(c)).length"></span>`.
- [x] 2.2 Add a sibling `<div class="filter-popover" x-show="filterOpen" x-cloak @click.outside="filterOpen = false" @keydown.escape.window="filterOpen = false">` containing: a `<label class="t-mono-label">` heading (`FILTER CARDS`), an `<input type="text" x-ref="filterInput" x-model="filter" placeholder="Type to filter...">`, and a conditional `<a class="clear-link" x-show="filter !== ''" @click.prevent="filter = ''">Clear filter</a>` link.

## 3. Column body templates (`internal/server/web/index.html`)

- [x] 3.1 Switch every column `<template x-for>` that currently iterates `cardsByColumn(col)` to iterate `filteredCardsByColumn(col)`. Leave the column header's `list-count` expression bound to `cardsByColumn(col).length` (total, not filtered).
- [x] 3.2 In each column body, add a `<template x-if="filter !== '' && cardsByColumn(col).length > 0 && filteredCardsByColumn(col).length === 0">` that renders `<div class="no-matches">No matches</div>`. Keep the existing V1 empty placeholder unchanged.

## 4. CSS (`internal/server/web/style.css`)

- [x] 4.1 Add `.filter-popover` styles: `position: absolute`, anchored under the topbar right cluster (top: 100%, right: 0, margin-top: 8px), width 280px, padding 14px, background `var(--surface)`, border `1px solid var(--border)`, `border-radius: var(--rounded-xl)`, soft shadow, z-index above the board.
- [x] 4.2 Add `.filter-btn.state-active` styles: surface fill (`background: var(--surface)`), matching the active-state visual described in `refs/design.md` §"TopBar".
- [x] 4.3 Add `.filter-badge` styles: mono-counter typography token (or `.t-mono-counter` utility class), small inline pill, sits to the right of the `Filter` label.
- [x] 4.4 Add `.no-matches` styles: `font-style: italic`, color `var(--text-faint)`, body-typography size, centered or left-aligned inside the column body to match the existing `.empty` placeholder rhythm.
- [x] 4.5 Add `.clear-link` styles: inline link beneath the input, body-typography, hover/focus underline.
- [x] 4.6 Add `[x-cloak]` rule (if not already present) to suppress popover flash on initial render.

## 5. Server tests (`internal/server/server_test.go`)

- [x] 5.1 Extend the existing `GET /` body assertion test (or add a sibling test) to assert that the rendered HTML contains the Filter button (search for both the class selector `filter-btn` and the literal label `Filter`).
- [x] 5.2 Add an assertion that the rendered HTML contains the filter popover container (search for the class `filter-popover`).

## 6. Verify gate

- [x] 6.1 Run `go test ./...` and confirm all existing and new tests pass.
- [x] 6.2 Run `go vet ./...` and confirm no new vet issues.
- [x] 6.3 Manually verify in a browser: open the viewer, click Filter → popover opens with input focused; type a substring → non-matching cards disappear, badge shows match count, button shows active state; press Escape → popover closes, filter still applied; click Filter → reopens with text preserved; click `Clear filter` → all cards visible, badge gone; reload the page → filter state cleared.
