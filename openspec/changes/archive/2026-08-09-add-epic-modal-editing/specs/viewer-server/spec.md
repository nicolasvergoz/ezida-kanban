## MODIFIED Requirements

### Requirement: `PATCH /api/cards/:id` updates a card with partial fields

`PATCH /api/cards/:id` SHALL accept an `application/json` body whose keys are a subset of `{title, description, tags, priority, epic, color}`. The handler MUST decode the body into a `board.CardPatch`, call `board.UpdateCard`, persist via `board.Save`, and respond with `{"card": {...}}` containing the post-update card. Response `Content-Type` MUST be `application/json`. Keys absent from the request body MUST leave the corresponding card field untouched on disk.

`epic` and `color` are the storage of the epic relation and its identity. An `epic` naming a card that may not be the target — unknown, the card itself, a card that already carries an epic, or a target refused because the patched card has children of its own — MUST be refused with `400 INVALID_EPIC`. A `color` that is not a well-formed hex value MUST be refused with `400 INVALID_COLOR`. Palette names are NOT accepted on the wire; only hex reaches the file, and only hex is accepted here.

Both refusals MUST use the standard error envelope and MUST carry the offending value in `details`. Neither MUST surface as `500 IO_ERROR`: they are invalid arguments, not I/O failures, and a client has no message to display without a typed code.

Attaching a child to a target that carries no color MUST assign that target a palette color in the same write, and MUST NOT require a second request.

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

#### Scenario: Attach a card to an epic

- **WHEN** `PATCH /api/cards/<id>` with body `{"epic":"rl4m9x"}` is called and `rl4m9x` is a card carrying no epic
- **THEN** the response status MUST be `200`
- **AND** the response body's `card.epic` equals `"rl4m9x"`
- **AND** the on-disk card `rl4m9x` MUST carry a non-empty hex `color`

#### Scenario: Detach a card from its epic

- **WHEN** `PATCH /api/cards/<id>` with body `{"epic":""}` is called against a card carrying an epic
- **THEN** the response status MUST be `200`
- **AND** the response body MUST NOT contain an `epic` key on the card
- **AND** the on-disk card MUST NOT contain an `epic` key

#### Scenario: Unknown epic target returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"epic":"zzzzzz"}` is called and no card has id `zzzzzz`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_EPIC`
- **AND** the body's `error.message` MUST state that no card on this board carries that id
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Self-reference returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"epic":"<id>"}` is called
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_EPIC`

#### Scenario: Nesting a card under an existing child returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"epic":"<other>"}` is called and `<other>` already carries an epic
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_EPIC`

#### Scenario: Giving a parent an epic of its own returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"epic":"<other>"}` is called and at least one card declares `<id>` as its epic
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_EPIC`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Set a color by hex

- **WHEN** `PATCH /api/cards/<id>` with body `{"color":"#10b981"}` is called
- **THEN** the response status MUST be `200`
- **AND** the response body's `card.color` equals `"#10b981"`

#### Scenario: Clear a color by sending empty string

- **WHEN** `PATCH /api/cards/<id>` with body `{"color":""}` is called against a card carrying a color
- **THEN** the response status MUST be `200`
- **AND** the response body MUST NOT contain a `color` key on the card

#### Scenario: Palette name as color returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"color":"blue"}` is called
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_COLOR`
- **AND** the body's `error.details` MUST carry the offending value

#### Scenario: Malformed hex color returns 400

- **WHEN** `PATCH /api/cards/<id>` with body `{"color":"#12"}` is called
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_COLOR`

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
