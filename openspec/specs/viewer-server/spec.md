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
`schema_version`, `columns`, `priorities`, `cards_per_column`, and
`cards`. The `cards` array MUST include the full `description` field
for every card. Response `Content-Type` MUST be `application/json`.

#### Scenario: Valid board

- **WHEN** `GET /api/board` is called against a server whose
  `kanban.toml` contains 2 columns and 3 cards
- **THEN** the response status is `200`
- **AND** `Content-Type` is `application/json`
- **AND** the body's `schema_version` equals `1`
- **AND** `cards_per_column` reflects the per-column count
- **AND** each card in `cards` has a `description` field (may be
  empty string)

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
