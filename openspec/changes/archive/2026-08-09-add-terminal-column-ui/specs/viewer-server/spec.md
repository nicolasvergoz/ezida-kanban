## MODIFIED Requirements

### Requirement: `PATCH /api/columns/:name` renames a column

`PATCH /api/columns/:name` SHALL accept an `application/json` body
carrying two optional keys, `{"name": "<new-name>", "done": <bool>}`,
and apply whichever are present to the column named by `:name`.
`:name` MUST be URL-decoded by Go's `r.PathValue` before lookup.

The handler MUST reject a body carrying neither key with 400
`INVALID_BODY`. A `name` that is present but empty or whitespace-only
MUST be rejected with 400 `INVALID_BODY`.

Before any mutation, the handler MUST verify that `:name` is present in
`[board].columns` and MUST respond 400 `COLUMN_NOT_FOUND` otherwise,
regardless of which keys the body carries. This check applies even when
the requested change would have been a no-op.

When `name` is present, the handler MUST call `board.RenameColumn`
(which updates `b.Board.Columns`, carries the terminal marker across the
rename, and rewrites every card whose `column` field referenced the old
name). If `from == to`, the rename MUST succeed as a no-op and emit no
rename echo.

When `done` is present, the handler MUST apply it via
`board.SetDoneColumn` **after** any rename, and to the post-rename name.
`true` marks the column terminal; `false` clears the marker. Applying a
value the column already carries MUST succeed as a no-op.

The handler MUST persist via `board.Save` and respond 200 with
`{"columns": [...], "renamed": {"from": "<old>", "to": "<new>"}}`, where
`renamed` is present only when a rename actually occurred. The response
shape is otherwise unchanged: the terminal marker is NOT echoed here,
because the sibling column endpoints do not echo it either and the page
learns the new state from the `board-changed` refetch. The `*` marker
used in `kanban.toml` MUST NEVER appear in any response field.

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

#### Scenario: Unknown column is refused even when the body is a no-op

- **WHEN** `PATCH /api/columns/ghost` is called with body
  `{"name":"ghost"}` and `ghost` is not in `[board].columns`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `COLUMN_NOT_FOUND`
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

#### Scenario: Body with neither key returns 400

- **WHEN** `PATCH /api/columns/todo` is called with body `{}`
- **THEN** the response status MUST be `400`
- **AND** the body's `error.code` MUST be `INVALID_BODY`
- **AND** the on-disk `kanban.toml` MUST be byte-unchanged

#### Scenario: Marking a column terminal without renaming it

- **WHEN** `PATCH /api/columns/shipped` is called with body
  `{"done":true}` against a server whose `[board].columns` is
  `["todo","shipped"]`
- **THEN** the response status MUST be `200`
- **AND** the response body MUST NOT carry a `renamed` key
- **AND** the on-disk `[board].columns` MUST equal
  `["todo","shipped*"]`
- **AND** a subsequent `GET /api/board` MUST report `done_columns`
  equal to `["shipped"]`

#### Scenario: Clearing the terminal marker without renaming

- **WHEN** `PATCH /api/columns/done` is called with body
  `{"done":false}` against a server whose `[board].columns` is
  `["todo","done*"]`
- **THEN** the response status MUST be `200`
- **AND** the on-disk `[board].columns` MUST equal `["todo","done"]`
- **AND** a subsequent `GET /api/board` MUST report `done_columns`
  equal to `[]`

#### Scenario: Toggling to the value already held is a no-op success

- **WHEN** `PATCH /api/columns/done` is called with body
  `{"done":true}` and `done` is already terminal
- **THEN** the response status MUST be `200`
- **AND** the on-disk `[board].columns` MUST still equal
  `["todo","done*"]`

#### Scenario: Rename and mark terminal in one request

- **WHEN** `PATCH /api/columns/review` is called with body
  `{"name":"shipped","done":true}` against a server whose
  `[board].columns` is `["todo","review"]`
- **THEN** the response status MUST be `200`
- **AND** the response body's `columns` MUST equal `["todo","shipped"]`
- **AND** the on-disk `[board].columns` MUST equal
  `["todo","shipped*"]`

#### Scenario: Rename and clear the marker in one request

- **WHEN** `PATCH /api/columns/done` is called with body
  `{"name":"shipped","done":false}` against a server whose
  `[board].columns` is `["todo","done*"]`
- **THEN** the response status MUST be `200`
- **AND** the on-disk `[board].columns` MUST equal
  `["todo","shipped"]`

#### Scenario: A rename alone preserves the marker

- **WHEN** `PATCH /api/columns/done` is called with body
  `{"name":"shipped"}` against a server whose `[board].columns` is
  `["todo","done*"]`
- **THEN** the response status MUST be `200`
- **AND** the on-disk `[board].columns` MUST equal
  `["todo","shipped*"]`

#### Scenario: The on-disk marker never reaches the response

- **WHEN** any successful `PATCH /api/columns/:name` returns a body
  against a board carrying a terminal column
- **THEN** no string in `columns` or `renamed` MUST end with `*`

#### Scenario: SSE board-changed fires after a marker toggle

- **WHEN** a client is subscribed to `/api/events` and a successful
  `PATCH /api/columns/:name` carrying `done` completes
- **THEN** the client MUST receive a `board-changed` event within
  500 ms following the request's response
