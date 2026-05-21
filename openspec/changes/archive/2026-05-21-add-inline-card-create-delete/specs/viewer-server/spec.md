## ADDED Requirements

### Requirement: `POST /api/cards` creates a new card

`POST /api/cards` SHALL accept an `application/json` body of the
shape `{"column": "<name>", "title": "<text>", "description"?: "<text>", "priority"?: "<value>", "tags"?: ["<tag>", ...]}`.
The handler MUST validate the body in this order:

1. The JSON MUST decode into the expected struct; otherwise return
   `400` with `error.code = INVALID_BODY`.
2. `strings.TrimSpace(title)` MUST be non-empty; otherwise return
   `400` with `error.code = MISSING_TITLE` and the on-disk
   `kanban.toml` MUST be byte-unchanged.
3. Every element of `tags` (if present) MUST have a non-empty
   `strings.TrimSpace`; otherwise return `400` with
   `error.code = INVALID_TAG` and `details.tag` set to the
   offending value; the on-disk file MUST be byte-unchanged.
4. `column` MUST equal one of `b.Board.Columns`; otherwise return
   `404` with `error.code = COLUMN_NOT_FOUND` and
   `details.column` set to the offending value; the on-disk file
   MUST be byte-unchanged.
5. If `priority` is non-empty it MUST equal one of
   `b.Board.Priorities`; otherwise return `400` with
   `error.code = INVALID_PRIORITY` and `details.priority` set to
   the offending value; the on-disk file MUST be byte-unchanged.

On success, the handler MUST:

- Generate a fresh 6-character ID via `board.NewUniqueID` against
  the existing card IDs.
- Build a `board.Card` with `Title = strings.TrimSpace(title)`,
  the requested `Column`, the supplied `Description` (defaulting
  to `""`), the supplied `Priority` (defaulting to `""`), the
  supplied `Tags` (defaulting to `[]`), and
  `CreatedAt = UpdatedAt = time.Now().UTC().Truncate(time.Second)`.
- Append it via `board.AppendCardToColumn`.
- Persist via `board.Save`.
- Respond with status `201`, `Content-Type: application/json`, and
  body `{"card": {...}}` containing the new card via
  `cardToResponse`.

`board.NewUniqueID`'s `ErrIDExhausted` MUST surface as `500
IO_ERROR` via the existing `httpError` catch-all.

#### Scenario: Successful create with title only

- **WHEN** `POST /api/cards` is called with body
  `{"column":"todo","title":"Draft v1"}` against a board whose
  `[board].columns` includes `todo`
- **THEN** the response status MUST be `201`
- **AND** `Content-Type` MUST be `application/json`
- **AND** the body MUST contain a `card` object whose `title`
  equals `"Draft v1"`, whose `column` equals `"todo"`, whose `id`
  matches `^[0-9a-z]{6}$`, and whose `created_at` equals
  `updated_at`
- **AND** the on-disk `kanban.toml` MUST contain a `[[cards]]`
  block with the same `id` appended to the `todo` column

#### Scenario: Successful create with all optional fields

- **WHEN** `POST /api/cards` is called with body
  `{"column":"todo","title":"Refactor auth","description":"split out tokens","priority":"high","tags":["security","tech-debt"]}`
  and `high` is in `[board].priorities`
- **THEN** the response status MUST be `201`
- **AND** the response `card.description` equals
  `"split out tokens"`
- **AND** the response `card.priority` equals `"high"`
- **AND** the response `card.tags` equals `["security","tech-debt"]`

#### Scenario: Unknown column returns 404

- **WHEN** `POST /api/cards` is called with body
  `{"column":"ghost","title":"x"}` and `ghost` is not in
  `[board].columns`
- **THEN** the response status MUST be `404`
- **AND** the body's `error.code` MUST be `COLUMN_NOT_FOUND`
- **AND** the body's `error.details.column` MUST equal `"ghost"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Empty title returns 400

- **WHEN** `POST /api/cards` is called with body
  `{"column":"todo","title":"   "}` (whitespace-only)
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `MISSING_TITLE`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Missing title key returns 400

- **WHEN** `POST /api/cards` is called with body
  `{"column":"todo"}` (no `title` field)
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `MISSING_TITLE`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Unknown priority returns 400

- **WHEN** `POST /api/cards` is called with body
  `{"column":"todo","title":"x","priority":"urgent"}` and
  `urgent` is not in `[board].priorities`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_PRIORITY`
- **AND** the body's `error.details.priority` MUST equal
  `"urgent"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Empty-string tag returns 400

- **WHEN** `POST /api/cards` is called with body
  `{"column":"todo","title":"x","tags":["good",""]}`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_TAG`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Malformed JSON returns 400

- **WHEN** `POST /api/cards` is called with a body that is not
  valid JSON (e.g. truncated, plain text)
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Created card is appended to the end of its column

- **WHEN** `POST /api/cards` succeeds and the target column
  already contains 3 cards
- **THEN** the on-disk ordering of cards within that column MUST
  place the new card last (4th in column-relative order), matching
  `board.AppendCardToColumn` semantics

#### Scenario: Created card carries equal `created_at` and `updated_at`

- **WHEN** any successful `POST /api/cards` returns
- **THEN** the response body's `card.created_at` MUST equal
  `card.updated_at` (both timestamps come from a single
  `time.Now().UTC().Truncate(time.Second)` call at creation)

#### Scenario: Non-POST methods are rejected

- **WHEN** `GET /api/cards` is called
- **THEN** the response status MUST be `405` (or `404` if the
  router does not differentiate methods on the path; either is
  acceptable in v1)

### Requirement: `DELETE /api/cards/:id` removes a card

`DELETE /api/cards/:id` SHALL accept an empty request body, call
`board.DeleteCard(b, id)`, persist the result via `board.Save`,
and respond with status `200`, `Content-Type: application/json`,
and body `{"deleted": "<id>"}`. If no card matches `id`, the
handler MUST respond with status `404` and
`error.code = CARD_NOT_FOUND` via the existing `httpError`
mapping, and the on-disk `kanban.toml` MUST be byte-unchanged.

#### Scenario: Successful delete

- **WHEN** `DELETE /api/cards/<id>` is called against a board
  whose `[[cards]]` array contains a card with that `id`
- **THEN** the response status MUST be `200`
- **AND** `Content-Type` MUST be `application/json`
- **AND** the body MUST equal `{"deleted":"<id>"}` (the `id` echoed
  back)
- **AND** the on-disk `kanban.toml` MUST no longer contain a
  `[[cards]]` block with that `id`

#### Scenario: Unknown card returns 404

- **WHEN** `DELETE /api/cards/zzzzzz` is called and no card has
  `id = "zzzzzz"`
- **THEN** the response status MUST be `404`
- **AND** the body's `error.code` MUST be `CARD_NOT_FOUND`
- **AND** the body's `error.details.id` MUST equal `"zzzzzz"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Delete preserves the order of remaining cards

- **WHEN** a board contains cards `[a, b, c]` (in slice order)
  and `DELETE /api/cards/b` succeeds
- **THEN** the on-disk `[[cards]]` blocks MUST appear in the
  order `[a, c]`
- **AND** the surviving cards' fields MUST be byte-unchanged
  apart from their position in the slice

#### Scenario: Non-DELETE methods are rejected

- **WHEN** `POST /api/cards/<id>` is called (without a `/move`
  suffix)
- **THEN** the response status MUST be `405` (or `404` if the
  router does not differentiate methods on the path; either is
  acceptable in v1)

### Requirement: `POST /api/cards` and `DELETE /api/cards/:id` fire SSE `board-changed`

Every successful card write through the new endpoints MUST broadcast a
`board-changed` SSE event to all subscribed clients. The new endpoints
rely on the existing fsnotify-based watcher (viewer-server
"Server watches kanban.toml" requirement) to deliver the broadcast on
every successful write.
No new code is required for the broadcast — it is a consequence of
calling `board.Save` — but this requirement encodes the observable
behaviour the UI depends on (see ADR 0002 §D9).

#### Scenario: Successful create broadcasts board-changed

- **WHEN** a single client is subscribed to `/api/events` and that
  same client issues `POST /api/cards` with a valid body
- **THEN** the client MUST receive a `board-changed` event within
  500 ms following the request's response

#### Scenario: Successful delete broadcasts board-changed

- **WHEN** a single client is subscribed to `/api/events` and that
  same client issues `DELETE /api/cards/<id>` for an existing
  card
- **THEN** the client MUST receive a `board-changed` event within
  500 ms following the request's response

#### Scenario: Failed create does not broadcast

- **WHEN** a single client is subscribed to `/api/events` and a
  `POST /api/cards` with body `{"column":"todo","title":""}`
  returns `400 MISSING_TITLE`
- **THEN** the client MUST NOT receive a `board-changed` event
  within 500 ms following the request's response (no write
  occurred)
