## ADDED Requirements

### Requirement: `PATCH /api/cards/:id` updates a card with partial fields

`PATCH /api/cards/:id` SHALL accept an `application/json` body whose keys are a subset of `{title, description, tags, priority}`. The handler MUST decode the body into a `board.CardPatch`, call `board.UpdateCard`, persist via `board.Save`, and respond with `{"card": {...}}` containing the post-update card. Response `Content-Type` MUST be `application/json`. Keys absent from the request body MUST leave the corresponding card field untouched on disk.

#### Scenario: Successful patch of title only

- **WHEN** `PATCH /api/cards/<id>` with body `{"title":"New title"}` is called
- **THEN** the response status MUST be `200`
- **AND** the response body's `card.title` equals `"New title"`
- **AND** the response body's `card.description` equals the pre-patch value
- **AND** the on-disk card reflects the new title

#### Scenario: Successful patch of multiple fields

- **WHEN** `PATCH /api/cards/<id>` with body `{"title":"New","tags":["a","b"],"priority":"high"}` is called
- **THEN** the response status MUST be `200`
- **AND** the response body's `card` reflects all three new values

#### Scenario: Clear priority by sending empty string

- **WHEN** `PATCH /api/cards/<id>` with body `{"priority":""}` is called against a card with `priority="high"`
- **THEN** the response status MUST be `200`
- **AND** the response body's `card.priority` equals `""`

#### Scenario: Clear tags by sending empty array

- **WHEN** `PATCH /api/cards/<id>` with body `{"tags":[]}` is called against a card with `tags=["x"]`
- **THEN** the response status MUST be `200`
- **AND** the response body's `card.tags` equals `[]`

#### Scenario: Empty title returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"title":""}` is called
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `MISSING_TITLE`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Unknown priority returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"priority":"urgent"}` is called and `urgent` is not in `[board].priorities`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_PRIORITY`

#### Scenario: Empty-string tag returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"tags":["good",""]}` is called
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_TAG`

#### Scenario: Unknown card returns 404

- **WHEN** `PATCH /api/cards/zzzzzz` with any valid body is called and no card has id `zzzzzz`
- **THEN** the response status MUST be `404`
- **AND** the body's `error.code` MUST be `CARD_NOT_FOUND`

#### Scenario: Malformed JSON returns 400

- **WHEN** `PATCH /api/cards/<id>` is called with a non-JSON body
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`

#### Scenario: PATCH refreshes updated_at

- **WHEN** any successful patch is applied
- **THEN** the response body's `card.updated_at` MUST be strictly later than the pre-patch value

#### Scenario: Non-PATCH methods are rejected

- **WHEN** `GET /api/cards/<id>` is called
- **THEN** the response status MUST be `405` (or `404` if the router doesn't differentiate; either is acceptable in v1)
