## ADDED Requirements

### Requirement: Page subscribes to `/api/events` on load

The page SHALL open an `EventSource` connection to `/api/events` after the initial board load completes. On receiving an `event: board-changed`, the page SHALL refetch `/api/board` and re-render. The browser's native auto-reconnect SHALL handle dropped connections.

#### Scenario: External change triggers a refetch

- **WHEN** the page is open and the watcher fires (e.g. due to a CLI command in another terminal)
- **THEN** the page MUST issue a fresh `GET /api/board` request within 1 s of the event
- **AND** the rendered DOM MUST reflect the new board state

#### Scenario: EventSource auto-reconnects after a server restart

- **WHEN** the server is restarted (process exits and starts again on the same port)
- **THEN** the page's `connected` indicator MUST eventually return to the connected state without a user-initiated reload

### Requirement: Topbar shows connection status

The topbar SHALL render a small dot element next to the application name whose class reflects the live SSE connection state: `on` when the EventSource is open, `off` when it is closed or in retry. The dot MUST be visually distinguishable in the two states (e.g. green vs gray).

#### Scenario: Dot reflects open connection

- **WHEN** the EventSource is open
- **THEN** the topbar dot's class list MUST contain `on`

#### Scenario: Dot reflects closed connection

- **WHEN** the EventSource is in the closed state (server killed, network dropped)
- **THEN** the topbar dot's class list MUST contain `off`

### Requirement: Open edit modal closes on external change

If the edit modal (from V3) is open at the moment an external change event arrives, the page SHALL close the modal without prompting and discard any unsaved draft. The page MUST NOT show a confirmation dialog before discarding.

#### Scenario: Modal open when external change fires

- **WHEN** the user has the modal open with unsaved edits and an external change event is received
- **THEN** the modal MUST close
- **AND** the rendered card MUST display the values from the refetched board (not the discarded draft)

#### Scenario: No prompt before discard

- **WHEN** the modal closes due to an external change
- **THEN** the page MUST NOT have called `window.confirm` or otherwise blocked on user input
