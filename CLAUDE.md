# Working on ezida

A file-based Kanban CLI plus an embedded web viewer, backed by a
single `kanban.toml`. Go module at the root; the viewer is one JSX
file transpiled in the browser by a vendored Babel.

## Verify with this, not by eye

```sh
./scripts/verify.sh          # gofmt, vet, go test, browser tests
./scripts/verify.sh --go     # Go gate only, no browser needed
./scripts/verify.sh --visual # also compare against pixel baselines
```

Run the full loop before calling any change done. It is the whole
point of the browser tests: **do not verify viewer behaviour by
dumping the DOM, injecting `<script>` into a copy of `index.html`, or
reading a screenshot.** Those techniques miss things a real browser
catches — see "What the browser tests found" below.

First run on a fresh checkout needs the browser once:

```sh
npm install && npx playwright install chromium
```

## The browser tests

`e2e/*.spec.ts`, driven by `@playwright/test`. Each test compiles the
CLI from the working tree, copies a fixture board to a temp dir, boots
the real `ezida serve` on it, and drives the page. One run therefore
covers the Go handlers, the JSON wire, the wire↔UI adapter, and the
rendering — a broken `cardResponse` field fails a rendering test.

- `e2e/fixtures.ts` — the `board` fixture (server per test), the
  `theme` fixture, helpers (`openBoard`, `card`, `visibleCardIds`,
  `openFilter`, `rgbOf`, `contrast`). It also fails any test whose
  page logged a console error or threw.
- `e2e/fixtures/*.toml` — board fixtures. `epics.toml` has two epics,
  an unrelated card and two terminal columns; `plain.toml` has none of
  it and is the "unchanged for anyone not using the feature" guard.
- Add a fixture per file with `test.use({ fixture: "plain.toml" })`.

Read colours with `rgbOf`, never by parsing `getComputedStyle`:
`color-mix(in oklch, …)` resolves to `oklab(…)`, and a naive regex
reads those numbers as RGB and silently returns nonsense.

Visual comparisons are opt-in (`PW_VISUAL=1`) because their baselines
are captured on one machine's font stack. Regenerate with
`npm run test:e2e:update-snapshots` and look at the diff before
accepting it.

### What they cannot tell you

- **Whether it looks good.** They assert structure, behaviour and
  contrast. Taste, spacing and hierarchy still need a human.
- **Native HTML5 drag and drop.** Playwright's mouse synthesis does
  not reliably drive `dataTransfer`, so card reordering across columns
  is not covered. Assert `draggable` attributes and the underlying
  move endpoint instead, and ask the user to confirm real dragging.

### What the browser tests found

Real interaction beats synthesised events. Both of these were live
before the tests existed:

- **Inline title edit is unreachable with a mouse.** `CardItem` wires
  `onDoubleClick` on `.card-title` to the inline composer, but the
  card's own `onClick` opens the detail modal, and a real double-click
  delivers that click first. A synthesised `dblclick` reaches the
  editor; a user never does. Pinned in `e2e/epic-focus.spec.ts` as a
  known defect, asserting current behaviour so a fix fails loudly.

## Conventions worth knowing

- **The wire is normalised.** `GET /api/board` returns every card, so
  it carries raw `epic` and `color` ids and no `epic_title`,
  `children` or `progress`. The client resolves relations in
  `toUiBoard`'s index — one pass per load, the single place that knows
  a reference can dangle.
- **`output.ExportCard` and `server.cardResponse` are parallel
  structs.** A field added to one must be added to the other in the
  same change; `TestWireShape_ExportMatchesBoard` enforces it by
  comparing json tags, omitempty included.
- **Terminal columns** are spelled `done*` on disk and never anywhere
  else — not in memory, not on the wire, not in the DOM.
- **Every added viewer signal is conditional.** A board with no epics
  must render pixel-identically to one built before the feature; there
  is a test for exactly that.
- `./ezida` at the repo root is a developer's own build and is
  routinely several schema versions stale. Build from the working tree
  rather than running it.

## Planning workflow

Specs and change history live in `openspec/`. The user drives the
OpenSpec cadence themselves — do not invoke `/opsx:*` on their behalf.

Work lands on `main`. No pull requests — do not offer to open one.
