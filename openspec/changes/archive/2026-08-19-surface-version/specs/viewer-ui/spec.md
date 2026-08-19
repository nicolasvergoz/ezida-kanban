## ADDED Requirements

### Requirement: Topbar exposes a server status overlay

The topbar SHALL render a clickable dot in its right zone that opens a
small popover — the `ServerStatus` overlay — listing server-side facts
that are not board content. The overlay is opened by clicking the dot
and closed by clicking outside it, by pressing Escape, or by
selecting an item that does not itself open a nested control.

The overlay MUST contain exactly three rows, in this order: `Status`,
`Storage`, and `Version`.

#### Scenario: Overlay opens on click

- **WHEN** the user clicks the server-status dot in the topbar
- **THEN** the `ServerStatus` overlay is open
- **AND** its first row's label is `Status`

#### Scenario: Overlay closes on outside click

- **WHEN** the overlay is open and the user clicks outside it
- **THEN** the overlay closes

#### Scenario: Overlay closes on Escape

- **WHEN** the overlay is open and the user presses Escape
- **THEN** the overlay closes

### Requirement: Version row shows the build version

The `ServerStatus` overlay SHALL render a `Version` row whose value is
the `version` field from the most recent `/api/board` response, shown
verbatim with no parsing or reformatting. The row is placed after the
`Storage` row.

#### Scenario: Version row present in the overlay

- **WHEN** the `ServerStatus` overlay is opened in a running viewer
- **THEN** the overlay contains a row whose label is `Version`
- **AND** its value is the `version` string from the last `/api/board`
  response

#### Scenario: Local build renders "dev"

- **WHEN** the viewer is served by a binary built without an ldflags
  override
- **THEN** the `Version` row's value is `"dev"`

#### Scenario: Release build renders the tag

- **WHEN** the viewer is served by a binary built with
  `server.Version=v0.4.0-beta`
- **THEN** the `Version` row's value is `"v0.4.0-beta"`

#### Scenario: Version row is stable across refetches

- **WHEN** the board is refetched (via SSE or the refresh control)
  while the overlay is open
- **THEN** the `Version` row's value is unchanged