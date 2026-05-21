## ADDED Requirements

### Requirement: Cards are draggable across and within columns

The embedded page SHALL initialize Sortable.js on every column's card list so that cards can be dragged between columns and reordered within a column. The card's body (no separate handle) MUST be the drag affordance. Each `<li.card>` MUST carry `data-card-id="<id>"`; each `<ul.cards>` MUST carry `data-column="<column-name>"` so the drop handler can read them without DOM traversal.

#### Scenario: Drag card to another column

- **WHEN** the user drags a card from `todo` and drops it on `done`
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with body `{"column":"done","position":<int>}`
- **AND** the card visually appears in the `done` column at the dropped slot before the request resolves

#### Scenario: Drag card within the same column

- **WHEN** the user drags a card from position 0 of `todo` and drops it at position 2
- **THEN** a `POST /api/cards/<id>/move` request MUST fire with body `{"column":"todo","position":2}`
- **AND** the card visually appears at the new slot before the request resolves

### Requirement: Drop failure resets the UI from the server

If the move request returns a non-2xx response, the page SHALL refetch `/api/board` and re-render so the displayed state matches disk. The page MUST NOT attempt a manual revert (server is source of truth per ADR 0002 §D3).

#### Scenario: Server rejects the move

- **WHEN** the move endpoint returns `404 CARD_NOT_FOUND` (e.g. the card was deleted via CLI between page load and drop)
- **THEN** the page MUST refetch `/api/board`
- **AND** the rendered DOM MUST reflect the refetched state (the dropped card is no longer present)
- **AND** the browser console MUST contain an error log describing the failure

### Requirement: Sortable.js is vendored, not loaded from CDN

The page SHALL reference `/static/vendor/sortable.min.js` as a `<script defer>`. The file MUST be present in `internal/server/web/vendor/sortable.min.js` and MUST be served verbatim through the embedded `/static/*` route. The page MUST NOT load Sortable from a remote URL at runtime.

#### Scenario: Sortable reachable via static route

- **WHEN** `GET /static/vendor/sortable.min.js` is called against the running server
- **THEN** the response status MUST be `200`
- **AND** the body's first bytes MUST match the embedded file

#### Scenario: No external sortable script

- **WHEN** the page source of `GET /` is inspected
- **THEN** no `<script>` tag has a `src` attribute pointing outside `/static/`
