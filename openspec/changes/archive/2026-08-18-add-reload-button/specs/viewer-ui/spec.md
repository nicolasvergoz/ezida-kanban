## ADDED Requirements

### Requirement: Topbar exposes a manual refresh action

The topbar SHALL render a refresh control in its right zone,
positioned immediately after the connection status (to the right of
it), and SHALL keep the control visible at all times — regardless of
whether the SSE connection is `online`, `offline`, or still
`connecting`.

Activating the control SHALL re-fetch the board in place via the same
`GET /api/board` path used by the initial load, and SHALL re-render
from the fresh response. The page MUST NOT perform a full page reload
(`location.reload` MUST NOT be invoked) and MUST NOT re-navigate.

While the re-fetch is in flight, the control SHALL expose a visible
busy state (e.g. a spinning icon) and SHALL ignore further clicks. The
control MUST return to its resting state when the request settles,
whether it succeeded or failed.

Activating the control MUST NOT alter the active filter, the open
modal, or the theme.

#### Scenario: Refresh control is present and always visible

- **WHEN** the page loads with the SSE connection online
- **THEN** the topbar contains a button labelled `Refresh`
- **AND** the button appears in DOM order immediately after the
  connection-status element
- **AND** when the SSE connection is offline or connecting, the same
  button remains present and enabled

#### Scenario: Clicking Refresh re-fetches the board in place

- **WHEN** the user clicks the refresh control
- **THEN** a `GET /api/board` request MUST be issued
- **AND** the rendered DOM MUST reflect the freshly fetched board
- **AND** the browser URL MUST be unchanged
- **AND** no full page reload MUST occur

#### Scenario: Refresh shows a busy state while the fetch is in flight

- **WHEN** the user clicks the refresh control and the `GET /api/board`
  request is still pending
- **THEN** the control MUST display a visible busy affordance (e.g. a
  spinning icon)
- **AND** additional clicks on the control MUST NOT issue another
  `GET /api/board` request
- **AND** when the request settles, the control MUST return to its
  resting state

#### Scenario: Refresh preserves viewer context

- **WHEN** the user has an active filter query or an open card modal
  and clicks the refresh control
- **THEN** the filter query MUST remain unchanged
- **AND** an open modal MUST remain open
- **AND** the active theme MUST be unchanged
