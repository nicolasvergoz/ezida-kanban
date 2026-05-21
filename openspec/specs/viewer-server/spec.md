# viewer-server

### Requirement: `ezida serve` launches an HTTP server on localhost

`ezida serve [--port=N] [--no-open]` SHALL bind an HTTP server on
`127.0.0.1` at port `N` (default `7777`), print
`→ Ezida viewer running at http://127.0.0.1:<port>` on stdout, and
block until `SIGINT` or `SIGTERM`. The server MUST never bind on
`0.0.0.0` or any non-loopback address in v1.

#### Scenario: Default port

- **WHEN** `ezida serve --no-open` is invoked in a directory with a
  valid `kanban.toml` and port `7777` is free
- **THEN** the process binds `127.0.0.1:7777`
- **AND** stdout contains the line `→ Ezida viewer running at http://127.0.0.1:7777`
- **AND** the process keeps running until `SIGINT`

#### Scenario: Custom port via `--port`

- **WHEN** `ezida serve --port=9000 --no-open` is invoked and port
  `9000` is free
- **THEN** the process binds `127.0.0.1:9000`
- **AND** stdout reflects the chosen port

#### Scenario: Bind address is loopback-only

- **WHEN** `ezida serve --no-open` is running on default port
- **THEN** a connection from any IP other than `127.0.0.1` MUST be
  refused at the TCP layer (server never bound a public interface)

### Requirement: Port fallback covers 11 ports

If the starting port is in use, the server SHALL try the next ports
in sequence up to a total window of 11 (starting port plus 10
successors). The first free port is used. If every port in the
window is busy, the server exits with `PORT_UNAVAILABLE`.

#### Scenario: First port busy, second free

- **WHEN** `ezida serve --no-open` is invoked while another process
  holds `127.0.0.1:7777` but `7778` is free
- **THEN** the server binds `127.0.0.1:7778`
- **AND** stdout reflects `:7778`

#### Scenario: Entire window busy

- **WHEN** `ezida serve --no-open` is invoked while ports `7777`
  through `7787` are all in use
- **THEN** the process exits non-zero
- **AND** the error code (JSON mode) is `PORT_UNAVAILABLE`
- **AND** the error details include the starting port and window size

#### Scenario: Non-EADDRINUSE error surfaces immediately

- **WHEN** the listener returns an error other than "address in use"
  (e.g. permission denied)
- **THEN** the server does NOT advance to the next port
- **AND** the original error is returned to the caller

### Requirement: Browser opens unless `--no-open`

On successful bind, the server SHALL attempt to open the chosen URL
in the user's default browser using `open` (darwin) or `xdg-open`
(linux). If `--no-open` is passed, the browser MUST NOT be launched.
A failure to launch the browser MUST NOT abort the server; it logs
a warning on stderr and continues.

#### Scenario: Browser open succeeds

- **WHEN** `ezida serve` is invoked without `--no-open` on a desktop
  session
- **THEN** the platform-appropriate "open URL" command is executed
  with the chosen URL
- **AND** the server keeps running regardless of the command's exit

#### Scenario: `--no-open` skips launch

- **WHEN** `ezida serve --no-open` is invoked
- **THEN** no browser-launch command is executed

#### Scenario: Browser launch failure does not crash the server

- **WHEN** `ezida serve` is invoked on a system without
  `xdg-open` in `PATH`
- **THEN** stderr contains a warning naming the failure
- **AND** the server continues to accept HTTP connections

### Requirement: Graceful shutdown on `SIGINT` or `SIGTERM`

The server SHALL install a handler for `SIGINT` and `SIGTERM`. On
either signal, it MUST stop accepting new connections, drain
in-flight requests with a 5 s timeout, then exit with code `0`.

#### Scenario: `Ctrl+C` while idle

- **WHEN** `ezida serve --no-open` is running with no in-flight
  requests and the process receives `SIGINT`
- **THEN** the process exits with code `0` within 1 s

#### Scenario: `SIGTERM` while serving a slow request

- **WHEN** the server is mid-response on `/api/board` and the
  process receives `SIGTERM`
- **THEN** the in-flight request completes
- **AND** the process exits with code `0` no later than 5 s after
  the signal

### Requirement: `GET /api/board` returns the current board

`GET /api/board` SHALL load `kanban.toml` from the current working
directory at request time and respond with a JSON object containing
`schema_version`, `columns`, `priorities`, `cards_per_column`,
`cards`, and `project_name`. The `cards` array MUST include the
full `description` field for every card. Response `Content-Type`
MUST be `application/json`.

The top-level `project_name` field is a string set at server start
to `filepath.Base(filepath.Dir(<resolved boardPath>))` — i.e. the
parent-directory name of the resolved `kanban.toml` path. It MUST
fall back to the literal string `"Ezida"` when the computed
basename is empty, equal to `"."`, or equal to the platform path
separator. The value MUST NOT change for the lifetime of the
process (it is not re-evaluated when the board file changes).

#### Scenario: Valid board

- **WHEN** `GET /api/board` is called against a server whose
  `kanban.toml` contains 2 columns and 3 cards
- **THEN** the response status is `200`
- **AND** `Content-Type` is `application/json`
- **AND** the body's `schema_version` equals `1`
- **AND** `cards_per_column` reflects the per-column count
- **AND** each card in `cards` has a `description` field (may be
  empty string)
- **AND** the body contains a top-level string field `project_name`

#### Scenario: Project name reflects parent directory

- **WHEN** `GET /api/board` is called against a server whose
  resolved board path is `/tmp/my-project/kanban.toml`
- **THEN** the response body's `project_name` equals `"my-project"`

#### Scenario: Project name falls back to "Ezida" at filesystem root

- **WHEN** `GET /api/board` is called against a server whose
  resolved board path produces an empty or `"."` parent-directory
  basename
- **THEN** the response body's `project_name` equals `"Ezida"`

#### Scenario: Project name is stable across requests

- **WHEN** `GET /api/board` is called twice against the same
  running server with a board file rewritten in between
- **THEN** both responses contain the same `project_name` value

#### Scenario: Board file missing

- **WHEN** `GET /api/board` is called and no `kanban.toml` exists at
  the resolved path
- **THEN** the response status is `500`
- **AND** the body is `{"error":{"code":"BOARD_NOT_FOUND",...}}`

#### Scenario: Board file has wrong schema version

- **WHEN** `GET /api/board` is called against a `kanban.toml` whose
  `schema_version` is not `1`
- **THEN** the response status is `500`
- **AND** the body's `error.code` is `SCHEMA_VERSION_MISMATCH`

### Requirement: Static assets served from embedded FS

`GET /` SHALL serve the contents of the embedded `web/index.html`
file with `Content-Type: text/html; charset=utf-8`. `GET /static/*`
SHALL serve files from the embedded `web/` subtree (excluding
`index.html`) preserving relative paths.

#### Scenario: Index page served

- **WHEN** `GET /` is called
- **THEN** the response status is `200`
- **AND** `Content-Type` starts with `text/html`
- **AND** the body matches the embedded `web/index.html` byte-for-byte

#### Scenario: Static asset served

- **WHEN** `GET /static/app.js` is called and `web/app.js` exists in
  the embedded tree
- **THEN** the response status is `200`
- **AND** the body matches `web/app.js` byte-for-byte

#### Scenario: Unknown static asset

- **WHEN** `GET /static/nope.js` is called and the file is not in
  the embedded tree
- **THEN** the response status is `404`

### Requirement: HTTP error envelope matches CLI JSON contract

Server-side errors SHALL respond with
`{"error":{"code":"<UPPER_SNAKE>","message":"<sentence>","details":{...}}}`
and an HTTP status code derived from the error category:

- `400` for client errors (malformed body, invalid input).
- `404` for unknown resources (card ID, column name).
- `500` for board-load failures, validation failures, and unexpected
  I/O errors.

Error codes MUST reuse the existing CLI namespace (ADR 0001 §D8)
where applicable: `BOARD_NOT_FOUND`, `SCHEMA_VERSION_MISMATCH`,
`VALIDATION_FAILED`, `IO_ERROR`. New codes introduced in this phase:
`PORT_UNAVAILABLE` (process-level, surfaced via stdout/stderr, not
HTTP).

#### Scenario: Unknown route returns 404 JSON

- **WHEN** `GET /api/does-not-exist` is called
- **THEN** the response status is `404`
- **AND** the body's `error.code` is a stable enumeration value (e.g.
  `NOT_FOUND`)

#### Scenario: Board validation error surfaces

- **WHEN** `GET /api/board` is called against a `kanban.toml` that
  parses but fails validation (e.g. duplicate card IDs)
- **THEN** the response status is `500`
- **AND** the body's `error.code` is `VALIDATION_FAILED`

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

### Requirement: Server watches `kanban.toml` for external changes

The server SHALL start a `fsnotify`-based watcher on the resolved board file at startup and SHALL re-arm the watch (`fsw.Add(path)`) after each Rename/Create event so atomic rewrites (temp + rename) continue to be detected. The watcher MUST debounce a burst of events using a 200 ms timer before notifying downstream consumers. The watcher MUST run for the lifetime of the server process and MUST exit cleanly when the server's root context is cancelled.

#### Scenario: Single external write fires one debounced event

- **WHEN** an external process atomically rewrites `kanban.toml` once
- **THEN** the watcher MUST deliver exactly one event on its `Events()` channel within 500 ms of the rewrite

#### Scenario: Burst of writes coalesces into one event

- **WHEN** an editor saves `kanban.toml` 3 times within 100 ms (simulating a fast typewriter / hot-reload tool)
- **THEN** the watcher MUST deliver at most 1 event within 500 ms following the burst

#### Scenario: Watcher survives a rename

- **WHEN** the file is replaced via temp + rename (the pattern used by `board.Save`) twice in a row, 1 s apart
- **THEN** both rewrites MUST produce a downstream event (re-arming the watch keeps it alive)

### Requirement: `GET /api/events` exposes a Server-Sent Events stream

`GET /api/events` SHALL return `Content-Type: text/event-stream` and keep the connection open. On connect, the server MUST send a `retry: 2000` directive so browsers reconnect at 2 s on disconnect. The server MUST emit `: ping` heartbeat comments every 30 s. When the watcher fires, the server MUST emit `event: board-changed\ndata: \n\n` to every connected client.

#### Scenario: Client receives connect headers and retry directive

- **WHEN** a client opens `GET /api/events`
- **THEN** the response `Content-Type` MUST equal `text/event-stream`
- **AND** the first bytes received MUST contain `retry: 2000`

#### Scenario: External change broadcasts to all clients

- **WHEN** two clients are subscribed to `/api/events` and the board file is rewritten externally
- **THEN** both clients MUST receive an `event: board-changed` event within 500 ms

#### Scenario: Heartbeat keeps the connection alive

- **WHEN** a client stays subscribed to `/api/events` for 35 s with no external changes
- **THEN** the client MUST have received at least one `: ping` comment line from the server

#### Scenario: Client disconnect frees the subscription

- **WHEN** a connected client closes its connection
- **THEN** the server's broker MUST drop the corresponding channel within 1 s (so subsequent broadcasts do not block on it)

### Requirement: Watcher and SSE stream shut down with the server

On `SIGINT` or `SIGTERM`, the server SHALL cancel the root context, which MUST cause the watcher's `Run` to return and every active SSE handler to exit its loop. The 5 s shutdown drain established by V1 MUST still complete cleanly.

#### Scenario: Ctrl+C while clients are subscribed

- **WHEN** the server is running with at least one SSE client subscribed and the process receives `SIGINT`
- **THEN** the SSE client's connection MUST be closed within the 5 s drain window
- **AND** the server process MUST exit with code `0` within 5 s of the signal

### Requirement: Viewer's own writes are tolerated, not suppressed

The server MUST NOT attempt to distinguish its own writes from external ones. When a viewer write (V2 `POST .../move` or V3 `PATCH /api/cards/:id`) succeeds, the watcher MUST fire normally and the broker MUST broadcast `board-changed` to every connected client (including the originator).

#### Scenario: Move triggers a refetch on the originating client

- **WHEN** a single client is subscribed and that same client issues `POST /api/cards/<id>/move`
- **THEN** the client MUST receive a `board-changed` event within 500 ms following the request's response

#### Scenario: Patch triggers a refetch on the originating client

- **WHEN** a single client is subscribed and that same client issues `PATCH /api/cards/<id>`
- **THEN** the client MUST receive a `board-changed` event within 500 ms following the request's response

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
