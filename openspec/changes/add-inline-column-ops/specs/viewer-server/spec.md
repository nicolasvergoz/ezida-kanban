## ADDED Requirements

### Requirement: `POST /api/columns` creates a column

`POST /api/columns` SHALL accept an `application/json` body
`{"name": "<name>"}`, validate the name, append it to
`b.Board.Columns` via `board.AddColumn`, persist via `board.Save`,
and respond with `{"columns": [...]}` containing the full updated
column list. Response `Content-Type` MUST be `application/json` and
status MUST be `201` on success. The body's `name` MUST be trimmed
before validation; empty after trim MUST return `INVALID_BODY`.
Duplicate names MUST return `COLUMN_ALREADY_EXISTS` (400).

#### Scenario: Successful column creation

- **WHEN** `POST /api/columns` is called with body `{"name":"review"}`
  against a server whose board columns are `["todo","done"]`
- **THEN** the response status MUST be `201`
- **AND** the response `Content-Type` MUST be `application/json`
- **AND** the response body's `columns` MUST equal
  `["todo","done","review"]`
- **AND** the on-disk `kanban.toml`'s `[board].columns` MUST reflect
  the appended column

#### Scenario: Duplicate column rejected

- **WHEN** `POST /api/columns` is called with body `{"name":"todo"}`
  against a server whose board columns include `todo`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_ALREADY_EXISTS`
- **AND** the body's `error.details.name` MUST equal `"todo"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Empty name rejected

- **WHEN** `POST /api/columns` is called with body `{"name":""}` or
  `{"name":"   "}`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Malformed JSON body returns 400

- **WHEN** `POST /api/columns` is called with a body that is not
  valid JSON
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: SSE board-changed fires after success

- **WHEN** a client is subscribed to `/api/events` and a successful
  `POST /api/columns` completes
- **THEN** the client MUST receive a `board-changed` event within
  500 ms following the request's response

### Requirement: `PATCH /api/columns/:name` renames a column

`PATCH /api/columns/:name` SHALL accept an `application/json` body
`{"name": "<new-name>"}`, validate the new name, call
`board.RenameColumn` (which updates `b.Board.Columns` and rewrites
every card whose `column` field referenced the old name), persist
via `board.Save`, and respond with `{"columns": [...], "renamed":
{"from": "<old>", "to": "<new>"}}` and HTTP 200. `:name` MUST be
URL-decoded by Go's `r.PathValue` before lookup. If
`from == to`, the operation MUST succeed as a no-op (still write
the file but emit no rename).

#### Scenario: Successful rename propagates to cards

- **WHEN** `PATCH /api/columns/todo` is called with body
  `{"name":"backlog"}` against a server whose board has columns
  `["todo","done"]` and 3 cards with `column="todo"`
- **THEN** the response status MUST be `200`
- **AND** the response body's `columns` MUST equal
  `["backlog","done"]`
- **AND** the response body's `renamed.from` MUST equal `"todo"`
- **AND** the response body's `renamed.to` MUST equal `"backlog"`
- **AND** every previously-`todo` card's on-disk `column` field MUST
  now equal `"backlog"`

#### Scenario: Rename to identical name is a no-op success

- **WHEN** `PATCH /api/columns/todo` is called with body
  `{"name":"todo"}`
- **THEN** the response status MUST be `200`
- **AND** the response body's `columns` MUST be unchanged

#### Scenario: Unknown source column returns 400

- **WHEN** `PATCH /api/columns/ghost` is called with body
  `{"name":"backlog"}` and `ghost` is not in `[board].columns`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_NOT_FOUND`
- **AND** the body's `error.details.column` MUST equal `"ghost"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: New name already exists returns 400

- **WHEN** `PATCH /api/columns/todo` is called with body
  `{"name":"done"}` and `done` is already in `[board].columns`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_ALREADY_EXISTS`
- **AND** the body's `error.details.name` MUST equal `"done"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Empty new name rejected

- **WHEN** `PATCH /api/columns/todo` is called with body
  `{"name":""}` or `{"name":"   "}`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Malformed JSON body returns 400

- **WHEN** `PATCH /api/columns/todo` is called with a body that is
  not valid JSON
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`

### Requirement: `DELETE /api/columns/:name` deletes a column

`DELETE /api/columns/:name` SHALL refuse the operation when the
column does not exist (404 `COLUMN_NOT_FOUND`), when it is the only
remaining column in `b.Board.Columns` (400
`CANNOT_DELETE_LAST_COLUMN`), or when it contains any cards (400
`COLUMN_HAS_CARDS`). On success, it MUST call `board.DeleteColumn`,
persist via `board.Save`, and respond with `{"columns": [...]}`
containing the post-delete column list. HTTP status MUST be `200`.

#### Scenario: Successful delete of an empty column

- **WHEN** `DELETE /api/columns/review` is called against a server
  whose board columns are `["todo","done","review"]` and no card
  has `column="review"`
- **THEN** the response status MUST be `200`
- **AND** the response body's `columns` MUST equal
  `["todo","done"]`
- **AND** the on-disk `kanban.toml`'s `[board].columns` MUST reflect
  the deletion

#### Scenario: Unknown column returns 404

- **WHEN** `DELETE /api/columns/ghost` is called and `ghost` is not
  in `[board].columns`
- **THEN** the response status MUST be `404`
- **AND** the body's `error.code` MUST be `COLUMN_NOT_FOUND`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Last column refuses with CANNOT_DELETE_LAST_COLUMN

- **WHEN** `DELETE /api/columns/todo` is called against a server
  whose `[board].columns` is `["todo"]` and no card references
  `todo`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `CANNOT_DELETE_LAST_COLUMN`
- **AND** the body's `error.details.name` MUST equal `"todo"`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Column with cards refuses with COLUMN_HAS_CARDS

- **WHEN** `DELETE /api/columns/todo` is called and 2 cards have
  `column="todo"`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_HAS_CARDS`
- **AND** the body's `error.details.column` MUST equal `"todo"`
- **AND** the body's `error.details.cards` MUST be an array of 2
  objects each containing `id` and `title` matching the blocking
  cards
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: SSE board-changed fires after success

- **WHEN** a client is subscribed to `/api/events` and a successful
  `DELETE /api/columns/:name` completes
- **THEN** the client MUST receive a `board-changed` event within
  500 ms following the request's response

### Requirement: `POST /api/columns/move` reorders a column

`POST /api/columns/move` SHALL accept an `application/json` body
`{"name": "<name>", "position": <int>}`, call `board.MoveColumn`
with the parsed name and position, persist via `board.Save`, and
respond with `{"columns": [...]}` containing the post-move column
list. HTTP status MUST be `200`. `position` MUST be 0-indexed and
clamped to `[0, N-1]` by the underlying `MoveColumn` helper per
ADR 0002 §D11 — out-of-range values are accepted and silently
clamped, not an error. Cards MUST NOT be touched by this
operation.

#### Scenario: Successful reorder

- **WHEN** `POST /api/columns/move` is called with body
  `{"name":"done","position":0}` against a server whose board
  columns are `["todo","ongoing","done"]`
- **THEN** the response status MUST be `200`
- **AND** the response body's `columns` MUST equal
  `["done","todo","ongoing"]`
- **AND** the on-disk `kanban.toml`'s `[board].columns` MUST reflect
  the new order

#### Scenario: No-op when already at target position

- **WHEN** `POST /api/columns/move` is called with body
  `{"name":"todo","position":0}` and `todo` is already at index 0
- **THEN** the response status MUST be `200`
- **AND** the response body's `columns` MUST be unchanged

#### Scenario: Position out of range is silently clamped

- **WHEN** `POST /api/columns/move` is called with body
  `{"name":"todo","position":999}` against a 3-column board
- **THEN** the response status MUST be `200`
- **AND** the named column MUST end up at the last index (index 2)

#### Scenario: Negative position clamps to 0

- **WHEN** `POST /api/columns/move` is called with body
  `{"name":"done","position":-5}`
- **THEN** the response status MUST be `200`
- **AND** the named column MUST end up at index 0

#### Scenario: Unknown column returns 400

- **WHEN** `POST /api/columns/move` is called with body
  `{"name":"ghost","position":0}` and `ghost` is not in
  `[board].columns`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_NOT_FOUND`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Malformed JSON body returns 400

- **WHEN** `POST /api/columns/move` is called with a body that is
  not valid JSON
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`

#### Scenario: Cards untouched by reorder

- **WHEN** any successful `POST /api/columns/move` completes
- **THEN** the on-disk `[[cards]]` blocks MUST be byte-identical to
  their pre-move state (same order, same fields, same timestamps)

### Requirement: Column endpoints reuse the JSON error envelope

All column endpoints MUST emit error responses using the existing
JSON envelope. The four column endpoints (`POST /api/columns`,
`PATCH /api/columns/:name`, `DELETE /api/columns/:name`,
`POST /api/columns/move`) SHALL match the envelope shape used by the
card endpoints per ADR 0001 §D8 and ADR 0002 §D7:

```
{"error":{"code":"<UPPER_SNAKE>","message":"<sentence>","details":{...}}}
```

New wire codes introduced by this requirement (per ADR 0003 §D9):

- `COLUMN_ALREADY_EXISTS` (400) — body's `name` collides with an
  existing column.
- `CANNOT_DELETE_LAST_COLUMN` (400) — DELETE would empty
  `[board].columns`.
- `COLUMN_HAS_CARDS` (400) — DELETE refused because the column
  contains cards. `details.cards` MUST be an array of
  `{id, title}` objects.

Existing codes reused: `INVALID_BODY`, `COLUMN_NOT_FOUND`,
`BOARD_NOT_FOUND`, `SCHEMA_VERSION_MISMATCH`, `VALIDATION_FAILED`,
`IO_ERROR`.

#### Scenario: Error envelope shape

- **WHEN** any column endpoint returns an error
- **THEN** the response `Content-Type` MUST be `application/json`
- **AND** the body MUST be JSON-decodable
- **AND** the body's top-level key MUST be `error`
- **AND** `error.code` MUST be present and non-empty
- **AND** `error.message` MUST be present and non-empty

#### Scenario: New wire codes are stable strings

- **WHEN** any of `COLUMN_ALREADY_EXISTS`,
  `CANNOT_DELETE_LAST_COLUMN`, or `COLUMN_HAS_CARDS` is returned
- **THEN** the literal `error.code` string MUST match the code
  exactly (UPPER_SNAKE_CASE), with no version suffix or namespace
  prefix
