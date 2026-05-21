## ADDED Requirements

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
