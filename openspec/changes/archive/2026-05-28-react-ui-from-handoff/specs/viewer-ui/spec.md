## ADDED Requirements

### Requirement: Vendored React, ReactDOM, and Babel-standalone are served from embedded FS

The page SHALL reference `/static/vendor/react.production.min.js`,
`/static/vendor/react-dom.production.min.js`, and
`/static/vendor/babel.min.js` as `<script>` tags. Each asset MUST
be present in `internal/server/web/vendor/` and MUST be served
verbatim through the existing `/static/*` route. The page MUST NOT
load React, ReactDOM, Babel, or any other JS from a remote URL at
runtime.

#### Scenario: React reachable via static route

- **WHEN** `GET /static/vendor/react.production.min.js` is called
  against the running server
- **THEN** the response status is `200`
- **AND** the body's first bytes match the embedded file

#### Scenario: ReactDOM reachable via static route

- **WHEN** `GET /static/vendor/react-dom.production.min.js` is
  called against the running server
- **THEN** the response status is `200`

#### Scenario: Babel reachable via static route

- **WHEN** `GET /static/vendor/babel.min.js` is called against the
  running server
- **THEN** the response status is `200`

#### Scenario: No external script tags

- **WHEN** the page source of `GET /` is inspected
- **THEN** no `<script>` tag has a `src` attribute pointing outside
  `/static/` (font stylesheets at `fonts.googleapis.com` etc. are
  also forbidden — vendored fonts are used)

### Requirement: JSX is transpiled in the browser via Babel-standalone

The page SHALL load `app.jsx` with `<script type="text/babel"
src="/static/app.jsx" data-presets="react">`. Babel-standalone MUST
load before the JSX script. The server SHALL serve `.jsx` files with
a JavaScript MIME type (`application/javascript` or
`text/javascript`) so the browser's network panel does not flag a
MIME warning. The repository MUST NOT contain a Node build step,
bundler config, or pre-transpiled `.js` output for the React app.

#### Scenario: `.jsx` served with JS MIME

- **WHEN** `GET /static/app.jsx` is called against the running
  server
- **THEN** the response `Content-Type` header is
  `application/javascript` or `text/javascript` (possibly with a
  `; charset=utf-8` suffix)

#### Scenario: No build artifacts present

- **WHEN** the repository is inspected
- **THEN** there is no `package.json`, `node_modules/`, `dist/`,
  or `build/` directory under `internal/server/web/`

### Requirement: Wire shape ↔ UI shape adapter

`app.jsx` SHALL contain a single adapter boundary that converts the
server's `/api/board` JSON envelope (`{ columns[], cards[{ id,
title, column, priority, tags, description, created_at, updated_at
}], priorities[], priority_colors{}, project_name }`) into the
React component tree's working shape (`{ title, lists: [{ id,
title, cards: [{ id, text, tags, priority, description, createdAt,
updatedAt }] }] }`) and vice versa for outbound mutations. List
identity MUST be the server column name (not a synthetic UUID).
Mutation handlers MUST translate UI-shape values to server-shape
request bodies before calling fetch.

#### Scenario: Server load translates to UI shape

- **WHEN** the server returns a card `{ id:"X", title:"Hi",
  column:"todo", created_at:"2026-01-01T00:00:00Z" }`
- **THEN** the React tree exposes that card as `{ id:"X",
  text:"Hi", createdAt:"2026-01-01T00:00:00Z" }` inside
  `lists.find(l => l.id === "todo").cards`

#### Scenario: List id is the column name

- **WHEN** the server has columns `["backlog","todo","done"]`
- **THEN** the React tree's `lists` array has `id` values
  `"backlog"`, `"todo"`, `"done"` in order

### Requirement: Priorities and priority colors come from the server

The UI SHALL source its list of priorities and their colors from
`/api/board.priorities` and `/api/board.priority_colors` for both
the priority picker (filter popover and card-detail modal) and the
per-card priority dot. The UI MUST NOT hard-code priority
identifiers, labels, or color swatches. When the server provides
zero priorities, the priority field MUST be hidden from both the
filter popover and the card-detail modal.

#### Scenario: Server-defined priorities render in the modal

- **WHEN** `/api/board` returns `priorities: ["low","medium","high"]`
  and `priority_colors: { low:"#22c55e", medium:"#f59e0b",
  high:"#ef4444" }`
- **THEN** the card-detail-modal priority listbox shows exactly
  three items in that order, each with the matching color dot

#### Scenario: Zero priorities hides the field

- **WHEN** `/api/board` returns `priorities: []`
- **THEN** the card-detail modal MUST NOT render a "Priority" field
- **AND** the filter popover MUST NOT render the priority pills

## MODIFIED Requirements

### Requirement: Cards are draggable across and within columns

The embedded page SHALL use the HTML5 Drag-and-Drop API on every
card and column so that cards can be dragged between columns and
reordered within a column. The card body (no separate handle) MUST
be the drag affordance (`draggable="true"`). On drop, the page MUST
issue `POST /api/cards/{id}/move` with `{ column, position }`
derived from the drop target column's name and the insertion index
relative to the column's current card list.

#### Scenario: Drag card to another column

- **WHEN** the user drags a card from `todo` and drops it on `done`
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with
  body `{"column":"done","position":<int>}`
- **AND** the card visually appears in the `done` column at the
  dropped slot before the request resolves

#### Scenario: Drag card within the same column

- **WHEN** the user drags a card from position 0 of `todo` and
  drops it at position 2
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with
  body `{"column":"todo","position":2}`
- **AND** the card visually appears at the new slot before the
  request resolves

#### Scenario: Drop indicator above or below the hovered card

- **WHEN** the user drags a card over another card and the cursor
  is in the upper half of that card
- **THEN** a 2px accent line appears above the hovered card

- **WHEN** the cursor is in the lower half
- **THEN** the 2px accent line appears below the hovered card

### Requirement: Topbar brand binds to the server-provided project name

The topbar brand element SHALL render the value of `project_name`
returned by `/api/board`. The brand MUST be non-interactive: not
editable, not hoverable, not focusable. The page MUST NOT expose a
UI affordance to change the board title — `project_name` is derived
server-side from the working directory and is the single source of
truth for the topbar.

#### Scenario: Brand reflects project_name

- **WHEN** `/api/board` returns `project_name: "my-project"`
- **THEN** the topbar brand element's text content is `MY-PROJECT`
  (uppercase per the brand typography)

#### Scenario: Brand is not editable

- **WHEN** the user clicks the brand element
- **THEN** no input or contenteditable element appears in its
  place

## REMOVED Requirements

### Requirement: Vendored Alpine.js is served from embedded FS

**Reason:** Alpine.js is no longer part of the stack. React (with
Babel-standalone for JSX) replaces it. Covered by the new
requirement "Vendored React, ReactDOM, and Babel-standalone are
served from embedded FS".

**Migration:** Remove `internal/server/web/vendor/alpine.min.js`.
Add React + ReactDOM + Babel-standalone under the same vendor
directory per the new requirement.

### Requirement: Sortable.js is vendored, not loaded from CDN

**Reason:** Drag-and-drop is implemented with the native HTML5 DnD
API in the React component tree. Sortable.js is no longer used.
Covered by the modified requirement "Cards are draggable across and
within columns".

**Migration:** Remove `internal/server/web/vendor/sortable.min.js`.
No replacement vendor file is needed.
