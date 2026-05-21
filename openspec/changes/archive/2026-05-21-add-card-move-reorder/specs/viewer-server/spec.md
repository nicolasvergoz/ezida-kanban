## ADDED Requirements

### Requirement: `POST /api/cards/:id/move` relocates a card

`POST /api/cards/:id/move` SHALL accept an `application/json` body `{"column": "<name>", "position": <int>}`, call `board.MoveCard` with those arguments, persist the result via `board.Save`, and respond with `{"card": {...}}` containing the post-move card. The response `Content-Type` MUST be `application/json`. `position` MUST be 0-indexed and clamped by the underlying `MoveCard` primitive (no client-visible error for out-of-range positions).

#### Scenario: Successful cross-column move

- **WHEN** `POST /api/cards/<id>/move` with body `{"column":"done","position":0}` is called against a server whose board has the card in `todo`
- **THEN** the response status MUST be `200`
- **AND** the response body's `card.column` equals `"done"`
- **AND** the underlying `kanban.toml` reflects the new column for that card

#### Scenario: Successful within-column reorder

- **WHEN** `POST /api/cards/<id>/move` with body `{"column":"todo","position":0}` is called against a card currently at position 2 in `todo`
- **THEN** the response status MUST be `200`
- **AND** the on-disk card order within `todo` MUST place the moved card first

#### Scenario: Unknown card returns 404

- **WHEN** `POST /api/cards/zzzzzz/move` with any valid body is called and no card has id `zzzzzz`
- **THEN** the response status MUST be `404`
- **AND** the body's `error.code` MUST be `CARD_NOT_FOUND`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Unknown column returns 400

- **WHEN** `POST /api/cards/<id>/move` with body `{"column":"ghost","position":0}` is called and `ghost` is not in `[board].columns`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_NOT_FOUND`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Malformed JSON body returns 400

- **WHEN** `POST /api/cards/<id>/move` is called with a body that is not valid JSON (e.g. truncated)
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`

#### Scenario: Position out of range is silently clamped

- **WHEN** `POST /api/cards/<id>/move` with body `{"column":"todo","position":999}` is called against a board where `todo` has 2 cards
- **THEN** the response status MUST be `200`
- **AND** the moved card MUST be placed at the end of `todo`

#### Scenario: Non-POST methods rejected

- **WHEN** `GET /api/cards/<id>/move` is called
- **THEN** the response status MUST be `405` (or `404` if the router doesn't differentiate methods on the path; either is acceptable in v1)
